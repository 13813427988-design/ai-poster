# ai-poster Docker 部署实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 ai-poster 部署到只装了 Docker 的 x86_64 服务器,通过 `http://<IP>` 访问,后端接 pollinations 真实文生图。

**Architecture:** docker compose 双容器。nginx 容器托管前端 dist 并把 `/api/`、`/static/` 反代到 backend 容器;backend 不对外暴露端口,海报落宿主 bind mount。生图 provider 由 `AI_PROVIDER` 显式选择。

**Tech Stack:** Go 1.25 + Gin,React 19 + Vite 8,nginx:alpine,docker compose。

**Spec:** `docs/superpowers/specs/2026-07-30-docker-deploy-design.md`

## Global Constraints

- Go builder 镜像 `golang:1.25-alpine`(go.mod 声明 `go 1.25.0`),runtime `alpine:3.21`。
- Node builder 镜像 `node:22-alpine`。
- 目标架构 x86_64 / amd64,在服务器本地 build,不用镜像仓库。
- 字体路径 `/usr/share/fonts/noto/NotoSansCJK-Regular.ttc`(已核实为 `font-noto-cjk` 在 alpine v3.21 x86_64 的实际安装路径)。freetype 支持 `.ttc`,解析集合中第一个字体(`truetype/truetype.go:542,557`),无需 `.ttf` 回退。
- pollinations endpoint:`https://image.pollinations.ai/prompt/{escaped}`,查询参数 `width`、`height`、`nologo=true`、`seed`。
- 后端已有路由:`GET /healthz`、`POST /generate`、`GET /static/*`。nginx 用 `proxy_pass` 尾斜杠剥掉 `/api` 前缀。
- `proxy_read_timeout 180s`(默认 60s 会截断慢生图请求)。
- 现有代码库**没有任何 Go 测试**。本计划建立约定:测试与被测文件同目录,`*_test.go`,标准 `testing` 包,不引入断言库。
- 每个任务结束时提交,commit message 用英文,body 说明 why。

---

### Task 1: 提交已有的 modelproxy 实现

当前 `backend/service/modelproxy_client.go` 是 untracked,`config/config.go` 与 `main.go` 的改动未提交。先把这份既有工作单独固化成一个 commit,与后续部署改动分开,便于回溯。

**Files:**
- Commit(不修改内容): `backend/service/modelproxy_client.go`、`backend/config/config.go`、`backend/main.go`

**Interfaces:**
- Consumes: 无
- Produces: `service.NewModelProxyAIClient(endpoint, token, model, size, samplesDir, publicURL string) *ModelProxyAIClient`,满足 `service.AIClient` 接口;`config.Config` 字段 `ModelProxyEndpoint`、`ModelProxyToken`、`ModelProxyModel`、`ImageSize`

- [ ] **Step 1: 确认编译与 vet 通过**

```bash
cd backend && go build ./... && go vet ./...
```

Expected: 无输出(两条命令都静默成功)

- [ ] **Step 2: 确认待提交范围就是这三个文件**

```bash
cd /Users/jyxc/projects/ai-poster && git status --short
```

Expected: 恰好三行 —— ` M backend/config/config.go`、` M backend/main.go`、`?? backend/service/modelproxy_client.go`

- [ ] **Step 3: 提交**

