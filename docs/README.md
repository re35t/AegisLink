# AegisLink technical documentation

This directory documents the current Go/React Personal Agent OS. It is organized by authority so implemented behavior is not confused with planning material.

For a fast orientation, read in this order:

1. [Architecture overview](./00-overview/architecture-overview.md) / [中文](./00-overview/architecture-overview_cn.md): deployable units, four primary paths, module ownership, and repository map.
2. [Cognitive Profile and publication](./02-architecture/cognitive-profile-and-publication.md) / [中文](./02-architecture/cognitive-profile-and-publication_cn.md): Memory -> Impression -> Confirmed Fact -> Profile -> AgentFacts/AgentCard.
3. [Distributed Agent Index](./02-architecture/distributed-agent-index.md) / [中文](./02-architecture/distributed-agent-index_cn.md): standalone Index boundary and the Register/Publish/Search protocol.
4. [pgvector Discovery query pipeline](./02-architecture/pgvector-discovery-query-pipeline.md) / [中文](./02-architecture/pgvector-discovery-query-pipeline_cn.md): Fact Vector publication, exact/HNSW execution, storage schema, and AgentAddr results.
5. [Security boundary](./06-security/security-boundary.md) / [中文](./06-security/security-boundary_cn.md): trust, ownership, Tool, Curator, signing, and publication constraints.
6. [HTTP and streaming boundaries](./07-api/http-api.md) / [中文](./07-api/http-api_cn.md): REST, AG-UI, durable events, and Host-scoped public routes.
7. [PostgreSQL schema guide](./08-data-model/postgres-schema.md) / [中文](./08-data-model/postgres-schema_cn.md): authoritative storage ownership and revisions.

## Sources of truth

1. [`../contracts/http/v1/openapi.yaml`](../contracts/http/v1/openapi.yaml) defines the Agent Server REST contract; [`../index/contracts/http/v1/openapi.yaml`](../index/contracts/http/v1/openapi.yaml) independently defines the Index contract.
2. [`../migrations`](../migrations) defines the Agent Server PostgreSQL schema; [`../index/migrations`](../index/migrations) independently defines the Index Registry and pgvector schema.
3. [`../internal`](../internal), [`../index/internal`](../index/internal), and [`../web/src`](../web/src) define runtime behavior within their process boundaries.
4. Accepted [ADRs](./adr) explain architectural decisions; later ADRs override earlier ones when they say so explicitly.
5. This documentation explains the implementation but must not redefine contracts or schema independently.

## Current system reference

- [Architecture overview](./00-overview/architecture-overview.md) / [中文](./00-overview/architecture-overview_cn.md)
- [Cognitive Profile and publication](./02-architecture/cognitive-profile-and-publication.md) / [中文](./02-architecture/cognitive-profile-and-publication_cn.md)
- [Distributed Agent Index](./02-architecture/distributed-agent-index.md) / [中文](./02-architecture/distributed-agent-index_cn.md)
- [PostgreSQL + pgvector Discovery pipeline](./02-architecture/pgvector-discovery-query-pipeline.md) / [中文](./02-architecture/pgvector-discovery-query-pipeline_cn.md)
- [Glossary](./00-overview/glossary.md) / [中文](./00-overview/glossary_cn.md)
- [Product roadmap](./01-product/roadmap.md) / [中文](./01-product/roadmap_cn.md)
- [HTTP and streaming boundaries](./07-api/http-api.md) / [中文](./07-api/http-api_cn.md)
- [PostgreSQL schema guide](./08-data-model/postgres-schema.md) / [中文](./08-data-model/postgres-schema_cn.md)
- [Security boundary](./06-security/security-boundary.md) / [中文](./06-security/security-boundary_cn.md)
- [Local development](./10-ops/local-dev.md) / [中文](./10-ops/local-dev_cn.md)
- [Docker Compose](./10-ops/docker-compose.md) / [中文](./10-ops/docker-compose_cn.md)
- [CI](./10-ops/ci-cd.md) / [中文](./10-ops/ci-cd_cn.md)

The architectural summary is intentionally centralized in the overview and cognitive/publication documents. API, schema, and security guides describe only their own authority boundaries instead of copying a second full system design.

## Frontend engineering

- [`frontend/PRODUCT.md`](frontend/PRODUCT.md): product objects, workflows, and maturity criteria.
- [`frontend/DESIGN.md`](frontend/DESIGN.md): layout, tokens, hierarchy, responsive behavior, and visual QA.
- [`frontend/ARCHITECTURE.md`](frontend/ARCHITECTURE.md): React/Vite ownership and state boundaries.
- [`frontend/COMPONENTS.md`](frontend/COMPONENTS.md): component inventory and reuse policy.
- [`frontend/INTERACTIONS.md`](frontend/INTERACTIONS.md): Run, Tool, recovery, error, and Agent Profile interactions.
- [`frontend/references/lobehub.md`](frontend/references/lobehub.md): non-authoritative product/design reference.

Substantial frontend work also follows [`../skills/frontend-engineering/SKILL.md`](../skills/frontend-engineering/SKILL.md).

## Decisions and plans

- ADR 0001 is explicitly superseded by ADR 0004.
- ADR 0002 is superseded by ADR 0014, which limits Ed25519 signing to AgentFacts publication rather than pretending it is a general Agent identity protocol.
- ADR 0003–0016 record accepted decisions. ADR 0011 revises the MCP ownership statement in ADR 0007. ADR 0016 defines the independent three-stage Index boundary; the current implementation now covers its initial Register, Publish, and Search slices.
- [`01-product/personal-agent-os-plan_cn.md`](./01-product/personal-agent-os-plan_cn.md) is a planning record. Its implemented/deferred checklists are useful context, but current code, contracts, migrations, and ADRs take precedence.

The removed prototype documents described a TypeScript CLI, `soul.md`, JSONL history, Chroma/RAG, agent routing, organizations, gRPC, WebSocket, IM adapters, and Kubernetes services that do not exist in this repository. They were short placeholders rather than maintained specifications; Git history remains the archive if that research is needed.
