package config

import "testing"

func TestLoadDefaultsToPollinations(t *testing.T) {
	t.Setenv("AI_PROVIDER", "")
	if got := Load().AIProvider; got != ProviderPollinations {
		t.Errorf("AIProvider = %q, want %q", got, ProviderPollinations)
	}
}

func TestLoadReadsAIProvider(t *testing.T) {
	for _, want := range ValidProviders {
		t.Setenv("AI_PROVIDER", want)
		if got := Load().AIProvider; got != want {
			t.Errorf("AI_PROVIDER=%q gave AIProvider=%q", want, got)
		}
	}
}

// Validate 的放行集合必须就是 ValidProviders:清单里的每个值都得被接受
// (modelproxy 补上它必需的 token),否则"加进清单"和"真的能用"就脱钩了。
func TestValidateAcceptsEveryValidProvider(t *testing.T) {
	for _, provider := range ValidProviders {
		cfg := &Config{AIProvider: provider, ModelProxyToken: "sk-test"}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() rejected ValidProviders entry %q: %v", provider, err)
		}
	}
}

// 大小写和空格不该导致启动失败——归一化处理。
func TestLoadNormalizesAIProvider(t *testing.T) {
	t.Setenv("AI_PROVIDER", "  Pollinations  ")
	if got := Load().AIProvider; got != ProviderPollinations {
		t.Errorf("AIProvider = %q, want %q", got, ProviderPollinations)
	}
}

// 纯空格必须回退到默认,不能变成空串。envOr 先判空会认为 "   " 非空,
// 所以归一化必须在取默认值之前做。
func TestLoadTreatsBlankAIProviderAsDefault(t *testing.T) {
	t.Setenv("AI_PROVIDER", "   ")
	if got := Load().AIProvider; got != ProviderPollinations {
		t.Errorf("AIProvider = %q, want %q", got, ProviderPollinations)
	}
}

func TestValidateRejectsUnknownProvider(t *testing.T) {
	t.Setenv("AI_PROVIDER", "gpt-image")
	if err := Load().Validate(); err == nil {
		t.Error("Validate() = nil, want error for unknown provider")
	}
}

// modelproxy 缺 token 必须启动失败,而不是安静回退 mock。
func TestValidateRequiresTokenForModelProxy(t *testing.T) {
	t.Setenv("AI_PROVIDER", ProviderModelProxy)
	t.Setenv("MODELPROXY_TOKEN", "")
	if err := Load().Validate(); err == nil {
		t.Error("Validate() = nil, want error when modelproxy has no token")
	}

	t.Setenv("MODELPROXY_TOKEN", "sk-test")
	if err := Load().Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil when token is set", err)
	}
}

func TestValidateAcceptsPollinationsWithoutToken(t *testing.T) {
	t.Setenv("AI_PROVIDER", ProviderPollinations)
	t.Setenv("MODELPROXY_TOKEN", "")
	if err := Load().Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil (pollinations needs no credentials)", err)
	}
}

// mock 也是合法 provider,不能因为它没凭证就被当成配错。
// 少了这条,把 ProviderMock 从 Validate 首个 case 里删掉不会有测试报警。
func TestValidateAcceptsMockWithoutToken(t *testing.T) {
	t.Setenv("AI_PROVIDER", ProviderMock)
	t.Setenv("MODELPROXY_TOKEN", "")
	if err := Load().Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil (mock needs no credentials)", err)
	}
}

// 纯空白 token 等于没配,必须启动失败。env-file 很容易带出尾随空格/换行,
// 那种 token 建出的客户端会每请求失败,比启动即失败难查得多。
func TestValidateRejectsWhitespaceOnlyToken(t *testing.T) {
	t.Setenv("AI_PROVIDER", ProviderModelProxy)
	for _, token := range []string{"   ", "\t", "sk-real\n"} {
		t.Setenv("MODELPROXY_TOKEN", token)
		cfg := Load()
		err := cfg.Validate()
		if token == "sk-real\n" {
			// 有实义内容的 token 不该被拒——但放行的理由是它已经在 Load()
			// 里被 trim 成了真正可用的 "sk-real",而不是"尾换行无害"。
			// 尾换行恰恰是致命的:net/http 会在发出前用
			// `invalid header field value for "Authorization"` 拒掉带 \r\n 的
			// header,启动成功、/healthz 200,而 100% 的 /generate 都失败。
			if err != nil {
				t.Errorf("MODELPROXY_TOKEN=%q: Validate() = %v, want nil", token, err)
			}
			// 光放行不够:存下来的值必须是干净的,否则 Validate 通过只是把
			// 启动即失败换成了每请求失败。
			if cfg.ModelProxyToken != "sk-real" {
				t.Errorf("MODELPROXY_TOKEN=%q: stored token = %q, want %q (必须在 Load 时 trim)",
					token, cfg.ModelProxyToken, "sk-real")
			}
			continue
		}
		if err == nil {
			t.Errorf("MODELPROXY_TOKEN=%q: Validate() = nil, want error", token)
		}
	}
}

// env-file 带 CRLF 时 token 会是 "sk-real\r\n";\r 和 \n 一样会被 net/http 拒。
func TestLoadTrimsTokenCRLF(t *testing.T) {
	for _, raw := range []string{"sk-real\r", "sk-real\n", "sk-real\r\n", "  sk-real  "} {
		t.Setenv("MODELPROXY_TOKEN", raw)
		if got := Load().ModelProxyToken; got != "sk-real" {
			t.Errorf("MODELPROXY_TOKEN=%q: stored = %q, want %q", raw, got, "sk-real")
		}
	}
}

// endpoint/model/size 同样是 env 派生值,带上尾随换行不会启动失败,
// 只会在运行时变成 URL 解析失败或被上游拒的请求参数。
func TestLoadTrimsOtherModelProxyValues(t *testing.T) {
	t.Setenv("MODELPROXY_ENDPOINT", "https://example.com/v1/images/generations\n")
	t.Setenv("MODELPROXY_MODEL", " doubao-seedream-4.0\r\n")
	t.Setenv("IMAGE_SIZE", "1024x1536\n")
	t.Setenv("PORT", " 9090\n")
	cfg := Load()
	for _, tc := range []struct{ name, got, want string }{
		{"ModelProxyEndpoint", cfg.ModelProxyEndpoint, "https://example.com/v1/images/generations"},
		{"ModelProxyModel", cfg.ModelProxyModel, "doubao-seedream-4.0"},
		{"ImageSize", cfg.ImageSize, "1024x1536"},
		{"Port", cfg.Port, "9090"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// 纯空白的可选值必须落回默认/空,不能变成"已设置"的垃圾值:
// IMAGE_SIZE="  " 若原样存下会作为 size 发给上游,而它本意是不传。
func TestLoadTreatsWhitespaceOnlyValuesAsUnset(t *testing.T) {
	t.Setenv("PORT", "   ")
	t.Setenv("IMAGE_SIZE", "  \n")
	cfg := Load()
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default 8080", cfg.Port)
	}
	if cfg.ImageSize != "" {
		t.Errorf("ImageSize = %q, want empty (不传 size)", cfg.ImageSize)
	}
}
