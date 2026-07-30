# ai-poster backend

最小可跑的海报生成 demo：用户给 `prompt + title`，服务端文生图 + 文字合成 + 返回海报 URL。

## 跑起来

```bash
cd backend
go mod tidy
go run .
```

默认监听 `:8080`。

## 接口

```bash
curl -X POST http://localhost:8080/generate \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"日落海边的渔船","title":"夏日海报"}'
```

返回：

```json
{"url":"http://localhost:8080/static/posters/<uuid>.png"}
```

浏览器打开返回的 URL 看海报。

## 字体

> Docker 镜像已经装好 `font-wqy-zenhei` 并把 `FONT_PATH` 指向它，本节只影响**不用 Docker、直接 `go run .`** 的场景。

合成标题需要字体，默认从 `static/fonts/default.ttf` 读（可用 `FONT_PATH` 指到别处）。

**硬约束：字体必须是 glyf 轮廓，不能是 CFF/OTTO 轮廓。** 本项目用
`github.com/golang/freetype`，它的 `truetype.Parse` 只解析 glyf；遇到 CFF/OTTO
会直接报 `bad TTF version`。注意这个区别在**轮廓格式**，不在文件后缀——`.ttc`
字体集合是可以的（镜像里用的 `wqy-zenhei.ttc` 就是 glyf，freetype 会取集合中第一个字体）。

推荐（都是 glyf，实测可用）：

- [文泉驿正黑 / 微米黑](http://wenq.org/wqy2/)（Debian/Ubuntu: `fonts-wqy-zenhei`，Alpine: `font-wqy-zenhei`）

**不要用**（CFF/OTTO 轮廓，一定 parse 失败）：

- 思源黑体 / Source Han Sans
- Noto Sans CJK（`NotoSansCJK-Regular.ttc` 同样是 OTTO）

⚠️ 字体加载失败或缺失时，PosterComposer 只打一行日志就**跳过标题文字**，接口照样
返回 200 和一张图——所以症状是"海报没标题但没有任何报错"。换字体后记得确认标题真的画上了。

## 配置（环境变量）

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `8080` | 监听端口 |
| `PUBLIC_URL` | `http://localhost:<PORT>` | 拼海报对外 URL 用 |
| `STATIC_DIR` | `static` | 静态资源根目录 |
| `FONT_PATH` | `<STATIC_DIR>/fonts/default.ttf` | TTF 字体路径 |

## 当前状态

- [x] Gin 项目搭建
- [x] `/generate` 接口
- [x] PromptService（包装 prompt 加修饰词）
- [x] AI 模型调用（**Mock**：本地生成纯色渐变占位图；AIClient 接口已留，未来接 OpenAI / 火山只需替换实现）
- [x] 图片下载（http.Client GET）
- [x] 海报文字合成（freetype + image/draw，底部居中白字黑描边）
- [x] 返回海报 URL

## 后续接真模型

实现 `service.AIClient`：

```go
type OpenAIClient struct { /* ... */ }
func (c *OpenAIClient) Generate(ctx context.Context, prompt string) (string, error) { /* ... */ }
```

然后在 `main.go` 把 `service.NewMockAIClient(...)` 替换成 `NewOpenAIClient(...)` 即可。
