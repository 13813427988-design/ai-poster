FROM golang:1.25-alpine AS builder

# 部署目标在国内,proxy.golang.org 不可达(dial tcp 142.250.73.113:443: i/o timeout),
# go mod download 会直接失败。用 ARG 而不是硬编码 ENV:在能直连的环境里
# 可以 --build-arg GOPROXY=https://proxy.golang.org,direct 覆盖回官方源。
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
# 同上用 ARG,置空即回落官方 CDN。
ARG APK_MIRROR=mirrors.tencent.com
RUN if [ -n "$APK_MIRROR" ]; then \
        sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
    fi \
# ca-certificates: 后端要 HTTPS 调 image.pollinations.ai,裸 alpine 无根证书会 x509 失败
# font-wqy-zenhei: 标题合成需要中文字体,缺失时 PosterComposer 会静默跳过文字。
# 这里不能用 font-noto-cjk —— NotoSansCJK-Regular.ttc 是 CFF/OTTO 轮廓,
# 而 github.com/golang/freetype 只认 glyf 轮廓,truetype.Parse 会报
# "bad TTF version"(它递归进 ttc 的第一个字体后,sfnt 版本是 'OTTO' 而非 0x00010000)。
# wqy-zenhei.ttc 是 glyf 轮廓,实测 truetype.Parse 通过且覆盖简中。
    && apk add --no-cache ca-certificates font-wqy-zenhei

WORKDIR /app
COPY --from=builder /out/ai-poster /app/ai-poster

# freetype 支持 .ttc,取集合中第一个字体(truetype/truetype.go:542,557)
ENV FONT_PATH=/usr/share/fonts/wqy-zenhei/wqy-zenhei.ttc \
    STATIC_DIR=/app/static \
    PORT=8080

EXPOSE 8080
CMD ["/app/ai-poster"]
