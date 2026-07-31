# 部署

一台 x86_64 Linux 服务器,装好 Docker(自带 `docker compose` v2),能访问 GitHub 与
`image.pollinations.ai`。整套栈两个容器:`nginx`(前端静态资源 + 反代,对外 80)和
`backend`(Go,不映射端口,只在 compose 网络内可达)。

## 首次部署

```bash
git clone https://github.com/13813427988-design/ai-poster.git
cd ai-poster
cp .env.example .env
vi .env                      # 至少要把 PUBLIC_URL 改成 http://<你的服务器IP>

# 建目录并把 owner 改成容器内的 uid —— 这一步不能省,原因见下
mkdir -p data/posters data/samples
docker run --rm -v "$PWD/data:/data" alpine:3.21 chown -R 10001:10001 /data

docker compose up -d --build
```

首次构建要拉 4 个基础镜像(golang / node / alpine / nginx),在国内机器上这段可能比编译本身慢,
耐心等。构建峰值内存实测约 500MB,1.9G 的小机器够用。

完成后浏览器打开 `http://<你的服务器IP>`。自检:

```bash
docker compose ps            # 两个容器都是 running,backend 是 healthy
curl -sf http://localhost/   # 前端 200
docker compose logs backend | grep -E "provider|font|listening"
```

最后一条**别写成 `| tail -20`**:healthcheck 每 10s 打一行 `GET "/healthz"`,
跑了 200 秒之后 tail 窗口里就只剩 healthz 噪音,`provider=` 和 `listening on` 早被顶出去了 ——
自检看着"没有那两行",实际只是被冲掉了。用 `grep` 按关键字捞,任何运行时长都成立。
（泛用排查还是 `docker compose logs backend --tail 50`。）

### 为什么必须 chown 10001

后端进程以非 root 的 uid 10001 运行,而 `./data/{posters,samples}` 是宿主目录 bind mount 进去的,
挂载会把宿主的 owner 原样带进容器,盖掉镜像里已经 chown 过的 `/app/static/*`。

这个错配**不会自己暴露**:目录是存在的,`MkdirAll` 直接返回 nil,`/healthz` 照样 200,
healthcheck 也是 healthy —— 只是每次 `POST /generate` 都失败在 `permission denied`。
一个"健康但干不了正事"的栈,不知道的话能查很久。

所以启动时加了一次真实写入探测:写不进就直接 `log.Fatalf` 并 crash-loop。
**如果 backend 启动时报"目录不可写",就是漏了这一步,跑上面那条 `docker run ... chown` 即可。**
它借的是 docker 自己的 root,宿主不需要 sudo。

### 起不来:80 端口被占用,或机器上已有一套旧部署

`up -d` 报 `Error starting userland proxy: listen tcp4 0.0.0.0:80: bind: address already in use`
时,先查是谁占着 80:

```bash
sudo ss -ltnp | grep :80          # 看进程名/PID(不加 sudo 只能看到在听,看不到是谁)
docker ps --filter publish=80     # 如果是容器,这条直接给出容器名
docker compose ls                 # 本机所有 compose 项目及其 compose 文件路径
```

如果占用者是**同一个项目的旧部署**(比如上次装在别的目录),必须**回到它自己的目录**去停:

```bash
cd /path/to/旧部署目录 && docker compose down
```

`docker compose down` 是按 **project** 生效的,而 project 名默认取**所在目录名**。
在新目录里执行 `down` 停的是新 project,旧那套照样占着 80 —— 会表现成"我明明 down 过了还是端口冲突"。
`docker compose ls` 输出里的 CONFIG FILES 一列就是各 project 的目录,照着 `cd` 过去即可。

占用者不是 docker(比如宿主自带的 nginx/apache)就停掉它,或者改 compose 里 nginx 的对外端口 ——
**改了端口记得同步改 `.env` 里的 `PUBLIC_URL`**(见下文)。

## 更新

```bash
git pull
docker compose up -d --build
```

只有变化的镜像层会重建。依赖清单没动时(`go.mod`/`package-lock.json` 不变)命中缓存,很快。

只改 `.env`(比如换 `AI_PROVIDER`、改 `PUBLIC_URL`)不需要重建:

```bash
docker compose up -d
```

### 不用 git 而是从本地同步代码时

`rsync` 必须排除 `.env`,否则本地的占位配置会覆盖服务器上的真实配置:

