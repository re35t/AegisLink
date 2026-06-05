# 架构概览

AegisLink 使用分层架构：

- 表现层：CLI、Web UI、应用 SDK、IM 适配器。
- 业务层：agent client、运行时、RAG pipeline、路由、权限检查。
- 数据层：PostgreSQL 元数据、向量数据库、文件记忆、审计日志。

Phase 1 聚焦本地 agent client、`soul.md` 记忆、JSONL 对话历史、hash embedding，以及兼容 Chroma 的内存向量适配器。
