-- +goose Up
CREATE TABLE user_preferences (
    principal_id text PRIMARY KEY REFERENCES human_principals(id) ON DELETE CASCADE,
    language text NOT NULL DEFAULT 'system'
        CHECK (language IN ('system', 'en', 'zh-CN')),
    theme text NOT NULL DEFAULT 'system'
        CHECK (theme IN ('system', 'light', 'dark')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO user_preferences (principal_id)
SELECT id FROM human_principals
ON CONFLICT (principal_id) DO NOTHING;

-- +goose Down
DROP TABLE user_preferences;
