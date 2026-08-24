# ADR 0011：用户 MCP 插件库、Agent 绑定与单次 Run 工具选择

状态：已接受；只读工具首版已实现。

日期：2026-08-24。

## 背景

MCP Server 由 Human Principal 安装，但具体 Tool 的使用权限属于某个 Personal Agent。若每个 Agent 重复保存一份 Server 配置，会重复发现结果，也不利于后续多 Agent 管理。浏览器提交的 Tool 定义同样不能作为授权依据，否则客户端可能绕过所有权、风险、连接和启用状态校验。

Composer 还需要一个可扩展的 `@` 目录。首版只展示 MCP Tools，但数据结构应允许以后增加其他对象，且不能为此再建立一份重复的 Tool Registry。

## 决策

MCP 能力状态拆成三个持久化层级：

```text
Human Principal 的 MCP 插件库
  mcp_servers -> mcp_tools
        │
        ▼
Personal Agent 启用关系
  agent_mcp_servers -> agent_mcp_tools
        │
        ▼
Run execution_policy 快照
  auto | force-tool-once
```

- `mcp_servers` 与发现得到的 `mcp_tools` 属于登录 Principal。Tool 使用稳定且不透明的 ID；刷新同名 Tool 时保留 ID 与 Agent 绑定，新发现 Tool 因没有绑定而默认关闭。
- `agent_mcp_servers` 与 `agent_mcp_tools` 分别保存每个 Agent 的启用状态。启用 Tool 会确保对应 Server 已绑定，但不会连带启用其他 Tool。
- `GET /agents/{agentId}/mentions` 是插件库和 Agent 绑定的查询投影，不建立第二张 `tool_list` 表。首版只支持 `mcp-tool`，并返回 ready、待启用、离线、关闭或待审批等可用状态。
- 浏览器只通过 AG-UI `forwardedProps` 提交不透明 Mention ID 和 `force-tool-once` action。Conversation Service 从权威 Conversation 得到 Agent，并在创建 Run 前按登录 Principal、Agent 绑定、Server 状态、Tool 状态和风险等级重新解析。
- 每个 Run 保存结构化 `execution_policy` JSON 快照，`run.started` 事件也记录可公开快照，支持回放和审计。
- Eino 只接收已经授权的 qualified Tool name。每个 Run 创建独立、不可变的模型包装器，仅在第一次模型调用时设置 `ToolChoiceForced` 和唯一允许的 Tool；后续恢复自动模式。模型忽略或调用错误 Tool 时使用稳定错误码终止 Run。
- 首版只允许 read-only Tool。`external-write` 和 `destructive` 必须等待持久化的逐次审批流程。

## 影响

- 同一 Principal 可以只安装一次 Server，同时保持多个 Agent 的工具权限互相隔离。
- 浏览器提交的 AG-UI `tools`、Tool 名称和 Schema 永远不是授权输入。
- 删除用户插件库中的 Server 会影响所有 Agent，前端必须按跨 Agent 破坏性操作确认。
- Mention Catalog 后续可增加新的 `kind`、`category`、`group` 与 `action`，而无需改变 Run 的服务端授权规则。
- 本 ADR 修正 ADR 0007 中“MCP Server 直接属于 Agent”的表述：Memory 仍直接属于 Agent；Skill 与 MCP 均采用 Principal 所有、Agent 绑定模型。

## 暂缓

- MCP credential 与 OAuth。
- 写入/破坏性 Tool 审批与幂等执行。
- Skills、内置工具、Agent 或人员等 Mention kind。
- 单次 Run 多个强制选择或多 Agent 调度。
