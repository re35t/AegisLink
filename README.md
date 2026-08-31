# AegisLink

AegisLink is a local-first personal Agent Web application. The current release is a readable Go modular monolith with an `assistant-ui` React chat interface, an AG-UI execution protocol, an in-process Eino ReAct Agent Runtime, an extensible model-provider registry, cookie-based account sessions, and PostgreSQL-backed conversations and replayable run events. The repository also contains a separately runnable `aegislink-index` with AgentAddr registration, complete vector snapshot publication, and exact cosine search for cross-server Agent discovery. DeepSeek is the active default provider for the Personal Agent server; the Index does not use model credentials.

The product has four deliberately separate paths:

```text
Owner / Web ──REST──────────────> durable resources and policy
Conversation ──AG-UI────────────> active Run ──Harness ──Runtime ──Eino
Successful Run ──durable job────> Curator ──Runtime ──Eino ──Impression / Fact Candidate
AgentProfile ──Disclosure───────> AgentFacts (published) / AgentCard (draft)
```

`AgentProfile` is the private self model used by the owner and Runtime. `AgentFacts` is a filtered, signed, expiring external trust manifest. The broad capability AgentCard remains an owner Draft; an enabled Collaboration Policy produces a separate text-only, Session-secured official A2A 1.0 card. See the [architecture overview](docs/00-overview/architecture-overview.md) and [ADR 0018](docs/adr/0018-secure-session-scoped-a2a-collaboration.md).

## Stack

- Go 1.26, Gin, Eino ADK
- PostgreSQL, GORM (pgx driver), Goose migrations
- React 19, Vite, assistant-ui, AG-UI, TanStack Router, TanStack Query
- OpenAPI-generated frontend types

## Repository map

```text
cmd/aegislink-server/       process entry point
internal/app/               composition root and worker lifecycle
internal/httpapi/           Gin, sessions, REST, AG-UI, Host routes
internal/conversation/      Run orchestration and persisted event lifecycle
internal/harness/           Agent context, instructions, policy, and capability assembly
internal/runtime/           domain-neutral Eino Run and model providers
internal/curator/           isolated no-Tool cognitive model adapter
internal/impression/        Impression/Fact Candidate domain and worker
internal/agent/             Agent and AgentProfile aggregation/policy
internal/discovery/         AgentFacts filtering, signing, query, revocation
internal/a2a/               official A2A AgentCard Draft mapping/readiness
internal/postgres/          all runtime PostgreSQL access through GORM
internal/{account,memory,skills,mcp,catalog}/
                            remaining product domains
migrations/                 ordered Goose schema migrations
contracts/http/v1/          OpenAPI source of truth
web/                        React/Vite application
docs/                       maintained architecture, security, data, ops, ADRs
index/                      independent Agent Index process, contract, and private ports
```

Dependency direction is HTTP -> domain services -> Harness -> Runtime -> Eino. Gin stays in `internal/httpapi`, provider SDK types stay in `internal/runtime`, and GORM/persistence models stay in `internal/postgres`. `internal/app` only wires these boundaries and owns shutdown.

## Agent Runtime

- `ModelRegistry` resolves `MODEL_DRIVER` to a model provider. `deepseek` and `openai-compatible` are registered without exposing model SDK types to the conversation layer.
- `internal/runtime` owns the domain-neutral Eino `ChatModelAgent` Run, model/tool event translation, and provider registry. It does not import AegisLink domain packages.
- `internal/harness` resolves authenticated Agent context, builds instructions and authorized Tools, and delegates each Run to Runtime. Eino can run up to `AGENT_MAX_ITERATIONS` model/tool cycles; the default is 8.
- The Harness supplies `get_current_time`, progressive Skill loading, Discovery, and approved read-only MCP Tools per Run. Long-term Memory is loaded from the authenticated Principal + Agent scope as user-controlled context.
- `MODEL_ID` is a stable model-profile identifier so multiple configured models can be added later without changing the conversation contract.
- Successful Runs enqueue a durable PostgreSQL curator task. `internal/curator` uses a separate one-iteration, no-Tool Runtime to maintain fallible short-term Impressions and conservative Fact candidates; only owner-confirmed Facts become trusted Profile state. Confirmed Facts and at most 12 ranked Impressions enter later Harness context with explicit low-priority boundaries.

At startup the server applies Goose migrations, recovers interrupted Runs, and starts one curator worker. Conversation history, Run events, curator jobs, Profile revisions, and publication state survive process restarts because PostgreSQL is authoritative.

## Web and AG-UI

- REST continues to manage conversations, history, and run controls. `POST /api/v1/ag-ui` accepts a standard `RunAgentInput` and streams AG-UI SSE events.
- assistant-ui now owns the Thread, Message, Composer, cancellation, and auto-scroll experience; `@assistant-ui/react-ag-ui` provides the protocol runtime.
- PostgreSQL history remains authoritative. Historical messages supplied by an AG-UI client do not replace server-side conversation history.
- The current AG-UI slice covers Run lifecycle, text streaming, cancellation, and persisted Tool-call events rendered through assistant-ui. Approvals, attachments, and native AG-UI stream resumption remain future work.

## Memory, Skills, and MCP

