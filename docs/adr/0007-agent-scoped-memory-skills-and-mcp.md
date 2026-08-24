# ADR 0007: Scope Memory, Skills, and MCP to an Agent

Status: Accepted; V0 implemented.

MCP ownership and binding details are revised by [ADR 0011](0011-user-mcp-library-agent-bindings-and-run-tool-selection.md).

Date: 2026-08-23.

## Decision

Memory, Skill bindings, MCP servers, and MCP tools are authorized by both the authenticated Human Principal and the target Personal Agent. Management endpoints are nested under `/agents/{agentId}`; the Principal ID always comes from the server-side session.

Memory and MCP connections are directly Agent-scoped. Principal-owned Skill packages contain immutable versions, while `agent_skills` stores the selected version and enablement per Agent. Every run resolves an `AgentContext` from the authoritative Conversation Agent before invoking Eino.

Memory V0 is explicit and user-editable. Skills follow Agent Skills `SKILL.md` metadata and progressive disclosure. MCP uses the official Go SDK and Streamable HTTP. Discovered tools are disabled by default because server annotations are untrusted hints; only explicitly enabled read-only tools enter the runtime. External-write and destructive tools remain blocked until per-call Approval exists.

## Consequences

- A future account can own multiple Agents without sharing Memory, enabled Skills, or MCP authority accidentally.
- PostgreSQL queries and foreign keys carry both Principal and Agent scope.
- V0 does not yet provide automatic memory extraction/vector retrieval, Skill directory bundles/scripts, MCP OAuth/secrets, or write/destructive Approval.

## References

- [Model Context Protocol](https://modelcontextprotocol.io/specification/2025-06-18/basic/index) and [official Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [Agent Skills specification](https://github.com/agentskills/agentskills/blob/main/docs/specification.mdx) and [client implementation](https://github.com/agentskills/agentskills/blob/main/docs/client-implementation/adding-skills-support.mdx)
- [Open WebUI Memory](https://github.com/open-webui/docs/blob/main/docs/features/chat-conversations/memory.mdx)
- [AegisLink LobeHub reference study](../frontend/references/lobehub.md)
