package service

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
)

// PollinationsAIClient 用 pollinations 免费文生图服务。
//
// 该 endpoint 直接响应图片字节(JPEG),所以 Generate 只拼 URL 返回,由下游
// ImageDownloader 去取——省一次落盘中转,也天然不经过本服务自身地址。
//
// 注意:同 prompt + 同 seed 服务端返回完全相同的图(实测两次请求 md5 一致),
// 因此每次注入随机 seed,保证"重新生成"真的能出新图。
//
// 免费匿名服务,无 SLA:匿名调用有分辨率上限(请求 1024x1536 实返 627x940),
// 可能限流或变更。
type PollinationsAIClient struct {
	baseURL string
	width   string // 空串表示不传,走服务端默认
	height  string
}

// 编译期断言:改 Generate 签名或改名会在这里直接编译失败,而不是等到 main.go 注入时才发现。
var _ AIClient = (*PollinationsAIClient)(nil)

func NewPollinationsAIClient(size string) *PollinationsAIClient {
	c := &PollinationsAIClient{baseURL: "https://image.pollinations.ai/prompt/"}
	c.width, c.height = parseSize(size)
	return c
}

// parseSize 解析 "1024x1536"。格式不合法或非正整数时返回两个空串,调用方省略参数。
func parseSize(size string) (width, height string) {
	w, h, ok := strings.Cut(size, "x")
	if !ok {
		return "", ""
	}
	wn, err := strconv.Atoi(w)
	if err != nil || wn <= 0 {
		return "", ""
	}
	hn, err := strconv.Atoi(h)
	if err != nil || hn <= 0 {
		return "", ""
	}
	return w, h
}

func (c *PollinationsAIClient) Generate(_ context.Context, prompt string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("pollinations: empty prompt")
	}

	q := url.Values{}
	q.Set("nologo", "true")
	// 随机 seed 绕开服务端按 prompt 的缓存;不需要密码学强度
	q.Set("seed", strconv.Itoa(rand.Intn(1_000_000_000)))
	if c.width != "" && c.height != "" {
		q.Set("width", c.width)
		q.Set("height", c.height)
	}

	return c.baseURL + url.PathEscape(prompt) + "?" + q.Encode(), nil
}
