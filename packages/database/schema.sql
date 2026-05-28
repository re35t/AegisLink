CREATE TABLE users (
  id UUID PRIMARY KEY,
  display_name TEXT NOT NULL,
  email TEXT UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE agents (
  id UUID PRIMARY KEY,
  owner_user_id UUID NOT NULL REFERENCES users(id),
  display_name TEXT NOT NULL,
  public_key TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('active', 'disabled', 'deleted')),
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE organizations (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE org_memberships (
  org_id UUID NOT NULL REFERENCES organizations(id),
  user_id UUID NOT NULL REFERENCES users(id),
  role TEXT NOT NULL,
  PRIMARY KEY (org_id, user_id)
);

CREATE TABLE memory_items (
  id UUID PRIMARY KEY,
  agent_id UUID NOT NULL REFERENCES agents(id),
  source_type TEXT NOT NULL CHECK (source_type IN ('conversation', 'soul_md', 'document', 'external_app')),
  content TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}',
  visibility TEXT NOT NULL CHECK (visibility IN ('private', 'owner', 'team', 'org', 'public')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE agent_capabilities (
  id UUID PRIMARY KEY,
  issuer_agent_id UUID REFERENCES agents(id),
  subject_agent_id UUID NOT NULL REFERENCES agents(id),
  audience_agent_id UUID REFERENCES agents(id),
  actions TEXT[] NOT NULL,
  resources TEXT[] NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ
);

CREATE TABLE audit_logs (
  id UUID PRIMARY KEY,
  actor_agent_id UUID REFERENCES agents(id),
  action TEXT NOT NULL,
  resource TEXT NOT NULL,
  decision TEXT NOT NULL CHECK (decision IN ('allow', 'deny')),
  reason TEXT,
  request JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
