# ADR 0003：使用 PostgreSQL 作为权威存储

## 状态

已接受并实现；具体持久化机制由 ADR 0013 补充。

## 决策

PostgreSQL 是 Account/Principal、Agent/Profile、Conversation、Message、Run/可回放 Run Event、Memory、不可变 Skill Package/Version/File 与 Agent Binding，以及 MCP Library/Binding Metadata 的权威存储。

有序 Goose migration 定义 Schema；运行时 Repository 按 ADR 0013 使用 GORM。客户端 Cache、AG-UI 输入历史、Runtime State 和模型供应商 Payload 都不能替代 PostgreSQL 权威数据。

Organization、公开 Agent Directory、加密 Identity、Vector Index 和 Audit Service Record 不属于当前 Schema，也不能由本决策推导为已实现。
