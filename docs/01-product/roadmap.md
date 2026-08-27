# Product roadmap

## Implemented baseline

- Go/Gin modular monolith, PostgreSQL/GORM/Goose, React/Vite, AG-UI, and assistant-ui.
- Account sessions, one default Personal Agent, durable Conversations/Runs/Events, cancellation, replay, and interrupted-Run recovery.
- Explicit Agent-scoped Memory, versioned Skill bundles, Principal MCP library with Agent bindings, read-only dynamic Tools, and typed Composer capability selection.
- Agent Profile owner view, active Impression curation, Fact confirmation inbox, Runtime context injection, enforced Disclosure Policy, signed AgentFacts publication, and owner-only AgentCard Draft.
- Standalone `aegislink-index` with independent PostgreSQL, authenticated AgentAddr registration, public resolve, health/readiness, and replaceable Discovery ports.

## Next

1. Complete production-grade Run recovery and native AG-UI resume semantics.
2. Add persisted, idempotent approval for external-write and destructive Tools before enabling them.
3. Add secure MCP credential/OAuth storage and redaction boundaries.
4. Improve multiple-Agent lifecycle management only when a concrete second-Agent workflow exists.
5. Continue the controlled-network Index after implemented stage-one AgentAddr allocation/persistence: shared-encoder Fact Vector snapshot publication, then pgvector exact/HNSW Search that directly returns AgentAddr.
6. Add automatic long-term Memory derivation/vector retrieval only with review, provenance, deletion, and privacy controls.

## Deferred

Public AgentCard/A2A invocation, federated/global Index routing, LSH, third-party attestation, delegated ownership, cross-Agent orchestration, organizations, marketplaces, gRPC, WebSocket, Redis, and Kubernetes are outside the current release. The current Index has stage-one address allocation/persistence but no vector publication or search; LSH is no longer on the planned path.
