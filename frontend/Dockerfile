FROM node:22-alpine AS builder
WORKDIR /src

# 部署目标在国内。实测本机拉 registry.npmjs.org 的 typescript-5.7.2.tgz(4.2MB)
# 用了 40.4s(~105KB/s),而 registry.npmmirror.com 同一个包 0.35s(~12MB/s),
# 相差两个数量级。这个锁文件有 243 个包,走官方源足够把 npm ci 拖到几十分钟,
# 且表现为"卡住"而不是报错。同 backend/Dockerfile 的 GOPROXY,用 ARG 而不是
# 硬编码:能直连的环境里 --build-arg NPM_REGISTRY=https://registry.npmjs.org
# 即可回官方源,置空(--build-arg NPM_REGISTRY=)则用镜像默认配置。
#
# 换源不牺牲完整性:package-lock.json 为全部 243 个包各记了 integrity
# (sha512),npm ci 逐包校验,源不可信也改不了内容 —— 与 go.sum 同理。
#
# 改写后必须显式验证生效,不能设完就当成了:npm 对 registry 值做规范化
# (可能补尾斜杠),写错的 key 或被 /root/.npmrc、NPM_CONFIG_* 环境变量覆盖时
# npm config set 不会报错,构建只会悄悄退回 ~105KB/s 的官方源。所以下面对
# npm config get registry 的实际返回值做断言,不匹配就 exit 1,让它响亮地失败。
ARG NPM_REGISTRY=https://registry.npmmirror.com
RUN if [ -n "$NPM_REGISTRY" ]; then \
        npm config set registry "$NPM_REGISTRY"; \
        actual="$(npm config get registry)"; \
        if [ "$actual" != "$NPM_REGISTRY" ] && [ "$actual" != "$NPM_REGISTRY/" ]; then \
            echo "FATAL: npm registry override did not take effect." >&2; \
            echo "  wanted: '$NPM_REGISTRY' (or with trailing slash)" >&2; \
            echo "  actual: '$actual'" >&2; \
            echo "  Something is overriding it (a baked-in .npmrc or an NPM_CONFIG_REGISTRY env)." >&2; \
            echo "  Fix the override, or build with --build-arg NPM_REGISTRY= to use npm's default." >&2; \
            exit 1; \
        fi; \
    fi

# 依赖单独一层:只改代码时命中缓存
COPY package.json package-lock.json ./
RUN npm ci

COPY . .
# build = tsc -b && vite build,类型错误会让构建失败(预期行为)
RUN npm run build

FROM nginx:alpine
COPY --from=builder /src/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
