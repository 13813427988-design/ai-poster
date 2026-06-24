# ai-poster Frontend — 设计文档

日期：2026-06-24

## 1. 背景

`ai-poster` 后端（Gin）已实现 `POST /generate {prompt, title} -> {url}`，海报图存于 `static/posters/<uuid>.png`。`frontend/` 目录目前为空，仅有 `.gitkeep` 占位。

本设计为 frontend 的首版实现方案，目标是做一个**完整产品形态**的单页 Web 应用：用户输入 prompt + title，调用后端生成海报，保留本地历史，可一键应用预设模板。

## 2. 关键决策

| 决策点 | 选择 | 理由 |
|---|---|---|
| 范围 | 完整产品形态（历史 + 模板 + 下载 + loading + 错误处理） | 用户选择 |
| 技术栈 | React + Vite + TypeScript + Tailwind CSS | 生态成熟、Vite 启动快、TS 类型安全 |
| 布局 | 单页三区块（左表单 / 右预览 / 下历史） | 用户选择，所有要素一屏可见 |
| 状态管理 | `useReducer` + 自写 localStorage hook | YAGNI：只有一个 API、状态集中在 App，引状态管理库过度设计 |
| 预设模板 | 6 个预设按钮，一键填充 `prompt + title` | 用户选择 |
| Dev 后端对接 | Vite proxy `/api → http://localhost:8080` | 无需改后端 CORS |
| Loading 形态 | 预览区 skeleton + spinner | 视觉反馈清晰 |
| 错误处理 | 表单下方 inline 错误条 + toast 不做 | 简单够用 |
| 历史持久化 | localStorage，上限 50 条，可清空 | demo 阶段无需服务端 |
| 测试 | Vitest + React Testing Library，reducer / api / 组件单测 | E2E / 视觉回归不做 |

## 3. 文件结构

```
ai-poster/frontend/
├── package.json
├── vite.config.ts            # dev proxy /api → http://localhost:8080
├── tsconfig.json
├── tailwind.config.js
├── postcss.config.js
├── index.html
└── src/
    ├── main.tsx              # React 入口
    ├── App.tsx               # 唯一容器，持有 reducer 状态与 handlers
    ├── index.css             # Tailwind directives
    ├── types.ts              # GenerateRequest, GenerateResponse, HistoryItem
    ├── api.ts                # generatePoster(req): Promise<GenerateResponse>
    ├── presets.ts            # 6 个预设模板
    ├── reducer.ts            # appReducer + Action 类型 + initialState
    └── components/
        ├── PromptForm.tsx    # 左侧
        ├── Preview.tsx       # 右侧
        └── History.tsx       # 下方
```

## 4. 组件与数据流

`App` 是唯一的"智能"组件，三个子组件均为纯展示组件，由 props 驱动。

```
App (useReducer)
├── PromptForm   props: form, status, presets, onChange, onPresetClick, onSubmit
├── Preview      props: status, current, error
└── History      props: items, onSelect, onClear
```

**生成流程**：
1. 用户填表单或点预设 → `SET_FIELD` / `APPLY_PRESET`
2. 点提交 → `SUBMIT`（status: loading）
3. `api.generatePoster()`
   - 成功 → `SUCCESS`（设置 current，history 头插，超过 50 砍尾）
   - 失败 → `FAILURE`（设置 error）

**点击历史项** → `RESTORE`（把该项恢复到表单和当前预览区，方便再生成或下载）。

**清空历史** → `CLEAR_HISTORY`（带二次确认）。

## 5. 类型与 API

```ts
// types.ts
export type GenerateRequest = { prompt: string; title: string };
export type GenerateResponse = { url: string };
export type HistoryItem = {
  id: string;          // crypto.randomUUID()
  prompt: string;
  title: string;
  url: string;
  createdAt: number;   // Date.now()
};

// api.ts
export async function generatePoster(req: GenerateRequest): Promise<GenerateResponse> {
  const res = await fetch("/api/generate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${await res.text()}`);
  return res.json();
}
```

`vite.config.ts` 的 proxy 配置：

```ts
server: {
  proxy: {
    "/api": {
      target: "http://localhost:8080",
      changeOrigin: true,
      rewrite: (path) => path.replace(/^\/api/, ""),
    },
  },
}
```

后端 `POST /generate` 实际路径不带 `/api` 前缀，rewrite 去掉。后端 URL 不变。

## 6. Reducer / 状态形态

```ts
type Status = "idle" | "loading" | "success" | "error";

type State = {
  form: { prompt: string; title: string };
  current: HistoryItem | null;
  status: Status;
  error: string | null;
  history: HistoryItem[];
};

