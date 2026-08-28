# AegisLink Index

`aegislink-index` is the standalone service for cross-server Agent discovery. The initial version implements the complete three-stage protocol: authenticated AgentAddr allocation, atomic publication of complete Fact Vector snapshots, and exact cosine search returning ranked AgentAddr candidates. It has independent PostgreSQL + pgvector persistence and health/readiness endpoints. It never stores AgentFacts text and does not expose public address resolution.

The Index uses its own database and registration credential. It does not use Agent Server model credentials or read the Agent Server database:

```dotenv
INDEX_SERVER_ADDRESS=127.0.0.1:4331
INDEX_SERVER_SHUTDOWN_TIMEOUT=10s
INDEX_DATABASE_URL=postgres://aegislink_index:aegislink_index@127.0.0.1:55434/aegislink_index?sslmode=disable
INDEX_REGISTRATION_TOKEN=replace-with-at-least-32-random-characters
INDEX_QUERY_TOKEN=replace-with-a-separate-32-character-token
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

The response contains only `schemaVersion`, the Index-assigned `agentAddr`, and `createdAt`. Replaying the same registration token and `Idempotency-Key` returns the same stored AgentAddr. Use that address with `PUT /api/v1/registry/agents/{agentAddr}/representation`; query with `POST /api/v1/discovery/search`. The complete payloads and errors are defined in [`contracts/http/v1/openapi.yaml`](contracts/http/v1/openapi.yaml).

Publication accepts 0–64 vectors with the fixed 1536-dimensional MVP Encoder Profile. Every request is a complete monotonically versioned replacement; an empty snapshot removes previous vectors. Search defaults to Top-5, caps Top-K at 50, groups multiple Fact Vectors into one candidate per AgentAddr, and uses exact cosine as the initial correctness baseline.

`sourceSetDigest` is `sha256:` plus the SHA-256 of all vectors sorted by `vectorId`, hashing each `vectorId` and `sourceDigest` as a four-byte big-endian byte length followed by the UTF-8 bytes. The helper used by the Index is `discovery.SourceSetDigest`. The complete architecture is documented in the [pgvector Discovery pipeline](../docs/02-architecture/pgvector-discovery-query-pipeline.md) / [中文](../docs/02-architecture/pgvector-discovery-query-pipeline_cn.md).

## 中文

`aegislink-index` 是跨 Agent Server Discovery 的独立服务。初版已经实现完整三阶段协议：带认证的 AgentAddr 分配、Fact Vector 完整快照原子替换，以及直接返回 AgentAddr 候选的 exact cosine 检索。它使用独立 PostgreSQL + pgvector，提供 Health/Readiness，不保存 AgentFacts 原文，也不暴露 public resolve。

Index 使用自己的数据库和注册凭据，不使用 Agent Server 模型密钥，也不读取 Agent Server 数据库。先运行 `make dev-index-db`，再运行 `make dev-index`。注册请求体固定为 `{}`，注册成功或幂等重放都会返回已持久化的同一 AgentAddr。

发布接口每次接收完整、单调递增的 0–64 条向量快照；空快照会清除旧向量。检索默认 Top-5、最大 Top-50，并把同一 Agent 的多条 Fact Vector 聚合为一个候选。详细 Payload、摘要规则与错误码以 OpenAPI 和三阶段技术设计为准，不包含 Facts URL、LSH 或 AgentFacts 回源链路。
