---
name: frontend-engineering
description: Design, implement, refactor, or review AegisLink frontend pages and components using the repository's Personal Agent product, assistant-ui, AG-UI, design-system, state, accessibility, and browser-QA conventions.
---

# AegisLink frontend engineering

Use this workflow for frontend features, page work, component refactors, redesign requests, and UI bugs in the AegisLink repository. Do not use it for backend-only or documentation-only tasks unless frontend rules are the deliverable.

## Establish context

1. Read the nearest `AGENTS.md`.
2. Read `docs/frontend/PRODUCT.md` and `docs/frontend/DESIGN.md`.
3. Read the relevant sections of `docs/frontend/ARCHITECTURE.md`, `docs/frontend/COMPONENTS.md`, and `docs/frontend/INTERACTIONS.md`.
4. Inspect the affected route in code and, when it exists, in a browser.
5. Classify the task as page, feature, component, redesign, or bug. State a short implementation plan before editing.

Do not assume the requested feature already exists. Conversation is currently implemented; Agent profile, Memory, Skills, MCP management, and Runs/Traces are incremental product slices.

## Make reuse and ownership decisions

Before creating UI, search in this order:

1. existing `web/src` components and patterns;
2. assistant-ui primitives for thread, message, composer, tool, approval, attachment, action, and history responsibilities;
3. installed dependencies;
4. a new AegisLink semantic component only for a real uncovered responsibility.

Identify where state belongs:

- TanStack Query for server state;
- router state for navigable selection/filter state;
- local React state for ephemeral presentation state;
- assistant-ui AG-UI runtime for active Agent execution;
- Go/PostgreSQL for durable workflow, authorization, and replay.

Do not add another UI library, styling system, state manager, or hand-written chat primitive to solve a local convenience problem.

## Implement a coherent slice

- Preserve the canonical Agent/Conversation/Memory/Skill/MCP Server object model.
- Change OpenAPI first for public contract changes and regenerate the frontend schema.
- Keep pages compositional and transport details out of presentation components.
- Use documented spacing, typography, radius, color roles, and surface hierarchy.
- Include applicable loading, empty, error, retry, disabled, pending, selected, and focus-visible states as part of the slice.
- Keep external effects observable, cancellable when possible, and approval-gated according to server policy.
- Avoid broad folder migration or visual redesign unless it is necessary for the requested outcome.

## Inspect and refine

Static success is not completion. After implementation:

1. Run focused type/lint/tests during iteration.
2. Render the affected flow in a real browser.
3. Inspect the relevant states and approximately 1440x900, 1024x768, and 390x844 for major visual work.
4. Check hierarchy, overflow, scroll ownership, long content, navigation collapse, composer/control reachability, keyboard focus, dialogs, and error recovery.
5. Compare the result against the review checklist below and fix concrete problems.
6. Run `make check`, `make test`, and `make build` before handoff.

For Conversation work, browser acceptance includes create, stream, reload/reconnect, and cancel. Use a fake endpoint in automated tests; use real model credentials only for an explicitly authorized local smoke test.

## Review checklist

- **Product:** Is the surface's purpose and primary action clear? Does it use canonical objects and terms?
- **Information architecture:** Is grouping logical, navigation shallow enough, and important state visible?
- **Hierarchy:** Are typography and whitespace doing the work, or are cards/borders compensating for weak structure?
- **Components:** Were existing and assistant-ui primitives reused? Is responsibility focused and duplication removed?
- **States:** Are loading, empty, error, retry, disabled, pending, active/selected, hover, and focus-visible states covered where applicable?
- **Responsive:** Do desktop, constrained desktop, and mobile preserve the primary workflow without accidental overflow or scroll traps?
- **Accessibility:** Do keyboard order, semantic controls, accessible names, visible focus, contrast, and reduced motion remain usable?
- **Production maturity:** What specifically still makes this look or behave like a generated prototype? Fix those issues before completion.

Report what changed, what was reused, which states and viewports were inspected, commands run, and any boundary not verified. Never claim browser, responsive, accessibility, or real-runtime behavior from compilation alone.
