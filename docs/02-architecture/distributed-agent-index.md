# AegisLink Standalone Agent Index Architecture

## Status

The repository already contains an independent Go/Gin `aegislink-index` process, health/readiness, its own PostgreSQL connection, and stage-one address-only registration. Registration accepts an empty object and allocates and persists AgentAddr; name, Facts URL, cache TTL, LSH JSON, and public resolve are absent. Vector publication and search are not implemented.

Index continues through exactly three stages:

1. Agent Server registers and receives an Index-allocated AgentAddr.
2. Agent Server encodes public, indexable AgentFacts with the shared Encoder Profile and uploads a complete vector snapshot.
3. Caller encodes a Query Vector with the same profile; Index searches and directly returns AgentAddr candidates.

See [PostgreSQL + pgvector Three-Stage Discovery Design](./pgvector-discovery-query-pipeline.md) for payloads, sequence diagrams, schema, exact/HNSW SQL, and migration details.

## Deployment boundary

```text
Publishing Agent Server             Standalone Index                 Calling Agent Server
-----------------------             ----------------                 --------------------
AgentFacts + Disclosure  --vector--> Register / Publish API          Query intent
Pinned Encoder                       PostgreSQL + pgvector  <--vector-- Same Encoder
Authoritative raw Facts              Vector candidate search  --addr--> Discovery caller
```

- Index has independent deployment, migrations, and configuration and never connects to an Agent Server database.
- `internal/discovery` owns AgentFacts/disclosure; `index/internal/discovery` owns vector snapshots and retrieval.
- Gin stays in `index/internal/httpapi`; GORM runtime access stays in `index/internal/postgres`; Goose owns schema changes.
- Index runs no encoder and needs no model credential.
- Communication after selecting AgentAddr is outside this Discovery slice.

## Ownership

Agent Server owns complete AgentFacts, disclosure, canonicalization, the versioned Encoder Profile, source digests, and local publication audit.

Index owns allocated AgentAddr values, current representation revisions, profile/source-set metadata, at most 64 Fact Vectors per Agent, status, idempotency digests, and timestamps.

Index stores no Facts URL, AgentFacts JSON, raw Fact text, name, cache TTL, LSH, Hash Signature, routing facet, inverted posting, Conversation, Memory, prompt, credential, or private Skill content.

## Three-stage protocol

### Register

`POST /api/v1/registry/agents` accepts an empty JSON object and allocates `agent_<ULID>` under authentication and idempotency semantics. The response contains only schema version, AgentAddr, and creation time.

### Publish / Update

`PUT /api/v1/registry/agents/{agentAddr}/representation` accepts a complete vector snapshot with a monotonically increasing revision. Agent Server performs disclosure, canonicalization, and encoding before upload. Index atomically replaces the snapshot in one PostgreSQL transaction.

### Search

`POST /api/v1/discovery/search` accepts profile, Query Vector, and Top-K. Exact cosine is the baseline; pgvector HNSW is enabled only when scale requires it. Multiple Fact Vectors are grouped into one candidate per AgentAddr, and Search directly returns AgentAddr values.

The protocol contains no public AgentAddr resolve, Facts URL fetch, AgentFacts post-search verification, or LSH branch.

## Evolution principles

- Start with exact cosine for the controlled network.
- Enable HNSW only after Recall@K, MRR, and p95 latency measurements justify it.
- Atomically replace all vectors for an Agent so deleted Facts cannot remain searchable.
- Activate one Encoder Profile network-wide; upgrades explicitly re-encode and switch profiles.
- If PostgreSQL capacity is exceeded, replace the `VectorSearch` adapter without changing Register/Publish/Search.
- Do not implement LSH or use NANDA URL/URN resolution as the data model.

## Current versus target

| Capability                         | Current repository           | Next target                                 |
| ---------------------------------- | ---------------------------- | ------------------------------------------- |
| Independent Index process/database | Implemented                  | Retain                                      |
| AgentAddr registration             | Implemented                  | Empty request; persist and return AgentAddr |
| Facts URL / empty LSH              | Removed                      | Never enter contract or new storage         |
| Representation publication         | Port skeleton only           | Complete vector snapshot replacement        |
| PostgreSQL pgvector                | Not implemented              | Fixed-dimension Fact Vector table           |
| Discovery Search                   | Not implemented              | Query Vector to AgentAddr Top-K             |
| LSH / Hash Index                   | Legacy port skeleton remains | Remove; never implement                     |

Public HTTP changes must update `index/contracts/http/v1/openapi.yaml` before runtime implementation. This architecture document does not make target routes implemented.
