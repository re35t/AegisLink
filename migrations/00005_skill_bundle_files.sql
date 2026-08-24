-- +goose Up
CREATE TABLE skill_version_files (
    version_id text NOT NULL REFERENCES skill_versions(id) ON DELETE CASCADE,
    path text NOT NULL CHECK (char_length(path) BETWEEN 1 AND 240),
    media_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0 AND size_bytes <= 1048576),
    content_hash text NOT NULL,
    text_readable boolean NOT NULL DEFAULT false,
    content bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (version_id, path),
    CHECK (octet_length(content) = size_bytes)
);

INSERT INTO skill_version_files (
    version_id, path, media_type, size_bytes, content_hash, text_readable, content, created_at
)
SELECT id, 'SKILL.md', 'text/markdown', octet_length(convert_to(manifest_content, 'UTF8')),
       content_hash, true, convert_to(manifest_content, 'UTF8'), created_at
FROM skill_versions;

CREATE INDEX skill_version_files_version_path_idx
    ON skill_version_files (version_id, path);

-- +goose Down
DROP TABLE skill_version_files;
