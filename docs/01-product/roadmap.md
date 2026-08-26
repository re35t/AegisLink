# Product roadmap

## Implemented baseline

- Go/Gin modular monolith, PostgreSQL/GORM/Goose, React/Vite, AG-UI, and assistant-ui.
- Account sessions, one default Personal Agent, durable Conversations/Runs/Events, cancellation, replay, and interrupted-Run recovery.
- Explicit Agent-scoped Memory, versioned Skill bundles, Principal MCP library with Agent bindings, read-only dynamic Tools, and typed Composer capability selection.
- Agent Profile owner view, active Impression curation, Fact confirmation inbox, Runtime context injection, enforced Disclosure Policy, signed AgentFacts publication, and owner-only AgentCard Draft.

## Next

1. Complete production-grade Run recovery and native AG-UI resume semantics.
2. Add persisted, idempotent approval for external-write and destructive Tools before enabling them.
3. Add secure MCP credential/OAuth storage and redaction boundaries.
4. Improve multiple-Agent lifecycle management only when a concrete second-Agent workflow exists.
5. Add automatic long-term Memory derivation/vector retrieval only with review, provenance, deletion, and privacy controls.

## Deferred

Public AgentCard/A2A invocation, central AgentFacts Index registration/search, third-party attestation, delegated ownership, cross-Agent routing, organizations, marketplaces, gRPC, WebSocket, Redis, Kubernetes, and multi-Agent orchestration are outside the current release.
