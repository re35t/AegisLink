# ADR 0015: Separate the Agent Harness from the Eino Runtime

Status: Accepted.

Date: 2026-08-26.

## Context

The original Eino adapter accumulated Agent context resolution, prompt composition, Skill and MCP Tools, selection policy, and the unrelated Impression curator. This made the Runtime depend on Conversation, Agent, and Impression domain types and obscured the boundary between a generic Agent Run and AegisLink product behavior.

## Decision

- `internal/runtime` implements one domain-neutral Eino Agent Run. It owns Eino/model-provider types, generic messages and Tools, streaming events, iteration limits, and generic per-Run Tool choice.
- `internal/harness` resolves Principal/Agent-scoped context, composes instructions, assembles authorized capabilities, applies AegisLink execution policy, and implements the `conversation.Harness` port.
- `internal/curator` implements `impression.Curator` with a dedicated one-iteration, no-Tool Runtime.
- The Agent execution direction is `conversation -> harness -> runtime -> Eino`; the cognitive path is `impression.Worker -> curator -> runtime -> Eino`.
- Runtime must not import AegisLink domain packages. Only Runtime may directly import Eino SDK packages.

## Consequences

Conversation persistence and cancellation remain independent from Eino. Product capabilities can evolve in Harness without expanding the Runtime contract, and the Curator cannot acquire conversation Tools accidentally.
