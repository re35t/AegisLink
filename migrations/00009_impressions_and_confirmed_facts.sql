-- +goose Up
ALTER TABLE agent_profiles
    ADD COLUMN context_revision bigint NOT NULL DEFAULT 1 CHECK (context_revision >= 1);

CREATE TABLE agent_impressions (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    scope text NOT NULL CHECK (scope IN ('user', 'task', 'project', 'environment', 'relationship')),
    impression_kind text NOT NULL CHECK (impression_kind IN (
        'current-task', 'recent-interest', 'knowledge-exposure', 'acquired-information',
        'open-loop', 'temporary-preference', 'working-style-observation', 'recent-decision'
    )),
    summary text NOT NULL CHECK (char_length(summary) BETWEEN 1 AND 4000),
    details_json jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details_json) = 'object'),
    tags jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(tags) = 'array'),
    confidence double precision NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    salience double precision NOT NULL CHECK (salience >= 0 AND salience <= 1),
    first_observed_at timestamptz NOT NULL,
    last_observed_at timestamptz NOT NULL,
    expires_at timestamptz,
    decay_half_life_seconds bigint NOT NULL DEFAULT 2592000 CHECK (decay_half_life_seconds > 0),
    status text NOT NULL CHECK (status IN ('active', 'resolved', 'stale', 'superseded', 'dismissed')),
    superseded_by_id text,
    generator_model text NOT NULL DEFAULT '',
    generator_run_id text NOT NULL DEFAULT '',
    prompt_version text NOT NULL DEFAULT '',
    generated_at timestamptz NOT NULL,
    owner_edited boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE,
    CHECK (expires_at IS NULL OR expires_at > first_observed_at),
    CHECK (last_observed_at >= first_observed_at)
);

CREATE INDEX agent_impressions_agent_status_observed_idx
    ON agent_impressions (agent_id, status, last_observed_at DESC);

CREATE TABLE agent_impression_evidence (
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    impression_id text NOT NULL,
    evidence_kind text NOT NULL CHECK (evidence_kind IN ('message', 'run', 'tool-result', 'memory', 'impression', 'legacy')),
    evidence_id text NOT NULL,
    digest text NOT NULL DEFAULT '',
    observed_at timestamptz NOT NULL,
    PRIMARY KEY (impression_id, evidence_kind, evidence_id),
    FOREIGN KEY (impression_id, agent_id, owner_principal_id)
        REFERENCES agent_impressions (id, agent_id, owner_principal_id) ON DELETE CASCADE
);

CREATE TABLE agent_fact_candidates (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    subject_kind text NOT NULL CHECK (subject_kind IN ('agent', 'user', 'project', 'task')),
    namespace text NOT NULL CHECK (char_length(namespace) BETWEEN 1 AND 80),
    fact_key text NOT NULL CHECK (char_length(fact_key) BETWEEN 1 AND 120),
    value_json jsonb NOT NULL CHECK (jsonb_typeof(value_json) = 'object'),
    value_hash text NOT NULL,
    rationale text NOT NULL DEFAULT '' CHECK (char_length(rationale) <= 2000),
    confidence double precision NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    candidate_version bigint NOT NULL DEFAULT 1 CHECK (candidate_version >= 1),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'promoted', 'rejected')),
    generator_model text NOT NULL DEFAULT '',
    generator_run_id text NOT NULL DEFAULT '',
    prompt_version text NOT NULL DEFAULT '',
    proposed_at timestamptz NOT NULL,
    reviewed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX agent_fact_candidates_pending_value_idx
    ON agent_fact_candidates (agent_id, subject_kind, namespace, fact_key, value_hash)
    WHERE status = 'pending';

CREATE INDEX agent_fact_candidates_agent_status_idx
    ON agent_fact_candidates (agent_id, status, proposed_at DESC);

CREATE TABLE agent_fact_candidate_sources (
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    candidate_id text NOT NULL,
    impression_id text NOT NULL,
    PRIMARY KEY (candidate_id, impression_id),
    FOREIGN KEY (candidate_id, agent_id, owner_principal_id)
        REFERENCES agent_fact_candidates (id, agent_id, owner_principal_id) ON DELETE CASCADE,
    FOREIGN KEY (impression_id, agent_id, owner_principal_id)
        REFERENCES agent_impressions (id, agent_id, owner_principal_id) ON DELETE CASCADE
);

CREATE TABLE agent_confirmed_facts (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    candidate_id text,
    subject_kind text NOT NULL CHECK (subject_kind IN ('agent', 'user', 'project', 'task')),
    namespace text NOT NULL CHECK (char_length(namespace) BETWEEN 1 AND 80),
    fact_key text NOT NULL CHECK (char_length(fact_key) BETWEEN 1 AND 120),
    value_json jsonb NOT NULL CHECK (jsonb_typeof(value_json) = 'object'),
    confidence double precision NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    confirmation_method text NOT NULL CHECK (confirmation_method IN ('owner-confirmed', 'deterministic-verification')),
    confirmed_by text,
    confirmed_at timestamptz NOT NULL,
    valid_from timestamptz,
    valid_until timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, agent_id, owner_principal_id),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE,
    CHECK (valid_until IS NULL OR valid_from IS NULL OR valid_until > valid_from)
);

CREATE INDEX agent_confirmed_facts_agent_active_idx
    ON agent_confirmed_facts (agent_id, namespace, fact_key) WHERE revoked_at IS NULL;

