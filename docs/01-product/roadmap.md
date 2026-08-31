# Product roadmap

## Implemented baseline

- Go/Gin modular monolith, PostgreSQL/GORM/Goose, React/Vite, AG-UI, and assistant-ui.
- Account sessions, one default Personal Agent, durable Conversations/Runs/Events, cancellation, replay, and interrupted-Run recovery.
- Explicit Agent-scoped Memory, versioned Skill bundles, Principal MCP library with Agent bindings, read-only dynamic Tools, and typed Composer capability selection.
- Agent Profile owner view, active Impression curation, Fact confirmation inbox, Runtime context injection, enforced Disclosure Policy, signed AgentFacts publication, and owner-only AgentCard Draft.
- Standalone `aegislink-index` with independent PostgreSQL + pgvector, authenticated AgentAddr registration, complete vector snapshot replacement, exact cosine Search, and health/readiness.
- Owner-opt-in same-Server Assistance Requests, scoped/expiring Collaboration Sessions, official A2A 1.0 Tasks, text-only ephemeral target Invocations, audit, and revocation.

## Next

1. Complete production-grade Run recovery and native AG-UI resume semantics.
2. Add persisted, idempotent approval for external-write and destructive Tools before enabling them.
3. Add secure MCP credential/OAuth storage and redaction boundaries.
4. Improve multiple-Agent lifecycle management only when a concrete second-Agent workflow exists.
5. Add AgentAddr-scoped publisher credentials and Query rate limiting to the three-stage Index MVP; add HNSW only after evaluation proves a benefit.
6. Add automatic long-term Memory derivation/vector retrieval only with review, provenance, deletion, and privacy controls.

## Deferred

Federated cross-Server A2A routing, general public invocation outside an accepted Session, A2A streaming/push, global Index routing, LSH, third-party attestation, delegated ownership, cross-Agent orchestration, organizations, marketplaces, gRPC, WebSocket, Redis, and Kubernetes are outside the current release.