```bash
rsync -av --delete --exclude .env --exclude .git --exclude data/ ./ user@server:/opt/ai-poster/
```

## 构建只能用 docker compose build

用 `docker compose build` / `docker compose up -d --build`。**不要用裸 `docker build`** ——
部署机没装 buildx CLI 插件,`DOCKER_BUILDKIT=1 docker build` 会直接失败。
Compose 2.40.3 是把 buildx 作为库内嵌进去的,所以 `docker compose build` 不受影响。

### 国内镜像源(可覆盖)

两个 Dockerfile 默认走国内源,因为部署机**完全连不上** `proxy.golang.org`,
`dl-cdn.alpinelinux.org` 实测只有 ~20KB/s,npm 官方源 ~105KB/s(一个 4MB 的包要 40s,
锁文件里有 243 个包)。表现是"卡住"而不是报错,所以默认值给的是能用的那个。

换源不牺牲完整性:go.sum 逐模块校验哈希,apk 用 `/etc/apk/keys` 里的官方签名校验每个包,
`npm ci` 按 lockfile 里的 sha512 逐包校验。源不可信也改不了内容。

在能直连的环境构建,用 ARG 覆盖回官方源:

```bash
docker compose build \
  --build-arg GOPROXY=https://proxy.golang.org,direct \
  --build-arg APK_MIRROR= \
  --build-arg NPM_REGISTRY=https://registry.npmjs.org
```

（`APK_MIRROR` 置空即回落官方 CDN。）镜像改写失败时构建会显式 `exit 1` 报错,
不会悄悄退回慢源。

## 配置

### PUBLIC_URL(必填,最容易配错)

必须是**浏览器可达的绝对地址**,例如 `http://124.220.39.227`,或你的域名。

后端返回给前端的海报 URL 是用它拼出来的绝对地址,浏览器(以及下载按钮)会原样去取。
写成 `localhost`、容器名或内网地址,页面本身能打开,但海报链接在浏览器里打不开。
如果 nginx 映射到了 80 以外的端口,`PUBLIC_URL` 也要带上那个端口。

### AI_PROVIDER

| 值 | 说明 |
|---|---|
| `pollinations` | 默认。免费匿名,无需凭证,公网可达 |
| `modelproxy` | 需要 `MODELPROXY_TOKEN`,**且 endpoint 要容器内可达**。代码里的默认 endpoint 是 10.x 内网地址,公网 VPS 根本路由不到,必须显式配 `MODELPROXY_ENDPOINT`。token 缺失会启动即失败 |
| `mock` | 本地渐变占位图,不出网。用于排查"是不是外部服务的问题" |

改这一项只要编辑 `.env` 后 `docker compose up -d`,不需要重建镜像。

### 字体:不要覆盖 FONT_PATH

镜像装的是 `font-wqy-zenhei`,`FONT_PATH` 已在 Dockerfile 里指好,别在 compose 或 `.env` 里改。

原因:合成标题用的 freetype 只认 **glyf** 轮廓,遇到 CFF/OTTO 轮廓会解析失败。
所以 **Noto Sans CJK / Source Han Sans 用不了**,而且是**静默失败** —— 海报照样返回、照样 200,
只是没有标题文字,日志里只有一行提示。判断标准是轮廓格式而不是扩展名:
`.ttc` 本身没问题,`wqy-zenhei.ttc` 实测可用。

## 日常运维

### 单独重建 backend 之后必须重启 nginx

nginx.conf 里 `proxy_pass` 写的是字面主机名 `backend`,nginx 在**读配置时解析一次并永久缓存那个 IP**。
单独重建/重启 backend 会让它换一个容器 IP,nginx 仍指向已经死掉的旧地址,对外全是 502。

```bash
docker compose up -d --build backend
docker compose restart nginx      # 这行不能省
```

完整的 `docker compose up -d --build` 不需要这步(nginx 一起重建了)。

### 重启机器之后 nginx 会 flap 几秒

`depends_on: condition: service_healthy` 只在 `docker compose up` 这条路径上生效。
VPS 重启或 `systemctl restart docker` 之后,是 `restart: unless-stopped` 把两个容器拉起来的,
顺序不确定。nginx 若先起,会因为解析不到 `backend` 而硬退出(`host not found in upstream`),
然后按重启策略反复重启,直到 backend 起来。最终自愈,中间会 flap 几秒。
看到 nginx 重启计数涨了、期间 502,这是已知行为,不是故障。

