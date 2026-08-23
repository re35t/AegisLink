# Roadmap

当前阶段以 [Personal Agent OS 阶段实施规划](./personal-agent-os-plan_cn.md) 为准：后端使用 Go/Gin，Agent Runtime 使用 Eino，PostgreSQL 作为权威数据源；Agent 与 Web 采用 AG-UI；前端保留 React/Vite，并采用 `assistant-ui` 组件库。先完成单个个人 Agent 的 Web、Memory、Skills 和 MCP，并同步建设 Event、Tracing、Permission 与 Approval 基础。

当前不实现注册中心、组织、Group、Network、A2A、签名 RPC、gRPC 或 IM 适配器。这里原先描述的 Multi-Agent/中央服务路线保留为远期研究，只有 Personal Agent 阶段通过验收并出现真实的第二个独立 Agent 需求后才重新设计。

实施顺序：

1. Go/Gin AG-UI endpoint、`assistant-ui` Chat Thread，以及 Event、Trace、Permission 和 Approval 基础。
2. Memory V0 与用户可审查的记忆管理。
3. 兼容 `SKILL.md` 的 Skills V0。
4. 服务端 MCP Manager 与工具权限。
5. 基于 `assistant-ui` 统一 Chat、Tool、Attachment、Approval、History 和 MCP 交互，并完成 Memory、Skills、Profile 和 Runs/Traces 页面。
