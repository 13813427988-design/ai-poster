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
