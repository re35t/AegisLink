# AegisLink frontend design rules

These rules refine the current interface incrementally. They do not authorize a visual rewrite or a new component-library dependency.

## Product personality

AegisLink should combine agent-oriented information density, conversational simplicity, and restrained operational precision. The result should feel like a focused agent workspace: private, observable, capable, and under user control. Do not copy LobeHub, ChatGPT, or Linear branding, layout pixel-for-pixel, or component code.

Use direct product copy. Prefer `Create conversation`, `Cancel run`, `Retry run`, and `Disconnect MCP server` over `OK`, `Submit`, or promotional adjectives. In-progress labels describe the active operation, for example `Connecting…` or `Cancelling…`.

## Application layout

The preferred desktop shell has up to three regions:

1. **Primary navigation**: Agent and first-class product objects.
2. **Workspace**: Conversation or the selected object’s main task.
3. **Optional inspector**: run context, tool activity, Memory, Skill, or MCP detail relevant to the current selection.

The inspector is contextual and optional; do not reserve empty space for it. Chat reading content should normally stay within 760–880 px even when the workspace is wider. Operational tables and object lists may use the available workspace width.

Current implementation uses a 272 px conversation sidebar and one workspace. Evolve it incrementally:

- At wide desktop widths, keep primary navigation stable and open an inspector only when it adds task-relevant detail.
- Near 1024 px, prefer one navigation rail/sidebar plus the workspace; inspector content becomes a drawer or dedicated route.
- Near 390 px, navigation becomes an overlay/drawer, headers lose nonessential metadata, and the composer remains reachable without horizontal scrolling.
- Exactly one element owns vertical scrolling in each major region. Avoid nested full-height scroll containers unless a fixed header/composer requires them.

## Design values and tokens

Introduce shared CSS custom properties when a value is reused across features. Until a token is extracted, reuse the closest existing value rather than adding a near-duplicate.

### Spacing

Use the scale `4 / 8 / 12 / 16 / 24 / 32 / 48`. A 6 px or 10 px value is acceptable for tightly coupled icon/control geometry already present, but new layout gaps and padding should come from the main scale.

### Radius

- 5–6 px: controls and compact list selections.
- 8–10 px: menus, popovers, tool rows, and small surfaces.
- 12–16 px: large dialogs, composer containers, and independent panels.
- 999 px: pills, status indicators, and circular avatars only.

Do not create multiple almost-identical radii within one component family.

### Typography

Use no more than five semantic levels:

| Level           | Typical use                     | Guidance                                             |
| --------------- | ------------------------------- | ---------------------------------------------------- |
| Page title      | selected Agent/object           | 20–24 px, strong, one per workspace                  |
| Section heading | grouped capability or panel     | 16–18 px, semibold                                   |
| Body            | messages, descriptions, forms   | 14–16 px, readable line height                       |
| Secondary       | helper text, muted descriptions | 12–14 px, lower contrast                             |
| Metadata        | timestamps, IDs, event labels   | 11–12 px, never the only carrier of critical meaning |

Avoid changing font size merely to make unrelated elements look different. Use weight, spacing, and alignment first.

### Color and surfaces

Preserve the current neutral shell and lime identity accent while the design system matures. New colors must have a semantic role: canvas, surface, elevated surface, text, muted text, separator, accent, success, warning, danger, or focus. Never use color alone to communicate run/tool status.

Do not turn every section into a card. Establish hierarchy in this order:

1. typography;
2. whitespace;
3. alignment;
4. subtle separators;
5. background surfaces;
6. borders or cards when an object is independently actionable, selectable, movable, or expandable.

## Components and actions

Use four action levels:

- **Primary**: the single dominant action in a local context.
- **Default**: ordinary commands such as retry, edit, or connect.
- **Ghost**: low-emphasis contextual actions such as copy or open details.
- **Destructive**: irreversible or externally destructive actions; require explicit scope and appropriate confirmation.

Avoid multiple visually dominant buttons in one header, form, dialog, or empty state. Icon-only controls require an accessible name and tooltip when the icon is not universally clear.

Every interactive component must define hover, active/selected, focus-visible, disabled, and pending behavior. Pending actions should prevent accidental duplicate submissions without disabling unrelated navigation.

## Motion and feedback

Use motion to explain state changes, not decorate idle screens. Respect `prefers-reduced-motion`. Streamed text, status transitions, drawers, and tool expansion may animate subtly; cancellation, errors, and approvals must remain immediately legible without motion.

## Visual QA targets

For substantial page or shell changes, inspect approximately:

- 1440 × 900: wide workspace and optional inspector behavior.
- 1024 × 768: constrained desktop/tablet navigation and content width.
- 390 × 844: mobile drawer, composer reachability, dialogs, and scroll ownership.

At each target check overflow, spacing rhythm, hierarchy, selected navigation, long names, empty/loading/error states, keyboard focus, dropdown/dialog positioning, and whether the primary action remains clear.
