package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
)

// AIClient 文生图模型客户端接口。后续接 OpenAI / 火山方舟 等，只需新增实现替换 main.go 注入。
type AIClient interface {
	// Generate 基于 prompt 生成一张图，返回**可被下游 downloader 通过 HTTP GET 拿到**的 URL。
	Generate(ctx context.Context, prompt string) (imageURL string, err error)
}

// MockAIClient demo 阶段用，把 prompt md5 -> 颜色，渲染一张 1024x1536 的竖版渐变图，
// 写到 samplesDir 后返回 publicURL/<samples 子路径>/<hash>.png。
// 同一 prompt hash 命中已有文件直接复用（节省 IO）。
type MockAIClient struct {
	samplesDir string // 文件落盘目录（绝对或相对均可）
	publicURL  string // 拼接对外 URL 用，如 http://localhost:8080
	urlPath    string // URL 子路径，固定 /static/samples
}

// 编译期断言:改 Generate 签名或改名会在这里直接编译失败。
var _ AIClient = (*MockAIClient)(nil)

func NewMockAIClient(samplesDir, publicURL string) *MockAIClient {
	return &MockAIClient{
		samplesDir: samplesDir,
		publicURL:  publicURL,
		urlPath:    "/static/samples",
	}
}

func (c *MockAIClient) Generate(_ context.Context, prompt string) (string, error) {
	sum := md5.Sum([]byte(prompt))
	fileName := hex.EncodeToString(sum[:]) + ".png"
	fullPath := filepath.Join(c.samplesDir, fileName)

	// 已存在则复用
	if _, err := os.Stat(fullPath); err == nil {
		return c.composeURL(fileName), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat sample file: %w", err)
	}

	img := generateGradientFromHash(sum, 1024, 1536)
	if err := writePNG(fullPath, img); err != nil {
		return "", err
	}
	return c.composeURL(fileName), nil
}

func (c *MockAIClient) composeURL(fileName string) string {
	// 用 url.JoinPath 防 publicURL 末尾有/无 / 的边界
	u, _ := url.JoinPath(c.publicURL, c.urlPath, fileName)
	return u
}

// generateGradientFromHash 根据 hash 选两个反差色作为对角渐变。简单但确定性 + 视觉上每条 prompt 不一样。
func generateGradientFromHash(hash [16]byte, width, height int) *image.RGBA {
	c1 := color.RGBA{R: hash[0], G: hash[1], B: hash[2], A: 255}
	c2 := color.RGBA{R: hash[3], G: hash[4], B: hash[5], A: 255}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			t := float64(x+y) / float64(width+height)
			r := uint8(float64(c1.R)*(1-t) + float64(c2.R)*t)
			g := uint8(float64(c1.G)*(1-t) + float64(c2.G)*t)
			b := uint8(float64(c1.B)*(1-t) + float64(c2.B)*t)
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create png: %w", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}
	return nil
}
