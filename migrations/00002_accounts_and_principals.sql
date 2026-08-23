-- +goose Up
ALTER TABLE users RENAME TO human_principals;

ALTER TABLE human_principals
    ADD COLUMN status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE agents RENAME COLUMN owner_id TO owner_principal_id;
ALTER TABLE conversations RENAME COLUMN owner_id TO owner_principal_id;
ALTER INDEX conversations_owner_updated_idx RENAME TO conversations_owner_principal_updated_idx;

CREATE TABLE user_accounts (
    id text PRIMARY KEY,
    principal_id text NOT NULL UNIQUE REFERENCES human_principals(id) ON DELETE CASCADE,
    email text NOT NULL UNIQUE CHECK (email = lower(email)),
    password_hash text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE account_sessions (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);

CREATE INDEX account_sessions_account_expires_idx
    ON account_sessions (account_id, expires_at DESC);

-- +goose Down
DROP INDEX account_sessions_account_expires_idx;
DROP TABLE account_sessions;
DROP TABLE user_accounts;

ALTER TABLE conversations RENAME COLUMN owner_principal_id TO owner_id;
ALTER TABLE agents RENAME COLUMN owner_principal_id TO owner_id;
ALTER INDEX conversations_owner_principal_updated_idx RENAME TO conversations_owner_updated_idx;

ALTER TABLE human_principals
    DROP COLUMN updated_at,
    DROP COLUMN status;

ALTER TABLE human_principals RENAME TO users;
