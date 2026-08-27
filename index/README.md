# AegisLink Index

`aegislink-index` is the standalone service for cross-server Agent discovery. The current slice implements authenticated allocation and durable storage of opaque AgentAddr values, independent PostgreSQL persistence, health/readiness, and replaceable Registry/Discovery ports. It does not expose public address resolution and does not yet publish or search vectors.

The Index uses its own database and registration credential. It does not use Agent Server model credentials or read the Agent Server database:

```dotenv
INDEX_SERVER_ADDRESS=127.0.0.1:4331
INDEX_SERVER_SHUTDOWN_TIMEOUT=10s
INDEX_DATABASE_URL=postgres://aegislink_index:aegislink_index@127.0.0.1:55434/aegislink_index?sslmode=disable
INDEX_REGISTRATION_TOKEN=replace-with-at-least-32-random-characters
```

```bash
make dev-index-db
make dev-index

curl http://127.0.0.1:4331/healthz
curl http://127.0.0.1:4331/readyz
curl -i -X POST http://127.0.0.1:4331/api/v1/registry/agents \
  -H "Authorization: Bearer $INDEX_REGISTRATION_TOKEN" \
  -H 'Idempotency-Key: local-agent-one' \
  -H 'Content-Type: application/json' \
  -d '{}'
```

The response contains only `schemaVersion`, the Index-assigned `agentAddr`, and `createdAt`. Replaying the same registration token and `Idempotency-Key` returns the same stored AgentAddr. The contract is [`contracts/http/v1/openapi.yaml`](contracts/http/v1/openapi.yaml); the ownership and future retrieval design are in the [Index architecture](../docs/02-architecture/distributed-agent-index.md).

Stage one now removes name, Facts URL, cache TTL, and LSH from AgentAddr. The remaining Publish/Search stages, pgvector storage, and exact/HNSW retrieval are documented in the [pgvector Discovery pipeline](../docs/02-architecture/pgvector-discovery-query-pipeline.md) / [中文](../docs/02-architecture/pgvector-discovery-query-pipeline_cn.md).

The target design next stores Agent Server-generated Fact Vector snapshots and returns AgentAddr candidates from Vector Search. It has no Facts URL or LSH path.

## 中文

`aegislink-index` 是跨 Agent Server Discovery 的独立服务。当前已实现带 Bearer Token 的不透明 AgentAddr 分配与持久化、独立 PostgreSQL、Health/Readiness，以及可替换的 Registry/Discovery ports；不再暴露公开地址解析，尚未实现向量发布或检索。

Index 使用自己的数据库和注册凭据，不使用 Agent Server 模型密钥，也不读取 Agent Server 数据库。先运行 `make dev-index-db`，再运行 `make dev-index`。注册请求体固定为 `{}`，注册成功或幂等重放都会返回已持久化的同一 AgentAddr。

后续阶段将保存 Agent Server 生成的 Fact Vector 完整快照，并通过 Vector Search 返回 AgentAddr 候选；不包含 Facts URL 或 LSH 链路。
