package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ImageDownloader 把 URL 指向的图片下载到本地文件。
// mock 模式下 URL 也是本地的，但走 HTTP 路径保持流程一致，方便未来切到真模型时无缝。
type ImageDownloader struct {
	client *http.Client

	// selfPrefix 非空时,命中该前缀的 URL 会被改写到 localAddr。
	// 用途:容器内取自己刚落盘的图,不依赖 hairpin NAT 经公网绕回。
	selfPrefix string
	localAddr  string
}

func NewImageDownloader() *ImageDownloader {
	return &ImageDownloader{client: &http.Client{Timeout: 30 * time.Second}}
}

// WithSelfRewrite 让 Download 把指向 publicURL 的请求改写到 localAddr。
// publicURL 为空时不启用改写。返回 d 本身,便于链式调用。
func (d *ImageDownloader) WithSelfRewrite(publicURL, localAddr string) *ImageDownloader {
	d.selfPrefix = strings.TrimSuffix(publicURL, "/")
	d.localAddr = strings.TrimSuffix(localAddr, "/")
	return d
}

// rewriteSelf 命中自地址前缀则替换为本地地址,否则原样返回。
func (d *ImageDownloader) rewriteSelf(imageURL string) string {
	if d.selfPrefix == "" || !strings.HasPrefix(imageURL, d.selfPrefix) {
		return imageURL
	}
	return d.localAddr + strings.TrimPrefix(imageURL, d.selfPrefix)
}

// Download GET imageURL 写到 destPath。父目录不存在时自动创建。
func (d *ImageDownloader) Download(ctx context.Context, imageURL, destPath string) error {
	reqURL := d.rewriteSelf(imageURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed, status=%s url=%s", resp.Status, reqURL)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	return nil
}
