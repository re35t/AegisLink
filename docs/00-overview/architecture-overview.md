# Architecture Overview

AegisLink uses a layered architecture:

- Presentation: CLI, Web UI, app SDKs, IM adapters.
- Business: agent client, runtime, RAG pipeline, routing, permission checks.
- Data: PostgreSQL metadata, vector DB, file memory, audit logs.

Phase 1 focuses on local agent client, soul.md memory, JSONL conversation history, hash embeddings, and a Chroma-compatible in-memory vector adapter.
