# ai-poster Docker 部署设计

日期:2026-07-30

## 目标

把 ai-poster(Go+Gin backend + React/Vite frontend)部署到一台只装了 Docker 的
x86_64 服务器上,通过 `http://<服务器IP>` 访问完整应用,后端接真实文生图模型
(modelproxy)。

## 约束与前提

- 服务器:x86_64 / amd64,已装 Docker(含 `docker compose`),能访问 GitHub 与
  `models-proxy.stepfun-inc.com`。
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

文生图慢。后端给 modelproxy 留了 120s 超时(`modelproxy_client.go` 的
`http.Client{Timeout: 120s}`),而 nginx `proxy_read_timeout` 默认 60s。不改的话真实
生图会在 nginx 层先断,前端看到 504 而后端仍在正常执行 —— 症状酷似应用 bug。设为
`180s`。

### ca-certificates 必须装

后端要 HTTPS 调 `models-proxy.stepfun-inc.com`。裸 alpine 无根证书,会直接 x509 失败。

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
PUBLIC_URL=http://<服务器IP>
MODELPROXY_TOKEN=<token>
MODELPROXY_MODEL=doubao-seedream-4.0
IMAGE_SIZE=
```

token 只存在服务器的 `.env`,不进镜像、不进 git。

### .dockerignore

排除 `node_modules`、`backend/static/posters`、`backend/static/samples`
(当前已积累 12M 海报,无必要进构建上下文)。`dist` 已被 `frontend/.gitignore` 排除,
但本地开发会生成它,所以 `.dockerignore` 里也列一份,避免本地构建时把过时产物送进
上下文。

## 附带修复:图片下载自回环

`service/downloader.go`。

问题:`modelproxy_client.go` 在代理只返回 `b64_json`(无 `url`)时,把图落盘后返回
`PUBLIC_URL/static/samples/...`,接着 `ImageDownloader` 用 HTTP GET 这个地址 ——
容器需要经自己的公网 IP 绕回自身。许多云环境 hairpin NAT 不通,此处会失败。豆包
seedream 通常直接返回 `url`,不一定触发,但触发后排查成本高。

修复:给 `ImageDownloader` 加可选的自地址改写。`Download` 时若目标 URL 前缀命中
`PUBLIC_URL`,改写为 `http://127.0.0.1:<PORT>` 再发请求,不依赖 hairpin。

测试(先写测试):

- `publicURL=http://1.2.3.4`、URL `http://1.2.3.4/static/samples/a.png` → 实际请求
  打到本地回环。
- 外部 URL(如模型 CDN 地址)原样透传,不改写。

## 前置工作

两项在部署前必须完成,否则方案跑不通:

1. **提交 modelproxy 实现**。`backend/service/modelproxy_client.go` 当前 untracked,
   `config/config.go`、`main.go` 的改动未提交 —— 这三者构成接真模型的全部实现。
   服务器 clone 拿不到,只会静默回退 mock。单独提一个 commit。
2. **合并到 main**。前端全部实现只在 `feat/frontend` 分支;`main` 落后 10 个 commit
   且有 2 个未 push。合并 `feat/frontend` → `main` 并 push,服务器跑 `main`。

## 部署流程

```bash
git clone https://github.com/13813427988-design/ai-poster.git && cd ai-poster
cp .env.example .env && vi .env      # 填 IP 和 token
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
   `using modelproxy`(证明真模型生效,而非静默回退 mock)
5. 下载海报**打开查看**:标题文字是否画上 → 验证字体
6. 浏览器访问 `http://<IP>`,走完整生成流程并点击下载按钮

第 5、6 步不可省略。字体解析失败与跨域下载都属于"日志干净但结果错误"的失败模式。

## 明确不做(YAGNI)

- HTTPS / 域名 / 证书自动续期
- 海报自动清理(目前 12M,以后按需加 cron)
- CI / 自动部署
- 多副本 / 负载均衡
- 镜像仓库(在服务器本地 build)
