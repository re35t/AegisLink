# ADR 0013：PostgreSQL 持久化统一使用 GORM

## 状态

已接受。

## 背景

Go 后端的 Repository 原先分散使用 `database/sql` 与 pgx 特定错误处理。随着 Personal Agent OS 增加 Profile、Skill、MCP 和可回放 Run 状态，统一的 context 传递、事务所有权、结果处理和持久化规范比维护第二套底层数据库风格更重要。

## 决策

- `internal/postgres` 下所有运行时数据库操作统一使用 GORM。
- Repository 构造函数接收 `*gorm.DB`；GORM model 与 clause 仅存在于 PostgreSQL adapter 内。
- 复杂 JOIN、锁、PostgreSQL 数组、CTE 与 `RETURNING` 操作，在 Query Builder 会掩盖约束时可以使用 GORM `Raw` 或 `Exec`。
- Repository 方法必须使用 `WithContext(ctx)` 和 GORM 管理的 `Transaction` callback。
- Domain Service 与 HTTP Handler 不依赖 GORM。
- Goose 仍是唯一数据库结构迁移机制，禁止使用 `AutoMigrate`。

## 结果

持久化 adapter 只保留一套连接和事务抽象，同时在必要位置显式保留 PostgreSQL 特性。集成测试继续使用真实 PostgreSQL，因为 GORM 不能替代数据库约束、锁和 migration 验证。
