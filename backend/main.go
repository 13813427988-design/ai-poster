package main

import (
	"fmt"
	"log"
	"os"

	"ai-poster/backend/config"
	"ai-poster/backend/handler"
	"ai-poster/backend/service"

	"github.com/gin-gonic/gin"
)

// newAIClient 按 provider 构造生图客户端。
// 未知 provider 必须返回 error 而不是 nil client:gin.Default() 装了 Recovery,
// nil client 只会每请求 panic 后被兜成 500,而 /healthz 照样 200——
// 那正是本开关要防的"看着健康但什么都生不出来"。
func newAIClient(cfg *config.Config) (service.AIClient, error) {
	switch cfg.AIProvider {
	case config.ProviderPollinations:
		return service.NewPollinationsAIClient(cfg.ImageSize), nil
	case config.ProviderModelProxy:
		return service.NewModelProxyAIClient(cfg.ModelProxyEndpoint, cfg.ModelProxyToken,
			cfg.ModelProxyModel, cfg.ImageSize, cfg.SamplesDir, cfg.PublicURL), nil
	case config.ProviderMock:
		return service.NewMockAIClient(cfg.SamplesDir, cfg.PublicURL), nil
	default:
		// 正常路径下 config.Validate() 已挡掉未知值,这里是第二道闸:
		// 两处的 provider 列表若哪天不同步,启动就会失败而非静默返回 nil。
		return nil, fmt.Errorf("unhandled AI_PROVIDER %q", cfg.AIProvider)
	}
}

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
	aiClient, err := newAIClient(cfg)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("ai-poster provider=%s image_size=%q", cfg.AIProvider, cfg.ImageSize)
	// endpoint 不可达是已知部署风险,启动时打出来便于排查;token 不打。
	if cfg.AIProvider == config.ProviderModelProxy {
		log.Printf("modelproxy endpoint=%s, model=%s", cfg.ModelProxyEndpoint, cfg.ModelProxyModel)
	}
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
