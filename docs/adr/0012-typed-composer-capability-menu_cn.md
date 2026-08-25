# ADR 0012：Composer 类型化能力菜单

## 状态

已接受。

## 决策

Conversation Composer 使用一个服务端投影目录，但条目明确区分三种类型：

- `mcp-tool`，动作是 `force-tool-once`；
- `skill`，动作是 `use-skill-once`；
- `discovery`，动作是 `discover-once`。

Composer 左侧使用紧凑的 `+` 菜单。一级菜单包含 MCP、Skills 和 Discovery，选中类别后在右侧打开子列表。assistant-ui 的 `@` Trigger 继续复用同一目录和选择状态。

浏览器只提交不透明的 Mention ID 和 action。服务端在创建 Run 前重新解析所有权、Agent 绑定、启用状态和实际执行策略。类型化策略写入 `runs.execution_policy`，并随 `run.started` 事件持久化。

选择 Skill 时，Runtime 首步强制调用现有 `load_skill`，同时校验模型请求的名称必须等于用户选中的 Skill。选择 Discovery 时，Runtime 首步强制调用本地 `discover_capabilities`，列出当前 Agent 已启用的 Skills 和 MCP Tools。它不代表互联网 Agent 发现、能力市场或 NANDA 发布。

## 影响

MCP Tool、Skill 和 Discovery 虽共享一个 Picker，但不会被错误地统一成 Tool 语义。未来可以继续增加类别，同时不需要提前引入 Agent Network 或 Marketplace 领域模型。
