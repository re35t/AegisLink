# AegisLink

AegisLink 是一个本地优先的个人 Agent Web 应用。当前版本采用易读的 Go 模块化单体、基于 `assistant-ui` 的 React 聊天界面、AG-UI 执行协议、进程内 Eino ReAct Agent Runtime、可扩展模型 Provider、Cookie 账户会话，以及由 PostgreSQL 持久化的对话和可回放 Run 事件。当前默认 Provider 使用 DeepSeek API。

## 技术栈

- Go 1.26、Gin、Eino ADK
- PostgreSQL、pgx、Goose migrations
- React 19、Vite、assistant-ui、AG-UI、TanStack Router、TanStack Query
- OpenAPI 生成前端类型

## Agent Runtime

- 执行方向固定为 `Conversation -> Harness -> Runtime -> Eino`。`internal/runtime` 只实现领域无关的 Eino Agent Run，不依赖 AegisLink 领域包。
- `internal/harness` 负责 Agent 上下文、Instruction、选择策略与授权 Tool 装配；Eino `ChatModelAgent` 最多执行 `AGENT_MAX_ITERATIONS` 轮模型与工具循环，默认 8 轮。
- Harness 每 Run 提供 `get_current_time`、渐进式 Skill 加载、Discovery 与已授权的只读 MCP Tool。
- `internal/curator` 使用独立的一轮、无 Tool Runtime 维护 Impression 与待确认 Fact Candidate。
- `ModelRegistry` 根据 `MODEL_DRIVER` 解析模型 Provider；当前注册 `deepseek` 和 `openai-compatible`，Conversation/Harness 层不依赖任何模型 SDK。
- `MODEL_ID` 是稳定的模型配置标识，后续可在不改变 Conversation 业务接口的情况下扩展多个模型配置。

## Web 与 AG-UI

- 普通 REST API 继续管理 Conversation、历史和 Run 控制；`POST /api/v1/ag-ui` 接收标准 `RunAgentInput` 并返回 AG-UI SSE 事件。
- assistant-ui 的 Thread、Message、Composer、停止生成和自动滚动已经接入，`@assistant-ui/react-ag-ui` 负责协议运行时。
- PostgreSQL 历史是权威数据；AG-UI 请求携带的旧消息不会替代服务端会话历史。
- 当前 AG-UI 切片覆盖 Run 生命周期、文本流和取消。结构化 Tool UI、Approval、附件和 AG-UI 原生断线续流仍是后续工作。

## 账户与 Personal Agent

- 注册会在同一个 PostgreSQL 事务中创建登录 Account、对应的 Human Principal（API 中的 `User`）和一个默认 Personal Agent。
- 登录返回当前 User 与 Personal Agent，并创建服务端不透明 Session。浏览器只获得 `HttpOnly`、`SameSite=Strict` Cookie，数据库只保存 Session Token 的哈希。
- 密码使用 Argon2id 哈希；现有 Agent、Conversation、Message、Run 和 Event 操作都按登录 Principal 做所有权隔离。
- 本地 HTTP 开发使用 `AUTH_COOKIE_SECURE=false`；生产 HTTPS 环境必须设为 `true`。

当前 DeepSeek 配置示例：

```dotenv
MODEL_ID=deepseek-primary
MODEL_DRIVER=deepseek
MODEL_BASE_URL=https://api.deepseek.com
MODEL_NAME=deepseek-v4-flash
AGENT_MAX_ITERATIONS=8
```

API key 只通过未提交的 `.env` 中 `MODEL_API_KEY` 提供，不能写入文档、日志或代码。

`CURATOR_MODEL_*` 可选，未设置的 Provider/模型字段会逐项回退到 `MODEL_*`。Curator 默认继续使用轻量的 `deepseek-v4-flash`，启用 JSON Output，并通过 `CURATOR_MODEL_THINKING=disabled` 关闭思考模式，避免结构化后台任务把输出预算消耗在 reasoning 上；只有实际证据表明需要时才改为 `enabled`。

## 本地运行

```bash
cp .env.example .env
# 填写 MODEL_API_KEY，按需调整 MODEL_BASE_URL 和 MODEL_NAME。
pnpm install
make dev-db
make dev-server
```

另开终端运行：

```bash
make dev-web
```

浏览器打开 `http://127.0.0.1:5173`，API 默认监听 `http://127.0.0.1:4321`。

首次打开会进入注册/登录页。注册成功后会自动进入与该账户默认 Personal Agent 绑定的工作区。

如果通过 OneAPI 等 OpenAI-compatible 网关调用 DeepSeek：

```dotenv
MODEL_DRIVER=openai-compatible
MODEL_BASE_URL=https://your-gateway.example/v1
MODEL_NAME=your-deepseek-model
```

## 验证

```bash
make generate
make check
make test
make build
```

设置 `TEST_DATABASE_URL` 后，`go test ./...` 会同时运行 PostgreSQL Repository 集成测试。

## API

- `POST /api/v1/auth/register`、`POST /api/v1/auth/login`、`POST /api/v1/auth/logout`
- `GET /api/v1/auth/session`
- `GET /api/v1/bootstrap`、`GET /api/v1/agents`
- `POST /api/v1/ag-ui`
- Conversation、Message、Run 与 SSE Event 接口详见 `contracts/http/v1/openapi.yaml`

## 当前边界

当前版本已实现邮箱密码登录、服务端 Session、Principal 拥有 Personal Agent，以及现有聊天链路的所有权隔离。邮箱验证、密码重置、MFA、登录限流、委托访问、Ed25519 Agent Identity、Capability、跨 Agent 通信、RAG、MCP/外部 Tools、Tool/Approval UI、Redis、WebSocket 和沙箱 Runner 尚未实现。`docs` 中保留了旧原型的研究文档，但当前实现以本 README、ADR 0004、ADR 0005 和 ADR 0006 为准。