```bash
cd /Users/jyxc/projects/ai-poster
git add backend/service/modelproxy_client.go backend/config/config.go backend/main.go
git commit -m "$(cat <<'EOF'
feat(backend): add modelproxy AI client for real image generation

Implements AIClient against an OpenAI-compatible /v1/images/generations
endpoint. Handles both response shapes: passes through `url` when the
proxy returns one, and decodes `b64_json` to disk otherwise so the
downstream downloader contract (an HTTP-GETtable URL) stays unchanged.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: downloader 自地址改写(修 hairpin NAT)

`MockAIClient` 和 `ModelProxyAIClient`(b64 分支)都返回 `PUBLIC_URL/static/samples/...`,`ImageDownloader` 随后用 HTTP GET 取回 —— 容器要经自己的公网 IP 绕回自身,许多云环境 hairpin NAT 不通。这影响 mock 和 modelproxy 两种 provider,所以哪怕只用 mock 验证部署链路也必须先修。

**Files:**
- Modify: `backend/service/downloader.go`
- Create: `backend/service/downloader_test.go`
- Modify: `backend/main.go`(注入 selfURL / localAddr)

**Interfaces:**
- Consumes: Task 1 的 `config.Config`
- Produces: `service.NewImageDownloader() *ImageDownloader` 签名不变(保持向后兼容);新增 `func (d *ImageDownloader) WithSelfRewrite(publicURL, localAddr string) *ImageDownloader`,返回 `d` 本身以便链式调用

- [ ] **Step 1: 写失败的测试**

Create `backend/service/downloader_test.go`:

```go
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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./service/ -run TestDownload -v`
Expected: 编译失败,`d.WithSelfRewrite undefined (type *ImageDownloader has no field or method WithSelfRewrite)`

- [ ] **Step 3: 最小实现**

Modify `backend/service/downloader.go` —— 给 struct 加两个字段:

```go
type ImageDownloader struct {
	client *http.Client

	// selfPrefix 非空时,命中该前缀的 URL 会被改写到 localAddr。
	// 用途:容器内取自己刚落盘的图,不依赖 hairpin NAT 经公网绕回。
	selfPrefix string
	localAddr  string
}
```

在 `NewImageDownloader` 之后加:

```go
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
```

在 `Download` 里把构造请求那一行改为先改写:

```go
func (d *ImageDownloader) Download(ctx context.Context, imageURL, destPath string) error {
	reqURL := d.rewriteSelf(imageURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
```

并把后面那句错误信息里的 `imageURL` 换成 `reqURL`,让日志显示实际请求的地址:

```go
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed, status=%s url=%s", resp.Status, reqURL)
	}
```

在 import 块加 `"strings"`。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./service/ -run TestDownload -v`
Expected: 三个测试全 PASS

- [ ] **Step 5: main.go 注入改写**

Modify `backend/main.go`,把 `downloader := service.NewImageDownloader()` 改为:

```go
	// 容器内取自己落盘的图走回环,不依赖 hairpin NAT
	downloader := service.NewImageDownloader().
		WithSelfRewrite(cfg.PublicURL, "http://127.0.0.1:"+cfg.Port)
```

- [ ] **Step 6: 确认编译通过**

Run: `cd backend && go build ./... && go vet ./...`
Expected: 无输出

- [ ] **Step 7: 提交**

```bash
cd /Users/jyxc/projects/ai-poster
git add backend/service/downloader.go backend/service/downloader_test.go backend/main.go
git commit -m "$(cat <<'EOF'
fix(backend): route self-targeted downloads via loopback

MockAIClient and ModelProxyAIClient's b64 branch both return
PUBLIC_URL/static/samples/..., which the downloader then fetches over
HTTP. In a container that means reaching the host's own public IP, which
fails wherever hairpin NAT is unavailable. Rewrite such URLs to
127.0.0.1:<PORT> so the request never leaves the container.

External URLs pass through untouched.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: PollinationsAIClient

新增第三个 `AIClient` 实现。pollinations 的 endpoint 直接响应图片字节,所以 `Generate` 只需拼出 URL 返回,不必自己发请求 —— 下游 `ImageDownloader` 会去取。这也意味着它天然不受 Task 2 那个自回环问题影响。

已实测确认(写进测试断言的依据):中文 prompt 无需翻译;`BuildPrompt` 产出的 405 字符转义 URL 正常;返回 JPEG;同 prompt + 同 seed 返回完全相同的图,故必须注入随机 seed。

**Files:**
- Create: `backend/service/pollinations_client.go`
- Create: `backend/service/pollinations_client_test.go`

**Interfaces:**
- Consumes: `service.AIClient` 接口(`Generate(ctx, prompt) (string, error)`)
- Produces: `service.NewPollinationsAIClient(size string) *PollinationsAIClient`,其中 `size` 为 `"1024x1536"` 格式或空串

- [ ] **Step 1: 写失败的测试**

Create `backend/service/pollinations_client_test.go`:

```go
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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./service/ -run TestPollinations -v`
Expected: 编译失败,`undefined: NewPollinationsAIClient`

- [ ] **Step 3: 最小实现**

Create `backend/service/pollinations_client.go`:

```go
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
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./service/ -run TestPollinations -v`
Expected: 三个测试全 PASS

- [ ] **Step 5: 用真实 endpoint 验一次(不进 CI,手动跑一次确认契约没变)**

```bash
cd backend && cat > /tmp/poll_live.go <<'EOF'
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"ai-poster/backend/service"
)

func main() {
	c := service.NewPollinationsAIClient("1024x1536")
	u, err := c.Generate(context.Background(), service.NewPromptService().BuildPrompt("日落海边的渔船"))
	if err != nil {
		panic(err)
	}
	fmt.Println("URL:", u)
	resp, err := http.Get(u)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	n, _ := io.Copy(io.Discard, resp.Body)
	fmt.Printf("status=%s content-type=%s bytes=%d\n", resp.Status, resp.Header.Get("Content-Type"), n)
}
EOF
go run /tmp/poll_live.go && rm /tmp/poll_live.go
```

Expected: `status=200 OK content-type=image/jpeg bytes=` 后跟一个 5 万以上的数字。若 status 非 200,说明服务契约变了,停下来重新评估,不要继续。

- [ ] **Step 6: 提交**

```bash
cd /Users/jyxc/projects/ai-poster
git add backend/service/pollinations_client.go backend/service/pollinations_client_test.go
git commit -m "$(cat <<'EOF'
feat(backend): add pollinations AI client

