-- +goose Up
ALTER TABLE mcp_tools RENAME TO mcp_tools_v1;
ALTER TABLE mcp_servers RENAME TO mcp_servers_v1;

CREATE TABLE mcp_servers (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL REFERENCES human_principals(id) ON DELETE CASCADE,
    name text NOT NULL,
    endpoint text NOT NULL,
    transport text NOT NULL DEFAULT 'streamable-http' CHECK (transport IN ('streamable-http')),
    status text NOT NULL DEFAULT 'unchecked' CHECK (status IN ('unchecked', 'connected', 'error')),
    protocol_version text,
    last_error text,
    last_checked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_principal_id, name),
    UNIQUE (id, owner_principal_id)
);

CREATE TABLE mcp_tools (
    id text PRIMARY KEY,
    server_id text NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    input_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    risk_level text NOT NULL DEFAULT 'read-only'
        CHECK (risk_level IN ('read-only', 'external-write', 'destructive')),
    discovered_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (server_id, name),
    UNIQUE (id, server_id)
);

CREATE TABLE agent_mcp_servers (
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    server_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    installed_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, server_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agents(id, owner_principal_id) ON DELETE CASCADE,
    FOREIGN KEY (server_id, owner_principal_id)
        REFERENCES mcp_servers(id, owner_principal_id) ON DELETE CASCADE
);

CREATE TABLE agent_mcp_tools (
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    server_id text NOT NULL,
    tool_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    installed_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, tool_id),
    FOREIGN KEY (agent_id, server_id)
        REFERENCES agent_mcp_servers(agent_id, server_id) ON DELETE CASCADE,
    FOREIGN KEY (tool_id, server_id)
        REFERENCES mcp_tools(id, server_id) ON DELETE CASCADE
);

INSERT INTO mcp_servers (
    id, owner_principal_id, name, endpoint, transport, status,
    protocol_version, last_error, last_checked_at, created_at, updated_at
)
SELECT id, owner_principal_id, name, endpoint, transport, status,
       protocol_version, last_error, last_checked_at, created_at, updated_at
FROM mcp_servers_v1;

INSERT INTO mcp_tools (
    id, server_id, name, description, input_schema, risk_level, discovered_at, updated_at
)
SELECT 'mcp-tool-' || md5(json_build_array(server_id, name)::text), server_id, name,
       description, input_schema, risk_level, discovered_at, updated_at
FROM mcp_tools_v1;

INSERT INTO agent_mcp_servers (
    owner_principal_id, agent_id, server_id, enabled, installed_at, updated_at
)
SELECT owner_principal_id, agent_id, id, enabled, created_at, updated_at
FROM mcp_servers_v1;

INSERT INTO agent_mcp_tools (
    owner_principal_id, agent_id, server_id, tool_id, enabled, installed_at, updated_at
)
SELECT servers.owner_principal_id, servers.agent_id, tools.server_id,
       'mcp-tool-' || md5(json_build_array(tools.server_id, tools.name)::text),
       tools.enabled, tools.discovered_at, tools.updated_at
FROM mcp_tools_v1 tools
JOIN mcp_servers_v1 servers ON servers.id=tools.server_id;

DROP TABLE mcp_tools_v1;
DROP TABLE mcp_servers_v1;

CREATE INDEX mcp_servers_owner_updated_idx ON mcp_servers(owner_principal_id, updated_at DESC);
CREATE INDEX agent_mcp_servers_agent_enabled_idx ON agent_mcp_servers(agent_id, enabled, updated_at DESC);
CREATE INDEX agent_mcp_tools_agent_enabled_idx ON agent_mcp_tools(agent_id, enabled, updated_at DESC);

ALTER TABLE runs
    ADD COLUMN execution_policy jsonb NOT NULL DEFAULT '{"mode":"auto"}'::jsonb;

-- +goose Down
ALTER TABLE runs DROP COLUMN execution_policy;

CREATE TABLE mcp_servers_v1 (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    name text NOT NULL,
    endpoint text NOT NULL,
    transport text NOT NULL DEFAULT 'streamable-http',
    enabled boolean NOT NULL DEFAULT true,
    status text NOT NULL DEFAULT 'unchecked',
    protocol_version text,
    last_error text,
    last_checked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_id, name),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agents(id, owner_principal_id) ON DELETE CASCADE
);

CREATE TABLE mcp_tools_v1 (
    server_id text NOT NULL REFERENCES mcp_servers_v1(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    input_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    enabled boolean NOT NULL DEFAULT false,
    risk_level text NOT NULL DEFAULT 'read-only',
    discovered_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (server_id, name)
);

INSERT INTO mcp_servers_v1 (
    id, owner_principal_id, agent_id, name, endpoint, transport, enabled, status,
    protocol_version, last_error, last_checked_at, created_at, updated_at
)
SELECT CASE
           WHEN row_number() OVER (PARTITION BY servers.id ORDER BY bindings.agent_id) = 1 THEN servers.id
           ELSE servers.id || '-' || substr(md5(bindings.agent_id), 1, 8)
       END,
       bindings.owner_principal_id, bindings.agent_id, servers.name,
       servers.endpoint, servers.transport, bindings.enabled, servers.status,
       servers.protocol_version, servers.last_error, servers.last_checked_at,
       servers.created_at, servers.updated_at
FROM mcp_servers servers
JOIN agent_mcp_servers bindings ON bindings.server_id=servers.id;

INSERT INTO mcp_tools_v1 (
    server_id, name, description, input_schema, enabled, risk_level, discovered_at, updated_at
)
SELECT legacy_servers.id, tools.name, tools.description, tools.input_schema,
       COALESCE(bindings.enabled, false), tools.risk_level, tools.discovered_at, tools.updated_at
FROM mcp_tools tools
JOIN mcp_servers servers ON servers.id=tools.server_id
JOIN mcp_servers_v1 legacy_servers
  ON legacy_servers.owner_principal_id=servers.owner_principal_id
 AND legacy_servers.name=servers.name
LEFT JOIN agent_mcp_tools bindings
  ON bindings.tool_id=tools.id AND bindings.agent_id=legacy_servers.agent_id;

DROP TABLE agent_mcp_tools;
DROP TABLE agent_mcp_servers;
DROP TABLE mcp_tools;
DROP TABLE mcp_servers;

ALTER TABLE mcp_servers_v1 RENAME TO mcp_servers;
ALTER TABLE mcp_tools_v1 RENAME TO mcp_tools;
CREATE INDEX mcp_servers_agent_enabled_idx ON mcp_servers(agent_id, enabled, updated_at DESC);
