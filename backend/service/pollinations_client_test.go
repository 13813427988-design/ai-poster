package service

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestPollinationsGenerateBuildsURL(t *testing.T) {
	c := NewPollinationsAIClient("1024x1536")

	got, err := c.Generate(context.Background(), "日落海边的渔船")
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", got, err)
	}
	if u.Host != "image.pollinations.ai" {
		t.Errorf("host = %q, want %q", u.Host, "image.pollinations.ai")
	}
	if u.Scheme != "https" {
		t.Errorf("scheme = %q, want %q", u.Scheme, "https")
	}
	// 中文 prompt 必须转义进 path;url.Parse 解码后应还原
	if !strings.HasSuffix(u.Path, "日落海边的渔船") {
		t.Errorf("decoded path = %q, want suffix %q", u.Path, "日落海边的渔船")
	}
	// 原始字符串里不能有裸中文(否则不是合法 URL)
	if strings.Contains(got, "日落") {
		t.Errorf("raw URL contains unescaped CJK: %q", got)
	}

	q := u.Query()
	if q.Get("width") != "1024" {
		t.Errorf("width = %q, want %q", q.Get("width"), "1024")
	}
	if q.Get("height") != "1536" {
		t.Errorf("height = %q, want %q", q.Get("height"), "1536")
	}
	if q.Get("nologo") != "true" {
		t.Errorf("nologo = %q, want %q", q.Get("nologo"), "true")
	}
	if q.Get("seed") == "" {
		t.Error("seed is empty; a random seed is required or the API returns a cached image")
	}
}

// 同 prompt 必须拿到不同 seed,否则 pollinations 每次返回同一张图。
func TestPollinationsGenerateVariesSeed(t *testing.T) {
	c := NewPollinationsAIClient("")

	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		got, err := c.Generate(context.Background(), "same prompt")
		if err != nil {
			t.Fatalf("Generate() error = %v, want nil", err)
		}
		u, _ := url.Parse(got)
		seen[u.Query().Get("seed")] = true
	}
	if len(seen) < 2 {
		t.Errorf("got %d distinct seeds across 20 calls, want >= 2", len(seen))
	}
}

// size 为空或格式不对时省略 width/height,走服务端默认尺寸。
func TestPollinationsGenerateOmitsBadSize(t *testing.T) {
	for _, size := range []string{"", "abc", "1024", "1024x", "x1536", "axb"} {
		c := NewPollinationsAIClient(size)
		got, err := c.Generate(context.Background(), "p")
		if err != nil {
			t.Fatalf("Generate() with size=%q error = %v, want nil", size, err)
		}
		u, _ := url.Parse(got)
		q := u.Query()
		if q.Get("width") != "" || q.Get("height") != "" {
			t.Errorf("size=%q produced width=%q height=%q, want both empty",
				size, q.Get("width"), q.Get("height"))
		}
	}
}
