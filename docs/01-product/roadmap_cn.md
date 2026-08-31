# 产品 Roadmap

## 已实现基线

- Go/Gin 模块化单体、PostgreSQL/GORM/Goose、React/Vite、AG-UI 与 assistant-ui。
- Account Session、一个默认 Personal Agent、持久化 Conversation/Run/Event、取消、回放和中断 Run 恢复。
- 显式 Agent 范围 Memory、版本化 Skill Bundle、Principal MCP Library 与 Agent Binding、只读动态 Tool，以及类型化 Composer 能力选择。
- Agent Profile Owner View、Impression 主动提炼、Fact 确认收件箱、Runtime Context 注入、强制 Disclosure Policy、签名 AgentFacts 发布与 Owner-only AgentCard Draft。
- 独立 `aegislink-index`、独立 PostgreSQL + pgvector、带认证的 AgentAddr 注册、完整向量快照替换、exact cosine Search 与 Health/Readiness。
- Owner 显式启用的同 Server Assistance Request、Scoped/Expiring Collaboration Session、官方 A2A 1.0 Task、文本临时 Invocation、审计与撤销。

## 下一阶段

1. 完善生产级 Run 恢复与原生 AG-UI Resume 语义。
2. 在启用 external-write/destructive Tool 前实现持久化、幂等的逐次 Approval。
3. 增加安全的 MCP Credential/OAuth 存储与脱敏边界。
4. 只有出现明确的第二 Agent 工作流后，才完善多 Agent 生命周期管理。
5. 为三阶段 Index 初版增加 AgentAddr Scoped Publisher Credential、Query Rate Limit；只有评测证明收益后再增加 HNSW Migration。
6. 只有具备审查、来源、删除和隐私控制时，才引入长期 Memory 自动派生与向量检索。

## 延后范围

跨 Server A2A 联邦路由、Accepted Session 之外的公开调用、A2A Streaming/Push、全球 Index 路由、LSH、第三方 Attestation、委托所有权、跨 Agent 编排、Organization、Marketplace、gRPC、WebSocket、Redis 和 Kubernetes 均不属于当前版本。
