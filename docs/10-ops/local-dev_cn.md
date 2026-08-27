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

独立 Index 是可选进程，并使用自己的 PostgreSQL：

```bash
make dev-index-db
make dev-index
curl http://127.0.0.1:4331/healthz
curl http://127.0.0.1:4331/readyz
```

Index 读取 `INDEX_SERVER_ADDRESS`、`INDEX_SERVER_SHUTDOWN_TIMEOUT`、`INDEX_DATABASE_URL` 与 `INDEX_REGISTRATION_TOKEN`，不需要主服务的 `DATABASE_URL`、`MODEL_API_KEY` 或 AgentFacts 加密密钥。Registry 数据库与 Agent Server 数据库相互独立。

Server 打开数据库时由 Goose 应用有序 migration。运行时持久化使用 GORM，并明确禁用 `AutoMigrate`。

Curator 配置是可选的：每个空的 Provider/模型字段都会回退到对应 `MODEL_*`。Curator 请求默认启用 JSON Output，并设置 `CURATOR_MODEL_THINKING=disabled`；思考模式开关由 DeepSeek Driver 应用。只有 AgentFacts 发布需要 `AGENT_KEY_ENCRYPTION_KEY`，其值必须是恰好 32 个随机字节的 Base64。该密钥必须保持稳定且保密；更换后需要轮换 Agent Signing Key。

## 验证

```bash
make generate
make check
make test
make build
```

`make check`、`make test` 和 `make build` 会覆盖两个 Go 进程；Build 会同时生成 `aegislink-server` 与 `aegislink-index`。没有设置各自专用数据库 URL 时会跳过 PostgreSQL 集成测试。请使用安全入口：

```bash
make test-integration
make test-index-integration
```

两个命令分别在 `55433` 与 `55435` 启动 `postgres-test` 和 `index-postgres-test`，使用 `aegislink_test` 与 `aegislink_index_test`。Repository 集成测试会清表，因此代码拒绝任何数据库名不以 `_test` 结尾的地址。

开发库和测试库使用不同命名卷。重启开发数据库会保留数据，删除它的 Volume 则不会。
