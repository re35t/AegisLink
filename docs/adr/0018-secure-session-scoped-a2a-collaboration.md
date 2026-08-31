# ADR 0018: Secure session-scoped A2A collaboration

- Status: Accepted
- Date: 2026-08-31

## Context

Index discovery returns opaque AgentAddr candidates but intentionally grants no invocation authority. Users may directly operate only their own Agents. A discovered Agent therefore cannot be attached to the caller's ordinary Conversation or exposed through its owner-scoped APIs.

## Decision

- Keep Assistance Request, target Owner policy, Evaluation Invocation, Collaboration Session, rate/idempotency checks, and audit in `internal/collaboration` on the Agent Server. V1 routes only Agents hosted by the same Server.
- Default target policy to disabled. A no-Tool, single-turn Evaluation Invocation may accept only after hard policy checks; the Server clamps scope and TTL.
- On acceptance, mint an opaque capability whose hash and AES-GCM ciphertext are stored. Never return the raw capability to a browser, model, owner API, event, or log.
- Use official A2A 1.0 AgentCard, Message, Task, Artifact, and JSON-RPC Handler types from `a2a-go/v2`. Map the AgentAddr to the A2A tenant and the Session ID to A2A context/Task ownership.
- Persist A2A Tasks with optimistic versions in PostgreSQL. Each new Message creates an ephemeral Collaboration Invocation that loads target instructions, public Profile context, and bounded Session Task history, then emits one text artifact and is discarded.
- V1 is text/plain only. It exposes no target private Memory, Impressions, Skills, MCP Tools, credentials, normal Conversations, streaming, or push notifications.

## Consequences

The target Agent remains a durable logical object while Runtime instantiation stays demand-driven. The caller never gains owner authority over the target. Cross-Server federation will require a separate Server identity, routing, signature, and trust design rather than weakening this same-Server boundary.