Needed because models-proxy.stepfun-inc.com resolves to 10.148.x.x, a
private address the deployment target cannot route to. Pollinations needs
no token and was verified reachable from that server (200, 2.9s).

Generate only builds the URL; the endpoint responds with image bytes
directly, so the existing downloader fetches it. A random seed is
injected per call because the service returns a byte-identical image for
a repeated prompt+seed, which would break the regenerate action.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: AI_PROVIDER 显式开关

当前 `main.go` 用"`MODELPROXY_TOKEN` 非空则真模型,否则静默回退 mock"的隐式判断。问题是配错 token 时服务安静地跑 mock,让人误以为在用真模型。改成显式配置 + 启动即报错。

**Files:**
- Modify: `backend/config/config.go`
- Create: `backend/config/config_test.go`
- Modify: `backend/main.go:24-33`(provider 选择那段)

**Interfaces:**
- Consumes: Task 3 的 `service.NewPollinationsAIClient(size string)`;Task 1 的 `service.NewModelProxyAIClient(...)`;已有的 `service.NewMockAIClient(samplesDir, publicURL string)`
- Produces: `config.Config` 新增字段 `AIProvider string`;常量 `config.ProviderPollinations = "pollinations"`、`config.ProviderModelProxy = "modelproxy"`、`config.ProviderMock = "mock"`

- [ ] **Step 1: 写失败的测试**

Create `backend/config/config_test.go`:

```go
package config

import "testing"

func TestLoadDefaultsToPollinations(t *testing.T) {
	t.Setenv("AI_PROVIDER", "")
	if got := Load().AIProvider; got != ProviderPollinations {
		t.Errorf("AIProvider = %q, want %q", got, ProviderPollinations)
	}
}

func TestLoadReadsAIProvider(t *testing.T) {
	for _, want := range []string{ProviderPollinations, ProviderModelProxy, ProviderMock} {
		t.Setenv("AI_PROVIDER", want)
		if got := Load().AIProvider; got != want {
			t.Errorf("AI_PROVIDER=%q gave AIProvider=%q", want, got)
		}
	}
}

// 大小写和空格不该导致启动失败——归一化处理。
func TestLoadNormalizesAIProvider(t *testing.T) {
	t.Setenv("AI_PROVIDER", "  Pollinations  ")
	if got := Load().AIProvider; got != ProviderPollinations {
		t.Errorf("AIProvider = %q, want %q", got, ProviderPollinations)
	}
}

// 纯空格必须回退到默认,不能变成空串。envOr 先判空会认为 "   " 非空,
// 所以归一化必须在取默认值之前做。
func TestLoadTreatsBlankAIProviderAsDefault(t *testing.T) {
	t.Setenv("AI_PROVIDER", "   ")
	if got := Load().AIProvider; got != ProviderPollinations {
		t.Errorf("AIProvider = %q, want %q", got, ProviderPollinations)
	}
}

func TestValidateRejectsUnknownProvider(t *testing.T) {
	t.Setenv("AI_PROVIDER", "gpt-image")
	if err := Load().Validate(); err == nil {
		t.Error("Validate() = nil, want error for unknown provider")
	}
}

// modelproxy 缺 token 必须启动失败,而不是安静回退 mock。
func TestValidateRequiresTokenForModelProxy(t *testing.T) {
	t.Setenv("AI_PROVIDER", ProviderModelProxy)
	t.Setenv("MODELPROXY_TOKEN", "")
	if err := Load().Validate(); err == nil {
		t.Error("Validate() = nil, want error when modelproxy has no token")
	}

	t.Setenv("MODELPROXY_TOKEN", "sk-test")
	if err := Load().Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil when token is set", err)
	}
}

func TestValidateAcceptsPollinationsWithoutToken(t *testing.T) {
	t.Setenv("AI_PROVIDER", ProviderPollinations)
	t.Setenv("MODELPROXY_TOKEN", "")
	if err := Load().Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil (pollinations needs no credentials)", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./config/ -v`
Expected: 编译失败,`undefined: ProviderPollinations`

- [ ] **Step 3: 最小实现**

Modify `backend/config/config.go`:

在 import 块加 `"fmt"` 和 `"strings"`。在 `Config` struct 定义前加常量:

