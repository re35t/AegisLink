# ADR 0011: User MCP library, Agent bindings, and per-Run Tool selection

Status: Accepted; initial read-only slice implemented.

Date: 2026-08-24.

## Context

An MCP Server is installed by a Human Principal, but permission to use its Tools belongs to a particular Personal Agent. Storing the same Server configuration separately for every Agent would duplicate discovery state and make future multi-Agent management harder. Treating a browser-provided Tool definition as authority would also allow a client to bypass server ownership, risk, connection, or enablement checks.

The composer additionally needs an extensible `@` catalog. The first slice only exposes MCP Tools, while the catalog shape must be able to add other kinds later without creating a second Tool registry.

## Decision

MCP capability state is split into three durable scopes:

```text
Human Principal MCP library
  mcp_servers -> mcp_tools
        │
        ▼
Personal Agent bindings
  agent_mcp_servers -> agent_mcp_tools
        │
        ▼
Run execution policy snapshot
  auto | force-tool-once
```

- `mcp_servers` and discovered `mcp_tools` belong to the authenticated Principal. A Tool has a stable opaque ID; refreshing a same-name Tool preserves that ID and its Agent bindings. Newly discovered Tools have no Agent binding and are therefore disabled.
- `agent_mcp_servers` and `agent_mcp_tools` store independent enablement per Agent. Enabling a Tool ensures its Server binding exists but does not enable sibling Tools.
- `GET /agents/{agentId}/mentions` is a projection over the library and Agent bindings, not a second `tool_list` table. The first supported kind is `mcp-tool` and availability explains whether an item is ready, needs enablement, is offline, is disabled, or requires approval.
- The browser sends only an opaque Mention ID and `force-tool-once` action through AG-UI `forwardedProps`. The Conversation service derives the authoritative Agent from the Conversation and resolves the selection again using the authenticated Principal, Agent bindings, Server state, Tool state, and risk level before creating the Run.
- Every Run stores an `execution_policy` JSON snapshot. `run.started` repeats the public snapshot for replay and inspection.
- Eino receives an already-authorized qualified Tool name. A per-Run immutable model wrapper applies `ToolChoiceForced` with that single allowed name to the first model call only. Later calls use normal automatic selection. Ignored or mismatched forced calls fail with stable codes.
- Only read-only Tools are eligible. External-write and destructive Tools remain blocked until a persisted per-call approval workflow exists.

## Consequences

- One Principal can install a Server once while keeping Tool authority isolated across multiple Agents.
- Client-supplied AG-UI `tools`, Tool names, and schemas are never authorization inputs.
- Deleting a library Server affects every Agent and must be presented as a destructive cross-Agent action.
- The Mention Catalog can add new `kind`, `category`, `group`, and `action` values later without changing the Run authorization rule.
- This ADR revises ADR 0007's statement that MCP Servers are directly Agent-scoped. Memory remains Agent-owned; Skill and MCP packages are Principal-owned with Agent bindings.

## Deferred

- MCP credentials and OAuth.
- Write/destructive Tool approval and idempotency.
- Mention kinds for Skills, built-in Tools, Agents, or people.
- Multiple simultaneous forced selections or multi-Agent scheduling.
