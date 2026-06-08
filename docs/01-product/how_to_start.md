# 基础对话后端实现路线

## 目标

在当前 AegisLink 代码基础上，搭建一个可调用 Model API 的基础对话能力：

- 对外提供一个后端对话接口，例如 `POST /api/chat`.
- 复用已有 `LocalAgentClient.ask` 流程：检索本地记忆、组装 prompt、调用 LLM、写入 JSONL conversation。
- 将 CLI 里的 `EchoLlm` 替换为真实 model provider adapter。
- 保持 Phase 1 的本地优先形态，不急于引入数据库、鉴权、流式传输和复杂 agent routing。

## 当前代码基础

当前仓库已经具备对话链路的大部分骨架：

- `packages/agent-client/src/agent-client.ts`
  - `LocalAgentClient.ask()` 已完成核心流程：
    - 根据问题检索 memory；
    - 将 memory 和用户问题组装成 prompt；
    - 调用 `LlmClient.complete()`；
    - 把 user/assistant turn 追加到 memory store；
    - 返回 `answer`、`conversationId`、`sources`。
- `packages/agent-client/src/types.ts`
  - 已定义 `LlmClient`、`AskInput`、`AskResult` 等接口。
- `apps/agent-cli/src/commands/ask.ts`
  - 已演示如何装配 `CompositeMemoryStore`、`JsonConversationStore`、`SoulMdStore` 和 `LocalAgentClient`。
  - 当前使用 `EchoLlm`，这是接真实 Model API 时最直接需要替换的点。
- `apps/server/src/http.ts`
  - 已有原生 Node HTTP server 和 `/api/*` 路由分发方式。
- `apps/server/src/aegislink-server.ts`
  - 当前主要负责 agent registry、capability、message routing、audit，不建议把 model 调用逻辑直接塞进这个类。

## 推荐架构

基础对话能力建议分成三层：

```text
HTTP API
  -> ChatService
    -> LocalAgentClient
      -> MemoryStore
      -> ModelLlmClient
```

### 1. Model Provider Adapter

新增一个实现 `LlmClient` 的 provider，例如：

```text
packages/agent-client/src/model-llm-client.ts
```

职责：

- 从配置读取：
  - `MODEL_API_KEY`
  - `MODEL_BASE_URL`
  - `MODEL_NAME`
  - `MODEL_TIMEOUT_MS`
- 将 `LlmClient.complete({ prompt })` 转换为具体 Model API 请求。
- 统一处理错误：
  - 缺少 API key；
  - provider 返回非 2xx；
  - 超时；
  - 响应结构不符合预期。
- 返回纯文本 answer，保持 `LocalAgentClient` 暂时不用关心 provider 细节。

第一版可以先只支持非流式文本返回。流式输出后面再扩展，不要一开始就把 HTTP response、SSE、provider stream 都绑在一起。

### 2. ChatService

在 server 侧新增轻量服务，例如：

```text
apps/server/src/chat-service.ts
```

职责：

- 根据请求里的 `agentId` 或默认值选择本地 agent。
- 复用 CLI 的 memory 装配方式：
  - `data/agents/{agentId}/conversations.jsonl`
  - `data/agents/{agentId}/soul.md`
- 创建或复用 `LocalAgentClient`。
- 调用 `client.ask()`。
- 返回 API 层需要的结构。

建议不要把 `ChatService` 放进 `AegisLinkServer` 内部。`AegisLinkServer` 当前更偏控制平面，负责注册、权限、路由、审计；基础对话是 agent runtime 能力，单独放置后续更容易替换成本地 worker、远程 agent 或队列任务。

### 3. HTTP Endpoint

在 `apps/server/src/http.ts` 增加：

```text
POST /api/chat
```

请求体建议：

```json
{
  "question": "What does my agent remember?",
  "agentId": "local-agent",
  "conversationId": "local",
  "useMemory": true
}
```

响应体建议：

```json
{
  "answer": "...",
  "conversationId": "local",
  "sources": [
    {
      "id": "...",
      "sourceType": "soul_md",
      "content": "...",
      "score": 0.81,
      "metadata": {}
    }
  ]
}
```

第一版只做同步 JSON 返回即可。

## 实现步骤

### Step 1: 扩展 LLM 抽象

当前 `LlmClient.complete({ prompt })` 足够跑通最小版本。建议先保留这个接口，最多补充可选字段：

```ts
export interface LlmClient {
  complete(input: {
    prompt: string;
    temperature?: number;
    maxTokens?: number;
  }): Promise<string>;
}
```

如果后续要支持多轮 messages、tool calls、streaming，再新增更丰富的接口，不要在第一版提前扩大复杂度。

### Step 2: 新增真实 Model adapter

新增 `ModelLlmClient`，实现 `LlmClient`。

第一版推荐配置：

