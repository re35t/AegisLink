# 本地开发

安装依赖并启动 PostgreSQL：

```bash
pnpm install
make dev-db
make dev-server
```

使用独立测试数据库运行 Repository 集成测试：

```bash
make test-integration
```

Repository 集成测试会清空数据表并回滚 migration。测试会拒绝数据库名不以 `_test` 结尾的 `TEST_DATABASE_URL`，严禁将开发数据库用于集成测试。

服务打开数据库时由 Goose 执行 migration。运行时查询和事务统一由 GORM 管理；项目明确禁用 `AutoMigrate`，确保本地与生产使用同一份可审计结构历史。
