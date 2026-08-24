-- +goose Up
CREATE UNIQUE INDEX agents_id_owner_principal_idx
    ON agents (id, owner_principal_id);

CREATE TABLE memories (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('semantic', 'episodic')),
    content text NOT NULL CHECK (char_length(content) BETWEEN 1 AND 4000),
    confidence double precision NOT NULL DEFAULT 1 CHECK (confidence >= 0 AND confidence <= 1),
    source_uri text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'forgotten')),
    last_confirmed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agents (id, owner_principal_id) ON DELETE CASCADE
);

CREATE INDEX memories_agent_status_updated_idx
    ON memories (agent_id, status, updated_at DESC);

CREATE TABLE skills (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL REFERENCES human_principals(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL,
    version text NOT NULL DEFAULT 'local',
    manifest_content text NOT NULL,
    content_hash text NOT NULL,
    source_type text NOT NULL DEFAULT 'inline' CHECK (source_type IN ('inline', 'local', 'git')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_principal_id, name),
    UNIQUE (id, owner_principal_id)
);

CREATE TABLE agent_skills (
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    skill_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    installed_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, skill_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agents (id, owner_principal_id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id, owner_principal_id)
        REFERENCES skills (id, owner_principal_id) ON DELETE CASCADE
);

CREATE INDEX agent_skills_agent_enabled_idx
    ON agent_skills (agent_id, enabled, updated_at DESC);

CREATE TABLE mcp_servers (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    name text NOT NULL,
    endpoint text NOT NULL,
    transport text NOT NULL DEFAULT 'streamable-http' CHECK (transport IN ('streamable-http')),
    enabled boolean NOT NULL DEFAULT true,
    status text NOT NULL DEFAULT 'unchecked' CHECK (status IN ('unchecked', 'connected', 'error')),
    protocol_version text,
    last_error text,
    last_checked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_id, name),
    UNIQUE (id, owner_principal_id, agent_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agents (id, owner_principal_id) ON DELETE CASCADE
);

CREATE INDEX mcp_servers_agent_enabled_idx
    ON mcp_servers (agent_id, enabled, updated_at DESC);

CREATE TABLE mcp_tools (
    server_id text NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    input_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    enabled boolean NOT NULL DEFAULT false,
    risk_level text NOT NULL DEFAULT 'read-only'
        CHECK (risk_level IN ('read-only', 'external-write', 'destructive')),
    discovered_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (server_id, name)
);

-- +goose Down
DROP TABLE mcp_tools;
DROP TABLE mcp_servers;
DROP TABLE agent_skills;
DROP TABLE skills;
DROP TABLE memories;
DROP INDEX agents_id_owner_principal_idx;
