# ADR 0017：将 Composer Discovery 接入 Agent Index

- 状态：Accepted
- 日期：2026-08-28

## 背景

ADR 0012 引入了类型化 Composer 能力菜单，并把 Discovery 初步定义为检查当前 Agent 已启用的 Skills 和 MCP Tools。ADR 0016 随后确立了独立 Agent Index，其 Search 阶段能够返回按相关度排序的不透明 AgentAddr 候选。若 Composer 入口继续执行本地检查，就会与已经实现的 Discovery 链路脱节，也会错误表达该入口的产品含义。

## 决策

- 只取代 ADR 0012 中 `discovery / discover-once` 的行为；保留类型化 Catalog、不透明 Mention ID、AG-UI selection 形状和持久化 Run execution policy。
- Catalog 在 `AegisLink Index` 分组下投影一个名为 `Find related Agents` 的条目，Mention ID 为 `discovery:agent-search`。Agent Index 或 Encoder 未配置时标记为 `server-offline` 并禁止选择。
- `internal/harness` 将该选择解析为首次强制调用 `discover_agents`。
- Tool 只接受一个必填语义 `query`；产品上固定最多返回五个候选，不允许模型选择 Top-K。
- 直接调用注入的 Agent Index 应用服务，不从 Server 回调自身 HTTP 接口。
- 读取当前 Agent 已注册的 AgentAddr，在 Index 上限允许时多取一个候选，保持 Index 排名过滤自身，再截断到请求的 Top-K。
- 只返回 Index 提供的 `agentAddr`、`score`、`matchedVectorId` 和 `representationRevision`。Agent 最终回复必须列出所有返回的 AgentAddr 与分数；空结果必须明确说明未找到；不得推断名称、能力或地址。
- 保留 Owner-scoped REST 资源 `POST /api/v1/agents/{agentId}/discovery/search`，作为同一应用服务的另一个 Adapter。

## 影响

Composer Discovery 现在执行真实的跨 Agent 相似度查询，同时维持现有 Conversation -> Harness -> Runtime 执行方向和持久化 AG-UI Tool 事件。Server 继续负责 Owner 校验和 Query 编码；独立 Index 只负责向量检索。

本阶段止于返回排序后的 AgentAddr 候选。AgentAddr Resolve、远程 AgentFacts 回源、Agent 间通信和独立搜索页面仍不在范围内。
