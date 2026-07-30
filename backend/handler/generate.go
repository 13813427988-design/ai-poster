package handler

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"ai-poster/backend/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requestTimeout 是单请求的总预算,必须小于 nginx 的 proxy_read_timeout(180s)。
//
// 为什么需要它:两个下游各自的超时是独立的上限,串起来会超过 nginx 的容忍度。
// modelproxy 路径最坏 = 120s 生图(modelproxy_client 的 http.Client.Timeout)
// + 120s 下载(downloader.downloadTimeout) = 240s > 180s。没有总预算时,
// 这种请求会由 nginx 先断:前端拿到一个没有信息量的 504,而后端还在跑,
// 最后把一张没人会取的海报写进 posters/。给足 10s 余量(合成+落盘实测 <1s)
// 收在 170s,让超时由后端自己判定 —— 前端拿到的是
// 500 "download bg: context deadline exceeded" 这种能直接定位到阶段的错误,
// nginx 的 180s 退化成真正的兜底。
//
// 这个数字与 nginx.conf 的 proxy_read_timeout 是一对约束,改一个要同时看另一个。
// pollinations 路径本身没有触碰这个上限的风险:它的 Generate 只拼 URL 不出网,
// 全部耗时都在那一次 120s 的 GET 里(实测 4.5s~36.2s)。
const requestTimeout = 170 * time.Second

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), requestTimeout)
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
