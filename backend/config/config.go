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
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
