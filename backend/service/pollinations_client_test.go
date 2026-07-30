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
	// path 必须落在 /prompt/ 下,换成别的段服务端不认(会 404,图也就取不到)
	if !strings.HasPrefix(u.Path, "/prompt/") {
		t.Errorf("decoded path = %q, want prefix %q", u.Path, "/prompt/")
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
// 这里不只要求"不全相同":seed 必须来自足够大的取值空间,否则"重新生成"仍有
// 相当概率撞回上一张图,是一种不报错的静默退化。20 次抽取自 1e9 空间,期望碰撞
// 概率约 2e-7,所以门槛压到 19(留 1 次碰撞余量),低熵 seed 会立刻失败。
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
	if len(seen) < 19 {
		t.Errorf("got %d distinct seeds across 20 calls, want >= 19; "+
			"seed must be drawn from a space large enough (~1e9) that repeat "+
			"generations practically never reuse a seed — a low-entropy seed would "+
			"silently give users the same poster again", len(seen))
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
