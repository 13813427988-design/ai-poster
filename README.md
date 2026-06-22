# ai-poster

最小可跑的 AI 海报生成 demo：用户给 `prompt + title`，服务端 → 文生图（mock）→ 文字合成 → 返回海报 URL。

## 仓库结构

```
ai-poster/
├── backend/       Go (Gin) 后端，文生图 + 合成 + 静态资源对外
└── frontend/      （占位，暂未实现）
```

## 快速开始

```bash
cd backend
go mod tidy
go run .
```

```bash
curl -X POST http://localhost:8080/generate \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"日落海边","title":"夏日海报"}'
```

详见 [backend/README.md](backend/README.md)。
