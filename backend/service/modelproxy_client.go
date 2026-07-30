package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// ModelProxyAIClient 通过 OpenAI 兼容的模型代理(/v1/images/generations)做真实文生图。
// 返回可被下游 downloader HTTP GET 的图片 URL:代理直接给 url 时透传;只给 b64_json 时
// 解码落盘到 samplesDir 并返回本服务的静态 URL,保证 AIClient 契约不变。
type ModelProxyAIClient struct {
	endpoint   string
	token      string
	model      string
	size       string
	samplesDir string
	publicURL  string
	urlPath    string
	httpClient *http.Client
}

// 编译期断言:改 Generate 签名或改名会在这里直接编译失败。
var _ AIClient = (*ModelProxyAIClient)(nil)

func NewModelProxyAIClient(endpoint, token, model, size, samplesDir, publicURL string) *ModelProxyAIClient {
	return &ModelProxyAIClient{
		endpoint:   endpoint,
		token:      token,
		model:      model,
		size:       size,
		samplesDir: samplesDir,
		publicURL:  publicURL,
		urlPath:    "/static/samples",
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

type imageGenRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Size   string `json:"size,omitempty"`
}

type imageGenResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

func (c *ModelProxyAIClient) Generate(ctx context.Context, prompt string) (string, error) {
	reqBody, _ := json.Marshal(imageGenRequest{
		Model:  c.model,
		Prompt: prompt,
		N:      1,
		Size:   c.size,
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call modelproxy: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("modelproxy status %d: %s", resp.StatusCode, string(raw))
	}

	var out imageGenResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("decode response: %w (raw=%s)", err, string(raw))
	}
	if len(out.Data) == 0 {
		return "", fmt.Errorf("modelproxy empty data: %s", string(raw))
	}

	d := out.Data[0]
	if d.URL != "" {
		return d.URL, nil
	}
	if d.B64JSON != "" {
		imgBytes, err := base64.StdEncoding.DecodeString(d.B64JSON)
		if err != nil {
			return "", fmt.Errorf("decode b64_json: %w", err)
		}
		sum := md5.Sum([]byte(prompt))
		fileName := hex.EncodeToString(sum[:]) + ".png"
		fullPath := filepath.Join(c.samplesDir, fileName)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return "", fmt.Errorf("mkdir: %w", err)
		}
		if err := os.WriteFile(fullPath, imgBytes, 0o644); err != nil {
			return "", fmt.Errorf("write image: %w", err)
		}
		u, _ := url.JoinPath(c.publicURL, c.urlPath, fileName)
		return u, nil
	}
	return "", fmt.Errorf("modelproxy response has neither url nor b64_json")
}
