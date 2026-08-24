# ADR 0010: Account settings and explicit interface preferences

## Status

Accepted

## Decision

AegisLink exposes authenticated account settings through REST. Account profile and interface preferences are owned by the Human Principal, not by a Personal Agent.

The first durable preferences are explicit `language` and `theme` fields in PostgreSQL. The API does not accept an arbitrary JSON settings bag. Password changes require the current password, keep the requesting session active, and revoke the account's other active sessions.

The React client reads settings through TanStack Query. A small provider derives the effective system language and color scheme and applies them to the document. Form drafts remain local component state and only replace Query data after a successful mutation.

## Consequences

- A future second Agent shares the owner's interface preferences but keeps its own Memory, Skills, MCP connections, and runtime configuration.
- New durable preferences require an explicit migration, validation, and API contract change.
- Email remains read-only until a verified email-change workflow exists.
- The current language slice covers the application navigation and settings surface; additional product copy can adopt the same translation boundary incrementally.
