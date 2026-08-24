-- +goose Up
CREATE TABLE skill_packages_v2 (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL REFERENCES human_principals(id) ON DELETE CASCADE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_principal_id, name),
    UNIQUE (id, owner_principal_id)
);

CREATE TABLE skill_versions (
    id text PRIMARY KEY,
    package_id text NOT NULL,
    owner_principal_id text NOT NULL,
    version text NOT NULL,
    description text NOT NULL,
    manifest_content text NOT NULL,
    content_hash text NOT NULL,
    source_type text NOT NULL DEFAULT 'inline' CHECK (source_type IN ('inline', 'local', 'git')),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (package_id, owner_principal_id)
        REFERENCES skill_packages_v2 (id, owner_principal_id) ON DELETE CASCADE,
    UNIQUE (package_id, version),
    UNIQUE (package_id, content_hash),
    UNIQUE (id, package_id, owner_principal_id)
);

CREATE TABLE agent_skills_v2 (
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    package_id text NOT NULL,
    version_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    installed_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, package_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agents (id, owner_principal_id) ON DELETE CASCADE,
    FOREIGN KEY (package_id, owner_principal_id)
        REFERENCES skill_packages_v2 (id, owner_principal_id) ON DELETE CASCADE,
    FOREIGN KEY (version_id, package_id, owner_principal_id)
        REFERENCES skill_versions (id, package_id, owner_principal_id) ON DELETE CASCADE
);

INSERT INTO skill_packages_v2 (id, owner_principal_id, name, created_at, updated_at)
SELECT id, owner_principal_id, name, created_at, updated_at
FROM skills;

INSERT INTO skill_versions (
    id, package_id, owner_principal_id, version, description,
    manifest_content, content_hash, source_type, created_at
)
SELECT id, id, owner_principal_id, version, description,
       manifest_content, content_hash, source_type, created_at
FROM skills;

INSERT INTO agent_skills_v2 (
    owner_principal_id, agent_id, package_id, version_id, enabled, installed_at, updated_at
)
SELECT owner_principal_id, agent_id, skill_id, skill_id, enabled, installed_at, updated_at
FROM agent_skills;

DROP TABLE agent_skills;
DROP TABLE skills;

ALTER TABLE skill_packages_v2 RENAME TO skill_packages;
ALTER TABLE agent_skills_v2 RENAME TO agent_skills;

CREATE INDEX agent_skills_agent_enabled_idx
    ON agent_skills (agent_id, enabled, updated_at DESC);
CREATE INDEX skill_versions_package_created_idx
    ON skill_versions (package_id, created_at DESC);

-- +goose Down
CREATE TABLE skills_v1 (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL REFERENCES human_principals(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL,
    version text NOT NULL DEFAULT 'local',
    manifest_content text NOT NULL,
    content_hash text NOT NULL,
    source_type text NOT NULL DEFAULT 'inline' CHECK (source_type IN ('inline', 'local', 'git')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_principal_id, name),
    UNIQUE (id, owner_principal_id)
);

CREATE TABLE agent_skills_v1 (
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    skill_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    installed_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, skill_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agents (id, owner_principal_id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id, owner_principal_id)
        REFERENCES skills_v1 (id, owner_principal_id) ON DELETE CASCADE
);

INSERT INTO skills_v1 (
    id, owner_principal_id, name, description, version,
    manifest_content, content_hash, source_type, created_at, updated_at
)
SELECT packages.id, packages.owner_principal_id, packages.name,
       selected.description, selected.version, selected.manifest_content,
       selected.content_hash, selected.source_type, packages.created_at, packages.updated_at
FROM skill_packages packages
JOIN LATERAL (
    SELECT versions.description, versions.version, versions.manifest_content,
           versions.content_hash, versions.source_type
    FROM skill_versions versions
    WHERE versions.package_id=packages.id
    ORDER BY versions.created_at DESC, versions.id DESC
    LIMIT 1
) selected ON true;

INSERT INTO agent_skills_v1 (
    owner_principal_id, agent_id, skill_id, enabled, installed_at, updated_at
)
SELECT owner_principal_id, agent_id, package_id, enabled, installed_at, updated_at
FROM agent_skills;

DROP TABLE agent_skills;
DROP TABLE skill_versions;
DROP TABLE skill_packages;

ALTER TABLE skills_v1 RENAME TO skills;
ALTER TABLE agent_skills_v1 RENAME TO agent_skills;

CREATE INDEX agent_skills_agent_enabled_idx
    ON agent_skills (agent_id, enabled, updated_at DESC);
