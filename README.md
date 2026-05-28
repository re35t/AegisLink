# AegisLink

AegisLink is a privacy-preserving personal agent communication platform. Each user owns a personal agent with local memory, RAG retrieval, and a crypto identity. Later phases add a central server for agent registry, organization structure, permission policy, routing, and audit.

## Phase 1

Implemented now:

- TypeScript pnpm monorepo skeleton.
- Local `agent-client` SDK with `ask`, `getMemory`, and `requestAgent` stub.
- Local `soul.md` memory and JSONL conversation persistence.
- RAG chunking, hash embeddings, and Chroma-compatible vector adapter.
- Ed25519 identity generation, signing, and verification.
- CLI demo.
- Docs scaffold for architecture, storage, server, security, APIs, data model, integrations, and ops.

## Commands

```bash
pnpm install
pnpm check
pnpm test
pnpm cli ask "What does my agent remember?"
```

## Layout

```text
apps/agent-cli
packages/agent-client
packages/storage-core
packages/storage-local
packages/rag-core
packages/vector-chroma
packages/crypto-identity
packages/protocol
docs
data/agents/local-agent
```
