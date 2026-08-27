# ADR 0016: Introduce a standalone pgvector Agent Index

- Status: Accepted
- Date: 2026-08-27
- Revised: 2026-08-27

## Context

AegisLink can produce disclosure-filtered AgentFacts on one Agent Server but needs an independent service for finding Agents across servers. Putting network Registry or vector-search state in the Personal Agent OS database would couple discovery scaling to Conversation/Profile storage and give Agent Server authority over shared Index state.

The first Registry prototype allocated Agent IDs but also carried name, Facts URL, cache TTL, and a reserved LSH object. Stage one has now migrated to address-only registration and durable `agent_registry` storage; URL-based AgentFacts resolution and LSH are removed from the selected direction.

## Decision

- Keep `aegislink-index` as an independent Go/Gin process under top-level `index/`, in the existing Go module.
- Keep `index/cmd/aegislink-index` as the process entry point and `index/internal` as its private application, HTTP, Registry, Discovery, and PostgreSQL implementation boundary.
- Keep `internal/discovery` responsible for AgentFacts/disclosure and `index/internal/discovery` responsible only for vector publication and candidate retrieval.
- Use exactly three protocol stages: Register, Publish/Update, and Search.
- Registration accepts no discovery data. Index allocates and returns an opaque `agent_<ULID>` AgentAddr.
- Agent Server retains raw AgentFacts, selects public/indexable Facts, and encodes them with one versioned network-wide Encoder Profile.
- Index stores complete versioned Fact Vector snapshots in its own PostgreSQL + pgvector database. It stores no Facts URL or Fact text.
- Callers encode Discovery Queries with the same profile. Index performs exact cosine or measured pgvector HNSW search, groups vector rows by AgentAddr, and directly returns ranked AgentAddr candidates.
- Define small Registry, RepresentationStore, and VectorSearch ports. Do not retain StructuredIndex, HashIndex, routing facets, LSH, or a generic multi-signal Ranker in the target model.
- Evolve the implemented legacy contract through an OpenAPI change and a new Goose migration. This stage is implemented; applied migrations remain immutable.

## Consequences

Index remains independently deployable and cannot access Agent Server databases. Stage one now allocates and durably stores opaque AgentAddr values. Index does not run an Encoder and needs no model key, but future publishers and callers must pin the same Encoder Profile.

Complete snapshot replacement prevents deleted Facts from remaining searchable. Exact cosine provides the correctness baseline; HNSW is enabled only after Recall@K, MRR, and latency measurements justify it.

Facts URL resolution, post-search AgentFacts fetching, LSH, and NANDA URL/URN data modeling are outside the chosen architecture. If PostgreSQL later reaches capacity, a dedicated Vector Database can replace the adapter without changing the three-stage protocol.
