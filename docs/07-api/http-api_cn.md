# HTTP 与流式协议边界

公开契约的权威来源是 [`../../contracts/http/v1/openapi.yaml`](../../contracts/http/v1/openapi.yaml)。接口变更必须先修改 OpenAPI 并运行 `make generate`，本文不维护第二份 Endpoint/Schema 清单。

## REST

REST 负责持久资源与控制操作：

- 认证、Session、Account Settings 与密码修改；
- Bootstrap、Agent List、Owner-only 版本化 Agent Instructions、Agent Profile、Impression 纠正/生命周期、Fact 审查/撤销与 Disclosure Policy；
- Owner Publication Settings、DNS 验证、签名密钥轮换、一次性 Token 创建与 AgentCard Draft；
- Agent Memory、Skill Package/Import/Binding、MCP Library 与 Agent Binding；
- Mention Catalog Projection；
- Conversation/History 查询、Run Event 回放与取消。

所有受保护路由都使用服务端 Session。Owner/Principal ID 不能作为客户端授权输入；Agent 范围路由必须验证所选 Agent 属于登录 Principal。Identity/Fact/Policy 使用 Profile Version，Impression/Candidate 使用 Context/Candidate Version。

## Host 范围 AgentFacts

DNS 验证并启用发布后，服务端只按真实 HTTP `Host` 精确选择 Agent，忽略 `X-Forwarded-Host`。Well-known 文档只返回 Public + Indexable Claim 并带 ETag/TTL；POST Query 避免 Selector 进入 URL Log，按匿名/Token Audience 过滤，返回 `no-store`，且不暴露被拒 Claim 是否存在。

## AG-UI

`POST /api/v1/ag-ui` 负责活动 Agent 执行。当前 Adapter 接收仅文本的最后一条用户输入和可选的不透明能力选择，并输出 AG-UI Run、Text Message 与 Tool Call SSE Event。

客户端提交的 AG-UI Tool Definition、Schema、名称或历史消息都不能授权 Tool。服务端从持久化状态确定 Conversation Agent，并在创建 Run 前重新解析每个 Selection。

## 持久化 Run Event

`GET /api/v1/runs/{runId}/events` 是受认证保护的持久化 SSE Stream，支持 `Last-Event-ID` 回放。在原生 AG-UI Resume 尚未完成期间，它继续作为恢复来源。Run Event 在 PostgreSQL 中按 Run 单调排序。

## 独立 Index

独立契约 [`../../index/contracts/http/v1/openapi.yaml`](../../index/contracts/http/v1/openapi.yaml) 暴露 Health/Readiness 和带 Bearer 认证的三阶段 Index：只接受空对象且支持幂等重放的 AgentAddr Registration、完整 Representation 替换与 Query Vector Search；不暴露公开 Resolve 或 AgentFacts 原文。

## 官方 A2A 协作

`GET /a2a/agents/{agentAddr}/.well-known/agent-card.json` 只为 Owner 已启用协作的 Agent 返回官方 A2A 1.0 AgentCard；`/a2a/agents/{agentAddr}/agent-card` 保留为显式 API 别名。Card 声明共享 JSON-RPC Interface、AgentAddr Tenant、`text/plain` 与不透明 Bearer Session。`POST /a2a` 由官方 Go SDK 处理 Session Scoped SendMessage、Get/List Task 与 Cancel Task；不接受普通用户 Cookie，也不支持 Streaming、Push 或外部 Capability 签发。

## 未提供接口

Agent Server 不提供跨 Server 联邦路由、Accepted Collaboration Session 之外的公开 A2A 调用、第三方 Attestation、WebSocket、gRPC、Organization 或外部 Session 签发接口。
