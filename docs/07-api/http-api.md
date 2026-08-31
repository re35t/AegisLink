# HTTP and streaming boundaries

The authoritative public contract is [`../../contracts/http/v1/openapi.yaml`](../../contracts/http/v1/openapi.yaml). Update it first and run `make generate`; never maintain a second endpoint/schema list in this document.

## REST

REST owns durable resources and control operations:

- authentication, session, Account settings, and password changes;
- bootstrap, Agent list, owner-only versioned Agent Instructions, Agent Profile, Impression correction/lifecycle, Fact review/revocation, and disclosure policies;
- owner publication settings, DNS verification, signing-key rotation, one-time access-token creation, and AgentCard Draft preview;
- Agent Memory, Skill packages/import/bindings, MCP library and Agent bindings;
- Mention Catalog projections;
- owner collaboration policy, Assistance Request/Session audit, and Session revocation;
- Conversation/history reads, Run event replay, and cancellation.

All protected routes use the server session. Owner or Principal IDs are never accepted as authorization inputs. Agent-scoped routes validate that the selected Agent belongs to the authenticated Principal. Identity/Fact/policy writes use Profile versions; Impression/candidate writes use context/candidate versions.

## Host-scoped AgentFacts

When publication is enabled after DNS verification, the real HTTP `Host` selects exactly one Agent; `X-Forwarded-Host` is ignored. `/.well-known/agentfacts.json` exposes only public and indexable claims with ETag/TTL caching. `POST /agentfacts/query` keeps selectors out of URL logs, applies anonymous/token audience filtering, returns `no-store`, and does not reveal whether denied claims exist. JWKS and revocation documents have separate well-known routes.

## AG-UI

`POST /api/v1/ag-ui` owns active Agent execution. The current adapter accepts text-only final user input plus an optional opaque capability selection, then emits AG-UI Run, text-message, and Tool-call SSE events.

The client does not authorize Tools by sending AG-UI Tool definitions, schemas, names, or history. The server derives the Conversation Agent and resolves every selection from durable state before creating the Run.

## Persisted Run events

`GET /api/v1/runs/{runId}/events` is the authenticated persisted SSE stream. It supports `Last-Event-ID` replay and remains the recovery source while native AG-UI resume is incomplete. Run events are ordered per Run in PostgreSQL.

## Official A2A collaboration

`GET /a2a/agents/{agentAddr}/.well-known/agent-card.json` returns an official A2A 1.0 AgentCard only when the target Owner enabled collaboration; `/a2a/agents/{agentAddr}/agent-card` remains an explicit API alias. The card declares the shared JSON-RPC interface, AgentAddr tenant, text/plain modes, and opaque Bearer Session security. `POST /a2a` is handled by the official Go SDK and supports Session-scoped SendMessage, Get/List Task, and Cancel Task. Streaming, push notification, external capability issuance, and ordinary user cookies are not accepted on this data plane.

## Standalone Index

The independent contract at [`../../index/contracts/http/v1/openapi.yaml`](../../index/contracts/http/v1/openapi.yaml) exposes health/readiness and all three Bearer-authenticated Index stages: empty-object AgentAddr registration with idempotent replay, complete Representation replacement, and Query Vector Search. It exposes no public resolve route or AgentFacts text.

## Not provided

The Agent Server does not provide federated Server routing, general public A2A invocation without an accepted Collaboration Session, third-party attestation, WebSocket, gRPC, Organization, or external Session issuance APIs.
