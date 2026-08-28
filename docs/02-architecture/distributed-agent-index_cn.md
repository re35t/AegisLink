# AegisLink 独立 Agent Index 架构

## 状态

仓库已经包含可独立运行的 `aegislink-index` Go/Gin 初版、Health/Readiness、独立 PostgreSQL + pgvector，以及 Register、Publish/Update、Search 三阶段。注册接受空对象并持久化 AgentAddr；发布以事务完整替换当前 Fact Vector 快照；检索使用 exact cosine，按 AgentAddr 聚合候选。不包含名称、Facts URL、Cache TTL、LSH JSON 或 public Resolve。

Index 按以下三个阶段继续演进：

1. Agent Server 注册并得到 Index 分配的 AgentAddr；
2. Agent Server 把公开可索引 AgentFacts 用统一 Encoder 编码后，上传完整向量快照；
3. Caller 用同一 Encoder 生成 Query Vector，Index 检索后直接返回 AgentAddr 候选。

完整的 Payload、时序图、pgvector Schema、Exact/HNSW SQL 和迁移方案见 [PostgreSQL + pgvector 三阶段 Discovery 技术设计](./pgvector-discovery-query-pipeline_cn.md)。

## 部署边界

```text
Publishing Agent Server             Standalone Index                 Calling Agent Server
-----------------------             ----------------                 --------------------
AgentFacts + Disclosure  --vector--> Register / Publish API          Query intent
Pinned Encoder                       PostgreSQL + pgvector  <--vector-- Same Encoder
Authoritative raw Facts              Vector candidate search  --addr--> Discovery caller
```

- Index 独立部署、独立迁移、独立配置，不连接 Agent Server 数据库。
- `internal/discovery` 管理 AgentFacts 与 Disclosure；`index/internal/discovery` 管理向量快照和检索。
- Gin 只存在于 `index/internal/httpapi`；运行时 PostgreSQL 访问只存在于 `index/internal/postgres` 并使用 GORM；Schema 使用 Goose。
- Index 不运行 Encoder，不需要模型密钥。
- Agent 间使用 AgentAddr 后如何建立通信不属于 Discovery Index。

## 数据所有权

Agent Server 权威持有：

- 完整 AgentFacts 与 Disclosure Policy；
- Fact 规范化逻辑；
- 版本化 Encoder Profile；
- 每次发布使用的 Source Digest 和本地发布审计记录。

Index 权威持有：

- Index 分配的 `AgentAddr`；
- 当前 Representation Revision；
- Encoder Profile 与 Source Set Digest；
- 每个 Agent 最多 64 个 Fact Vectors；
- AgentAddr 状态、幂等摘要和时间戳。

Index 不持有：

- Facts URL、AgentFacts JSON 或 Fact 原文；
- Agent 名称、注册 Cache TTL；
- LSH、Hash Signature、Routing Facet 或结构化倒排 Posting；
- Conversation、Memory、Prompt、Credential 或私有 Skill 内容。

## 三阶段协议

### Register

`POST /api/v1/registry/agents` 接受空 JSON object，通过认证与幂等键分配 `agent_<ULID>`。返回值只包含 Schema Version、AgentAddr 和创建时间。

### Publish / Update

`PUT /api/v1/registry/agents/{agentAddr}/representation` 接受完整、单调递增 Revision 的向量快照。Agent Server 在上传前完成 Disclosure、规范化和编码。Index 在单个 PostgreSQL 事务中替换旧快照。

### Search

`POST /api/v1/discovery/search` 接受 Encoder Profile、Query Vector 和 Top-K。Index 先用 exact cosine 建立基线，规模需要时使用 pgvector HNSW；多 Fact Vector 按 AgentAddr 聚合为一个候选，Search 直接返回 AgentAddr。

协议不包含 public AgentAddr Resolve、Facts URL Fetch、AgentFacts 回源验证或 LSH 分支。

## 演进原则

- 首个小规模网络用 exact cosine，正确性优先于 ANN 调参。
- 只有 Recall@K、MRR 和 p95 latency 证明收益后才启用 HNSW。
- 一个 Agent 的所有向量由完整快照原子替换，防止删除的 Fact 继续被召回。
- 全网同一时刻只激活一个 Encoder Profile；升级采用新 Profile 重编码和明确切换。
- PostgreSQL 容量不足时可替换 `VectorSearch` Adapter，但不改变 Register、Publish、Search 领域协议。
- 不再采用 LSH，也不以 NANDA 的 URL/URN 解析模型作为数据设计依据。

## 当前初版与后续扩展

| 能力                       | 当前仓库           | 下一版目标                       |
| -------------------------- | ------------------ | -------------------------------- |
| Index 独立进程与数据库     | 已实现             | 保留                             |
| AgentAddr 注册             | 已实现             | 空请求，仅返回并持久化 AgentAddr |
| Facts URL / 空 LSH         | 已移除             | 不再进入契约和新存储             |
| Representation Publication | 已实现完整快照替换 | AgentAddr 独立 Publisher Credential |
| Agent Server Profile Publisher | 已实现 setup 与变化同步 | 多副本场景的持久重试/Outbox          |
| PostgreSQL pgvector        | 已实现固定 1536 维 | 按实测容量扩展                   |
| Discovery Search           | 已实现 exact cosine | 评测达标后另加 HNSW Migration    |
| Agent Server Discovery Caller | 已实现 Query 编码与接口 | Run 级产品接入                    |
| LSH / Hash Index           | 已删除             | 不再实现                         |

公开 HTTP 形状以 `index/contracts/http/v1/openapi.yaml` 为准；后续变更仍必须先更新 OpenAPI，再进入运行时代码。