```go
// 生图 provider 取值。
const (
	ProviderPollinations = "pollinations" // 默认,无需凭证
	ProviderModelProxy   = "modelproxy"   // 需 token,且 endpoint 需内网可达
	ProviderMock         = "mock"         // 本地渐变占位图,不出网
)
```

在 `Config` struct 里加字段(放 `FontPath` 之后):

```go
	AIProvider string // 生图 provider,见 Provider* 常量
```

在 `Load()` 的返回值里加。注意归一化要在取默认值**之前**做:`envOr` 只判断空串,
`"   "` 会被当成有效值,若先取默认再 Trim 就会得到空串而非默认值。

```go
		AIProvider: normalizeProvider(os.Getenv("AI_PROVIDER")),
```

在 `envOr` 旁边加:

```go
// normalizeProvider 归一化大小写与空白;空白或空串回退默认 provider。
func normalizeProvider(v string) string {
	if s := strings.ToLower(strings.TrimSpace(v)); s != "" {
		return s
	}
	return ProviderPollinations
}
```

在文件末尾加:

```go
// Validate 检查配置自身一致性。返回非 nil 时调用方应让进程启动失败——
// 显式报错胜过静默降级:配错时安静跑 mock 会让人误以为在用真模型。
func (c *Config) Validate() error {
	switch c.AIProvider {
	case ProviderPollinations, ProviderMock:
		return nil
	case ProviderModelProxy:
		if c.ModelProxyToken == "" {
			return fmt.Errorf("AI_PROVIDER=%s requires MODELPROXY_TOKEN", ProviderModelProxy)
		}
		return nil
	default:
		return fmt.Errorf("unknown AI_PROVIDER %q, want one of %s/%s/%s",
			c.AIProvider, ProviderPollinations, ProviderModelProxy, ProviderMock)
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./config/ -v`
Expected: 六个测试全 PASS

- [ ] **Step 5: main.go 改用显式 switch**

Modify `backend/main.go`,把 `cfg := config.Load()` 之后紧接着加校验:

```go
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}
```

把整段 provider 选择(从 `var aiClient service.AIClient` 到那个 `else` 块结束)替换为:

```go
	var aiClient service.AIClient
	switch cfg.AIProvider {
	case config.ProviderPollinations:
		aiClient = service.NewPollinationsAIClient(cfg.ImageSize)
	case config.ProviderModelProxy:
		aiClient = service.NewModelProxyAIClient(cfg.ModelProxyEndpoint, cfg.ModelProxyToken,
			cfg.ModelProxyModel, cfg.ImageSize, cfg.SamplesDir, cfg.PublicURL)
	case config.ProviderMock:
		aiClient = service.NewMockAIClient(cfg.SamplesDir, cfg.PublicURL)
	}
	log.Printf("ai-poster provider=%s image_size=%q", cfg.AIProvider, cfg.ImageSize)
```

`switch` 不需要 `default`:`Validate()` 已排除未知值,漏网的话 `aiClient` 为 nil 会在首次请求时 panic,而 `Validate` 保证到不了那一步。

- [ ] **Step 6: 全量测试 + 编译**

Run: `cd backend && go build ./... && go vet ./... && go test ./...`
Expected: build/vet 无输出;`ok ai-poster/backend/config`、`ok ai-poster/backend/service`

- [ ] **Step 7: 手动确认启动日志与失败行为**

```bash
cd backend
AI_PROVIDER=pollinations timeout 3 go run . 2>&1 | head -3
echo "--- expect exit 1 below ---"
AI_PROVIDER=modelproxy MODELPROXY_TOKEN= go run . 2>&1 | head -2
```

Expected: 第一条含 `provider=pollinations` 并正常监听;第二条打印 `config: AI_PROVIDER=modelproxy requires MODELPROXY_TOKEN` 并立即退出(不是安静跑起来)

- [ ] **Step 8: 提交**

