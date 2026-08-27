-- +goose Up
CREATE TABLE agent_addresses (
    agent_id TEXT PRIMARY KEY,
    schema_version TEXT NOT NULL,
    agent_name TEXT NOT NULL,
    facts_url TEXT NOT NULL,
    ttl_seconds INTEGER NOT NULL,
    lsh_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    revision BIGINT NOT NULL DEFAULT 1,
    idempotency_key_hash BYTEA NOT NULL,
    request_digest BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT agent_addresses_agent_id_format CHECK (agent_id ~ '^agent_[0-9A-HJKMNP-TV-Z]{26}$'),
    CONSTRAINT agent_addresses_agent_name_length CHECK (char_length(agent_name) BETWEEN 1 AND 120),
    CONSTRAINT agent_addresses_facts_url_length CHECK (char_length(facts_url) BETWEEN 1 AND 2048),
    CONSTRAINT agent_addresses_ttl_range CHECK (ttl_seconds BETWEEN 60 AND 86400),
    CONSTRAINT agent_addresses_lsh_object CHECK (jsonb_typeof(lsh_json) = 'object'),
    CONSTRAINT agent_addresses_revision_positive CHECK (revision > 0),
    CONSTRAINT agent_addresses_idempotency_hash_length CHECK (octet_length(idempotency_key_hash) = 32),
    CONSTRAINT agent_addresses_request_digest_length CHECK (octet_length(request_digest) = 32),
    CONSTRAINT agent_addresses_facts_url_unique UNIQUE (facts_url),
    CONSTRAINT agent_addresses_idempotency_key_unique UNIQUE (idempotency_key_hash)
);

-- +goose Down
DROP TABLE agent_addresses;