type Action =
  | { type: "SET_FIELD"; field: "prompt" | "title"; value: string }
  | { type: "APPLY_PRESET"; preset: { prompt: string; title: string } }
  | { type: "SUBMIT" }
  | { type: "SUCCESS"; item: HistoryItem }
  | { type: "FAILURE"; message: string }
  | { type: "RESTORE"; item: HistoryItem }
  | { type: "CLEAR_HISTORY" };

const HISTORY_MAX = 50;

// SUCCESS 时
state.history = [item, ...state.history].slice(0, HISTORY_MAX);
```

`history` 是 reducer state 的一部分。持久化分两步在 `App.tsx` 内完成：

- **初始化**：`useReducer` 的 initial state 通过 lazy initializer 从 localStorage 读取 `history`（key: `ai-poster-history`，JSON 解析失败回退到空数组）。
- **写回**：`useEffect(() => localStorage.setItem(...), [state.history])`，监听 history 变化写回。

不引入独立的 `useLocalStorage` hook，逻辑就两段，写在 App.tsx 即可。`src/hooks/` 目录暂不创建。

## 7. UI 形态（Tailwind）

- **整体**：`min-h-screen bg-slate-50`，容器 `max-w-6xl mx-auto p-6`
- **三区块**：CSS grid，上半部分 `md:grid-cols-2 gap-6`（左表单 / 右预览），下方 `History` 全宽
- **PromptForm**：
  - `title` 输入框，必填，maxLength 30
  - `prompt` textarea，必填，maxLength 200，显示字符计数
  - 预设按钮区：6 个芯片按钮，横向 wrap，点击填充 form
  - 提交按钮：loading 时禁用并显示 spinner
- **Preview**：
  - idle 空态：灰色虚线框 + 提示文字
  - loading：skeleton 块（脉冲动画）+ 居中 spinner
  - success：海报图 + 标题/prompt + 下载按钮（`<a download>` 直链）
  - error：红色背景 inline 错误条
- **History**：
  - 网格 `grid-cols-4 md:grid-cols-6 gap-3`
  - 每张缩略图 hover 显示 prompt/title tooltip
  - 右上角 "清空" 按钮，点击二次确认弹窗

## 8. 预设模板

```ts
// presets.ts
export const PRESETS = [
  { label: "夏日海报", prompt: "夏日海边日落，渔船与晚霞", title: "夏日海报" },
  { label: "赛博朋克", prompt: "雨夜霓虹城市，赛博朋克街景", title: "Cyber City" },
  { label: "复古海报", prompt: "70 年代复古风海报，胶片颗粒感", title: "Retro Vibes" },
  { label: "极简风", prompt: "极简几何线条，纯色背景", title: "Minimal" },
  { label: "国风", prompt: "水墨山水，远山近水", title: "山水有相逢" },
  { label: "节日", prompt: "新年红色喜庆，烟花夜空", title: "新春快乐" },
];
```

实现期内容可以再调，结构（label/prompt/title）是确定的。

## 9. 错误处理

| 错误类型 | 表现 |
|---|---|
| 网络失败 / HTTP 4xx/5xx | `FAILURE` action，预览区改为红色错误条，显示 `error.message` |
| 表单校验失败（空 prompt 或空 title） | 提交按钮 disabled，无法点击发起请求 |
| localStorage JSON 解析失败 | 静默回退到空数组（在 reducer initializer 内 try/catch） |
| 后端返回非 JSON | api 层 `await res.json()` 抛错，落入网络失败分支 |

## 10. 测试策略

| 类型 | 范围 | 工具 |
|---|---|---|
| Reducer 纯函数 | 每个 action 至少一个用例，覆盖 history 头插、超长截断 | Vitest |
| API 层 | mock fetch，覆盖 200 / 4xx / 5xx 三种返回 | Vitest + msw（如已引入）或手 mock |
| 组件 | `PromptForm` 提交/校验/预设点击；`History` 渲染与清空 | React Testing Library |

**不做**：E2E（Playwright/Cypress）、视觉回归、Storybook。

## 11. YAGNI 明示（这版不做的事）

- 用户系统 / 鉴权
- 多模型选择 / 模型参数
- 历史搜索 / 筛选 / 标签
- 服务端历史持久化
- i18n（仅中文）
- 暗黑模式
- 移动端适配（桌面优先，能用即可）
- PWA / Service Worker

## 12. 实施顺序提示

按依赖从下往上：
1. 脚手架（Vite + TS + Tailwind 三件套）+ `vite.config.ts` proxy
2. `types.ts` + `api.ts` + `api.test.ts`
3. `reducer.ts` + `reducer.test.ts`
4. `presets.ts`
5. `components/PromptForm.tsx` + `History.tsx` + `Preview.tsx`（无状态，可并行）
6. `App.tsx`（组装 + localStorage 持久化）
7. 联调：跑通完整流程，验证 dev proxy

具体步骤拆分由 `writing-plans` skill 在下一步产出。