```bash
cd /Users/jyxc/projects/ai-poster
git add backend/config/config.go backend/config/config_test.go backend/main.go
git commit -m "$(cat <<'EOF'
feat(backend): select AI provider explicitly via AI_PROVIDER

Replaces the implicit "MODELPROXY_TOKEN set means use the real model,
otherwise silently fall back to mock" rule. That fallback made a
misconfigured deploy look healthy while quietly serving gradient
placeholders. Now the provider is named outright and Validate() fails
startup on an unknown provider or a credential-less modelproxy.

Defaults to pollinations, which needs no credentials.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: backend 镜像

**Files:**
- Create: `backend/Dockerfile`
- Create: `backend/.dockerignore`

**Interfaces:**
- Consumes: Task 4 的 `AI_PROVIDER` 配置;`config.Config` 的 `FontPath`
- Produces: 一个监听 `8080` 的镜像,`/healthz` 可用;字体在 `/usr/share/fonts/noto/NotoSansCJK-Regular.ttc`

- [ ] **Step 1: 写 Dockerfile**

Create `backend/Dockerfile`:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /src

# 依赖单独一层:只改代码时命中缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/ai-poster .

FROM alpine:3.21
# ca-certificates: 后端要 HTTPS 调 image.pollinations.ai,裸 alpine 无根证书会 x509 失败
# font-noto-cjk: 标题合成需要中文字体,缺失时 PosterComposer 会静默跳过文字
RUN apk add --no-cache ca-certificates font-noto-cjk

WORKDIR /app
COPY --from=builder /out/ai-poster /app/ai-poster

# freetype 支持 .ttc,取集合中第一个字体(truetype/truetype.go:542,557)
ENV FONT_PATH=/usr/share/fonts/noto/NotoSansCJK-Regular.ttc \
    STATIC_DIR=/app/static \
    PORT=8080

EXPOSE 8080
CMD ["/app/ai-poster"]
```

- [ ] **Step 2: 写 .dockerignore**

Create `backend/.dockerignore`:

```
# 运行时产物,不该进构建上下文(本地已积累 12M)
static/posters
static/samples

# 本地构建产物
backend
main
*.test
*.out
```

- [ ] **Step 3: 确认字体路径真实存在**

这一步不能靠推断 —— 在服务器上跑:

```bash
docker run --rm alpine:3.21 sh -c "apk add --no-cache font-noto-cjk >/dev/null 2>&1 && ls -la /usr/share/fonts/noto/NotoSansCJK-Regular.ttc"
```

Expected: 列出该文件,大小数十 MB。若 `No such file`,改跑 `find /usr/share/fonts -name '*CJK*'` 找到真实路径,并同步修正 Dockerfile 里的 `FONT_PATH`。

- [ ] **Step 4: 构建镜像**

```bash
cd backend && docker build -t ai-poster-backend:test .
```

Expected: 最后一行 `naming to docker.io/library/ai-poster-backend:test` 或 `Successfully tagged`

- [ ] **Step 5: 冒烟测试容器**

```bash
docker run -d --name aptest -p 18080:8080 -e PUBLIC_URL=http://localhost:18080 ai-poster-backend:test
sleep 2
docker logs aptest
curl -s http://localhost:18080/healthz; echo
```

Expected: 日志含 `provider=pollinations` 且**不含** `font not loaded`(出现 `font not loaded` 说明字体路径错了,回 Step 3);curl 返回 `ok`

- [ ] **Step 6: 端到端验证一张真海报(含中文标题)**

```bash
curl -s -X POST http://localhost:18080/generate \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"日落海边的渔船","title":"夏日海报"}'
```

Expected: 返回 `{"url":"http://localhost:18080/static/posters/<uuid>.png"}`。若返回 error,先看 `docker logs aptest`。

```bash
curl -s -o /tmp/poster.png "$(curl -s -X POST http://localhost:18080/generate -H 'Content-Type: application/json' -d '{"prompt":"雪山日出","title":"冬日海报"}' | sed 's/.*"url":"//;s/"}//')"
file /tmp/poster.png
```

Expected: `PNG image data, 627 x 940` 或类似尺寸

- [ ] **Step 7: 打开图确认标题文字真的画上去了**

```bash
open /tmp/poster.png    # macOS;服务器上改用 scp 拉回本地看
```

Expected: 图上能看到"冬日海报"四个白字带黑描边。**这一步不可跳过** —— 字体加载失败只打一行 log 就静默跳过文字,不会报错。

- [ ] **Step 8: 清理**

```bash
docker rm -f aptest
```

- [ ] **Step 9: 提交**