- Memory is explicit and reviewable: create, edit, confirm, and forget semantic or episodic entries. PostgreSQL is authoritative and forgotten entries stop entering new runs.
- Skills can be authored as inline Agent Skills-compatible `SKILL.md` instructions or imported from a local Markdown/ZIP bundle. A Principal-owned package contains immutable versions and normalized bundle files, while each Agent independently selects and enables one version. The runtime initially sees only name and description; it loads `SKILL.md` through `load_skill` and may read validated UTF-8 references or scripts through `read_skill_resource`. Imported scripts are never executed automatically.
- MCP V0 uses the official Go SDK and Streamable HTTP. Server discovery caches tool schemas and protocol metadata. Newly discovered tools are disabled because server annotations are untrusted hints.
- Only enabled `read-only` MCP tools enter the runtime. `external-write` and `destructive` tools remain blocked until per-call Approval is implemented.
- Every resource query is scoped by both authenticated `principal_id` and `agent_id`. This preserves isolation when one account can create multiple Personal Agents later.

## Account and Personal Agent

- Registration creates a login Account, its Human Principal (`User` in the API), and one default Personal Agent in a single PostgreSQL transaction. A new account then completes the basic Agent identity setup; the workspace opens only after the Agent is registered and its first vector snapshot is published to Index.
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

`CURATOR_MODEL_*` is optional and falls back field-by-field to `MODEL_*`. The curator keeps the lightweight `deepseek-v4-flash` model by default, enables JSON Output, and sets `CURATOR_MODEL_THINKING=disabled` so structured background curation does not spend its output budget on reasoning. Set that field to `enabled` only when a curator workload demonstrably needs it. AgentFacts signing additionally requires a stable base64 32-byte `AGENT_KEY_ENCRYPTION_KEY`; leaving it unset disables publication without disabling internal Profile/Impression behavior.

## Run locally

```bash
cp .env.example .env
# Set MODEL_API_KEY and adjust MODEL_BASE_URL / MODEL_NAME when needed.
pnpm install
make dev
```

`make dev` loads `.env` and `.env.local`, generates and reuses a stable local-only Agent encryption key when none is configured, waits for both development PostgreSQL databases, then starts the Agent Server, standalone Index, and Vite Web application together. Press `Ctrl-C` to stop the three local processes. The PostgreSQL containers and named volumes remain available for the next run.

The granular targets remain available when only one component is needed:

```bash
make dev-db
make dev-server
make dev-web
make dev-index-db
make dev-index
```

Open `http://127.0.0.1:5173`. The API listens on `http://127.0.0.1:4321`; Index health/readiness is available on `http://127.0.0.1:4331`.

It implements the three-stage Index API: authenticated AgentAddr registration, atomic complete Fact Vector snapshot replacement, and exact cosine Search returning AgentAddr candidates. Public resolve has been removed.

For an OpenAI-compatible gateway, set:

```dotenv
MODEL_DRIVER=openai-compatible
MODEL_BASE_URL=https://your-gateway.example/v1
MODEL_NAME=your-deepseek-model
```

## API

The authoritative endpoint and schema list is [`contracts/http/v1/openapi.yaml`](contracts/http/v1/openapi.yaml). Current resource groups cover health/readiness, authentication, Account settings, owner-only Agent Instructions, Agent Profile/Impression/Fact review, AgentFacts publication and Host-scoped query, AgentCard owner preview, Agent Memory and Skills, MCP bindings, Mention Catalog, Conversations, AG-UI execution, Run Event replay, and cancellation.

The standalone Index has its own narrow source of truth at [`index/contracts/http/v1/openapi.yaml`](index/contracts/http/v1/openapi.yaml). It contains health/readiness plus AgentAddr register, representation publication, and vector search; it does not change the Agent Server contract and has no public resolve route.

`POST /api/v1/ag-ui` streams active execution as AG-UI SSE. `GET /api/v1/runs/{runId}/events` exposes authenticated persisted replay and supports `Last-Event-ID`.

Owner routes under `/api/v1/agents/{agentId}` use the authenticated Principal and return Not Found for non-owned Agents. Public AgentFacts routes are selected by the normalized real HTTP `Host`; they do not trust `X-Forwarded-Host`. Collaboration-enabled Agents expose a registry-addressed official A2A 1.0 AgentCard, including per-Agent discovery at `/a2a/agents/{agentAddr}/.well-known/agent-card.json`, and a shared JSON-RPC endpoint. There is intentionally no ambiguous host-wide `/.well-known/agent-card.json` route for this multi-Agent server.

## Validate

```bash
make generate
make check
make test
make build
```

Run `make test-integration` and `make test-index-integration` to exercise the isolated Agent Server and Index databases. Destructive repository tests reject database names that do not end in `_test`.

Runtime persistence goes through GORM. Goose remains authoritative for ordered schema migrations; the application intentionally does not call `AutoMigrate`.

## Current boundary

This release implements private cognitive Agent Profiles, model-generated Impressions, owner-confirmed Facts, signed/revocable AgentFacts publication, Agent-scoped Memory, versioned Skills, MCP bindings, read-only Tool execution, owner isolation, the standalone three-stage Index MVP, and bounded same-Server cross-owner collaboration. Collaboration is owner-opt-in, text-only, asynchronous, session-scoped, expiring, revocable, audited, and carried as official A2A 1.0 Messages/Tasks; it never creates a normal Conversation for the target Agent. The Index does not yet implement scoped publisher credentials, query rate limiting, or evaluated HNSW retrieval. Federated cross-Server routing, third-party attestation, email verification, password reset, MFA, login rate limiting, delegated access, multiple-Agent creation UI, automatic long-term Memory extraction/vector retrieval, executable Skill scripts, MCP OAuth/secret storage, write/destructive Tool Approval, Redis, WebSocket, and sandboxed runners remain out of scope. See [ADR 0018](docs/adr/0018-secure-session-scoped-a2a-collaboration.md).
