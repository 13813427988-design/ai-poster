package config

import (
	"os"
	"path/filepath"
)

// Config 服务运行参数。所有字段都可被同名（大写）环境变量覆盖。
type Config struct {
	Port       string // 监听端口，默认 8080
	PublicURL  string // 对外可访问的 URL 前缀，用于拼海报链接
	StaticDir  string // 静态资源根目录
	PostersDir string // 海报输出目录
	SamplesDir string // mock AI 模式下的占位图目录
	FontPath   string // 标题用的 TTF 字体（缺失时合成不画文字）

	// 文生图模型代理（OpenAI 兼容 /v1/images/generations）。
	// ModelProxyToken 为空时回退到 Mock 生图。
	ModelProxyEndpoint string // 代理地址
	ModelProxyToken    string // Bearer token
	ModelProxyModel    string // 文生图模型
	ImageSize          string // 生成尺寸(可选)，如 1024x1536
}

func Load() *Config {
	port := envOr("PORT", "8080")
	publicURL := envOr("PUBLIC_URL", "http://localhost:"+port)
	staticDir := envOr("STATIC_DIR", "static")
	return &Config{
		Port:       port,
		PublicURL:  publicURL,
		StaticDir:  staticDir,
		PostersDir: filepath.Join(staticDir, "posters"),
		SamplesDir: filepath.Join(staticDir, "samples"),
		FontPath:   envOr("FONT_PATH", filepath.Join(staticDir, "fonts", "default.ttf")),

		ModelProxyEndpoint: envOr("MODELPROXY_ENDPOINT", "https://models-proxy.stepfun-inc.com/v1/images/generations"),
		ModelProxyToken:    envOr("MODELPROXY_TOKEN", ""),
		ModelProxyModel:    envOr("MODELPROXY_MODEL", "doubao-seedream-4.0"),
		ImageSize:          envOr("IMAGE_SIZE", ""),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