```bash
cd /Users/jyxc/projects/ai-poster
git add backend/Dockerfile backend/.dockerignore
git commit -m "$(cat <<'EOF'
build(backend): add Dockerfile

Two-stage: static CGO_ENABLED=0 binary on alpine:3.21. Installs
ca-certificates (pollinations is HTTPS; bare alpine has no root store)
and font-noto-cjk (a missing font makes PosterComposer skip the title
silently rather than error).

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: frontend 镜像 + nginx 配置

**Files:**
- Create: `frontend/nginx.conf`
- Create: `frontend/Dockerfile`
- Create: `frontend/.dockerignore`

**Interfaces:**
- Consumes: Task 5 的 backend 镜像(在 compose 网络里以主机名 `backend` 可达,端口 8080)
- Produces: 监听 80 的 nginx 镜像,`/api/*` 与 `/static/*` 反代到 backend,其余走 SPA fallback

- [ ] **Step 1: 本地确认前端能构建**

`npm run build` 跑的是 `tsc -b && vite build`,类型错误会让镜像构建失败。先在本地暴露问题:

```bash
cd frontend && npm run build
```

Expected: 生成 `dist/`,无 TS 错误。若报错,先修好类型问题再继续 —— 别在 docker build 里排查这个。

- [ ] **Step 2: 写 nginx.conf**

Create `frontend/nginx.conf`:

```nginx
server {
    listen 80;
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    client_max_body_size 2m;

    # 尾斜杠让 /api/generate 转发为 /generate,与 vite dev server 的 rewrite 行为一致
    location /api/ {
        proxy_pass http://backend:8080/;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        # 文生图慢;nginx 默认 60s 会在后端仍在正常执行时截断,前端看到 504
        proxy_read_timeout 180s;
        proxy_send_timeout 180s;
    }

    # 海报与占位图由 backend 提供
    location /static/ {
        proxy_pass http://backend:8080/static/;
        proxy_set_header Host $host;
        proxy_read_timeout 180s;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

- [ ] **Step 3: 写 Dockerfile**

Create `frontend/Dockerfile`:

```dockerfile
FROM node:22-alpine AS builder
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
# build = tsc -b && vite build,类型错误会让构建失败(预期行为)
RUN npm run build

FROM nginx:alpine
COPY --from=builder /src/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

- [ ] **Step 4: 写 .dockerignore**

Create `frontend/.dockerignore`:

```
node_modules
# dist 在镜像内构建;本地残留的过时产物不该进上下文
dist
dist-ssr
*.local
.oxlintrc.json
```

- [ ] **Step 5: 构建镜像**

```bash
cd frontend && docker build -t ai-poster-frontend:test .
```

Expected: 构建成功。`npm ci` 那层耗时较久(依赖多)。

- [ ] **Step 6: 确认 dist 真的进了镜像**

```bash
docker run --rm ai-poster-frontend:test ls /usr/share/nginx/html
```

Expected: 列出 `index.html`、`assets`、`favicon.svg`

- [ ] **Step 7: 确认 nginx 配置语法正确**

```bash
docker run --rm ai-poster-frontend:test nginx -t 2>&1
```

Expected: `syntax is ok` 和 `test is successful`。注意此时 `backend` 主机名无法解析,但 `nginx -t` 不解析 upstream,所以不影响。

- [ ] **Step 8: 提交**

```bash
cd /Users/jyxc/projects/ai-poster
git add frontend/Dockerfile frontend/nginx.conf frontend/.dockerignore
git commit -m "$(cat <<'EOF'
build(frontend): add Dockerfile and nginx reverse proxy config

nginx serves dist and proxies /api/ and /static/ to the backend, so the
app stays same-origin: the existing fetch("/api/generate") works
unchanged and blob downloads of poster URLs never hit CORS.

proxy_read_timeout is raised to 180s because nginx's 60s default cuts off
slow generations while the backend is still working, surfacing as a 504
that looks like an application bug.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: compose 编排与环境模板

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`
- Modify: `.gitignore`(加 `.env` 与 `data/`)

**Interfaces:**
- Consumes: Task 5 的 `backend/Dockerfile`、Task 6 的 `frontend/Dockerfile`
- Produces: `docker compose up -d --build` 起完整栈;宿主 `./data/posters`、`./data/samples` 持久化

- [ ] **Step 1: 写 docker-compose.yml**

Create `docker-compose.yml`:

```yaml
services:
  backend:
    build: ./backend
    env_file: .env
    restart: unless-stopped
    # 不映射 ports:仅 compose 网络内可达,减少对外暴露面
    volumes:
      - ./data/posters:/app/static/posters
      - ./data/samples:/app/static/samples
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://127.0.0.1:8080/healthz"]
      interval: 10s
      timeout: 3s
      retries: 5
      start_period: 5s

  nginx:
    build: ./frontend
    restart: unless-stopped
    ports:
      - "80:80"
    depends_on:
      backend:
        condition: service_healthy
```

- [ ] **Step 2: 写 .env.example**

Create `.env.example`:

```
# 必填:浏览器可达的地址。后端用它拼返回给前端的海报 URL,
# 写成 localhost 或容器内地址会让前端拿到打不开的链接。
PUBLIC_URL=http://YOUR_SERVER_IP

# pollinations(默认,无需 token) | modelproxy(需内网可达) | mock
AI_PROVIDER=pollinations

# 生成尺寸。注意 pollinations 匿名调用有上限,请求 1024x1536 实返约 627x940
IMAGE_SIZE=1024x1536

# 仅 AI_PROVIDER=modelproxy 时需要;留空会让服务启动失败(有意为之)
MODELPROXY_TOKEN=
MODELPROXY_MODEL=doubao-seedream-4.0
```

- [ ] **Step 3: 更新 .gitignore**

Modify `.gitignore`,在文件末尾追加:

```
# 环境变量(含凭证),只存在服务器本地
.env

# compose bind mount 的运行时数据
data/
```

- [ ] **Step 4: 确认 compose 配置能解析**

```bash
cd /Users/jyxc/projects/ai-poster
cp .env.example .env
docker compose config >/dev/null && echo "compose config OK"
```

Expected: `compose config OK`

- [ ] **Step 5: 确认 .env 不会被提交**

```bash
git check-ignore -v .env
```

Expected: 输出 `.gitignore:<行号>:.env	.env`。若无输出,说明 ignore 规则没生效,回 Step 3。

- [ ] **Step 6: 提交**

```bash
cd /Users/jyxc/projects/ai-poster
git add docker-compose.yml .env.example .gitignore
git commit -m "$(cat <<'EOF'
build: add docker compose stack

nginx is the only service with a published port; backend stays on the
compose network. Posters and samples bind-mount to ./data so they
survive container rebuilds.

.env holds PUBLIC_URL and any credentials and is gitignored; .env.example
is the template. With the default pollinations provider, PUBLIC_URL is
the only value that must be filled in.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: 部署文档 + 合并到 main

前端全部实现只在 `feat/frontend`;`main` 落后 10 个 commit 且有 2 个未 push。服务器要跑 `main`,所以先合并。

**Files:**
- Create: `DEPLOY.md`
- Modify: 分支 `main`(合并 `feat/frontend`)

**Interfaces:**
- Consumes: Task 1-7 的全部产出
- Produces: `origin/main` 含完整可部署代码

- [ ] **Step 1: 写 DEPLOY.md**

Create `DEPLOY.md`:

```markdown
# 部署

前提:服务器已装 Docker(含 `docker compose`),能访问 GitHub 与
`image.pollinations.ai`,x86_64。

## 首次部署

```bash
git clone https://github.com/13813427988-design/ai-poster.git
cd ai-poster
cp .env.example .env
vi .env                      # 把 PUBLIC_URL 改成 http://<你的服务器IP>
mkdir -p data/posters data/samples
docker compose up -d --build
```

浏览器打开 `http://<你的服务器IP>`。

## 更新

```bash
git pull && docker compose up -d --build
```

只有变化的镜像层会重建。

## 排查

| 现象 | 检查 |
|---|---|
| 前端能开但生成报错 | `docker compose logs backend` |
| 海报链接打不开 | `.env` 里 `PUBLIC_URL` 是否为浏览器可达地址(非 localhost) |
| 海报没有标题文字 | backend 日志是否有 `font not loaded` |
| 生成卡住后 504 | nginx `proxy_read_timeout`(已设 180s) |
| 同一描述总出同一张图 | 应已由随机 seed 避免;检查是否误用了 `AI_PROVIDER=mock` |

`AI_PROVIDER` 可选 `pollinations`(默认,无需凭证)、`modelproxy`(需
`MODELPROXY_TOKEN`,且 endpoint 需内网可达)、`mock`(本地渐变图,不出网)。
改完 `.env` 后 `docker compose up -d` 重启生效。

## 已知限制

- pollinations 是免费匿名服务,无 SLA,可能限流;匿名调用有分辨率上限(请求
  1024x1536 实返约 627x940)。
- 海报不自动清理,`data/posters` 会持续增长,需要时手动清或加 cron。
- 仅 HTTP,无 HTTPS。
```

- [ ] **Step 2: 提交文档**

```bash
cd /Users/jyxc/projects/ai-poster
git add DEPLOY.md
git commit -m "$(cat <<'EOF'
docs: add deployment guide

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 3: 确认工作区干净、测试全绿**

```bash
cd backend && go test ./... && cd .. && git status --short
```

Expected: 测试 `ok`;`git status --short` 只显示 `.env`(已 ignore 则无输出)

- [ ] **Step 4: 合并到 main**

```bash
cd /Users/jyxc/projects/ai-poster
git checkout main
git merge feat/frontend --no-ff -m "$(cat <<'EOF'
Merge feat/frontend: React UI + Docker deployment

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

Expected: 合并成功。有冲突则停下报告,不要自行猜测解法。

- [ ] **Step 5: 确认 main 上代码完整**

```bash
git log --oneline -3
ls docker-compose.yml .env.example DEPLOY.md backend/Dockerfile frontend/Dockerfile
cd backend && go build ./... && go test ./...
```

Expected: 五个文件都在;build 无输出;测试 `ok`

- [ ] **Step 6: push**

```bash
cd /Users/jyxc/projects/ai-poster && git push origin main
```

Expected: push 成功

---

### Task 9: 服务器部署与验证

在服务器上执行。这个任务没有代码产出,只有部署动作和验证 —— 但每一步的期望输出都必须实际看到,不接受"应该能跑"。

**Files:**
- 无(仅服务器操作)

**Interfaces:**
- Consumes: Task 8 push 到 `origin/main` 的代码
- Produces: `http://<IP>` 上运行的服务

- [ ] **Step 1: clone 并配置**

```bash
git clone https://github.com/13813427988-design/ai-poster.git
cd ai-poster
cp .env.example .env
```

编辑 `.env`,把 `PUBLIC_URL` 改为 `http://<服务器公网IP>`(不带端口,nginx 在 80)。

```bash
mkdir -p data/posters data/samples
grep PUBLIC_URL .env
```

Expected: 显示你填的真实 IP,不是 `YOUR_SERVER_IP`

- [ ] **Step 2: 起栈**

```bash
docker compose up -d --build
```

Expected: 两个服务都 `Started`。首次构建需要几分钟(`npm ci` + `go mod download`)。

- [ ] **Step 3: 确认容器状态**

```bash
docker compose ps
```

Expected: `backend` 状态 `Up (healthy)`,`nginx` 状态 `Up`。backend 若是 `unhealthy` 或 `Restarting`,看 `docker compose logs backend`。

- [ ] **Step 4: 确认 provider 是真模型**

```bash
docker compose logs backend | grep -E "provider|font"
```

Expected: 含 `provider=pollinations`;**不含** `font not loaded`。若看到 `provider=mock`,说明 `.env` 配错了。

- [ ] **Step 5: healthz**

```bash
curl -s http://localhost/healthz; echo
```

Expected: `ok`

- [ ] **Step 6: 通过 nginx 生成一张海报**

```bash
curl -s -X POST http://localhost/api/generate \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"日落海边的渔船","title":"夏日海报"}'
```

Expected: `{"url":"http://<你的IP>/static/posters/<uuid>.png"}` —— 注意 URL 里必须是你的真实 IP(证明 `PUBLIC_URL` 配对了),且路径能通过 nginx 访问。

- [ ] **Step 7: 确认海报 URL 可达**

```bash
curl -sI "$(curl -s -X POST http://localhost/api/generate -H 'Content-Type: application/json' -d '{"prompt":"雪山日出","title":"冬日海报"}' | sed 's/.*"url":"//;s/"}//')" | head -3
```

Expected: `HTTP/1.1 200 OK` 且 `Content-Type: image/png`

- [ ] **Step 8: 确认随机 seed 生效**

```bash
for i in 1 2; do
  curl -s -X POST http://localhost/api/generate -H 'Content-Type: application/json' \
    -d '{"prompt":"同一个描述","title":"测试"}' | sed 's/.*posters\///;s/"}//'
done
ls -la data/posters/*.png | tail -2
md5sum $(ls -t data/posters/*.png | head -2)
```

Expected: 两个 md5 **不同**。相同说明 seed 没生效,回 Task 3 检查。

- [ ] **Step 9: 确认持久化**

```bash
BEFORE=$(ls data/posters/*.png | wc -l)
docker compose down && docker compose up -d
sleep 5
AFTER=$(ls data/posters/*.png | wc -l)
echo "before=$BEFORE after=$AFTER"
```

Expected: 两个数字相等且大于 0(容器重建后海报仍在)

- [ ] **Step 10: 浏览器端到端**

浏览器打开 `http://<服务器IP>`:

1. 页面正常渲染(不是 nginx 默认页、不是 404)
2. 填 prompt 和标题,点生成 → 几秒后出现海报预览
3. **看海报上有没有标题文字** → 验证字体
4. 点下载按钮 → 文件成功下载(验证同源 blob 下载没被 CORS 拦)
5. 同一 prompt 再生成一次 → 出来的图和上一张不同

Expected: 五项全部成立。第 3、4 项不可跳过 —— 字体失败和跨域下载失败都是"日志干净但结果错误"的类型。

- [ ] **Step 11: 报告结果**

如实报告:哪几步通过、哪几步没通过、实际输出是什么。有任何一步没达到预期就不要声称部署完成。

---

## 附:任务依赖

```
Task 1 (提交 modelproxy)
  └─ Task 2 (downloader 自回环)
       └─ Task 3 (pollinations client)
            └─ Task 4 (AI_PROVIDER 开关)
                 └─ Task 5 (backend 镜像)
                      └─ Task 6 (frontend + nginx 镜像)
                           └─ Task 7 (compose)
                                └─ Task 8 (文档 + 合 main)
                                     └─ Task 9 (部署验证)
```

严格串行。Task 5 起需要服务器上的 Docker(本地无 docker 命令)。
