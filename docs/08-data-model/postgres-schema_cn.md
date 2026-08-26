# PostgreSQL Schema 指南

权威 Schema 是 [`../../migrations`](../../migrations) 下的有序 SQL。本文只说明所有权和表职责，不重复每个 Column 或 Constraint。

## Account 与 Agent 所有权

- `human_principals` 是授权与所有权根。
- `user_accounts` 保存邮箱密码登录状态；`account_sessions` 保存 Session Token Hash；`user_preferences` 保存显式 Language/Theme。
- `agents` 归属于一个 Human Principal，并作为 Agent Name、Description 与独立版本化私有 System Prompt 的权威来源。Runtime 会把 Name/Description 动态组合为结构化 Identity 数据，而不会复制进持久化 Prompt。
- 注册在同一事务中创建 Account、Principal、默认 Agent、Preferences 与 Agent Profile。

## Agent Profile

- `agent_profiles` 是以 `agent_id` 为 Key 的一对一扩展；`version` 跟踪 Identity、Confirmed Fact 与 Policy，`context_revision` 跟踪 Impression 与 Candidate。
- `agent_impressions` 永久保存短期观察历史，`agent_impression_evidence` 保存证据链；Freshness 由观察时间、过期时间和半衰期动态计算。
- `agent_fact_candidates` 与 source 表组成 Owner 收件箱；`agent_confirmed_facts` 保存独立的确认、有效期与撤销状态。
- `agent_profile_jobs` 是持久 PostgreSQL Curator 队列，使用 lease 与 `FOR UPDATE SKIP LOCKED`。
- `agent_profile_disclosure_policies` 只按 Identity、Capability、Confirmed Fact 与 Endpoint Subject 保存策略。
- 有效 Capability 从 Runtime、Skill 与 MCP 权威状态实时聚合，不复制到第二张 Capability 表。

## AgentFacts 发布

- `agent_publication_settings` 保存唯一非空 Hostname、DNS Challenge 与 TTL。
- `agent_signing_keys` 保存 Ed25519 Public Key 和 AES-256-GCM 加密的 Private Key。
- `agent_access_tokens` 只保存 SHA-256 Token Hash 与精确 Audience。
- `agent_facts_publications` 保存不可变 Payload、Digest、JWS、过期、替换与撤销状态。

## Conversation Ledger

- `conversations` 同时归属于 Principal 与 Agent。
- `messages` 保存有序持久 Transcript。
- `runs` 保存 Lifecycle、Failure、下一事件 Sequence，以及不可变 JSON Execution Policy Snapshot。
- `run_events` 在单个 Run 内有序并支持 SSE 回放，但不能替代 Audit Log。

## Memory、Skills 与 MCP

- `memories` 直接属于 Agent，并支持 Active/Forgotten Lifecycle。
- `skill_packages` 属于 Principal；`skill_versions` 与 `skill_version_files` 保存不可变内容；`agent_skills` 为每个 Agent 选择并启用一个 Version。
- `mcp_servers` 与 `mcp_tools` 组成 Principal Library；`agent_mcp_servers` 与 `agent_mcp_tools` 保存每个 Agent 的独立启用状态。

## 演进与访问

Schema 变更必须新增 Goose migration，不能重写已应用 migration。运行时通过 `internal/postgres.Database` 与 GORM 访问；GORM 类型不得越过 PostgreSQL Adapter，并禁止 `AutoMigrate`。

破坏性 Repository 集成测试必须使用数据库名以 `_test` 结尾的专用数据库。
