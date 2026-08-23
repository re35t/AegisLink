-- +goose Up
CREATE TABLE users (
    id text PRIMARY KEY,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE agents (
    id text PRIMARY KEY,
    owner_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    system_prompt text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE conversations (
    id text PRIMARY KEY,
    owner_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id text NOT NULL REFERENCES agents(id),
    title text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE messages (
    id text PRIMARY KEY,
    conversation_id text NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    run_id text,
    role text NOT NULL CHECK (role IN ('user', 'assistant')),
    content text NOT NULL,
    sequence bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (conversation_id, sequence)
);

CREATE TABLE runs (
    id text PRIMARY KEY,
    conversation_id text NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    input_message_id text NOT NULL REFERENCES messages(id),
    status text NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    failure_code text,
    next_event_sequence bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz
);

ALTER TABLE messages
    ADD CONSTRAINT messages_run_id_fkey FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX runs_one_active_per_conversation
    ON runs (conversation_id)
    WHERE status IN ('queued', 'running');

CREATE TABLE run_events (
    run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    sequence bigint NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, sequence)
);

CREATE INDEX conversations_owner_updated_idx ON conversations (owner_id, updated_at DESC);
CREATE INDEX messages_conversation_sequence_idx ON messages (conversation_id, sequence);

-- +goose Down
DROP TABLE run_events;
DROP INDEX runs_one_active_per_conversation;
ALTER TABLE messages DROP CONSTRAINT messages_run_id_fkey;
DROP TABLE runs;
DROP TABLE messages;
DROP TABLE conversations;
DROP TABLE agents;
DROP TABLE users;
