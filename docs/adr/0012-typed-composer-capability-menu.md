# ADR 0012: Typed Composer capability menu

## Status

Accepted.

## Decision

The Conversation Composer uses one server-projected catalog with three explicit item kinds:

- `mcp-tool` with `force-tool-once`;
- `skill` with `use-skill-once`;
- `discovery` with `discover-once`.

The leading Composer action is a compact `+` menu. Its first level contains MCP, Skills, and Discovery, and the selected category opens a child list to the right. The assistant-ui `@` Trigger continues to consume the same catalog and selection state.

The browser sends only the opaque Mention ID and action. The server resolves ownership, Agent binding, enabled state, and the concrete execution policy before creating the Run. The typed policy is stored in `runs.execution_policy` and emitted in `run.started`.

Skill selection forces the existing `load_skill` tool once and validates that the model requested the selected Skill name. Discovery forces a local `discover_capabilities` tool that lists the current Agent's enabled Skills and MCP Tools. It does not implement Internet Agent discovery, a marketplace, or NANDA publication.

## Consequences

MCP Tools, Skills, and Discovery remain separate capability semantics even though they share one picker. New categories can be added without treating every capability as an MCP Tool. The runtime gains a useful local Discovery mode without prematurely adding an Agent network or marketplace domain.
