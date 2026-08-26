-- +goose Up
ALTER TABLE agents
    ADD COLUMN system_prompt_version bigint NOT NULL DEFAULT 1
        CHECK (system_prompt_version >= 1);

UPDATE agents
SET system_prompt = 'Be concise and reliable. Answer in the language used by the user.',
    updated_at = now()
WHERE system_prompt = 'You are Aegis, a concise and reliable personal assistant. Answer in the language used by the user.';

-- +goose Down
ALTER TABLE agents
    DROP COLUMN system_prompt_version;
