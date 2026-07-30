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

// probeWritable 真的往 dir 里写一个字节再删掉,用来确认目录可写。
// MkdirAll 成功不代表能写:目录已存在时它直接返回 nil,完全不碰权限。
// compose 的 bind mount 正好落在这个盲区——create_host_path 用宿主 uid 建好
// ./data/{posters,samples},挂进容器后盖掉镜像里已 chown 过的目录,
// 而进程以 uid 10001 运行,于是目录存在、启动正常、/healthz 200、healthcheck
// 通过,只有每次 POST /generate 在 downloader 里 permission denied。
// 所以必须显式写一次:把这种"健康但干不了正事"的静默错配变成启动即崩。
func probeWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		return err
	}
	// 无论后面成不成,别把探针文件留在海报目录里(它会被 /static 列出去)。
	defer os.Remove(f.Name())
	// 只 Create 不 Write 不足以覆盖只读挂载等"能建 inode 但写不进"的情况。
	if _, err := f.Write([]byte{0}); err != nil {
		f.Close()
		return err
	}
	return f.Close()
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
	// 目录建好 ≠ 能写,见 probeWritable 的注释。写不了就在这里响亮地挂掉,
	// 让运维看到 crash-loop 而不是一个通过 healthcheck 却每次生成都 500 的栈。
	for _, dir := range []string{cfg.PostersDir, cfg.SamplesDir} {
		if err := probeWritable(dir); err != nil {
			log.Fatalf("目录不可写 %s: %v\n"+
				"最可能的原因:该目录是 compose 的 bind mount(./data/...),owner 是宿主 uid,"+
				"而容器内进程以 uid 10001(app)运行,没有写权限。\n"+
				"修复(在 docker-compose.yml 所在目录执行):\n"+
				"  docker run --rm -v \"$PWD/data:/data\" alpine:3.21 chown -R 10001:10001 /data",
				dir, err)
		}
	}

	promptSvc := service.NewPromptService()
	aiClient, err := newAIClient(cfg)
	if err != nil {
		// 与上面的 "config:" 区分开:cfg.Validate() 已经放行了这个 provider,
		// 走到这里说明是 ValidProviders 与 newAIClient 没同步的内部接线 bug,
		// 不是运维的 env 文件写错了,别让人白翻配置。
		log.Fatalf("provider wiring: %v", err)
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
