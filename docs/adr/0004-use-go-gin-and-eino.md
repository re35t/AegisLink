# ADR 0004: Use Go, Gin, and an in-process Eino runtime

Status: Accepted.

## Context

The TypeScript prototype proved local memory, identity, and capability concepts but left the server in memory and did not provide a usable chat product. The first rebuild prioritizes readable code, fast delivery, and a durable Web conversation path.

## Decision

- Use a Go modular monolith and keep Gin limited to the HTTP gateway.
- Use Eino ADK `ChatModelAgent` and `Runner` as an in-process Agent Runtime.
- Support DeepSeek and OpenAI-compatible model endpoints through configuration.
- Store users, agents, conversations, messages, runs, and replayable events in PostgreSQL.
- Keep React, Vite, TanStack Router, and TanStack Query in the Web application.
- Remove all backend TypeScript and defer the prototype's capability, signing, RAG, and cross-agent features.

## Consequences

There is one deployable server and no second runtime protocol yet. Model SDK types cannot cross the runtime interface. PostgreSQL events provide SSE replay, while active Eino execution remains process-local and interrupted runs fail explicitly after a restart.
