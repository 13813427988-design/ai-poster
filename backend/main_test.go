package main

import (
	"fmt"
	"testing"

	"ai-poster/backend/config"
)

// provider -> 客户端具体类型的映射是本开关的全部价值所在,必须钉住:
// 之前只有手工跑过 pollinations 一条路径,把 mock 的 case 改成构造
// pollinations 客户端也能让全部测试通过。
func TestNewAIClientReturnsCorrectTypePerProvider(t *testing.T) {
	cases := []struct {
		provider string
		want     string
	}{
		{config.ProviderPollinations, "*service.PollinationsAIClient"},
		{config.ProviderModelProxy, "*service.ModelProxyAIClient"},
		{config.ProviderMock, "*service.MockAIClient"},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			cfg := &config.Config{
				AIProvider: tc.provider,
				PublicURL:  "http://localhost:8080",
				SamplesDir: t.TempDir(),
				// modelproxy 建客户端所需的最小字段
				ModelProxyEndpoint: "http://127.0.0.1:1/v1/images/generations",
				ModelProxyToken:    "sk-test",
				ModelProxyModel:    "test-model",
			}
			client, err := newAIClient(cfg)
			if err != nil {
				t.Fatalf("newAIClient(%q) returned error: %v", tc.provider, err)
			}
			if client == nil {
				t.Fatalf("newAIClient(%q) returned nil client", tc.provider)
			}
			if got := fmt.Sprintf("%T", client); got != tc.want {
				t.Errorf("newAIClient(%q) = %s, want %s", tc.provider, got, tc.want)
			}
		})
	}
}

// 未知 provider 必须报错,不能返回 nil client:gin 的 Recovery 会把
// nil client 的 panic 兜成 500,而 /healthz 仍然 200,故障不自我暴露。
func TestNewAIClientRejectsUnknownProvider(t *testing.T) {
	client, err := newAIClient(&config.Config{AIProvider: "gpt-image"})
	if err == nil {
		t.Error("newAIClient() = nil error, want error for unknown provider")
	}
	if client != nil {
		t.Errorf("newAIClient() = %T, want nil client for unknown provider", client)
	}
}

// 每个 Validate 放行的 provider,newAIClient 都必须能建出客户端。
// 遍历 config.ValidProviders（Validate 判定放行用的同一份清单）而不是本地再抄一份,
// 这样两个方向的漂移都会被抓到:从清单里删掉 provider 会让上面的类型断言测试失败,
// 往清单里加了 provider 却忘在 newAIClient 里接线,会在这里失败。
// 抄一份硬编码列表只能防前者——后者(新值根本不在遍历范围内)才是真正会发生的漂移。
func TestNewAIClientHandlesEveryValidatedProvider(t *testing.T) {
	for _, provider := range config.ValidProviders {
		cfg := &config.Config{
			AIProvider:      provider,
			SamplesDir:      t.TempDir(),
			ModelProxyToken: "sk-test",
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() rejected provider %q: %v", provider, err)
		}
		if _, err := newAIClient(cfg); err != nil {
			t.Errorf("Validate() accepted %q but newAIClient() failed: %v", provider, err)
		}
	}
}
