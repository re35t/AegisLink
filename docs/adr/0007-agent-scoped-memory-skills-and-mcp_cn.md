# ADR 0007：按 Agent 隔离 Memory、Skills 与 MCP

状态：已接受，V0 已实现。

MCP 的所有权与绑定模型已由 [ADR 0011](0011-user-mcp-library-agent-bindings-and-run-tool-selection_cn.md) 修订。

日期：2026-08-23。

## 背景

当前注册流程为一个 Human Principal 创建一个 Personal Agent，但产品模型允许未来一个用户创建多个 Agent。若 Memory、Skill 启用状态或 MCP 连接只按用户保存，多个 Agent 会共享上下文和外部权限，既破坏人格/任务边界，也扩大数据泄漏与误调用范围。

## 决策

- 所有管理 API 使用 `/agents/{agentId}/...`，服务端从 session 取得 `principal_id`，并同时校验 Agent ownership；浏览器不能提交或覆盖 Principal ID。
- `memories`、`mcp_servers` 直接保存 `owner_principal_id + agent_id`；所有查询同时使用两者。`mcp_tools` 通过所属 Server 继承相同作用域。
- Skill Package 和 immutable Version 属于 Principal，`agent_skills` 保存每个 Agent 独立选择的 Version 与启用状态。同名 Package 可有多个内容 hash 不同的 Version，Agent A 选择 v1 不会被 Agent B 安装 v2 覆盖。
- Agent Harness 根据权威 Conversation 所属 Agent 解析 Memory、enabled Skills 和 MCP Tools，前端传入的历史或 Agent ID 不能替换服务端归属。
- Memory V0 采用显式、可编辑、可确认和可遗忘的 PostgreSQL 记录；暂不自动抽取全部聊天，也不把完整历史等同于长期记忆。
- Skills V0 校验 Agent Skills `SKILL.md` 的 `name`/`description` frontmatter。Runtime 先看到元数据，只有任务匹配时才通过 `load_skill` 取得完整内容，采用渐进式披露。
- MCP V0 使用官方 Go SDK 与 Streamable HTTP。URL 默认只允许不含 userinfo、query 或 fragment 的 HTTPS，禁用 redirect/proxy 并阻止解析到私网、回环、链路本地等地址；本地开发例外必须显式设置 `MCP_ALLOW_PRIVATE_NETWORKS=true`。
- MCP Server 提供的 Tool annotation 只作为风险分类提示，发现后的工具一律默认关闭。只有用户显式启用且风险为 `read-only` 的工具进入 Runtime；`external-write` 与 `destructive` 在逐调用 Approval 完成前不可执行。

## 数据与调用边界

```text
authenticated principal
        │ owns
        ▼
personal agent
  ├── memories
  ├── agent_skills ── skill_versions ── skill_packages
  └── mcp_servers ── mcp_tools
        │
        ▼ resolve per run
internal/harness
  ├── memory context
  ├── skill catalog + load_skill
  └── enabled read-only MCP tools
        │
        ▼
internal/runtime -> Eino
```

Gin 和 cookie/session 类型只存在于 `internal/httpapi`/account 边界；Memory、Skills、MCP service 只使用 `context.Context`、Principal ID、Agent ID 和领域类型。MCP SDK 类型只留在 `internal/mcp`，Eino 类型只留在 `internal/runtime`，Harness 通过领域无关的 Tool 定义完成能力装配。

## V0 边界

- Memory 尚无模型候选提取、embedding/向量检索、衰减、冲突合并和命中 Trace。
- Skills 只支持 inline `SKILL.md`，尚无目录 bundle、scripts/references/assets、Git 来源和沙箱执行。
- MCP 尚无 OAuth、bearer secret reference、resources/prompts、并发配额和写操作 Approval。
- 当前产品仍只创建默认单 Agent，但 schema、repository、service 和 API 授权已按多 Agent 隔离；增加多 Agent 创建功能时不改变本 ADR 的作用域规则。

## 开源参考

- [Model Context Protocol 规范](https://modelcontextprotocol.io/specification/2025-06-18/basic/index)与[官方 Go SDK](https://github.com/modelcontextprotocol/go-sdk)：协议生命周期、Streamable HTTP、工具发现与调用。
- [Agent Skills specification](https://github.com/agentskills/agentskills/blob/main/docs/specification.mdx)与[client implementation](https://github.com/agentskills/agentskills/blob/main/docs/client-implementation/adding-skills-support.mdx)：`SKILL.md` 元数据约束与渐进式披露。
- [Open WebUI Memory](https://github.com/open-webui/docs/blob/main/docs/features/chat-conversations/memory.mdx)：用户可查看、编辑和删除的显式记忆体验。AegisLink 的不同点是进一步以 Agent 为隔离单位。
- [LobeHub](../frontend/references/lobehub.md)：只参考 Agent/Memory/Skill/MCP 的对象化信息架构，不引入其市场、团队或技术栈。
