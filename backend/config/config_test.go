package config

import "testing"

func TestLoadDefaultsToPollinations(t *testing.T) {
	t.Setenv("AI_PROVIDER", "")
	if got := Load().AIProvider; got != ProviderPollinations {
		t.Errorf("AIProvider = %q, want %q", got, ProviderPollinations)
	}
}

func TestLoadReadsAIProvider(t *testing.T) {
	for _, want := range []string{ProviderPollinations, ProviderModelProxy, ProviderMock} {
		t.Setenv("AI_PROVIDER", want)
		if got := Load().AIProvider; got != want {
			t.Errorf("AI_PROVIDER=%q gave AIProvider=%q", want, got)
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
// 那种 token 建出的客户端会每请求 401,比启动即失败难查得多。
func TestValidateRejectsWhitespaceOnlyToken(t *testing.T) {
	t.Setenv("AI_PROVIDER", ProviderModelProxy)
	for _, token := range []string{"   ", "\t", "sk-real\n"} {
		t.Setenv("MODELPROXY_TOKEN", token)
		err := Load().Validate()
		if token == "sk-real\n" {
			// 有实义内容的 token 不该被拒(尾换行本身不致命)
			if err != nil {
				t.Errorf("MODELPROXY_TOKEN=%q: Validate() = %v, want nil", token, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("MODELPROXY_TOKEN=%q: Validate() = nil, want error", token)
		}
	}
}
