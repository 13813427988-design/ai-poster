# ai-poster Docker 部署设计

日期:2026-07-30

## 目标

把 ai-poster(Go+Gin backend + React/Vite frontend)部署到一台只装了 Docker 的
x86_64 服务器上,通过 `http://<服务器IP>` 访问完整应用,后端接真实文生图模型
(pollinations)。

## 约束与前提

- 服务器:x86_64 / amd64,已装 Docker(含 `docker compose`),能访问 GitHub 与
  `image.pollinations.ai`(已在目标服务器实测:`code=200`,2.9s)。
- 访问方式:裸 IP + 80 端口,不配域名、不配 HTTPS。
- 代码分发:服务器上 `git clone`,跑 `main` 分支。
- 海报持久化:生成结果落宿主目录,容器重建后不丢。
- 字体:装进镜像(系统字体包),不依赖宿主文件。

## 架构

```
浏览器 ──80──> nginx 容器 ┬─ /         → /usr/share/nginx/html(前端 dist,SPA fallback)
                          ├─ /api/    → http://backend:8080/       (尾斜杠剥掉 /api 前缀)
                          └─ /static/ → http://backend:8080/static/ (海报与占位图)

                          backend 容器(不对外暴露端口)
                            ├─ /app/static/posters ← bind mount ./data/posters
                            └─ /app/static/samples ← bind mount ./data/samples
```

选型理由:nginx 反代方案不需要改任何应用代码。前端 `fetch("/api/generate")` 原样
可用(`proxy_pass` 尾斜杠等价于 vite dev server 的 `rewrite`);页面与海报 URL 同源,
`downloadPoster` 的 `fetch` + blob 下载不会触发 CORS。后续加 HTTPS 只需改 nginx 层。

被否决的替代方案:

- **Go 单容器托管前端**:少一个容器,但需修改 `main.go`(静态托管 + SPA fallback)
  与前端 API 路径,把部署关注点渗入应用代码,且没有地方挂缓存头/TLS。
- **前后端分端口 + CORS**:凭空增加跨域面,需要后端加 CORS 中间件和前端注入
  API base URL。

## 关键配置决策

### PUBLIC_URL

`PUBLIC_URL=http://<服务器IP>`,不带端口(nginx 监听 80)。

后端 `handler/generate.go` 与两个 AI client 都用 `PUBLIC_URL` 拼**绝对** URL 返回给
前端,所以它必须是浏览器可达的地址。写成 `localhost` 或容器内地址会让前端拿到打不开
的链接。

### backend 不暴露端口

compose 里 backend 服务不写 `ports`,只有 nginx 映射 `80:80`。后端 8080 仅在 compose
默认网络内可达,减少对外暴露面。

### samples 也要持久化

`static/samples` 不只是 mock 用途:`modelproxy_client.go` 在代理只返回 `b64_json` 时
也往这里落盘;mock 模式按 prompt 的 md5 缓存复用。两个目录都 bind mount。

### proxy_read_timeout 必须放大

文生图慢。pollinations 实测 3~6s,但 `modelproxy` 路径给了 120s 超时
(`modelproxy_client.go` 的 `http.Client{Timeout: 120s}`),而 nginx
`proxy_read_timeout` 默认 60s。不改的话慢请求会在 nginx 层先断,前端看到 504 而后端
仍在正常执行 —— 症状酷似应用 bug。设为 `180s`。

### ca-certificates 必须装

后端要 HTTPS 调 `image.pollinations.ai`。裸 alpine 无根证书,会直接 x509 失败。

## 生图 provider

保留两个实现,用 `AI_PROVIDER` 环境变量选择,默认 `pollinations`:

| 值 | 实现 | 说明 |
|---|---|---|
| `pollinations` | `PollinationsAIClient`(新增) | 默认。无需 token |
| `modelproxy` | `ModelProxyAIClient`(已有) | 需 `MODELPROXY_TOKEN`,且需内网可达 |
| `mock` | `MockAIClient`(已有) | 渐变占位图,不出网 |

`modelproxy` 保留但不作默认:`models-proxy.stepfun-inc.com` 解析到 `10.148.x.x`
私有地址,公网服务器路由不到。本地开发环境可达,故保留实现。

