package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/re35t/AegisLink/internal/agent"
)

type AgentProfileRepository struct {
	database *sql.DB
}

var _ agent.ProfileRepository = (*AgentProfileRepository)(nil)

func NewAgentProfileRepository(database *sql.DB) *AgentProfileRepository {
	return &AgentProfileRepository{database: database}
}

func (repository *AgentProfileRepository) GetProfile(ctx context.Context, principalID, agentID string) (agent.ProfileRecord, error) {
	var item agent.ProfileRecord
	err := repository.database.QueryRowContext(ctx, `
		SELECT agent_id, owner_principal_id, avatar_url, version, created_at, updated_at
		FROM agent_profiles WHERE owner_principal_id=$1 AND agent_id=$2`, principalID, agentID).Scan(
		&item.AgentID, &item.OwnerPrincipalID, &item.AvatarURL, &item.Version, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return agent.ProfileRecord{}, agent.ErrNotFound
	}
	if err != nil {
		return agent.ProfileRecord{}, fmt.Errorf("get agent profile: %w", err)
	}
	return item, nil
}

func (repository *AgentProfileRepository) ListProfileFacts(ctx context.Context, principalID, agentID string) ([]agent.ProfileFact, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, namespace, fact_key, value_json, source, confidence, valid_from, valid_until, created_at, updated_at
		FROM agent_profile_facts
		WHERE owner_principal_id=$1 AND agent_id=$2
		ORDER BY namespace, fact_key, created_at, id`, principalID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list agent profile facts: %w", err)
	}
	defer rows.Close()
	items := make([]agent.ProfileFact, 0)
	for rows.Next() {
		var item agent.ProfileFact
		var value []byte
		if err := rows.Scan(
			&item.ID, &item.Namespace, &item.Key, &value, &item.Source, &item.Confidence,
			&item.ValidFrom, &item.ValidUntil, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent profile fact: %w", err)
		}
		if err := json.Unmarshal(value, &item.Value); err != nil {
			return nil, fmt.Errorf("decode agent profile fact: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent profile facts: %w", err)
	}
	return items, nil
}

func (repository *AgentProfileRepository) ListMemoryProjections(ctx context.Context, principalID, agentID string) ([]agent.MemoryProjection, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT projections.id, projections.projection_type, projections.summary,
		       COALESCE(json_agg(sources.memory_id ORDER BY sources.memory_id)
		           FILTER (WHERE sources.memory_id IS NOT NULL), '[]'::json),
		       projections.confidence, projections.freshness, projections.generated_at,
		       projections.expires_at, projections.status
		FROM agent_memory_projections projections
		LEFT JOIN agent_memory_projection_sources sources
		  ON sources.projection_id=projections.id
		WHERE projections.owner_principal_id=$1 AND projections.agent_id=$2
		GROUP BY projections.id
		ORDER BY projections.generated_at DESC, projections.id`, principalID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list agent memory projections: %w", err)
	}
	defer rows.Close()
	items := make([]agent.MemoryProjection, 0)
	for rows.Next() {
		var item agent.MemoryProjection
		var sourceIDs []byte
		if err := rows.Scan(
			&item.ID, &item.Type, &item.Summary, &sourceIDs, &item.Confidence, &item.Freshness,
			&item.GeneratedAt, &item.ExpiresAt, &item.Status,
		); err != nil {
			return nil, fmt.Errorf("scan agent memory projection: %w", err)
		}
		if err := json.Unmarshal(sourceIDs, &item.SourceMemoryIDs); err != nil {
			return nil, fmt.Errorf("decode agent memory projection sources: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent memory projections: %w", err)
	}
	return items, nil
}

