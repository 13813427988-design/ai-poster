package handler

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"ai-poster/backend/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GenerateRequest struct {
	Prompt string `json:"prompt" binding:"required"`
	Title  string `json:"title"`
}

type GenerateResponse struct {
	URL string `json:"url"`
}

type GenerateHandler struct {
	promptSvc  *service.PromptService
	aiClient   service.AIClient
	downloader *service.ImageDownloader
	composer   *service.PosterComposer
	publicURL  string
	postersDir string
}

func NewGenerateHandler(
	promptSvc *service.PromptService,
	aiClient service.AIClient,
	downloader *service.ImageDownloader,
	composer *service.PosterComposer,
	publicURL string,
	postersDir string,
) *GenerateHandler {
	return &GenerateHandler{
		promptSvc:  promptSvc,
		aiClient:   aiClient,
		downloader: downloader,
		composer:   composer,
		publicURL:  publicURL,
		postersDir: postersDir,
	}
}

// Generate POST /generate
//
// 流程: PromptService 包装 prompt → AIClient 生成背景图 URL → ImageDownloader 落本地中间文件
//      → PosterComposer 把标题贴到背景上并写出最终海报 → 返回对外 URL。
//
// 失败时中间文件可能残留；本 handler 用 defer 清理 bg 文件。
func (h *GenerateHandler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 单请求 60s 兜底超时；下游各自还可有更短超时
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	fullPrompt := h.promptSvc.BuildPrompt(req.Prompt)

	bgURL, err := h.aiClient.Generate(ctx, fullPrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ai generate: " + err.Error()})
		return
	}

	id := uuid.New().String()
	bgLocal := filepath.Join(h.postersDir, id+"_bg.png")
	if err := h.downloader.Download(ctx, bgURL, bgLocal); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "download bg: " + err.Error()})
		return
	}
	defer os.Remove(bgLocal)

	posterPath := filepath.Join(h.postersDir, id+".png")
	if err := h.composer.Compose(bgLocal, req.Title, posterPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "compose: " + err.Error()})
		return
	}

	out, _ := url.JoinPath(h.publicURL, "/static/posters/", id+".png")
	c.JSON(http.StatusOK, GenerateResponse{URL: out})
}
