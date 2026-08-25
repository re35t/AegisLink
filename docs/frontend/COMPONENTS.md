# Frontend component policy

## Current inventory

| Responsibility                  | Existing implementation                                              | Preferred reuse path                                                                                                                    |
| ------------------------------- | -------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| App shell                       | `components/layout/AppShell.tsx`                                     | Reuse for product navigation, responsive navigation drawer, and workspace composition                                                   |
| Query composition               | `Workspace.tsx`                                                      | Keep remote coordination here until an owning feature query hook is justified                                                           |
| Conversation navigation         | `features/conversation/ConversationSidebar.tsx`                      | Reuse its search, loading, empty, error, selection, and Agent-status patterns                                                           |
| Conversation workspace          | `features/conversation/ConversationWorkspace.tsx`                    | Compose route-level Conversation states and assistant-ui; keep protocol details out                                                     |
| Agent chat runtime              | `AssistantThread.tsx`                                                | Extend assistant-ui primitives and the AG-UI runtime options here or in a future Conversation feature adapter                           |
| Mention Catalog                 | `MentionCatalog.tsx` + assistant-ui unstable Trigger Popover         | Keep unstable API usage inside this wrapper; `@` and the Composer `+` menu share typed MCP, Skill, and Discovery data and one selection |
| Thread/message/composer/actions | `@assistant-ui/react` primitives inside `AssistantThread.tsx`        | Configure, compose, and style these primitives; do not create parallel chat controls                                                    |
| Persisted history adapter       | assistant-ui `ThreadHistoryAdapter` in `AssistantThread.tsx`         | Keep PostgreSQL authoritative and translate API messages at this boundary                                                               |
| Markdown                        | `MarkdownText.tsx` using `@assistant-ui/react-markdown`              | Add safe render extensions here so all assistant messages behave consistently                                                           |
| REST API                        | `api/client.ts` + generated `api/schema.ts`                          | Add typed client operations after updating OpenAPI; never fetch ad hoc from a page                                                      |
| Server cache                    | shared TanStack `queryClient`                                        | Add object-specific query hooks as features grow; do not introduce duplicate caches                                                     |
| Skill management                | `features/capabilities/SkillsPage.tsx`                               | Reuse its custom editor, multipart import, version binding, bundle metadata, and Agent-scoped mutation states                           |
| Run recovery                    | `useRunEvents.ts` + `features/conversation/RunRecovery.tsx`          | Preserve until native AG-UI replay/resume replaces it with equivalent acceptance coverage                                               |
| Icons                           | `lucide-react`                                                       | Reuse existing icons and sizing conventions; do not add a second icon package                                                           |
| Global visual rules             | `styles.css`                                                         | Extract tokens and feature styles incrementally; do not add a styling framework for convenience                                         |
| Account settings                | `features/settings/SettingsPage.tsx`                                 | Reuse for profile, language/theme preferences, password change states, and responsive settings navigation                               |
| Interface preferences           | `features/settings/preferences.tsx`                                  | Read the Query-backed server preference and derive document language/theme; do not mirror it into another store                         |
| Agent Profile                   | `features/agent/AgentProfilePage.tsx` + `DisclosurePolicyEditor.tsx` | Keep identity and policy mutations Query-backed; aggregate server-projected capabilities rather than duplicating Skill/MCP state        |

## Reuse decision

Before adding a component:

1. Search `web/src` for an existing semantic component or pattern.
2. Search assistant-ui when the responsibility is thread, message, composer, attachment, tool, approval, action, or history UI.
3. Search the installed dependency set for an existing accessible primitive.
4. Extend or wrap an existing primitive when AegisLink needs domain semantics or consistent styling.
5. Create a new primitive only when its semantic responsibility is genuinely new and its ownership is clear.

Do not introduce duplicate buttons, dialogs, dropdowns, panels, empty states, loading indicators, tool-call containers, Agent avatars, or status indicators. Similar markup with different labels is usually one component or one documented pattern, not two primitives.

## When to extend

Extend an existing component when:

- interaction and accessibility semantics are unchanged;
- the variation can be expressed through composition, slots, or a small typed prop;
- the new behavior belongs to the same product object;
- the extension does not create a mode-heavy component with unrelated branches.

Prefer composition over boolean-prop accumulation. If a component needs many mutually dependent flags, split the workflow into named variants or feature components.

## When a new primitive is justified

A new shared primitive is justified when it standardizes one or more of:

- keyboard/focus behavior that would otherwise be duplicated;
- canonical object identity/status presentation;
- destructive confirmation and affected scope;
- loading/empty/error/retry semantics for a repeated object pattern;
- inspector or shell-region behavior shared by real features;
- a product-specific wrapper around assistant-ui that protects AG-UI, permission, or persistence invariants.

Keep primitives free of route-specific queries. Feature components may use query hooks; UI primitives receive data and callbacks.

`MentionCatalog.tsx` is the deliberate exception at the assistant-ui boundary: it coordinates the Agent-scoped Mention query, the compact two-level `+` menu, and explicit MCP Tool or Skill enable mutations because these states are part of the picker workflow. Other Composer components consume only the typed `SelectedMention` and do not depend on the unstable Trigger API.

## Component review questions

- Does the component own one recognizable responsibility?
- Could an existing assistant-ui or project primitive express it?
- Are remote state and protocol parsing outside presentation code?
- Are loading, empty, error, disabled, selected, and pending states explicit where applicable?
- Is the accessible name stable and meaningful?
- Does the component still work with long labels, narrow width, and keyboard navigation?
- Is styling based on documented tokens instead of one-off values?
