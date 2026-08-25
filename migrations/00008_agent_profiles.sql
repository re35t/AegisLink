-- +goose Up
CREATE TABLE agent_profiles (
    agent_id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    avatar_url text NOT NULL DEFAULT '' CHECK (char_length(avatar_url) <= 2048),
    version bigint NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agents (id, owner_principal_id) ON DELETE CASCADE
);

INSERT INTO agent_profiles (agent_id, owner_principal_id)
SELECT id, owner_principal_id FROM agents
ON CONFLICT (agent_id) DO NOTHING;

CREATE TABLE agent_profile_facts (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    namespace text NOT NULL CHECK (char_length(namespace) BETWEEN 1 AND 80),
    fact_key text NOT NULL CHECK (char_length(fact_key) BETWEEN 1 AND 120),
    value_json jsonb NOT NULL CHECK (jsonb_typeof(value_json) = 'object'),
    source text NOT NULL CHECK (source IN ('declared', 'memory_projection', 'runtime', 'imported')),
    confidence double precision NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    valid_from timestamptz,
    valid_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE,
    CHECK (valid_until IS NULL OR valid_from IS NULL OR valid_until > valid_from)
);

CREATE INDEX agent_profile_facts_agent_updated_idx
    ON agent_profile_facts (agent_id, updated_at DESC);

CREATE TABLE agent_memory_projections (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    projection_type text NOT NULL CHECK (char_length(projection_type) BETWEEN 1 AND 80),
    summary text NOT NULL CHECK (char_length(summary) BETWEEN 1 AND 4000),
    confidence double precision NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    freshness double precision NOT NULL CHECK (freshness >= 0 AND freshness <= 1),
    generated_at timestamptz NOT NULL,
    expires_at timestamptz,
    status text NOT NULL CHECK (status IN ('candidate', 'accepted', 'rejected', 'stale')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE,
    CHECK (expires_at IS NULL OR expires_at > generated_at)
);

CREATE INDEX agent_memory_projections_agent_status_idx
    ON agent_memory_projections (agent_id, status, generated_at DESC);

CREATE UNIQUE INDEX memories_id_agent_owner_idx
    ON memories (id, agent_id, owner_principal_id);

CREATE TABLE agent_memory_projection_sources (
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    projection_id text NOT NULL,
    memory_id text NOT NULL,
    PRIMARY KEY (projection_id, memory_id),
    FOREIGN KEY (projection_id, agent_id, owner_principal_id)
        REFERENCES agent_memory_projections (id, agent_id, owner_principal_id) ON DELETE CASCADE,
    FOREIGN KEY (memory_id, agent_id, owner_principal_id)
        REFERENCES memories (id, agent_id, owner_principal_id) ON DELETE CASCADE
);

CREATE TABLE agent_profile_disclosure_policies (
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    subject_type text NOT NULL CHECK (subject_type IN ('identity', 'capability', 'fact', 'projection')),
    subject_id text NOT NULL CHECK (char_length(subject_id) BETWEEN 1 AND 512),
    visibility text NOT NULL CHECK (visibility IN ('private', 'authenticated', 'restricted', 'public')),
    channels text[] NOT NULL DEFAULT ARRAY[]::text[],
    indexable boolean NOT NULL DEFAULT false,
    audiences text[] NOT NULL DEFAULT ARRAY[]::text[],
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, subject_type, subject_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE,
    CHECK (channels <@ ARRAY['runtime-context', 'agent-facts', 'agent-card']::text[]),
    CHECK (
        (visibility = 'private' AND NOT channels && ARRAY['agent-facts', 'agent-card']::text[] AND cardinality(audiences) = 0 AND indexable = false)
        OR (visibility = 'restricted' AND cardinality(audiences) > 0 AND indexable = false)
        OR (visibility = 'authenticated' AND cardinality(audiences) = 0 AND indexable = false)
        OR (visibility = 'public' AND cardinality(audiences) = 0 AND (indexable = false OR channels @> ARRAY['agent-facts']::text[]))
    )
);

CREATE INDEX agent_profile_disclosures_agent_idx
    ON agent_profile_disclosure_policies (agent_id, subject_type);

-- +goose Down
DROP TABLE agent_profile_disclosure_policies;
DROP TABLE agent_memory_projection_sources;
DROP INDEX memories_id_agent_owner_idx;
DROP TABLE agent_memory_projections;
DROP TABLE agent_profile_facts;
DROP TABLE agent_profiles;
