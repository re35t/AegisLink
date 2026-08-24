# Frontend architecture

## Current implementation

The frontend currently lives entirely under `web`:

```text
web/src/
├── api/
│   ├── client.ts           REST client and application-facing API types
│   └── schema.ts           generated from OpenAPI; never hand-edit
├── app/
│   ├── queryClient.ts      TanStack Query configuration
│   └── router.tsx          TanStack Router route composition
├── components/
│   ├── Workspace.tsx       query/mutation orchestration and feature composition
│   ├── AssistantThread.tsx assistant-ui + AG-UI runtime, Tool UI, and Run Inspector
│   ├── MarkdownText.tsx    shared assistant Markdown renderer
│   ├── layout/AppShell.tsx product navigation and responsive shell
│   └── useRunEvents.ts     transitional persisted-event reconnect path
├── features/conversation/
│   ├── ConversationSidebar.tsx  searchable Conversation navigation
│   ├── ConversationWorkspace.tsx Conversation page states and composition
│   └── RunRecovery.tsx          persisted active-Run recovery view
├── features/capabilities/
│   ├── CapabilityShell.tsx      shared Agent-scoped management shell
│   ├── MemoryPage.tsx           inspect/edit/confirm/forget Memory
│   ├── SkillsPage.tsx           author, import, version, and enable Skill bundles
│   └── McpPage.tsx              server discovery and tool permissions
└── styles.css              current global styles and component classes
```

Conversation, Memory, Skills, and MCP management now have real routes. Agent profile and Runs/Traces remain planned. Capability management is grouped because it shares Agent bootstrap/scope and compact CRUD patterns; backend domain ownership remains split across Memory, Skills, and MCP.

## Ownership rules

### Routes and pages

Routes select the product object and compose a shell plus feature entry points. They own route parameters, route-level error boundaries, and page metadata. They should not parse provider events or implement reusable forms.

### Shell and layout

The application shell owns primary navigation, responsive region behavior, and outlet placement. It must not own Memory extraction, MCP connection, Skill validation, or Runtime decisions.

`Workspace.tsx` owns Conversation query/mutation coordination and composes `AppShell`, `ConversationSidebar`, `ConversationWorkspace`, and `RunRecovery`. `CapabilityShell` provides the shared Agent scope and layout for Memory, Skills, and MCP pages. Keep product navigation and feature presentation inside their owning components rather than adding capability branches back into `Workspace.tsx`.

### Feature modules

Introduce `web/src/features/<object>/` when an object gains multiple components, hooks, and tests. A feature may contain its own components, query hooks, schemas for local forms, and adapters. Do not reorganize the existing tree only to match a theoretical structure.

Suggested incremental destinations are:

```text
web/src/
├── components/ui/         AegisLink-wide semantic primitives only
├── components/layout/     shell, navigation, inspector, responsive regions
└── features/
    ├── conversation/
    ├── agent/
    └── capabilities/       split further only when each object gains enough code
```

`features/conversation`, `features/capabilities`, and `components/layout` now exist because real implementation owns those boundaries. Split capability pages into separate directories only when they gain multiple private components/hooks; avoid directory depth that does not clarify ownership.

### Presentation and behavior

- Presentation components receive domain-shaped props and emit semantic callbacks.
- Query hooks coordinate server state, invalidation, and request status.
- assistant-ui Runtime adapters coordinate active Conversation execution.
- API clients serialize transport requests and normalize transport errors; they do not decide product workflow.
- Business decisions and durable authorization remain on the Go server.

Do not let a component both render a large surface and implement fetch/SSE parsing, caching, retry rules, protocol translation, and navigation.

## State model

Use TanStack Query for server state: Agent bootstrap data, Conversations, Messages, Runs, Memory, Skills, and MCP Servers. Invalidate or update query caches after mutations; do not duplicate them into React context or a general store.

Use local React state for transient presentation state such as an open drawer, selected tab, draft-only filter, or expanded tool row. State that must survive reload, be shared across clients, authorize an external effect, or support event replay belongs on the server.

Do not add a client state library until there is a demonstrated cross-route client-state problem that Query, router state, or local state cannot express cleanly.

## API and protocol boundaries

- Change public schemas in `contracts/http/v1/openapi.yaml` first and regenerate `web/src/api/schema.ts` with `make generate`.
- Use `web/src/api/client.ts` as the application-facing REST adapter rather than calling `fetch` throughout components.
- Use REST for durable resource CRUD and bootstrap/history reads.
- Use AG-UI through `@assistant-ui/react-ag-ui` for active Runs. Do not expose internal Eino or provider event shapes to React.
- PostgreSQL history is authoritative. assistant-ui history adapters translate persisted messages into UI messages; they do not become a second database.
- The transitional `useRunEvents.ts` recovery path should be removed only after AG-UI replay/resume has equivalent tested behavior.

## Reusable primitives

Prefer assistant-ui primitives for thread, message, composer, action bar, tool UI, attachments, and approval UI where their semantics fit. Wrap them for AegisLink domain rules and styling; do not fork their behavior casually.

Create an AegisLink-wide primitive only when at least two real consumers share a semantic contract or when accessibility/interaction behavior must be standardized before the second consumer arrives. A shared CSS class alone is not sufficient justification for a component.

## Testing strategy

- Unit-test API normalization, adapters, reducers, and nontrivial hooks with Vitest.
- Component-test states whose behavior is difficult to verify through pure functions.
- Use fake AG-UI/model endpoints in automated tests.
- Browser-test critical Personal Agent workflows against a local backend: create, stream, tool/approval state when implemented, cancel, reload/reconnect, and recover from failure.
- Visual browser review is required for substantial UI work even when unit tests pass.

The repository does not currently include a committed Playwright configuration. Use the available Codex in-app browser for interactive QA. Add Playwright later only when repeatable browser acceptance becomes a maintained CI responsibility, not merely to satisfy one change.
