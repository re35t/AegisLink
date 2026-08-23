# AegisLink engineering instructions

## Product scope

- Build AegisLink as a Personal Agent OS first. Do not introduce multi-agent network concepts unless the task explicitly requires them.
- Treat Agent, Conversation, Memory, Skill, and MCP Server as first-class product objects. Agent Identity, Credentials, Capabilities, Connections, Groups, and Networks are future objects, not reasons to prebuild abstractions now.
- Use canonical product terms consistently. An Agent is a persistent configurable entity; it is not merely the avatar behind a chat message.

## Backend boundaries

- Keep Gin inside `internal/httpapi`. Services and repositories use `context.Context` and domain types only.
- Keep orchestration in `internal/conversation`; do not put business decisions in handlers or SQL scanners.
- Keep model SDK types inside `internal/runtime`. The rest of the application consumes the small `conversation.Runtime` interface.
- PostgreSQL is authoritative for conversations, messages, runs, and replayable run events.
- Add schema changes as a new Goose migration. Never rewrite a migration that may have been applied.
- TypeScript is allowed only under `web` and frontend tooling. Backend and CLI code must be Go.

## Public contracts and agent protocol

- Update `contracts/http/v1/openapi.yaml` before changing a public HTTP request or response, then run `make generate`.
- Do not manually edit `web/src/api/schema.ts`; it is generated from OpenAPI.
- Use REST resources for durable CRUD and bootstrap data. Use AG-UI for active agent execution, lifecycle, text/tool events, cancellation, and eventual replay/resume.
- Keep AG-UI wire types inside `internal/httpapi` and model-provider types inside `internal/runtime`; translate both into application/domain types at their boundaries.
- Never log, persist, return, or commit model API keys.

## Frontend working agreement

For substantial frontend work, read the relevant files under `docs/frontend/` and use `skills/frontend-engineering/SKILL.md`. Start by inspecting the existing implementation and writing a short plan. Do not move directly from a requirement to generated JSX.

Current stack and ownership:

- React 19 + Vite + TypeScript under `web`.
- TanStack Router owns routing; TanStack Query owns remote/server state.
- `assistant-ui` owns chat thread, message, composer, action, and agent-native UI primitives.
- `@assistant-ui/react-ag-ui` and AG-UI own active agent execution transport.
- OpenAPI-generated types and `web/src/api/client.ts` form the normal REST boundary.
- Local component state is for ephemeral presentation state only. Do not mirror query data into a second client store.

Implementation rules:

- Pages and shell components compose features; they do not accumulate large business workflows.
- Before creating a component, search project components, assistant-ui primitives, and the current dependency set. Extend an existing semantic primitive before introducing a second button, panel, empty state, loading indicator, tool-call container, or agent status implementation.
- Keep presentation components independent of raw fetch/SSE details. Put remote-state coordination in API/query hooks and agent execution in the assistant-ui runtime adapter.
- Do not hand-roll chat primitives already supplied by assistant-ui. AegisLink-specific wrappers may add product semantics, persistence integration, permissions, or styling.
- Do not add a second component library, styling framework, state manager, or design-system dependency without a concrete gap and explicit justification.
- Reuse the spacing, radius, typography, color, and layout rules in `docs/frontend/DESIGN.md`. Do not add arbitrary one-off values when an existing token fits.
- Prefer typography, whitespace, alignment, and separators over wrapping every section in a card.
- Every data view must handle applicable loading, empty, error, and retry states. Every interactive control must handle hover, active, focus-visible, disabled, and pending states.
- Preserve keyboard semantics, visible focus, accessible names, and reduced-motion behavior. Use native controls unless a composite widget genuinely requires otherwise.
- Treat narrow layouts as product behavior, not CSS cleanup. Verify navigation, composer, inspectors, dialogs, and scroll ownership at the documented target widths.
- Do not declare UI complete merely because it compiles. Inspect it in a real browser and fix concrete hierarchy, overflow, state, or interaction problems before handoff.

## Frontend change workflow

1. Classify the task: page, feature, component, redesign, or bug.
2. Read `docs/frontend/PRODUCT.md`, `DESIGN.md`, and the relevant architecture/component/interaction sections.
3. Inspect the live route, existing components, API contracts, and all affected states.
4. State the reuse decisions and a small implementation plan.
5. Implement the smallest coherent slice; avoid repository-wide reorganization unless required by the slice.
6. Run static checks and focused tests while iterating.
7. Perform browser QA at the affected desktop and narrow-screen sizes.
8. Review duplication, accessibility, state coverage, and production maturity; refine before declaring completion.

## Validation

- Run `make check`, `make test`, and `make build` before handing off an implementation change.
- Set `TEST_DATABASE_URL` to run PostgreSQL repository integration tests.
- Use a fake model endpoint in automated tests. Real DeepSeek credentials are only for explicit local smoke tests.
- Public contract changes must leave `make generate` clean.
- Browser acceptance for the conversation path must cover creating a conversation, streaming a response, reconnecting/reloading, and cancelling a run.
- Major visual work must inspect approximately 1440x900, 1024x768, and 390x844, or explain why a target is not applicable. Check overflow, scroll ownership, spacing, navigation collapse, composer reachability, focus, dialogs, and error recovery.
- Documentation/rule-only changes do not require live model calls, but links, skill validation, formatting, and `git diff --check` must pass.