选择逻辑改为显式 switch,替代当前 `main.go` 里"`MODELPROXY_TOKEN` 非空则用真模型、
否则静默回退 mock"的隐式判断。显式配置的好处:配错时启动即报错,而不是安静地跑 mock
让人误以为在用真模型。

### PollinationsAIClient

`GET https://image.pollinations.ai/prompt/{urlencode(prompt)}?width=&height=&nologo=true&seed=`

实测结论(均已验证,非推断):

- 中文 prompt 直接生效,无需翻译;`BuildPrompt` 产出的 405 字符转义 URL 正常工作,
  产图为扁平插画风竖版、天空大片留白、无文字 —— 正合海报底图需要。
- 请求 `width=1024&height=1536` 实返 **627×940**:匿名调用有分辨率上限,服务端等比
  缩小。做海报够用,但拿不到高清原图。`IMAGE_SIZE` 仍按 `1024x1536` 格式填(与
  modelproxy 共用同一配置项),由 client 拆成 `width` / `height` 两个查询参数;拆解
  失败或为空时走 pollinations 默认尺寸。
- 返回 **JPEG**(非 PNG)。`poster.go` 已 `import _ "image/jpeg"`,合成无需改动。
- 同 prompt + 同 seed 返回**完全相同**的图(两次请求 md5 一致)。服务端按 prompt 缓存。

因此每次请求注入随机 seed,使同一描述可反复生成不同海报(符合"重新生成"按钮的预期)。
随机源用 `math/rand`,不需要密码学强度。

**不做静默降级**:调用失败时返回明确错误,不偷偷切 mock。静默降级会让"接口挂了"和
"图不好看"两种情况无法区分。

**无 SLA 提醒**:这是免费匿名服务,可能限流或变更。demo 足够;若转长期生产使用需换付费
接口。client 设 60s 超时。

## 组件

### backend/Dockerfile

两段式:

- builder:`golang:1.25-alpine`(go.mod 声明 `go 1.25.0`)。先 `COPY go.mod go.sum` +
  `go mod download` 独立成层,只改代码时命中依赖缓存;再
  `CGO_ENABLED=0 go build` 出静态二进制。
- runtime:`alpine:3.21` + `apk add --no-cache font-noto-cjk ca-certificates`。
  `FONT_PATH` 指向实际安装的字体文件路径。

**字体待验证点**:`freetype` 的 `truetype.Parse` 对 `.ttc`(TrueType Collection)
支持存疑。若解析失败,`PosterComposer` 只打一行 log 并静默跳过标题文字。因此镜像
构建后必须实际生成一张带 title 的海报并肉眼确认文字存在;失败则换单独的 `.ttf`
字体包(如 `wqy-zenhei`)。不靠推断,靠观察。

### frontend/Dockerfile

两段式:

- builder:`node:22-alpine`,`npm ci` + `npm run build`。build 脚本是
  `tsc -b && vite build`,类型错误会让构建失败(预期行为)。落地前先在本地跑一遍
  `npm run build` 确认当前代码能过。
- runtime:`nginx:alpine`,`dist` → `/usr/share/nginx/html`,附自定义 `nginx.conf`。

在镜像内 build 而非 COPY 本地 `dist`:`frontend/dist` 被 `frontend/.gitignore` 排除,
服务器 clone 后并不存在该目录,必须在镜像内构建。这同时保证前端产物与源码同步。

### nginx.conf

```nginx
client_max_body_size 2m;

location /api/ {
    proxy_pass http://backend:8080/;
    proxy_read_timeout 180s;
}
location /static/ {
    proxy_pass http://backend:8080/static/;
    proxy_read_timeout 180s;
}
location / {
    try_files $uri $uri/ /index.html;
}
```

### docker-compose.yml

- `backend`:`build: ./backend`,`env_file: .env`,`restart: unless-stopped`,
  两个 bind mount,healthcheck 打已有的 `/healthz`,无 `ports`。
- `nginx`:`build: ./frontend`,`ports: ["80:80"]`,`depends_on: [backend]`,
  `restart: unless-stopped`。

### .env / .env.example

`.env` 不进 git(追加到 `.gitignore`),提交一份 `.env.example` 作模板:

```
# 必填:浏览器可达的地址,决定返回给前端的海报 URL
PUBLIC_URL=http://<服务器IP>

# pollinations(默认,无需 token) | modelproxy(需内网) | mock
AI_PROVIDER=pollinations
IMAGE_SIZE=1024x1536

# 仅 AI_PROVIDER=modelproxy 时需要
MODELPROXY_TOKEN=
MODELPROXY_MODEL=doubao-seedream-4.0
```

默认配置下**唯一必填项是 `PUBLIC_URL`** —— pollinations 不需要凭证。若改用
`modelproxy`,token 只存在服务器的 `.env`,不进镜像、不进 git。

### .dockerignore

排除 `node_modules`、`backend/static/posters`、`backend/static/samples`
(当前已积累 12M 海报,无必要进构建上下文)。`dist` 已被 `frontend/.gitignore` 排除,
但本地开发会生成它,所以 `.dockerignore` 里也列一份,避免本地构建时把过时产物送进
上下文。

## 附带修复:图片下载自回环

`service/downloader.go`。

问题:`MockAIClient` 与 `ModelProxyAIClient`(代理只返回 `b64_json` 时)都把图落盘后
返回 `PUBLIC_URL/static/samples/...`,接着 `ImageDownloader` 用 HTTP GET 这个地址 ——
容器需要经自己的公网 IP 绕回自身。许多云环境 hairpin NAT 不通,此处会失败。

注意这**不只影响 modelproxy**:`AI_PROVIDER=mock` 在服务器上同样会踩到,因为 mock 也
走 `PUBLIC_URL` 拼 URL 再 HTTP 取回。所以哪怕只想用 mock 验证部署链路,这个修复也是
必需的。

`PollinationsAIClient` 不受影响:它返回的是 pollinations 自己的图片 URL(该 endpoint
直接响应图片字节),`downloader` 直接取外部地址,不经过本服务。

修复:给 `ImageDownloader` 加可选的自地址改写。`Download` 时若目标 URL 前缀命中
`PUBLIC_URL`,改写为 `http://127.0.0.1:<PORT>` 再发请求,不依赖 hairpin。

测试(先写测试):

- `publicURL=http://1.2.3.4`、URL `http://1.2.3.4/static/samples/a.png` → 实际请求
  打到本地回环。
- 外部 URL(如 `https://image.pollinations.ai/...`)原样透传,不改写。

## 前置工作

两项在部署前必须完成,否则方案跑不通:

1. **提交 modelproxy 实现**。`backend/service/modelproxy_client.go` 当前 untracked,
   `config/config.go`、`main.go` 的改动未提交。服务器 clone 拿不到。单独提一个 commit,
   与后续 pollinations / 部署改动分开。
2. **合并到 main**。前端全部实现只在 `feat/frontend` 分支;`main` 落后 10 个 commit
   且有 2 个未 push。合并 `feat/frontend` → `main` 并 push,服务器跑 `main`。

## 部署流程

```bash
git clone https://github.com/13813427988-design/ai-poster.git && cd ai-poster
cp .env.example .env && vi .env      # 至少填 PUBLIC_URL
mkdir -p data/posters data/samples
docker compose up -d --build
```

更新:`git pull && docker compose up -d --build`(只重建变化的层)。

## 验证

必须实际执行并观察,不接受"应该能跑":

1. `docker compose ps` → 两容器 up,backend healthy
2. `curl http://<IP>/healthz` → `ok`
3. `curl -X POST http://<IP>/api/generate -H 'Content-Type: application/json' -d
   '{"prompt":"日落海边的渔船","title":"夏日海报"}'` → 返回 URL
4. `curl -I <返回的URL>` → 200;`docker compose logs backend` 含
   `provider=pollinations`(证明用的是真模型,而非 mock)
5. 下载海报**打开查看**:标题文字是否画上 → 验证字体
6. 浏览器访问 `http://<IP>`,走完整生成流程并点击下载按钮
7. 同一 prompt 连发两次,确认两张海报**不同** → 验证随机 seed 生效

第 5、6、7 步不可省略。字体解析失败、跨域下载、seed 未生效都属于"日志干净但结果错误"
的失败模式。

## 明确不做(YAGNI)

- HTTPS / 域名 / 证书自动续期
- 海报自动清理(目前 12M,以后按需加 cron)
- CI / 自动部署
- 多副本 / 负载均衡
- 镜像仓库(在服务器本地 build)