func (repository *AgentProfileRepository) ListDisclosurePolicies(ctx context.Context, principalID, agentID string) (map[agent.PolicyKey]agent.DisclosurePolicy, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT subject_type, subject_id, visibility, array_to_json(channels), indexable, array_to_json(audiences)
		FROM agent_profile_disclosure_policies
		WHERE owner_principal_id=$1 AND agent_id=$2`, principalID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list agent profile disclosure policies: %w", err)
	}
	defer rows.Close()
	items := make(map[agent.PolicyKey]agent.DisclosurePolicy)
	for rows.Next() {
		var key agent.PolicyKey
		var policy agent.DisclosurePolicy
		var channels, audiences []byte
		if err := rows.Scan(&key.SubjectType, &key.SubjectID, &policy.Visibility, &channels, &policy.Indexable, &audiences); err != nil {
			return nil, fmt.Errorf("scan agent profile disclosure policy: %w", err)
		}
		if err := json.Unmarshal(channels, &policy.Channels); err != nil {
			return nil, fmt.Errorf("decode agent profile disclosure channels: %w", err)
		}
		if err := json.Unmarshal(audiences, &policy.Audiences); err != nil {
			return nil, fmt.Errorf("decode agent profile disclosure audiences: %w", err)
		}
		items[key] = policy
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent profile disclosure policies: %w", err)
	}
	return items, nil
}

func (repository *AgentProfileRepository) UpdateProfileIdentity(ctx context.Context, principalID, agentID string, update agent.ProfileUpdate) error {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin agent profile identity update: %w", err)
	}
	defer transaction.Rollback()
	if err := lockProfileVersion(ctx, transaction, principalID, agentID, update.ExpectedVersion); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `
		UPDATE agents
		SET name=COALESCE($3::text, name), description=COALESCE($4::text, description), updated_at=now()
		WHERE owner_principal_id=$1 AND id=$2`, principalID, agentID, update.Name, update.Description); err != nil {
		return fmt.Errorf("update agent identity: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, `
		UPDATE agent_profiles
		SET avatar_url=COALESCE($3::text, avatar_url), version=version+1, updated_at=now()
		WHERE owner_principal_id=$1 AND agent_id=$2`, principalID, agentID, update.AvatarURL); err != nil {
		return fmt.Errorf("advance agent profile identity: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit agent profile identity update: %w", err)
	}
	return nil
}

func (repository *AgentProfileRepository) UpdateDisclosurePolicies(ctx context.Context, principalID, agentID string, expectedVersion int64, changes []agent.PolicyChange) error {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin agent profile policy update: %w", err)
	}
	defer transaction.Rollback()
	if err := lockProfileVersion(ctx, transaction, principalID, agentID, expectedVersion); err != nil {
		return err
	}
	for _, change := range changes {
		channels, err := json.Marshal(change.Policy.Channels)
		if err != nil {
			return fmt.Errorf("encode agent profile disclosure channels: %w", err)
		}
		audiences, err := json.Marshal(change.Policy.Audiences)
		if err != nil {
			return fmt.Errorf("encode agent profile disclosure audiences: %w", err)
		}
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO agent_profile_disclosure_policies (
				owner_principal_id, agent_id, subject_type, subject_id, visibility, channels, indexable, audiences
			) VALUES (
				$1, $2, $3, $4, $5,
				ARRAY(SELECT jsonb_array_elements_text($6::jsonb)), $7,
				ARRAY(SELECT jsonb_array_elements_text($8::jsonb))
			)
			ON CONFLICT (agent_id, subject_type, subject_id) DO UPDATE
			SET visibility=EXCLUDED.visibility, channels=EXCLUDED.channels,
			    indexable=EXCLUDED.indexable, audiences=EXCLUDED.audiences, updated_at=now()`,
			principalID, agentID, change.SubjectType, change.SubjectID, change.Policy.Visibility,
			channels, change.Policy.Indexable, audiences,
		); err != nil {
			return fmt.Errorf("upsert agent profile disclosure policy: %w", err)
		}
	}
	if _, err := transaction.ExecContext(ctx, `
		UPDATE agent_profiles SET version=version+1, updated_at=now()
		WHERE owner_principal_id=$1 AND agent_id=$2`, principalID, agentID); err != nil {
		return fmt.Errorf("advance agent profile policy version: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit agent profile policy update: %w", err)
	}
	return nil
}

func lockProfileVersion(ctx context.Context, transaction *sql.Tx, principalID, agentID string, expectedVersion int64) error {
	var version int64
	err := transaction.QueryRowContext(ctx, `
		SELECT version FROM agent_profiles
		WHERE owner_principal_id=$1 AND agent_id=$2
		FOR UPDATE`, principalID, agentID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return agent.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock agent profile: %w", err)
	}
	if version != expectedVersion {
		return agent.ErrProfileConflict
	}
	return nil
}
