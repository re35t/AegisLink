# ADR 0015：分离 Agent Harness 与 Eino Runtime

状态：已接受。

日期：2026-08-26。

## 背景

原 Eino 适配器逐步混入 Agent 上下文解析、Prompt 组装、Skill/MCP Tool、选择策略，以及不属于对话执行链的 Impression Curator，导致 Runtime 反向依赖 Conversation、Agent 和 Impression 领域类型，也模糊了“通用 Agent Run”与 AegisLink 产品行为的边界。

## 决策

- `internal/runtime` 只实现一次领域无关的 Eino Agent Run，拥有 Eino/模型供应商类型、通用 Message/Tool、流式事件、迭代限制和通用的每 Run Tool choice。
- `internal/harness` 解析 Principal/Agent 范围上下文、组装指令和授权能力、应用 AegisLink execution policy，并实现 `conversation.Harness` 端口。
- `internal/curator` 使用专用的一轮、无 Tool Runtime 实现 `impression.Curator`。
- Agent 执行方向固定为 `conversation -> harness -> runtime -> Eino`；认知维护方向为 `impression.Worker -> curator -> runtime -> Eino`。
- Runtime 禁止导入 AegisLink 领域包，只有 Runtime 可以直接导入 Eino SDK。

## 后果

Conversation 的持久化与取消继续独立于 Eino；产品能力可以在不扩张 Runtime 契约的情况下演进；Curator 也不会意外获得对话 Tool。
