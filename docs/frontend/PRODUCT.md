# Personal Agent Console product rules

## Scope

The current product is the AegisLink Personal Agent Console: the primary workspace for one person to configure, run, observe, and improve a persistent Agent. It is not a generic chatbot demo and it is not yet a multi-agent network console.

The canonical object model is:

```text
Agent
├── Conversations
├── Memory
├── Skills
├── MCP Servers
├── Identity
└── Capabilities
```

Agent, Conversation, Memory, Skill, and MCP Server are first-class navigation and data concepts. Identity and Capabilities should be visible as agent context when implemented. Credentials, Connections, Groups, Discovery, and Agent Network remain future objects until a concrete workflow requires them.

Do not use Agent, assistant, bot, model, and chat interchangeably:

- **Agent**: persistent configuration, identity, capabilities, memory, skills, and integrations.
- **Conversation**: a durable interaction history with the Agent.
- **Run**: one observable, cancellable execution inside a Conversation.
- **Model**: a replaceable inference provider selected by the Runtime.

## Primary workflows

The Console must eventually make these workflows coherent:

1. Open the personal Agent and understand its identity, model, availability, and capabilities.
2. Start or continue a Conversation without navigating through configuration first.
3. Observe a Run: current status, tool requests/results, errors, cancellation, and recovery.
4. Inspect, correct, confirm, or forget Memory with source attribution.
5. Inspect, enable, disable, and understand Skills, including when a Skill was used.
6. Connect and manage MCP Servers, their health, tools, permissions, and credentials boundary.
7. Inspect Agent identity, status, configuration, and effective capabilities without exposing low-level implementation noise by default.

The default route should optimize workflows 1–3. Workflows 4–7 belong in progressively disclosed object views or an inspector, not in an overloaded chat composer.

## Product principles

### Agent-first

Show which Agent is active and make its configuration/capabilities reachable. A Conversation is one child of an Agent, not the entire product model. Model names are useful diagnostic metadata, not the product identity.

### Dense but calm

Use compact lists and metadata where scanning matters, but keep one clear reading path and one dominant local action. Avoid decorative dashboards, oversized illustrations, and repeated summary cards.

### Progressive disclosure

Keep the default conversation surface focused. Put run details, retrieved memories, tool payloads, MCP metadata, and advanced configuration behind expandable details or an inspector. Never hide approval consequences, failures, or destructive scope.

### Objects over arbitrary pages

Routes, headings, actions, and empty states should be organized around the canonical objects. Do not create miscellaneous “Tools”, “Extensions”, and “Integrations” screens that represent the same MCP/Skill capability differently.

### Observable agent behavior

For every Run, show a comprehensible lifecycle: queued/running, tool activity when relevant, awaiting approval, completed, failed, or cancelled. Preserve enough identifiers and event history for diagnostics without dumping raw chain-of-thought or provider payloads.

### Reversible user actions

Prefer disable, disconnect, undo, retry, edit, and forget workflows over silent destructive changes. State the affected object and scope before a destructive or external-effect action.

### Clear system status

Status labels must say what is happening and what the user can do next. Distinguish offline, reconnecting, running, waiting for approval, failed, and ready. Do not use a global spinner when only one region is pending.

### Avoid configuration overload

Provide safe defaults. Group advanced provider, memory, Skill, and MCP options by responsibility; reveal them when requested. Do not make a user understand Runtime or protocol internals to start a Conversation.

## Definition of a mature product surface

A surface is not complete until its purpose and primary action are clear, canonical objects are named consistently, async state is observable, destructive effects are controlled, and empty/error states offer a next action. A feature that only renders its success state is a prototype.
