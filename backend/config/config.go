package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// 生图 provider 取值。
const (
	ProviderPollinations = "pollinations" // 默认,无需凭证
	ProviderModelProxy   = "modelproxy"   // 需 token,且 endpoint 需内网可达
	ProviderMock         = "mock"         // 本地渐变占位图,不出网
)

// ValidProviders 是 provider 的唯一权威清单:Validate 由它决定放行哪些值,
// main_test 也遍历它去验证每个放行值都在 newAIClient 里接上了客户端。
// 只有共用同一份清单,"往清单里加了 provider 但忘了接线"才会在测试里暴露——
// 各处各写一份硬编码列表时,新增的值根本不会被遍历到。
var ValidProviders = []string{ProviderPollinations, ProviderModelProxy, ProviderMock}

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

// envOr 读环境变量并 TrimSpace 后返回,首尾空白视为未设置(取 fallback)。
// 这里统一 trim 而不是在各字段上单独 trim:本结构里所有 env 派生的值——端口、
// URL、路径、token、模型名、尺寸——首尾空白都没有语义,而 env-file 极易带出
// 尾随空格或 \r\n（CRLF 文件、`echo` 追加）。留着这些垃圾字符不会在启动时
// 报错,只会在运行时炸:token 带 \r\n 会被 net/http 以
// `invalid header field value for "Authorization"` 在发出前拒掉(每请求 100% 失败,
// 而 /healthz 照样 200),endpoint 带换行会 URL 解析失败,模型名/尺寸带换行会被上游拒。
// 在入口处 trim 一次,就不必指望每个新字段的作者都记得自己 trim。
func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
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
	if !slices.Contains(ValidProviders, c.AIProvider) {
		return fmt.Errorf("unknown AI_PROVIDER %q, want one of %s",
			c.AIProvider, strings.Join(ValidProviders, "/"))
	}
	// modelproxy 是唯一需要凭证的 provider,缺 token 必须启动即失败。
	// 这里仍用 TrimSpace 判空:Load() 出来的值已经 trim 过（见 envOr）,
	// 但 Config 也可能被直接构造(测试、将来的其它入口),纯空白 token
	// 等于没配,不能让它建出一个每请求都 401 的"真"客户端。
	if c.AIProvider == ProviderModelProxy && strings.TrimSpace(c.ModelProxyToken) == "" {
		return fmt.Errorf("AI_PROVIDER=%s requires MODELPROXY_TOKEN", ProviderModelProxy)
	}
	return nil
}
