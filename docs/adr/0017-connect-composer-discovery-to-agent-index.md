# ADR 0017: Connect Composer Discovery to the Agent Index

- Status: Accepted
- Date: 2026-08-28

## Context

ADR 0012 introduced a typed Composer capability menu and initially defined Discovery as a local inspection of the current Agent's enabled Skills and MCP Tools. ADR 0016 subsequently established a standalone Agent Index whose Search stage returns ranked, opaque AgentAddr candidates. Keeping the Composer entry local would leave that product surface disconnected from the implemented Discovery path and would misrepresent its purpose.

## Decision

- Supersede ADR 0012 only for the behavior of the `discovery / discover-once` selection. Retain the typed Catalog, opaque Mention ID, AG-UI selection shape, and durable Run execution policy.
- Project one Catalog item named `Find related Agents` under `AegisLink Index`, using Mention ID `discovery:agent-search`. Mark it `server-offline` and prevent selection when the Agent Index and encoder are not configured.
- Resolve that selection in `internal/harness` to a forced first call of `discover_agents`.
- Give the Tool one required semantic `query`. Fix its product limit at five candidates rather than allowing the model to select Top-K.
- Call the injected Agent Index application service directly. Do not make a Server-to-itself HTTP request.
- Read the current Agent's registered AgentAddr, over-fetch by one when the Index limit allows it, remove that address while preserving Index ranking, and truncate to the requested Top-K.
- Return only Index-provided `agentAddr`, `score`, `matchedVectorId`, and `representationRevision` values. The final Agent response must list every returned AgentAddr and score, state clearly when none were found, and must not infer names, capabilities, or addresses.
- Keep the owner-scoped REST resource `POST /api/v1/agents/{agentId}/discovery/search` as a second adapter over the same application service.

## Consequences

Composer Discovery now performs a real cross-Agent similarity search while retaining the existing Conversation -> Harness -> Runtime execution direction and durable AG-UI Tool events. The Server remains responsible for ownership checks and query encoding; the standalone Index remains responsible only for vector retrieval.

This slice ends when ranked AgentAddr candidates are returned. AgentAddr Resolve, remote AgentFacts fetch, Agent-to-Agent communication, and a dedicated search page remain out of scope.
