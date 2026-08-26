# ADR 0002: Ed25519 Agent identity

## Status

Superseded by ADR 0014.

## Context

The TypeScript prototype proposed Ed25519 keys for cross-runtime Agent identity and signed Agent-to-Agent messages. The current product has no public Agent identity, registration network, cross-Agent routing, request-signing contract, or key lifecycle.

## Decision

Do not create Agent keypairs or expose signing APIs in the current Personal Agent OS. Revisit the algorithm, key storage, rotation, revocation, recovery, and protocol only after a concrete external Agent communication workflow exists.

Database IDs, Account sessions, and disclosure policies are not cryptographic Agent identity.
