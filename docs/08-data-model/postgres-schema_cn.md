# PostgreSQL Schema

PostgreSQL 是 Account、Human Principal、Agent、Agent Profile、Conversation、Message、Run、可回放 Run Event、Memory、Skill 与 MCP 配置的权威存储。

## 所有权与身份

- `human_principals` 是个人数据的授权根。
- `user_accounts`、`account_sessions`、`user_preferences` 保存登录与 Human Account 状态。
- `agents` 归属于 Human Principal，并继续作为 Agent 名称、描述和 system prompt 的唯一来源。

## Agent Profile

- `agent_profiles` 与 `agents` 一对一，保存头像和用于乐观并发控制的 `version`。
- `agent_profile_facts` 保存带来源的结构化 JSON Fact。
- `agent_memory_projections` 与 `agent_memory_projection_sources` 保存派生摘要及其原始 Memory 溯源关系。
- `agent_profile_disclosure_policies` 按 identity、capability、fact、projection subject 保存披露意图。

## Runtime 与集成

- `conversations`、`messages`、`runs`、`run_events` 组成可回放的对话账本。
- `memories` 保存 Agent 范围内的 semantic/episodic Memory。
- Skill package/version/file 表保存不可变 Skill 内容，Agent binding 选择并启用版本。
- MCP server、tool 与 Agent binding 表保存发现结果和逐 Agent 启用状态。

数据库结构只能通过 `migrations/` 下按序执行的 Goose migration 演进。`internal/postgres` 的运行时 Repository 统一使用 GORM，禁止调用 `AutoMigrate`。
