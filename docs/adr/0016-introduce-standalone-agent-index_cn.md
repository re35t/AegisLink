# ADR 0016：引入独立 pgvector Agent Index

- 状态：Accepted
- 日期：2026-08-27
- 修订：2026-08-27

## 背景

AegisLink 已能在单个 Agent Server 上生成经过 Disclosure Policy 过滤的 AgentFacts，但需要独立服务在多个 Server 之间发现 Agent。若把网络 Registry 或 Vector Search 状态放进 Personal Agent OS 数据库，会把 Discovery 扩展与 Conversation/Profile 存储耦合，并让 Agent Server 对共享 Index 状态承担不合理的权威责任。

第一版 Registry 草案虽然能够分配 Agent ID，但同时携带名称、Facts URL、Cache TTL 和预留 LSH object。阶段一现已迁移到纯地址注册和持久化 `agent_registry`；新的产品方向不再采用 URL-based AgentFacts Resolution 或 LSH。

## 决策

- 保留顶层 `index/` 下独立的 Go/Gin `aegislink-index` 进程，MVP 继续位于同一 Go module。
- `index/cmd/aegislink-index` 保持为进程入口，`index/internal` 私有持有 Application、HTTP、Registry、Discovery 和 PostgreSQL 实现。
- `internal/discovery` 负责 AgentFacts/Disclosure；`index/internal/discovery` 只负责向量发布和候选检索。
- 协议严格只有 Register、Publish/Update、Search 三个阶段。
- 注册不携带发现数据；Index 分配并返回不透明的 `agent_<ULID>` AgentAddr。
- Agent Server 保留 AgentFacts 原文，选择 Public + Indexable Facts，并使用全网统一、版本化的 Encoder Profile 编码。
- Index 在独立 PostgreSQL + pgvector 中保存有版本的完整 Fact Vector 快照，不保存 Facts URL 或 Fact 原文。
- Caller 使用同一 Profile 编码 Discovery Query。Index 执行 exact cosine 或经过评测的 pgvector HNSW，按 AgentAddr 聚合并直接返回排序后的 AgentAddr 候选。
- 只保留 Registry、RepresentationStore 和 VectorSearch 小型 Ports。目标模型不再保留 StructuredIndex、HashIndex、Routing Facet、LSH 或通用多信号 Ranker。
- 旧契约必须通过 OpenAPI 变更和新增 Goose Migration 演进；该阶段已经实现，已经应用的 Migration 继续保持不可变。

## 影响

Index 继续独立部署且不能访问 Agent Server 数据库。阶段一现在会分配并持久化不透明 AgentAddr。Index 不运行 Encoder、不需要模型密钥，但未来所有 Publisher 和 Caller 必须固定使用相同 Encoder Profile。

完整快照替换保证删除的 Fact 不会继续被召回。Exact cosine 是正确性基线；只有 Recall@K、MRR 和延迟指标证明收益后才启用 HNSW。

Facts URL Resolution、Search 后 AgentFacts Fetch、LSH 和 NANDA URL/URN 数据模型都不属于选定架构。若 PostgreSQL 未来达到容量上限，可以替换专用 Vector Database Adapter，但不改变三阶段协议。
