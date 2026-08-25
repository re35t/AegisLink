# PostgreSQL Schema

PostgreSQL is authoritative for Accounts, Human Principals, Agents, Agent Profiles, Conversations, Messages, Runs, replayable Run Events, Memories, Skills, and MCP configuration.

## Ownership and identity

- `human_principals` is the authorization root for personal data.
- `user_accounts`, `account_sessions`, and `user_preferences` hold login and Human Account state.
- `agents` belongs to a Human Principal and remains the canonical source for Agent name, description, and system prompt.

## Agent Profile

- `agent_profiles` is a one-to-one extension of `agents` and owns the optimistic `version` and avatar URL.
- `agent_profile_facts` stores structured, attributed JSON facts.
- `agent_memory_projections` and `agent_memory_projection_sources` retain derived summaries and their source-Memory lineage.
- `agent_profile_disclosure_policies` stores per-subject disclosure intent for identity, capability, fact, and projection subjects.

## Runtime and integrations

- `conversations`, `messages`, `runs`, and `run_events` are the replayable conversation ledger.
- `memories` stores Agent-scoped semantic and episodic Memory.
- Skill package/version/file tables preserve immutable Skill content; Agent bindings select enabled versions.
- MCP server, tool, and Agent binding tables preserve discovered tool metadata and per-Agent enablement.

Schema evolution is performed only by ordered Goose migrations in `migrations/`. Runtime repositories under `internal/postgres` use GORM and must not invoke `AutoMigrate`.
