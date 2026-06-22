package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ImageDownloader 把 URL 指向的图片下载到本地文件。
// mock 模式下 URL 也是本地的，但走 HTTP 路径保持流程一致，方便未来切到真模型时无缝。
type ImageDownloader struct {
	client *http.Client
}

func NewImageDownloader() *ImageDownloader {
	return &ImageDownloader{client: &http.Client{Timeout: 30 * time.Second}}
}

// Download GET imageURL 写到 destPath。父目录不存在时自动创建。
func (d *ImageDownloader) Download(ctx context.Context, imageURL, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed, status=%s url=%s", resp.Status, imageURL)
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
