# AegisLink

AegisLink is a local-first personal Agent Web application. The current release is a readable Go modular monolith with an `assistant-ui` React chat interface, an AG-UI execution protocol, an in-process Eino ReAct Agent Runtime, an extensible model-provider registry, cookie-based account sessions, and PostgreSQL-backed conversations and replayable run events. DeepSeek is the active default provider.

## Stack

- Go 1.26, Gin, Eino ADK
- PostgreSQL, pgx, Goose migrations
- React 19, Vite, assistant-ui, AG-UI, TanStack Router, TanStack Query
- OpenAPI-generated frontend types

## Agent Runtime

- `ModelRegistry` resolves `MODEL_DRIVER` to a model provider. `deepseek` and `openai-compatible` are registered without exposing model SDK types to the conversation layer.
- Eino `ChatModelAgent` runs up to `AGENT_MAX_ITERATIONS` model/tool cycles; the default is 8.
- A read-only `get_current_time` tool proves the base ReAct loop. Enabled Agent Skills and approved read-only MCP tools are resolved per run and registered dynamically.
- Long-term semantic/episodic memories are loaded from the authenticated Principal + Agent scope and added as user-controlled context.
- `MODEL_ID` is a stable model-profile identifier so multiple configured models can be added later without changing the conversation contract.

## Web and AG-UI

- REST continues to manage conversations, history, and run controls. `POST /api/v1/ag-ui` accepts a standard `RunAgentInput` and streams AG-UI SSE events.
- assistant-ui now owns the Thread, Message, Composer, cancellation, and auto-scroll experience; `@assistant-ui/react-ag-ui` provides the protocol runtime.
- PostgreSQL history remains authoritative. Historical messages supplied by an AG-UI client do not replace server-side conversation history.
- The current AG-UI slice covers run lifecycle, text streaming, and cancellation. Structured Tool UI, approvals, attachments, and native AG-UI stream resumption remain future work.

## Memory, Skills, and MCP

- Memory is explicit and reviewable: create, edit, confirm, and forget semantic or episodic entries. PostgreSQL is authoritative and forgotten entries stop entering new runs.
- Skills can be authored as inline Agent Skills-compatible `SKILL.md` instructions or imported from a local Markdown/ZIP bundle. A Principal-owned package contains immutable versions and normalized bundle files, while each Agent independently selects and enables one version. The runtime initially sees only name and description; it loads `SKILL.md` through `load_skill` and may read validated UTF-8 references or scripts through `read_skill_resource`. Imported scripts are never executed automatically.
- MCP V0 uses the official Go SDK and Streamable HTTP. Server discovery caches tool schemas and protocol metadata. Newly discovered tools are disabled because server annotations are untrusted hints.
- Only enabled `read-only` MCP tools enter the runtime. `external-write` and `destructive` tools remain blocked until per-call Approval is implemented.
- Every resource query is scoped by both authenticated `principal_id` and `agent_id`. This preserves isolation when one account can create multiple Personal Agents later.

## Account and Personal Agent

- Registration creates a login Account, its Human Principal (`User` in the API), and one default Personal Agent in a single PostgreSQL transaction.
- Login returns the current User and Personal Agent and creates an opaque server-side session. The browser receives only an `HttpOnly`, `SameSite=Strict` cookie; the database stores only the session-token hash.
- Passwords are hashed with Argon2id. Existing agent, conversation, message, run, and event operations are scoped by the authenticated Principal.
- `AUTH_COOKIE_SECURE=false` supports local HTTP development. Set it to `true` behind production HTTPS.

Current DeepSeek configuration:

```dotenv
MODEL_ID=deepseek-primary
MODEL_DRIVER=deepseek
MODEL_BASE_URL=https://api.deepseek.com
MODEL_NAME=deepseek-v4-flash
AGENT_MAX_ITERATIONS=8
```

Provide the API key only through `MODEL_API_KEY` in an uncommitted `.env`; never place it in source, documentation, or logs.

## Run locally

```bash
cp .env.example .env
# Set MODEL_API_KEY and adjust MODEL_BASE_URL / MODEL_NAME when needed.
pnpm install
make dev-db
make dev-server
```

In another terminal:

```bash
make dev-web
```

Open `http://127.0.0.1:5173`. The API listens on `http://127.0.0.1:4321`.

For an OpenAI-compatible gateway, set:

```dotenv
MODEL_DRIVER=openai-compatible
MODEL_BASE_URL=https://your-gateway.example/v1
MODEL_NAME=your-deepseek-model
```

## API

- `GET /healthz`, `GET /readyz`
- `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`
- `GET /api/v1/auth/session`
- `GET /api/v1/bootstrap`, `GET /api/v1/agents`
- `GET|POST /api/v1/agents/:agentId/memories`
- `PATCH|DELETE /api/v1/agents/:agentId/memories/:memoryId`
- `GET|POST /api/v1/agents/:agentId/skills`
- `PATCH|DELETE /api/v1/agents/:agentId/skills/:skillId`
- `GET|POST /api/v1/agents/:agentId/mcp-servers`
- `PATCH|DELETE /api/v1/agents/:agentId/mcp-servers/:serverId`
- `POST /api/v1/agents/:agentId/mcp-servers/:serverId/refresh`
- `PATCH /api/v1/agents/:agentId/mcp-servers/:serverId/tools/:toolName`
- `POST /api/v1/ag-ui`
- `GET|POST /api/v1/conversations`
- `GET /api/v1/conversations/:conversationId`
- `POST /api/v1/conversations/:conversationId/messages`
- `GET /api/v1/runs/:runId/events`
- `POST /api/v1/runs/:runId/cancel`

The event endpoint is SSE and supports replay with `Last-Event-ID`. Public shapes are defined in `contracts/http/v1/openapi.yaml`.

## Validate

```bash
make generate
make check
make test
make build
```

Set `TEST_DATABASE_URL` to include the PostgreSQL repository integration test.

## Current boundary

This release implements email/password authentication, server-side sessions, Principal-owned Personal Agents, agent-scoped Memory/Skills/MCP V0, local Skill bundle import, read-only Skill resources, read-only dynamic MCP execution, and owner isolation. It does not yet include email verification, password reset, MFA, login rate limiting, delegated access, Ed25519 Agent identity, multiple-Agent creation UI, automatic memory extraction/vector retrieval, Git Skill sources/signatures, executable Skill scripts, MCP OAuth/secret storage, write/destructive Tool Approval, cross-agent communication, Redis, WebSocket, or sandboxed runners. Historical design documents are retained under `docs` and clearly separated from the current implementation baseline.
