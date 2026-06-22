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

合成标题需要 TTF 字体，放到 `static/fonts/default.ttf`。建议：

- [思源黑体](https://github.com/adobe-fonts/source-han-sans)
- [文泉驿微米黑](http://wenq.org/wqy2/)

字体缺失时合成会自动跳过文字，只输出 AI 生成的背景图（不会报错）。

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
