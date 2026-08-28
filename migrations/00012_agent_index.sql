-- +goose Up
ALTER TABLE agent_profiles
    ADD COLUMN configured_at timestamptz;

UPDATE agent_profiles
SET configured_at = updated_at;

CREATE TABLE agent_index_states (
    agent_id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_addr text UNIQUE,
    next_revision bigint NOT NULL DEFAULT 0 CHECK (next_revision >= 0),
    published_revision bigint NOT NULL DEFAULT 0 CHECK (published_revision >= 0),
    last_error text NOT NULL DEFAULT '',
    registered_at timestamptz,
    published_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE agent_index_states;
ALTER TABLE agent_profiles DROP COLUMN configured_at;
