# ADR 0005: Use AG-UI and assistant-ui

Status: Accepted, first slice implemented.

Date: 2026-08-22; first slice implemented on 2026-08-23.

## Context

ADR 0004 established the Go/Gin modular monolith, in-process Eino runtime, PostgreSQL, and React/Vite. The current Web application supports basic chat, SSE streaming, event replay, reconnect, and cancellation, but it does not yet have a unified component system for messages, tools, approvals, attachments, and MCP. The Agent-to-Web contract is also limited to a private subset of run events.

AegisLink is currently being developed as a Personal Agent OS. It should reuse mature Agent UI components and establish a stable frontend protocol for Memory, Skills, MCP, tools, approvals, and tracing.

## Decision

- Continue to use Go/Gin for the backend, with Gin confined to `internal/httpapi`.
- Continue to use Eino for the Agent Runtime; Eino and model SDK types remain inside `internal/runtime`.
- Keep PostgreSQL authoritative for conversations, messages, runs, events, and future Agent metadata.
- Adopt AG-UI for active Agent-to-Web interaction, including run lifecycle, text streaming, tool calls, state updates, cancellation, and human approval.
- Keep React 19 + Vite and do not migrate to Next.js.
- Adopt `assistant-ui` as the Agent UI component library and prefer `@assistant-ui/react-ag-ui` for the AG-UI endpoint.
- Reuse `assistant-ui` Thread, Composer, message parts, tool UI, attachments, approvals, history, and MCP management before building equivalents. Keep product-specific Memory, Skills, Profile, and Runs/Traces screens as AegisLink domain components.
- Continue to use OpenAPI for ordinary REST resources and control endpoints. AG-UI owns active Agent interaction; neither replaces the other.

## Boundaries

```text
React/Vite + assistant-ui
          │
       AG-UI
          │
Go/Gin HTTP adapter
          │
internal/conversation
          │
internal/runtime (Eino)
          │
     Model Provider

PostgreSQL persists conversations, runs, events, traces, and configuration metadata.
```

- AG-UI wire types must not enter conversation domain interfaces. The HTTP adapter maps them to and from internal run events.
- Eino SDK types must not enter HTTP handlers or Web contracts.
- MCP credentials remain server-side and must not be exposed through AG-UI, OpenAPI responses, run events, or logs.
- `assistant-ui` must not define backend domain models. Extend gaps through wrappers, slots, or custom message/tool components first.
- Keep the current SSE API available during migration, but do not extend a private event protocol that overlaps AG-UI.

## Consequences

- The backend now provides `POST /api/v1/ag-ui` and maps internal Run Events to AG-UI lifecycle and text events.
- The Web application now uses `@assistant-ui/react`, `@assistant-ui/react-ag-ui`, and `@ag-ui/client`; assistant-ui primitives own the normal chat thread.
- PostgreSQL history continues to load through REST. Historical messages in an AG-UI request are not trusted as authoritative history. The legacy Run Event SSE remains available for active-run replay after a page refresh during migration.
- The first slice covers text streaming, successful and failed terminal events, and request cancellation. Structured tool events, approvals, attachments, state, and native AG-UI reconnect/resume remain incremental work.
- Browser acceptance must cover text streaming, tool calls, approval, cancellation, reconnect, and event replay.
- CopilotKit, Next.js, and Multi-Agent/A2A are outside this decision.
