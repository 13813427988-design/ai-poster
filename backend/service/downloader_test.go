package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// 命中 publicURL 前缀的 URL 应改写到本地回环,不经公网绕回。
func TestDownloadRewritesSelfURLToLocal(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte("fake-image-bytes"))
	}))
	defer srv.Close()

	// srv.Listener.Addr() 形如 127.0.0.1:PORT,充当容器内的本地地址
	d := NewImageDownloader().WithSelfRewrite("http://1.2.3.4", "http://"+srv.Listener.Addr().String())

	dest := filepath.Join(t.TempDir(), "out.png")
	// 目标是公网自地址,真实环境下不可达;改写生效才能成功
	if err := d.Download(context.Background(), "http://1.2.3.4/static/samples/a.png", dest); err != nil {
		t.Fatalf("Download() error = %v, want nil", err)
	}
	if gotPath != "/static/samples/a.png" {
		t.Errorf("request path = %q, want %q", gotPath, "/static/samples/a.png")
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(b) != "fake-image-bytes" {
		t.Errorf("content = %q, want %q", b, "fake-image-bytes")
	}
}

// 外部 URL(如 pollinations)必须原样透传,不能被改写。
func TestDownloadLeavesExternalURLUnchanged(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.Write([]byte("external"))
	}))
	defer srv.Close()

	d := NewImageDownloader().WithSelfRewrite("http://1.2.3.4", "http://127.0.0.1:9")

	dest := filepath.Join(t.TempDir(), "out.png")
	if err := d.Download(context.Background(), srv.URL+"/img.jpg", dest); err != nil {
		t.Fatalf("Download() error = %v, want nil", err)
	}
	if !hit {
		t.Error("external server was not hit; URL was rewritten but should not have been")
	}
}

// 未配置改写时行为不变(向后兼容)。
func TestDownloadWithoutRewriteWorks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.png")
	if err := NewImageDownloader().Download(context.Background(), srv.URL, dest); err != nil {
		t.Fatalf("Download() error = %v, want nil", err)
	}
}
