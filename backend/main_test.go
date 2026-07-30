package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// probeWritable 的全部意义在于抓住"目录存在但写不进"——也就是 MkdirAll 返回 nil
// 的那个盲区(bind mount owner 不是容器内 uid)。只测可写目录返回 nil 是不够的:
// 把实现改成 os.Stat 也能过。所以这里必须有一个 0o555 目录的用例。
func TestProbeWritable(t *testing.T) {
	if err := probeWritable(t.TempDir()); err != nil {
		t.Errorf("probeWritable(可写目录) = %v, want nil", err)
	}

	t.Run("不可写目录", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("以 root 运行时权限位不生效")
		}
		dir := filepath.Join(t.TempDir(), "readonly")
		if err := os.Mkdir(dir, 0o555); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}
		// MkdirAll 对已存在的目录一律返回 nil——正是它漏掉、探针要补上的那一格。
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(已存在的只读目录) = %v, 前提假设不成立", err)
		}
		if err := probeWritable(dir); err == nil {
			t.Error("probeWritable(不可写目录) = nil, want error")
		}
	})
}

// 探针不能在目录里留下垃圾:posters 目录是 /static 对外暴露的。
func TestProbeWritableLeavesNoFile(t *testing.T) {
	dir := t.TempDir()
	if err := probeWritable(dir); err != nil {
		t.Fatalf("probeWritable: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("探针残留了 %d 个文件, want 0", len(entries))
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