```text
MODEL_API_KEY=...
MODEL_BASE_URL=https://api.openai.com/v1
MODEL_NAME=...
MODEL_TIMEOUT_MS=30000
```

实现建议：

- 使用 Node 22 内置 `fetch`，避免新增依赖。
- 使用 `AbortController` 做超时。
- provider 错误不要直接吞掉，转换成清晰的 `ModelProviderError`。
- 不在日志中输出 API key、完整 prompt 或用户隐私内容。

### Step 3: 抽出本地 agent 装配函数

CLI 和 server 都需要构建同样的本地 agent client，建议新增共享工厂：

```text
packages/agent-client/src/create-local-agent-client.ts
```

或者先在 `apps/server/src/chat-service.ts` 内部实现，等 CLI 也接真实 model 时再抽到 package。

第一版保守选择：先放 `apps/server/src/chat-service.ts`，减少跨 package 改动。

### Step 4: 新增 ChatService

`ChatService` 输入：

```ts
interface ChatRequestInput {
  question: string;
  agentId?: string;
  conversationId?: string;
  useMemory?: boolean;
}
```

行为：

- 默认 `agentId = "local-agent"`。
- 默认 `dataDir = data/agents/{agentId}`。
- 使用 `CompositeMemoryStore` 组合：
  - `JsonConversationStore`
  - `SoulMdStore`
- 注入 `ModelLlmClient`。
- 调用 `LocalAgentClient.ask()`。

### Step 5: 挂到 HTTP server

调整 `createAegisLinkHttpServer` options：

```ts
export interface CreateHttpServerOptions {
  service: AegisLinkServer;
  chat?: ChatService;
  publicDir?: string;
}
```

然后在 `handleApi()` 中增加 `POST /api/chat`。

校验规则：

- `question` 必须是非空字符串。
- `agentId`、`conversationId` 可选但必须是非空字符串。
- `useMemory` 可选，必须是 boolean。
- 如果未配置 `chat`，返回 501 或在启动时直接创建默认 ChatService。更推荐启动时创建，避免 endpoint 半可用。

### Step 6: 更新启动配置

在 `apps/server/src/main.ts` 中：

- 读取 model 相关环境变量。
- 创建 `ModelLlmClient`。
- 创建 `ChatService`。
- 传入 `createAegisLinkHttpServer({ service, chat })`。

开发期可以保留 fallback：

```text
MODEL_PROVIDER=echo
```

但真实调用路线里，缺少 `MODEL_API_KEY` 时应明确报错，避免以为已经在调用真实 model。

### Step 7: 测试

至少补三类测试：

- `ModelLlmClient`
  - mock `fetch` 成功返回；
  - provider 非 2xx；
  - 超时或网络失败。
- `ChatService`
  - 注入 fake LLM，确认 answer 返回；
  - 确认 conversation 会写入 JSONL；
  - 确认 `useMemory: false` 时不返回 sources。
- `HTTP /api/chat`
  - 正常请求返回 200；
  - 缺少 question 返回 400；
  - model provider 报错时返回可理解错误。

## 第一版验收标准

完成后应能用下面方式启动：

```bash
MODEL_API_KEY=... MODEL_NAME=... pnpm run dev:server
```

然后调用：

```bash
curl -sS http://127.0.0.1:4321/api/chat \
  -H "content-type: application/json" \
  -d '{
    "agentId": "local-agent",
    "conversationId": "local",
    "question": "What does my agent remember?"
  }'
```

应看到：

- 返回真实 model 生成的 `answer`。
- 返回与 `soul.md` 或历史 conversation 相关的 `sources`。
- `data/agents/local-agent/conversations.jsonl` 新增 user 和 assistant 两条记录。
- `pnpm check` 和 `pnpm test` 通过。

## 后续扩展

基础 JSON 对话跑通后，再考虑：

- `POST /api/chat/stream`：SSE 流式响应。
- 多 provider 支持：OpenAI-compatible、Anthropic-compatible、本地模型。
- 把 prompt builder 从 `LocalAgentClient` 中抽出，便于测试和多 agent persona。
- 引入 conversation 查询接口，例如 `GET /api/conversations/:conversationId`。
- 将本地 JSONL conversation 迁移或同步到 PostgreSQL。
- 接入 audit：记录 model 调用开始、结束、失败，但不要记录完整 prompt 和敏感 answer。
- 接入 permission：当远程 agent 或组织用户访问 `/api/chat` 时，再做 capability 检查。

## 推荐提交顺序

1. 新增 `ModelLlmClient` 和对应单元测试。
2. 新增 `ChatService`，先用 fake LLM 测通 memory + ask。
3. 增加 `POST /api/chat`。
4. 在 `main.ts` 接入环境变量配置。
5. 更新 README 或 local-dev 文档，补充启动和 curl 示例。

