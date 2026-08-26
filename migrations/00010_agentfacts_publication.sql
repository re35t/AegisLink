-- +goose Up
CREATE TABLE agent_publication_settings (
    agent_id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    public_id text NOT NULL UNIQUE,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    enabled boolean NOT NULL DEFAULT false,
    hostname text NOT NULL DEFAULT '',
    hostname_status text NOT NULL DEFAULT 'unconfigured' CHECK (hostname_status IN ('unconfigured', 'pending', 'verified')),
    dns_challenge text NOT NULL DEFAULT '',
    hostname_verified_at timestamptz,
    ttl_seconds bigint NOT NULL DEFAULT 86400 CHECK (ttl_seconds BETWEEN 3600 AND 2592000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE,
    CHECK (hostname <> '' OR hostname_status = 'unconfigured')
);

CREATE UNIQUE INDEX agent_publication_settings_hostname_unique_idx
    ON agent_publication_settings (hostname) WHERE hostname <> '';

INSERT INTO agent_publication_settings (agent_id, owner_principal_id, public_id)
SELECT agent_id, owner_principal_id, 'agent_' || md5(agent_id || ':' || clock_timestamp()::text || ':' || random()::text)
FROM agent_profiles
ON CONFLICT (agent_id) DO NOTHING;

CREATE TABLE agent_signing_keys (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    public_key bytea NOT NULL,
    encrypted_private_key bytea NOT NULL,
    nonce bytea NOT NULL,
    fingerprint text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'retired', 'revoked')),
    created_at timestamptz NOT NULL DEFAULT now(),
    retired_at timestamptz,
    revoked_at timestamptz,
    UNIQUE (id, agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX agent_signing_keys_one_active_idx
    ON agent_signing_keys (agent_id) WHERE status = 'active';

CREATE TABLE agent_access_tokens (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
    audience text NOT NULL CHECK (char_length(audience) BETWEEN 1 AND 120),
    scopes jsonb NOT NULL DEFAULT '["agent-facts:query"]'::jsonb CHECK (jsonb_typeof(scopes) = 'array'),
    expires_at timestamptz NOT NULL,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE,
    CHECK (scopes <@ '["agent-facts:query"]'::jsonb)
);

CREATE INDEX agent_access_tokens_agent_active_idx
    ON agent_access_tokens (agent_id, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE agent_facts_publications (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    signing_key_id text NOT NULL,
    profile_version bigint NOT NULL,
    payload_json jsonb NOT NULL CHECK (jsonb_typeof(payload_json) = 'object'),
    digest text NOT NULL,
    protected_header text NOT NULL,
    signature text NOT NULL,
    issued_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'superseded', 'revoked', 'expired')),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE,
    FOREIGN KEY (signing_key_id, agent_id, owner_principal_id)
        REFERENCES agent_signing_keys (id, agent_id, owner_principal_id)
);

CREATE UNIQUE INDEX agent_facts_publications_one_active_idx
    ON agent_facts_publications (agent_id) WHERE status = 'active';

CREATE INDEX agent_facts_publications_host_lookup_idx
    ON agent_facts_publications (agent_id, status, expires_at);

-- +goose Down
DROP TABLE agent_facts_publications;
DROP TABLE agent_access_tokens;
DROP TABLE agent_signing_keys;
DROP TABLE agent_publication_settings;
