# PostgreSQL schema guide

The authoritative schema is the ordered SQL under [`../../migrations`](../../migrations). This guide describes ownership and table responsibilities without duplicating every column or constraint.

The standalone Index has a separate authority boundary and migration history under [`../../index/migrations`](../../index/migrations). Runtime Registry access now uses `agent_registry`, which stores only AgentAddr, schema/status, idempotency/request digests, and timestamps. Migration 00002 copies previously allocated addresses into this table without copying name, Facts URL, cache TTL, or LSH. The legacy `agent_addresses` table remains only for migration compatibility. pgvector Representation tables from the [three-stage Discovery design](../02-architecture/pgvector-discovery-query-pipeline.md) are not implemented.

## Account and Agent ownership

- `human_principals` is the authorization and ownership root.
- `user_accounts` owns email/password login state; `account_sessions` stores hashed session tokens; `user_preferences` stores explicit language/theme values.
- `agents` belongs to one Human Principal and is canonical for Agent name, description, and the separately versioned private system prompt. Runtime composes name and description as structured identity data instead of copying them into the stored prompt.
- Registration creates Account, Principal, default Agent, preferences, and Agent Profile in one transaction.

## Agent Profile

- `agent_profiles` is a one-to-one extension keyed by `agent_id`; `version` tracks identity/Confirmed Fact/policy changes and `context_revision` tracks Impression/candidate changes.
- `agent_impressions` stores permanent short-term observation history; `agent_impression_evidence` preserves provenance. Active items become stale dynamically from observation time, expiry, and decay half-life rather than a stored freshness value.
- `agent_fact_candidates` and `agent_fact_candidate_sources` form the owner review inbox. `agent_confirmed_facts` is the separate trusted state with confirmation, validity, and revocation metadata.
- `agent_profile_jobs` is a durable PostgreSQL work queue. Successful Runs enqueue one idempotent `curate-run` job; workers use leases and `FOR UPDATE SKIP LOCKED`.
- `agent_profile_disclosure_policies` stores rules only for identity, capability, Confirmed Fact, and endpoint subjects.
- Effective capabilities are aggregated from Runtime, Skill, and MCP authority rather than copied into another capability table.

## AgentFacts publication

- `agent_publication_settings` binds one verified hostname and DNS challenge to an Agent.
- `agent_signing_keys` stores Ed25519 public keys and AES-256-GCM-encrypted private keys; master-key material never enters PostgreSQL.
- `agent_access_tokens` stores only SHA-256 token hashes and exact audiences.
- `agent_facts_publications` stores immutable canonical payloads, digests, JWS proofs, expiry, supersession, and revocation.

## Conversation ledger

- `conversations` belongs to both a Principal and an Agent.
- `messages` is the ordered durable transcript.
- `runs` stores lifecycle state, failure information, next event sequence, and an immutable JSON execution-policy snapshot.
- `run_events` is ordered per Run and supports SSE replay. Events are not an audit-log substitute.

## Memory, Skills, and MCP

- `memories` is directly Agent-scoped and supports active/forgotten lifecycle state.
- `skill_packages` belongs to a Principal; `skill_versions` and `skill_version_files` are immutable content; `agent_skills` selects and enables one Version per Agent.
- `mcp_servers` and `mcp_tools` form the Principal library. `agent_mcp_servers` and `agent_mcp_tools` hold per-Agent enablement.

## Evolution and access

Schema changes require a new Goose migration; applied migrations are never rewritten. Runtime access goes through `internal/postgres.Database` and GORM. GORM types do not cross the PostgreSQL adapter, and `AutoMigrate` is prohibited.

Destructive repository integration tests must use a dedicated database whose name ends in `_test`.
