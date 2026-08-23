# LobeHub reference study

This is a design-engineering reference, not an implementation dependency. The observations are based on LobeHub's official [repository overview](https://github.com/lobehub/lobehub), [DESIGN.md](https://github.com/lobehub/lobehub/blob/canary/DESIGN.md), and [AGENTS.md](https://github.com/lobehub/lobehub/blob/canary/AGENTS.md), inspected on 2026-08-23. Do not copy substantial source code, branding, or product scope.

## Information architecture

LobeHub treats Agent, Workspace, Memory, Skill, Integration/MCP, Provider, and evaluation resources as canonical product concepts. Conversation is central but sits within a broader Agent workspace. Memory is presented as structured and user-editable, while MCP is a capability/integration surface rather than a collection of chat-only buttons.

For AegisLink, adopt the object clarity but reduce scope to one personal Agent. Keep Agent, Conversation, Memory, Skill, and MCP Server visible as related objects. Do not introduce marketplace, team, community, evaluation, or multi-agent hierarchy before a real workflow exists.

## Layout anatomy

The useful pattern is a stable navigation context, a focused main task, and progressively disclosed detail/configuration. Dense lists serve selection; the main workspace serves creation or execution; drawers/panels reveal secondary context.

AegisLink should adapt this as an optional three-region shell rather than copying exact navigation. The current Conversation sidebar can evolve into object navigation plus a Conversation feature region when the next first-class surface is implemented. Mobile should collapse secondary regions instead of shrinking all columns.

## Component hierarchy

LobeHub's repository guidance defines a component selection order and keeps design rules outside individual feature prompts. The transferable lesson is explicit reuse priority, not its particular Ant Design/@lobehub/ui stack.

AegisLink's priority is:

1. existing AegisLink component;
2. assistant-ui primitive for agent/chat behavior;
3. an already installed accessible dependency;
4. a new AegisLink semantic primitive.

Do not adopt LobeHub's component packages or styling stack merely because they are mature; AegisLink currently has a smaller React/Vite surface and should avoid parallel systems.

## Interaction patterns

Patterns worth adopting include clear canonical verbs, local async feedback, controllable Memory, visible tool activity, progressively disclosed technical detail, and explicit recovery actions. Agent execution should feel observable without displaying raw internal reasoning.

For AegisLink this maps naturally to assistant-ui plus AG-UI lifecycle/tool/approval events, backed by PostgreSQL replay. Approval and external-effect scope should remain visible even in a simplified default interface.

## Design-system patterns

LobeHub's design documentation treats copy, terminology, feedback, and control as part of the design system, not merely colors and components. It also gives coding agents a short root rule file that points to deeper guidance.

AegisLink should adopt:

- stable canonical nouns and verbs;
- design rules expressed as observable UI decisions;
- a documented component reuse order;
- explicit loading/empty/error/approval behavior;
- responsive and visual inspection as completion criteria;
- concise root agent instructions linked to detailed documents.

## Patterns that do not fit AegisLink now

- Multi-agent teams, discovery, community, marketplace, and evaluation navigation.
- Broad theme customization or multiple conversation presentation modes.
- LobeHub's Next.js, Zustand/SWR, Ant Design, and proprietary UI package architecture.
- Dense settings for every model/provider capability on the default route.
- Copying its visual identity, terminology that conflicts with AegisLink objects, or large-scale component abstraction before real consumers exist.

## Proposed AegisLink adaptations

- Keep Conversation as the default workspace while making the persistent Agent identity and capabilities reachable.
- Introduce the application shell when building the next real object surface, not as an isolated mockup.
- Use an optional inspector for Run/tool/Memory context instead of adding permanent dashboard cards.
- Make Memory source, edit, confirmation, and forgetting controls visible when Memory V0 is implemented.
- Present MCP Servers through connection health, available tools, permission scope, and reconnect/error state—not a marketplace clone.
- Keep default screens simple; expose protocol IDs, raw tool payloads, and provider diagnostics behind details.

## Implemented foundation

The 2026-08-23 frontend foundation adopts the reference selectively: a compact product rail establishes Agent-object navigation, a separate searchable Conversation panel handles history, and the assistant-ui workspace remains content-first. Memory, Skills, and MCP are visible as disabled future destinations rather than fake working pages. AegisLink keeps its own neutral/lime identity, React/Vite stack, assistant-ui primitives, AG-UI transport, and PostgreSQL history model.
