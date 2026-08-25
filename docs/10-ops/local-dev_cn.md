# 本地开发

安装依赖并启动 PostgreSQL：

```bash
pnpm install
make dev-db
make dev-server
```

使用明确的数据库地址运行 Repository 集成测试：

```bash
TEST_DATABASE_URL='postgres://aegislink:aegislink@127.0.0.1:55432/aegislink?sslmode=disable' make test
```

服务打开数据库时由 Goose 执行 migration。运行时查询和事务统一由 GORM 管理；项目明确禁用 `AutoMigrate`，确保本地与生产使用同一份可审计结构历史。