### 日志

两个容器的 json-file 日志都封了顶(10MB × 3),不会写满磁盘。

```bash
docker compose logs -f backend
docker compose logs --tail=100 nginx
```

## 排查

| 现象 | 检查 |
|---|---|
| nginx 日志里 `host not found in upstream "backend"` | **是 backend 挂了**,先看 `docker compose logs backend`。这个错报在 nginx 上,但根因几乎总在 backend(启动即崩,常见是目录不可写) |
| 刚单独重建过 backend,然后全站 502 | `docker compose restart nginx`,上游 IP 缓存过期了 |
| 栈全是 healthy,但每次生成都报 permission denied | 漏了 `chown 10001:10001 data`,见上文 |
| 页面能开,海报链接在浏览器里打不开 | `.env` 里 `PUBLIC_URL` 写错(localhost / 内网地址 / 缺端口) |
| 海报出来了但没有标题文字 | 字体解析失败,`docker compose logs backend` 找字体相关日志;确认没有覆盖 `FONT_PATH` |
| 生成在 170s 左右超时 | 后端自己的单请求总预算(170s),nginx 兜底是 180s。多为上游生图太慢,换 `AI_PROVIDER=mock` 可确认是否外部原因 |
| 前端能开但生成报错 | `docker compose logs backend`,错误信息带阶段(生图 / 下载 / 合成) |
| 同一描述总出同一张图 | 已由每请求随机 seed 规避;若仍复现,确认不是 `AI_PROVIDER=mock` |
| backend 一直 unhealthy | `docker compose logs backend`;大概率是 `config:` 校验失败(比如 `modelproxy` 缺 token) |
| `up -d` 报 `address already in use`(80) | 端口被别的进程或旧部署占着,见上文"起不来:80 端口被占用";注意旧部署要**在它自己的目录里** `docker compose down` |
| 外部监控探活报服务 down,但浏览器正常 | 探针用了 **HEAD**。后端只注册了 `GET /healthz`,`HEAD /healthz` 返回 **404**。把监控改成 GET 即可(compose 自带的 healthcheck 用的是 wget GET,不受影响) |
| 自检看不到 `provider=` / `listening` 那两行 | 别用 `logs backend \| tail -20`,healthz 每 10s 一行会把它们顶出窗口;用 `docker compose logs backend \| grep -E "provider\|font\|listening"` |

## 已知限制

- **pollinations 无 SLA**。免费匿名服务,可能限流或抖动。匿名调用还有分辨率上限:
  请求 `1024x1536` 实返约 627x940。相同 prompt + seed 会返回字节完全相同的图,
  所以每次请求都注入了随机 seed。
- **海报不自动清理**。`data/posters` 会持续增长,没有回收机制。在意磁盘就加个 cron:

  ```cron
  30 4 * * * docker run --rm -v /opt/ai-poster/data:/data alpine:3.21 find /data/posters -type f -mtime +7 -delete
  ```

  (`-v` 用绝对路径,cron 里没有可用的 `$PWD`;把 `/opt/ai-poster` 换成你实际的 compose 目录。)

  **别"简化"成宿主直接 `find /opt/ai-poster/data/posters -mtime +7 -delete`** —— 那条跑不通。
  上文要求把 `data/` chown 给 uid 10001(容器内的用户),宿主用户(通常 uid 1000)于是对这个目录
  没有写权限,unlink 不掉里面的文件。恶心的是它**今天还静默成功**:目录里暂时没有超过 7 天的文件,
  find 无事可做,退出码 0,看着一切正常。等海报真的老过 7 天,才开始每个文件报一行
  `Permission denied` 到没人看的 cron 日志里,而磁盘继续涨 —— 正是本文反复警告的
  "看着没问题、其实啥也没干"那一类。
  借 docker 的 root 是本文档一贯的做法(见上面的 chown 那步),宿主不需要 sudo。
  操作者更习惯 sudo 的话,`sudo find /opt/ai-poster/data/posters -type f -mtime +7 -delete` 同样可行。
- **仅 HTTP,无 TLS**。要 HTTPS 得在前面再加一层终止(或改 nginx.conf 挂证书)。
- 部署机 1.9G 内存,构建能跑(峰值约 500MB),但首次拉基础镜像慢。
