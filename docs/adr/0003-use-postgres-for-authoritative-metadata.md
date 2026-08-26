# ADR 0003: Use PostgreSQL as the authoritative store

## Status

Accepted and implemented. Persistence mechanics are refined by ADR 0013.

## Decision

PostgreSQL is authoritative for Account and Principal state, Agents and Profiles, Conversations, Messages, Runs and replayable Run Events, Memory, immutable Skill packages/versions/files and Agent bindings, and MCP library/binding metadata.

Ordered Goose migrations define the schema. Runtime repositories use GORM as required by ADR 0013. Client caches, AG-UI input history, Runtime state, and model-provider payloads cannot replace PostgreSQL authority.

Organizations, public Agent directories, cryptographic identities, vector indexes, and audit-service records are not part of the current schema and are not implied by this decision.
