package main

import (
	"log"
	"os"

	"ai-poster/backend/config"
	"ai-poster/backend/handler"
	"ai-poster/backend/service"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}

	for _, dir := range []string{cfg.PostersDir, cfg.SamplesDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	promptSvc := service.NewPromptService()
	var aiClient service.AIClient
	switch cfg.AIProvider {
	case config.ProviderPollinations:
		aiClient = service.NewPollinationsAIClient(cfg.ImageSize)
	case config.ProviderModelProxy:
		aiClient = service.NewModelProxyAIClient(cfg.ModelProxyEndpoint, cfg.ModelProxyToken,
			cfg.ModelProxyModel, cfg.ImageSize, cfg.SamplesDir, cfg.PublicURL)
	case config.ProviderMock:
		aiClient = service.NewMockAIClient(cfg.SamplesDir, cfg.PublicURL)
	}
	log.Printf("ai-poster provider=%s image_size=%q", cfg.AIProvider, cfg.ImageSize)
	// 容器内取自己落盘的图走回环，不依赖 hairpin NAT
	downloader := service.NewImageDownloader().
		WithSelfRewrite(cfg.PublicURL, "http://127.0.0.1:"+cfg.Port)
	composer := service.NewPosterComposer(cfg.FontPath)

	gh := handler.NewGenerateHandler(promptSvc, aiClient, downloader, composer, cfg.PublicURL, cfg.PostersDir)

	r := gin.Default()
	// 静态资源：海报、占位图、字体都在 cfg.StaticDir 下
	r.Static("/static", cfg.StaticDir)
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
	r.POST("/generate", gh.Generate)

	addr := ":" + cfg.Port
	log.Printf("ai-poster listening on %s, public_url=%s", addr, cfg.PublicURL)
	if err := r.Run(addr); err != nil {
		log.Fatalf("gin run: %v", err)
	}
}
