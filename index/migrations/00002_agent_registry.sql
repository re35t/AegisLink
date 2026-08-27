-- +goose Up
CREATE TABLE agent_registry (
    agent_addr TEXT PRIMARY KEY,
    schema_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    idempotency_key_hash BYTEA NOT NULL,
    request_digest BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT agent_registry_addr_format CHECK (agent_addr ~ '^agent_[0-9A-HJKMNP-TV-Z]{26}$'),
    CONSTRAINT agent_registry_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT agent_registry_idempotency_hash_length CHECK (octet_length(idempotency_key_hash) = 32),
    CONSTRAINT agent_registry_request_digest_length CHECK (octet_length(request_digest) = 32),
    CONSTRAINT agent_registry_idempotency_key_unique UNIQUE (idempotency_key_hash)
);

-- Preserve previously allocated addresses and idempotency ownership while
-- deliberately dropping legacy name, Facts URL, cache TTL, and LSH data from
-- the new Registry model. The request digest is SHA-256 of the canonical empty
-- registration object: {}.
INSERT INTO agent_registry (
    agent_addr,
    schema_version,
    status,
    idempotency_key_hash,
    request_digest,
    created_at,
    updated_at
)
SELECT
    agent_id,
    'aegislink.agent-addr/0.2-draft',
    'active',
    idempotency_key_hash,
    decode('44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a', 'hex'),
    created_at,
    updated_at
FROM agent_addresses
ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE agent_registry;
