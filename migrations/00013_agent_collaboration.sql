-- +goose Up
CREATE TABLE agent_collaboration_policies (
    agent_id text PRIMARY KEY REFERENCES agents(id) ON DELETE CASCADE,
    owner_principal_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    max_session_ttl_seconds bigint NOT NULL DEFAULT 3600 CHECK (max_session_ttl_seconds BETWEEN 300 AND 86400),
    max_requests_per_hour integer NOT NULL DEFAULT 20 CHECK (max_requests_per_hour BETWEEN 1 AND 200),
    max_active_sessions integer NOT NULL DEFAULT 5 CHECK (max_active_sessions BETWEEN 1 AND 50),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id) REFERENCES agents(id, owner_principal_id) ON DELETE CASCADE
);

CREATE TABLE assistance_requests (
    id text PRIMARY KEY,
    requester_owner_principal_id text NOT NULL,
    requester_agent_id text NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    target_owner_principal_id text NOT NULL,
    target_agent_id text NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    target_agent_addr text NOT NULL,
    purpose text NOT NULL,
    requested_scopes jsonb NOT NULL,
    idempotency_key_hash bytea NOT NULL,
    request_digest bytea NOT NULL,
    status text NOT NULL CHECK (status IN ('evaluating', 'accepted', 'rejected', 'failed')),
    decision_code text NOT NULL DEFAULT '',
    session_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    evaluated_at timestamptz,
    UNIQUE (requester_agent_id, idempotency_key_hash),
    FOREIGN KEY (requester_agent_id, requester_owner_principal_id) REFERENCES agents(id, owner_principal_id) ON DELETE CASCADE,
    FOREIGN KEY (target_agent_id, target_owner_principal_id) REFERENCES agents(id, owner_principal_id) ON DELETE CASCADE
);
CREATE INDEX assistance_requests_target_created_idx ON assistance_requests (target_agent_id, created_at DESC);
CREATE INDEX assistance_requests_requester_created_idx ON assistance_requests (requester_agent_id, created_at DESC);

CREATE TABLE collaboration_sessions (
    id text PRIMARY KEY,
    assistance_request_id text NOT NULL UNIQUE REFERENCES assistance_requests(id) ON DELETE CASCADE,
    requester_owner_principal_id text NOT NULL,
    requester_agent_id text NOT NULL,
    target_owner_principal_id text NOT NULL,
    target_agent_id text NOT NULL,
    target_agent_addr text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'revoked', 'expired')),
    scopes jsonb NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    encrypted_token bytea NOT NULL,
    token_nonce bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    last_used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    FOREIGN KEY (requester_agent_id, requester_owner_principal_id) REFERENCES agents(id, owner_principal_id) ON DELETE CASCADE,
    FOREIGN KEY (target_agent_id, target_owner_principal_id) REFERENCES agents(id, owner_principal_id) ON DELETE CASCADE
);
ALTER TABLE assistance_requests
    ADD CONSTRAINT assistance_requests_session_fkey FOREIGN KEY (session_id) REFERENCES collaboration_sessions(id) ON DELETE SET NULL;
CREATE INDEX collaboration_sessions_requester_idx ON collaboration_sessions (requester_agent_id, created_at DESC);
CREATE INDEX collaboration_sessions_target_idx ON collaboration_sessions (target_agent_id, created_at DESC);

CREATE TABLE collaboration_invocations (
    id text PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('evaluation', 'collaboration')),
    assistance_request_id text REFERENCES assistance_requests(id) ON DELETE CASCADE,
    session_id text REFERENCES collaboration_sessions(id) ON DELETE CASCADE,
    task_id text,
    target_agent_id text NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('running', 'succeeded', 'failed', 'cancelled')),
    failure_code text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);
CREATE INDEX collaboration_invocations_session_idx ON collaboration_invocations (session_id, started_at DESC);

CREATE TABLE collaboration_a2a_tasks (
    id text PRIMARY KEY,
    session_id text NOT NULL REFERENCES collaboration_sessions(id) ON DELETE CASCADE,
    context_id text NOT NULL,
    state text NOT NULL,
    task_json jsonb NOT NULL,
    version bigint NOT NULL CHECK (version >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX collaboration_a2a_tasks_session_updated_idx ON collaboration_a2a_tasks (session_id, updated_at DESC, id);
CREATE INDEX collaboration_a2a_tasks_context_idx ON collaboration_a2a_tasks (session_id, context_id);

-- +goose Down
DROP TABLE collaboration_a2a_tasks;
DROP TABLE collaboration_invocations;
ALTER TABLE assistance_requests DROP CONSTRAINT assistance_requests_session_fkey;
DROP TABLE collaboration_sessions;
DROP TABLE assistance_requests;
DROP TABLE agent_collaboration_policies;
