# 本地开发

## 启动

```bash
cp .env.example .env
# 在 .env 中设置 MODEL_API_KEY。
pnpm install
make dev
```

`make dev` 会依次加载 `.env` 与 `.env.local`，等待两个开发 PostgreSQL 数据库健康，然后在同一终端监督 Agent Server、独立 Index 与 Vite Web。默认 Web 地址为 `http://127.0.0.1:5173`，API 为 `http://127.0.0.1:4321`，Index 为 `http://127.0.0.1:4331`；两个开发数据库分别暴露在 `127.0.0.1:55432` 与 `127.0.0.1:55434`。

如果环境文件没有设置 `AGENT_KEY_ENCRYPTION_KEY`，`make dev` 会首次生成一个仅供本地开发使用的稳定 32-byte Base64 密钥，保存到 Git 忽略且权限受限的 `.aegislink-dev/agent-key`。后续启动会复用该密钥，使 AgentFacts 与协作 Session 的密文在重启后仍可解密；显式环境配置始终优先。

按 `Ctrl-C` 会停止三个本地进程。开发数据库 Container 会继续运行，命名 Volume 也会保留；需要时可显式停止：

```bash
docker compose stop postgres index-postgres
```

需要细粒度启动时，原有的 `make dev-db`、`make dev-server`、`make dev-index-db`、`make dev-index` 与 `make dev-web` 仍可使用。

Index 读取 `INDEX_SERVER_ADDRESS`、`INDEX_SERVER_SHUTDOWN_TIMEOUT`、`INDEX_DATABASE_URL`、`INDEX_REGISTRATION_TOKEN` 与 `INDEX_QUERY_TOKEN`，不需要主服务的 `DATABASE_URL`、`MODEL_API_KEY` 或 AgentFacts 加密密钥。Index 的 pgvector 数据库与 Agent Server 数据库相互独立。

要让 Agent Server 的首次设置、Profile 同步和 Discovery 搜索连接到该进程，请配置 `AGENT_INDEX_BASE_URL`、`AGENT_INDEX_REGISTRATION_TOKEN` 与 `AGENT_INDEX_QUERY_TOKEN`；再通过 `DISCOVERY_ENCODER_BASE_URL` 和 `DISCOVERY_ENCODER_MODEL` 配置 OpenAI-compatible Encoder，Provider 需要鉴权时再设置 `DISCOVERY_ENCODER_API_KEY`。已有且完成配置的账号可以在关闭 Agent Index 时运行，但新注册账号只有在 Index 与 Encoder 可用后才能完成必需的首次设置。Agent Index 与 Encoder 的必填字段作为一组启用，部分配置会在启动时被拒绝；Encoder 必须生成 Index Profile 固定的 1536 维向量。

Server 打开数据库时由 Goose 应用有序 migration。运行时持久化使用 GORM，并明确禁用 `AutoMigrate`。

Curator 配置是可选的：每个空的 Provider/模型字段都会回退到对应 `MODEL_*`。Curator 请求默认启用 JSON Output，并设置 `CURATOR_MODEL_THINKING=disabled`；思考模式开关由 DeepSeek Driver 应用。AgentFacts 发布与跨 Agent Collaboration 都需要 `AGENT_KEY_ENCRYPTION_KEY`，其值必须是恰好 32 个随机字节的 Base64。该密钥必须保持稳定且保密；更换后需要轮换 Agent Signing Key，并会使尚未过期的 Collaboration Session 失效。

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
