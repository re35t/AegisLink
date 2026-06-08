# AegisLink

AegisLink is a privacy-preserving personal agent communication platform. Each user owns a personal agent with local memory, RAG retrieval, and an Ed25519 crypto identity. The server side provides agent registry, capability policy, signed message routing, and audit logs so agents can collaborate with least-privilege permissions.

The repository is currently an early TypeScript monorepo implementation. It is usable for local development and backend workflow verification, but it is not production-ready yet.

## Current Status

Implemented:

- TypeScript pnpm monorepo skeleton.
- Local `agent-client` SDK with `ask`, `getMemory`, and a remote-agent request placeholder.
- Local `soul.md` memory and JSONL conversation persistence.
- RAG chunking, hash embeddings, and a Chroma-compatible vector adapter.
- Ed25519 identity generation, canonical JSON signing, and signature verification.
- CLI demo for local agent questions.
- In-memory backend server MVP:
  - basic chat API backed by local memory and a configurable model provider;
  - agent registration and lookup;
  - agent status updates;
  - capability issue, list, and revoke;
  - signed message envelope validation;
  - capability-based message routing;
  - allow/deny audit logs;
  - JSON snapshot API for debugging.
- Basic browser console for registering agents, issuing capabilities, and inspecting server state.
- Architecture, storage, server, security, API, data model, integration, and ops docs scaffold.

Not implemented yet:

- PostgreSQL persistence for authoritative metadata.
- Real organization tree and role policy evaluation.
- WebSocket/gRPC streaming.
- Integration gateway and IM adapters.
- Production authentication, rate limiting, deployment manifests, and observability pipeline.
- Polished frontend product UI.

## Quick Start

Install dependencies:

```bash
pnpm install
```

Run type checks and tests:

```bash
pnpm check
pnpm test
```

Start the backend server and basic web console:

```bash
MODEL_PROVIDER=echo pnpm run dev:server
```

Then open:

```text
http://127.0.0.1:4321
```

Use `pnpm run dev:server` instead of `pnpm server`: `server` is also a pnpm built-in command, so `pnpm server` may not execute this repository's script.

To use another port:

```bash
PORT=4322 pnpm run dev:server
```

Health check:

```bash
curl -sS http://127.0.0.1:4321/healthz
```

Inspect the in-memory server state:

```bash
curl -sS http://127.0.0.1:4321/api/snapshot
```

Call the local chat API in echo mode:

```bash
curl -sS http://127.0.0.1:4321/api/chat \
  -H "content-type: application/json" \
  -d '{"question":"What does my agent remember?","agentId":"local-agent"}'
```

To call a real OpenAI-compatible model provider, start the server with:

```bash
MODEL_API_KEY=... MODEL_NAME=... pnpm run dev:server
```

Run the local CLI demo:

```bash
pnpm cli ask "What does my agent remember?"
```

## Backend API

The current backend is intentionally in-memory so the core design can be tested before wiring PostgreSQL.

Available REST endpoints:

- `GET /healthz`
- `GET /api/snapshot`
- `POST /api/chat`
- `GET /api/agents`
- `POST /api/agents`
- `PATCH /api/agents/:agentId/status`
- `GET /api/capabilities`
- `POST /api/capabilities`
- `POST /api/capabilities/:capabilityId/revoke`
- `GET /api/messages`
- `POST /api/messages/route`
- `GET /api/audit`

## Development Progress

Phase 1 is complete as a local-agent scaffold: local memory, CLI, RAG primitives, vector adapter, protocol types, and crypto identity are implemented and covered by tests.

Phase 2 has started. The backend MVP now covers the main server concepts from the docs: registry, permission capability checks, signed envelope verification, routing, and audit. The frontend is deliberately minimal and exists only to exercise backend flows.

Next backend priorities:

1. Move server state from memory into PostgreSQL using `packages/database/schema.sql`.
2. Add organization membership and role-aware policy evaluation.
3. Add request signing helpers to the client SDK so the browser/CLI can build valid signed envelopes end to end.
4. Add WebSocket delivery for routed messages.
5. Expand server tests around expiry, revocation, disabled agents, duplicate nonces, and audit queries.

## Layout

```text
apps/agent-cli          Local CLI demo
apps/server             Backend MVP and basic web console
packages/agent-client   Local agent SDK
packages/storage-core   Storage interfaces
packages/storage-local  Local soul.md and JSONL stores
packages/rag-core       Chunking, embeddings, retrieval
packages/vector-chroma  Chroma-compatible vector adapter
packages/crypto-identity Ed25519 identity and signing helpers
packages/permission-core Capability evaluator
packages/protocol       Message, event, and capability types
docs                    Design and operations documentation
data/agents/local-agent Local demo agent data
```
