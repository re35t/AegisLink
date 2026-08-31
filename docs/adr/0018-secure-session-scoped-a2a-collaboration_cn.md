# ADR 0018：安全的 Session Scoped A2A 协作

- 状态：Accepted
- 日期：2026-08-31

## 背景

Index Discovery 只返回不透明 AgentAddr，不授予调用权限。用户只能直接操作自己拥有的 Agent，因此被发现的 Agent 不能挂到调用方的普通 Conversation，也不能通过 Owner API 暴露。

## 决策

- Assistance Request、目标 Owner Policy、Evaluation Invocation、Collaboration Session、限流/幂等与审计归 `internal/collaboration`；V1 只路由同一 Agent Server 托管的 Agent。
- 目标策略默认关闭。Server 完成硬策略检查后，才运行无 Tool 的 Single-Turn Evaluation Invocation；模型不能扩大 Scope 或 TTL。
- 接受后生成不透明 Capability，只持久化 SHA-256 Hash 与 AES-GCM 密文；原始 Capability 不返回浏览器、模型、Owner API、Event 或日志。
- 直接使用 `a2a-go/v2` 的官方 A2A 1.0 AgentCard、Message、Task、Artifact 与 JSON-RPC Handler；AgentAddr 映射为 A2A Tenant，Session ID 映射为 Context 与 Task Owner。
- A2A Task 使用 PostgreSQL 和乐观版本持久化。每条新消息按需创建一次 Collaboration Invocation，只加载 B 的指令、公开 Profile 与有界 Session Task 历史，输出一个文本 Artifact 后立即释放。
- V1 只允许 `text/plain`，不加载 B 的私有 Memory、Impression、Skill、MCP Tool、Credential 或普通 Conversation，也不支持 Streaming 与 Push Notification。

## 影响

Agent B 作为逻辑实体长期存在，Runtime 始终按需实例化；调用方不会获得 B 的 Owner 权限。跨 Server 联邦必须另行设计 Server Identity、路由、签名与信任，不能削弱当前同 Server 安全边界。
