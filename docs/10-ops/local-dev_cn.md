# 本地开发

## 启动

```bash
cp .env.example .env
# 在 .env 中设置 MODEL_API_KEY。
pnpm install
make dev-db
make dev-server
```

在另一个终端运行 `make dev-web`。默认 Web 地址为 `http://127.0.0.1:5173`，API 为 `http://127.0.0.1:4321`，开发 PostgreSQL 暴露在 `127.0.0.1:55432`。

Server 打开数据库时由 Goose 应用有序 migration。运行时持久化使用 GORM，并明确禁用 `AutoMigrate`。

Curator 配置是可选的：每个空的 `CURATOR_MODEL_*` 字段都会回退到对应 `MODEL_*`。只有 AgentFacts 发布需要 `AGENT_KEY_ENCRYPTION_KEY`，其值必须是恰好 32 个随机字节的 Base64。该密钥必须保持稳定且保密；更换后需要轮换 Agent Signing Key。

## 验证

```bash
make generate
make check
make test
make build
```

未设置 `TEST_DATABASE_URL` 时，`make test` 会跳过 PostgreSQL 集成测试。请使用安全的项目入口：

```bash
make test-integration
```

该命令在 `55433` 端口启动 `postgres-test`、等待健康状态并使用 `aegislink_test`。Repository 集成测试会清表和回滚 migration，因此代码会拒绝任何数据库名不以 `_test` 结尾的地址。

开发库和测试库使用不同命名卷。重启开发数据库会保留数据，删除它的 Volume 则不会。
