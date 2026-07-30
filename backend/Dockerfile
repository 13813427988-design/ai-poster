FROM golang:1.25-alpine AS builder

# 部署目标在国内,proxy.golang.org 不可达(dial tcp 142.250.73.113:443: i/o timeout),
# go mod download 会直接失败。用 ARG 而不是硬编码 ENV:在能直连的环境里
# 可以 --build-arg GOPROXY=https://proxy.golang.org,direct 覆盖回官方源。
# 换镜像不牺牲完整性:go.sum 里的哈希对每个模块逐一校验,源不可信也改不了内容。
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /src

# 依赖单独一层:只改代码时命中缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/ai-poster .

FROM alpine:3.21

# 本机到 dl-cdn.alpinelinux.org 实测只有 ~20KB/s,字体包会拖成几十分钟。
# 同上用 ARG,置空(--build-arg APK_MIRROR=)即回落官方 CDN。
# 换镜像同样不牺牲完整性:apk 仍用 /etc/apk/keys 里的官方签名校验每个包。
#
# 改写后必须显式验证生效,不能只跑 sed 就当成了:
# Alpine >= 3.22(apk 3.0)把仓库配置从 /etc/apk/repositories 挪到了
# /etc/apk/repositories.d/,那时这条 sed 会变成静默 no-op,构建悄悄退回
# ~20KB/s 的官方源,只表现为"这次怎么这么慢",没有任何报错。所以下面
# grep 不中就 exit 1,让它响亮地失败。
#
# apk add 的两个包各有原因:
# ca-certificates: 后端要 HTTPS 调 image.pollinations.ai,裸 alpine 无根证书会 x509 失败。
# font-wqy-zenhei: 标题合成需要中文字体,缺失时 PosterComposer 只打一行日志就跳过文字,
#   不报错——海报照样返回,只是没标题。这里不能用 font-noto-cjk ——
#   NotoSansCJK-Regular.ttc 是 CFF/OTTO 轮廓,而 github.com/golang/freetype
#   只认 glyf 轮廓,truetype.Parse 会报 "bad TTF version"
#   (它递归进 ttc 的第一个字体后,sfnt 版本是 'OTTO' 而非 0x00010000)。
#   wqy-zenhei.ttc 是 glyf 轮廓,实测 truetype.Parse 通过且覆盖简中。
ARG APK_MIRROR=mirrors.tencent.com
RUN if [ -n "$APK_MIRROR" ]; then \
        sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
        grep -q "$APK_MIRROR" /etc/apk/repositories || { \
            echo "FATAL: apk mirror rewrite did not take effect." >&2; \
            echo "  '$APK_MIRROR' not found in /etc/apk/repositories after sed." >&2; \
            echo "  Alpine >= 3.22 (apk 3.0) moved repo config to /etc/apk/repositories.d/ —" >&2; \
            echo "  update this rewrite for the new layout, or build with --build-arg APK_MIRROR= to use the official CDN." >&2; \
            exit 1; \
        }; \
    fi \
    && apk add --no-cache ca-certificates font-wqy-zenhei

# 非 root 运行:海报是用户输入驱动写盘的,进程没有理由拿 root。
# 但不能只加一行 USER ——main.go 启动时对 posters/samples 做 MkdirAll,
# 失败会 log.Fatalf 直接退出;/app 归 root 时非 root 用户建不了子目录。
# 所以顺序必须是:建用户 → 预建两个目录 → chown → 再 USER。
RUN addgroup -S -g 10001 app && adduser -S -u 10001 -G app app

WORKDIR /app
COPY --from=builder /out/ai-poster /app/ai-poster
RUN mkdir -p /app/static/posters /app/static/samples \
    && chown -R app:app /app/static

# freetype 支持 .ttc,取集合中第一个字体(truetype/truetype.go:555,557)
# GIN_MODE=release:不设时 gin 跑 debug 模式,会把整张路由表和启动告警打进日志,
# 对外暴露的服务不该主动泄露路由清单,也别让噪音盖住真日志。
ENV FONT_PATH=/usr/share/fonts/wqy-zenhei/wqy-zenhei.ttc \
    STATIC_DIR=/app/static \
    PORT=8080 \
    GIN_MODE=release

USER app

EXPOSE 8080
CMD ["/app/ai-poster"]
