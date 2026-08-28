-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE discovery_representations (
    agent_addr TEXT PRIMARY KEY
        REFERENCES agent_registry(agent_addr) ON DELETE CASCADE,
    revision BIGINT NOT NULL,
    encoder_profile TEXT NOT NULL,
    source_set_digest TEXT NOT NULL,
    vector_count INTEGER NOT NULL,
    request_digest BYTEA NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT discovery_revision_positive CHECK (revision > 0),
    CONSTRAINT discovery_encoder_profile_supported CHECK (
        encoder_profile = 'aegislink-discovery-v1:text-embedding-model:1536:cosine'
    ),
    CONSTRAINT discovery_source_set_digest_format CHECK (
        source_set_digest ~ '^sha256:[0-9a-f]{64}$'
    ),
    CONSTRAINT discovery_vector_count CHECK (vector_count BETWEEN 0 AND 64),
    CONSTRAINT discovery_request_digest_length CHECK (octet_length(request_digest) = 32)
);

CREATE TABLE discovery_fact_vectors (
    agent_addr TEXT NOT NULL
        REFERENCES discovery_representations(agent_addr) ON DELETE CASCADE,
    vector_id TEXT NOT NULL,
    source_digest TEXT NOT NULL,
    embedding VECTOR(1536) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (agent_addr, vector_id),
    CONSTRAINT discovery_vector_id_format CHECK (
        vector_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'
    ),
    CONSTRAINT discovery_source_digest_format CHECK (
        source_digest ~ '^sha256:[0-9a-f]{64}$'
    )
);

-- Exact cosine is the initial correctness baseline. Add an HNSW index in a
-- later migration only after recall and latency measurements justify it.

-- +goose Down
DROP TABLE discovery_fact_vectors;
DROP TABLE discovery_representations;
DROP EXTENSION vector;
