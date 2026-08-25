package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/re35t/AegisLink/internal/agent"
	"gorm.io/gorm"
)

type AgentProfileRepository struct {
	database *gorm.DB
}

var _ agent.ProfileRepository = (*AgentProfileRepository)(nil)

func NewAgentProfileRepository(database *Database) *AgentProfileRepository {
	return &AgentProfileRepository{database: database.connection}
}

func (repository *AgentProfileRepository) GetProfile(ctx context.Context, principalID, agentID string) (agent.ProfileRecord, error) {
	var item agent.ProfileRecord
	err := scanOne(repository.database.WithContext(ctx), &item, `
		SELECT agent_id, owner_principal_id, avatar_url, version, created_at, updated_at
		FROM agent_profiles WHERE owner_principal_id=@p1 AND agent_id=@p2`, principalID, agentID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent.ProfileRecord{}, agent.ErrNotFound
	}
	if err != nil {
		return agent.ProfileRecord{}, fmt.Errorf("get agent profile: %w", err)
	}
	return item, nil
}

type profileFactRow struct {
	ID         string
	Namespace  string
	Key        string          `gorm:"column:fact_key"`
	Value      json.RawMessage `gorm:"column:value_json"`
	Source     string
	Confidence float64
	ValidFrom  *time.Time
	ValidUntil *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (repository *AgentProfileRepository) ListProfileFacts(ctx context.Context, principalID, agentID string) ([]agent.ProfileFact, error) {
	rows := make([]profileFactRow, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT id, namespace, fact_key, value_json, source, confidence, valid_from, valid_until, created_at, updated_at
		FROM agent_profile_facts
		WHERE owner_principal_id=@p1 AND agent_id=@p2
		ORDER BY namespace, fact_key, created_at, id`, principalID, agentID).Scan(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("list agent profile facts: %w", result.Error)
	}
	items := make([]agent.ProfileFact, 0, len(rows))
	for _, row := range rows {
		item := agent.ProfileFact{
			ID: row.ID, Namespace: row.Namespace, Key: row.Key, Source: row.Source,
			Confidence: row.Confidence, ValidFrom: row.ValidFrom, ValidUntil: row.ValidUntil,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
		if err := json.Unmarshal(row.Value, &item.Value); err != nil {
			return nil, fmt.Errorf("decode agent profile fact: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

type memoryProjectionRow struct {
	ID          string
	Type        string `gorm:"column:projection_type"`
	Summary     string
	SourceIDs   json.RawMessage `gorm:"column:source_ids"`
	Confidence  float64
	Freshness   float64
	GeneratedAt time.Time
	ExpiresAt   *time.Time
	Status      string
}

func (repository *AgentProfileRepository) ListMemoryProjections(ctx context.Context, principalID, agentID string) ([]agent.MemoryProjection, error) {
	rows := make([]memoryProjectionRow, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT projections.id, projections.projection_type, projections.summary,
		       COALESCE(json_agg(sources.memory_id ORDER BY sources.memory_id)
		           FILTER (WHERE sources.memory_id IS NOT NULL), '[]'::json) AS source_ids,
		       projections.confidence, projections.freshness, projections.generated_at,
		       projections.expires_at, projections.status
		FROM agent_memory_projections projections
		LEFT JOIN agent_memory_projection_sources sources
		  ON sources.projection_id=projections.id
		WHERE projections.owner_principal_id=@p1 AND projections.agent_id=@p2
		GROUP BY projections.id
		ORDER BY projections.generated_at DESC, projections.id`, principalID, agentID).Scan(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("list agent memory projections: %w", result.Error)
	}
	items := make([]agent.MemoryProjection, 0, len(rows))
	for _, row := range rows {
		item := agent.MemoryProjection{
			ID: row.ID, Type: row.Type, Summary: row.Summary, Confidence: row.Confidence,
			Freshness: row.Freshness, GeneratedAt: row.GeneratedAt, ExpiresAt: row.ExpiresAt, Status: row.Status,
		}
		if err := json.Unmarshal(row.SourceIDs, &item.SourceMemoryIDs); err != nil {
			return nil, fmt.Errorf("decode agent memory projection sources: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

type disclosurePolicyRow struct {
	SubjectType agent.ProfileSubjectType
	SubjectID   string
	Visibility  agent.Visibility
	Channels    json.RawMessage
	Indexable   bool
	Audiences   json.RawMessage
}

func (repository *AgentProfileRepository) ListDisclosurePolicies(ctx context.Context, principalID, agentID string) (map[agent.PolicyKey]agent.DisclosurePolicy, error) {
	rows := make([]disclosurePolicyRow, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT subject_type, subject_id, visibility, array_to_json(channels) AS channels,
		       indexable, array_to_json(audiences) AS audiences
		FROM agent_profile_disclosure_policies
		WHERE owner_principal_id=@p1 AND agent_id=@p2`, principalID, agentID).Scan(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("list agent profile disclosure policies: %w", result.Error)
	}
	items := make(map[agent.PolicyKey]agent.DisclosurePolicy, len(rows))
	for _, row := range rows {
		policy := agent.DisclosurePolicy{Visibility: row.Visibility, Indexable: row.Indexable}
		if err := json.Unmarshal(row.Channels, &policy.Channels); err != nil {
			return nil, fmt.Errorf("decode agent profile disclosure channels: %w", err)
		}
		if err := json.Unmarshal(row.Audiences, &policy.Audiences); err != nil {
			return nil, fmt.Errorf("decode agent profile disclosure audiences: %w", err)
		}
		items[agent.PolicyKey{SubjectType: row.SubjectType, SubjectID: row.SubjectID}] = policy
	}
	return items, nil
}

func (repository *AgentProfileRepository) UpdateProfileIdentity(ctx context.Context, principalID, agentID string, update agent.ProfileUpdate) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := lockProfileVersion(transaction, principalID, agentID, update.ExpectedVersion); err != nil {
			return err
		}
		if result := exec(transaction, `
			UPDATE agents
			SET name=COALESCE(CAST(@p3 AS text), name), description=COALESCE(CAST(@p4 AS text), description), updated_at=now()
			WHERE owner_principal_id=@p1 AND id=@p2`, principalID, agentID, update.Name, update.Description); result.Error != nil {
			return fmt.Errorf("update agent identity: %w", result.Error)
		}
		if result := exec(transaction, `
			UPDATE agent_profiles
			SET avatar_url=COALESCE(CAST(@p3 AS text), avatar_url), version=version+1, updated_at=now()
			WHERE owner_principal_id=@p1 AND agent_id=@p2`, principalID, agentID, update.AvatarURL); result.Error != nil {
			return fmt.Errorf("advance agent profile identity: %w", result.Error)
		}
		return nil
	})
}

func (repository *AgentProfileRepository) UpdateDisclosurePolicies(ctx context.Context, principalID, agentID string, expectedVersion int64, changes []agent.PolicyChange) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := lockProfileVersion(transaction, principalID, agentID, expectedVersion); err != nil {
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
			result := exec(transaction, `
				INSERT INTO agent_profile_disclosure_policies (
					owner_principal_id, agent_id, subject_type, subject_id, visibility, channels, indexable, audiences
				) VALUES (
					@p1, @p2, @p3, @p4, @p5,
					ARRAY(SELECT jsonb_array_elements_text(CAST(@p6 AS jsonb))), @p7,
					ARRAY(SELECT jsonb_array_elements_text(CAST(@p8 AS jsonb)))
				)
				ON CONFLICT (agent_id, subject_type, subject_id) DO UPDATE
				SET visibility=EXCLUDED.visibility, channels=EXCLUDED.channels,
				    indexable=EXCLUDED.indexable, audiences=EXCLUDED.audiences, updated_at=now()`,
				principalID, agentID, change.SubjectType, change.SubjectID, change.Policy.Visibility,
				string(channels), change.Policy.Indexable, string(audiences),
			)
			if result.Error != nil {
				return fmt.Errorf("upsert agent profile disclosure policy: %w", result.Error)
			}
		}
		result := exec(transaction, `
			UPDATE agent_profiles SET version=version+1, updated_at=now()
			WHERE owner_principal_id=@p1 AND agent_id=@p2`, principalID, agentID)
		if result.Error != nil {
			return fmt.Errorf("advance agent profile policy version: %w", result.Error)
		}
		return nil
	})
}

func lockProfileVersion(transaction *gorm.DB, principalID, agentID string, expectedVersion int64) error {
	var row struct{ Version int64 }
	err := scanOne(transaction, &row, `
		SELECT version FROM agent_profiles
		WHERE owner_principal_id=@p1 AND agent_id=@p2
		FOR UPDATE`, principalID, agentID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock agent profile: %w", err)
	}
	if row.Version != expectedVersion {
		return agent.ErrProfileConflict
	}
	return nil
}