CREATE TABLE agent_profile_jobs (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    source_run_id text NOT NULL UNIQUE REFERENCES runs(id) ON DELETE CASCADE,
    job_kind text NOT NULL CHECK (job_kind IN ('curate-run')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    lease_expires_at timestamptz,
    last_error_code text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (agent_id, owner_principal_id)
        REFERENCES agent_profiles (agent_id, owner_principal_id) ON DELETE CASCADE
);

CREATE INDEX agent_profile_jobs_claim_idx
    ON agent_profile_jobs (status, available_at, created_at);

INSERT INTO agent_impressions (
    id, owner_principal_id, agent_id, scope, impression_kind, summary, details_json,
    confidence, salience, first_observed_at, last_observed_at, expires_at,
    status, generator_model, prompt_version, generated_at, created_at, updated_at
)
SELECT id, owner_principal_id, agent_id, 'user',
       CASE projection_type
           WHEN 'current_task' THEN 'current-task'
           WHEN 'research_interest' THEN 'recent-interest'
           ELSE 'acquired-information'
       END,
       summary, jsonb_build_object('legacyProjectionType', projection_type),
       confidence, 0.5, generated_at, generated_at, expires_at,
       CASE status WHEN 'rejected' THEN 'dismissed' WHEN 'stale' THEN 'stale' ELSE 'active' END,
       'legacy-import', 'profile-v0', generated_at, created_at, updated_at
FROM agent_memory_projections;

INSERT INTO agent_impression_evidence (
    owner_principal_id, agent_id, impression_id, evidence_kind, evidence_id, observed_at
)
SELECT sources.owner_principal_id, sources.agent_id, sources.projection_id, 'memory',
       sources.memory_id, impressions.generated_at
FROM agent_memory_projection_sources sources
JOIN agent_memory_projections impressions ON impressions.id = sources.projection_id;

INSERT INTO agent_impressions (
    id, owner_principal_id, agent_id, scope, impression_kind, summary, details_json,
    confidence, salience, first_observed_at, last_observed_at, status,
    generator_model, prompt_version, generated_at, created_at, updated_at
)
SELECT 'legacy-fact-impression:' || id, owner_principal_id, agent_id, 'user',
       'acquired-information', 'Legacy Fact requires owner review: ' || namespace || '.' || fact_key,
       jsonb_build_object('legacyValue', value_json, 'legacySource', source),
       confidence, 0.5, created_at, updated_at, 'active',
       'legacy-import', 'profile-v0', created_at, created_at, updated_at
FROM agent_profile_facts;

INSERT INTO agent_fact_candidates (
    id, owner_principal_id, agent_id, subject_kind, namespace, fact_key, value_json,
    value_hash, rationale, confidence, generator_model, prompt_version,
    proposed_at, created_at, updated_at
)
SELECT id, owner_principal_id, agent_id, 'user', namespace, fact_key, value_json,
       md5(value_json::text), 'Imported from the pre-confirmation Profile Fact model.',
       confidence, 'legacy-import', 'profile-v0', created_at, created_at, updated_at
FROM agent_profile_facts;

INSERT INTO agent_fact_candidate_sources (owner_principal_id, agent_id, candidate_id, impression_id)
SELECT owner_principal_id, agent_id, id, 'legacy-fact-impression:' || id
FROM agent_profile_facts;

DELETE FROM agent_profile_disclosure_policies WHERE subject_type IN ('fact', 'projection');
ALTER TABLE agent_profile_disclosure_policies
    DROP CONSTRAINT agent_profile_disclosure_policies_subject_type_check;
ALTER TABLE agent_profile_disclosure_policies
    ADD CONSTRAINT agent_profile_disclosure_policies_subject_type_check
    CHECK (subject_type IN ('identity', 'capability', 'confirmed-fact', 'endpoint'));

UPDATE agent_profiles profiles
SET context_revision = context_revision + 1
WHERE EXISTS (SELECT 1 FROM agent_impressions impressions WHERE impressions.agent_id = profiles.agent_id);

DROP TABLE agent_memory_projection_sources;
DROP TABLE agent_memory_projections;
DROP TABLE agent_profile_facts;

-- +goose Down
CREATE TABLE agent_profile_facts (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    namespace text NOT NULL,
    fact_key text NOT NULL,
    value_json jsonb NOT NULL,
    source text NOT NULL DEFAULT 'runtime',
    confidence double precision NOT NULL,
    valid_from timestamptz,
    valid_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE agent_memory_projections (
    id text PRIMARY KEY,
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    projection_type text NOT NULL,
    summary text NOT NULL,
    confidence double precision NOT NULL,
    freshness double precision NOT NULL,
    generated_at timestamptz NOT NULL,
    expires_at timestamptz,
    status text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE agent_memory_projection_sources (
    owner_principal_id text NOT NULL,
    agent_id text NOT NULL,
    projection_id text NOT NULL,
    memory_id text NOT NULL,
    PRIMARY KEY (projection_id, memory_id)
);
ALTER TABLE agent_profile_disclosure_policies
    DROP CONSTRAINT agent_profile_disclosure_policies_subject_type_check;
ALTER TABLE agent_profile_disclosure_policies
    ADD CONSTRAINT agent_profile_disclosure_policies_subject_type_check
    CHECK (subject_type IN ('identity', 'capability', 'fact', 'projection'));
DROP TABLE agent_profile_jobs;
DROP TABLE agent_confirmed_facts;
DROP TABLE agent_fact_candidate_sources;
DROP TABLE agent_fact_candidates;
DROP TABLE agent_impression_evidence;
DROP TABLE agent_impressions;
ALTER TABLE agent_profiles DROP COLUMN context_revision;
