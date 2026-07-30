package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 生图 provider 取值。
const (
	ProviderPollinations = "pollinations" // 默认,无需凭证
	ProviderModelProxy   = "modelproxy"   // 需 token,且 endpoint 需内网可达
	ProviderMock         = "mock"         // 本地渐变占位图,不出网
)

// Config 服务运行参数。所有字段都可被同名（大写）环境变量覆盖。
type Config struct {
	Port       string // 监听端口，默认 8080
	PublicURL  string // 对外可访问的 URL 前缀，用于拼海报链接
	StaticDir  string // 静态资源根目录
	PostersDir string // 海报输出目录
	SamplesDir string // mock AI 模式下的占位图目录
	FontPath   string // 标题用的 TTF 字体（缺失时合成不画文字）
	AIProvider string // 生图 provider,见 Provider* 常量

	// 文生图模型代理（OpenAI 兼容 /v1/images/generations）。
	// 仅当 AIProvider 为 ProviderModelProxy 时生效，此时 ModelProxyToken 必填。
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
		AIProvider: normalizeProvider(os.Getenv("AI_PROVIDER")),

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

// normalizeProvider 归一化大小写与空白;空白或空串回退默认 provider。
func normalizeProvider(v string) string {
	if s := strings.ToLower(strings.TrimSpace(v)); s != "" {
		return s
	}
	return ProviderPollinations
}

// Validate 检查配置自身一致性。返回非 nil 时调用方应让进程启动失败——
// 显式报错胜过静默降级:配错时安静跑 mock 会让人误以为在用真模型。
func (c *Config) Validate() error {
	switch c.AIProvider {
	case ProviderPollinations, ProviderMock:
		return nil
	case ProviderModelProxy:
		if c.ModelProxyToken == "" {
			return fmt.Errorf("AI_PROVIDER=%s requires MODELPROXY_TOKEN", ProviderModelProxy)
		}
		return nil
	default:
		return fmt.Errorf("unknown AI_PROVIDER %q, want one of %s/%s/%s",
			c.AIProvider, ProviderPollinations, ProviderModelProxy, ProviderMock)
	}
}
