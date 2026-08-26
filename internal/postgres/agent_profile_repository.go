package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/agent"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	result := repository.database.WithContext(ctx).Table("agent_profiles").
		Select("agent_id, owner_principal_id, avatar_url, version, context_revision, created_at, updated_at").
		Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Take(&item)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return agent.ProfileRecord{}, agent.ErrNotFound
	}
	if result.Error != nil {
		return agent.ProfileRecord{}, fmt.Errorf("get agent profile: %w", result.Error)
	}
	return item, nil
}

type confirmedFactModel struct {
	ID                 string
	OwnerPrincipalID   string
	AgentID            string
	CandidateID        *string
	SubjectKind        agent.FactSubject
	Namespace          string
	FactKey            string
	Value              map[string]any `gorm:"column:value_json;serializer:json;type:jsonb"`
	Confidence         float64
	ConfirmationMethod string
	ConfirmedBy        *string
	ConfirmedAt        time.Time
	ValidFrom          *time.Time
	ValidUntil         *time.Time
	RevokedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (confirmedFactModel) TableName() string { return "agent_confirmed_facts" }

func (repository *AgentProfileRepository) ListConfirmedFacts(ctx context.Context, principalID, agentID string, includeRevoked bool) ([]agent.ConfirmedFact, error) {
	rows := make([]confirmedFactModel, 0)
	query := repository.database.WithContext(ctx).Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID)
	if !includeRevoked {
		query = query.Where("revoked_at IS NULL")
	}
	if result := query.Order("namespace, fact_key, confirmed_at, id").Find(&rows); result.Error != nil {
		return nil, fmt.Errorf("list confirmed Agent facts: %w", result.Error)
	}
	items := make([]agent.ConfirmedFact, 0, len(rows))
	for _, row := range rows {
		items = append(items, agent.ConfirmedFact{
			ID: row.ID, Subject: row.SubjectKind, Namespace: row.Namespace, Key: row.FactKey,
			Value: nonNilMap(row.Value), CandidateID: row.CandidateID, Confidence: row.Confidence,
			Confirmation: agent.FactConfirmation{ConfirmedAt: row.ConfirmedAt, Method: row.ConfirmationMethod},
			ValidFrom:    row.ValidFrom, ValidUntil: row.ValidUntil, RevokedAt: row.RevokedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

type disclosurePolicyRow struct {
	OwnerPrincipalID string
	AgentID          string
	SubjectType      agent.ProfileSubjectType
	SubjectID        string
	Visibility       agent.Visibility
	Channels         pq.StringArray `gorm:"type:text[]"`
	Indexable        bool
	Audiences        pq.StringArray `gorm:"type:text[]"`
}

func (disclosurePolicyRow) TableName() string { return "agent_profile_disclosure_policies" }

func (repository *AgentProfileRepository) ListDisclosurePolicies(ctx context.Context, principalID, agentID string) (map[agent.PolicyKey]agent.DisclosurePolicy, error) {
	rows := make([]disclosurePolicyRow, 0)
	result := repository.database.WithContext(ctx).
		Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Find(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("list agent profile disclosure policies: %w", result.Error)
	}
	items := make(map[agent.PolicyKey]agent.DisclosurePolicy, len(rows))
	for _, row := range rows {
		channels := make([]agent.DisclosureChannel, 0, len(row.Channels))
		for _, value := range row.Channels {
			channels = append(channels, agent.DisclosureChannel(value))
		}
		policy := agent.DisclosurePolicy{Visibility: row.Visibility, Indexable: row.Indexable, Channels: channels, Audiences: append([]string{}, row.Audiences...)}
		items[agent.PolicyKey{SubjectType: row.SubjectType, SubjectID: row.SubjectID}] = policy
	}
	return items, nil
}

func (repository *AgentProfileRepository) UpdateProfileIdentity(ctx context.Context, principalID, agentID string, update agent.ProfileUpdate) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := lockProfileVersion(transaction, principalID, agentID, update.ExpectedVersion); err != nil {
			return err
		}
		now := time.Now().UTC()
		agentUpdates := map[string]any{"updated_at": now}
		if update.Name != nil {
			agentUpdates["name"] = *update.Name
		}
		if update.Description != nil {
			agentUpdates["description"] = *update.Description
		}
		if result := transaction.Table("agents").Where("owner_principal_id = ? AND id = ?", principalID, agentID).Updates(agentUpdates); result.Error != nil {
			return fmt.Errorf("update agent identity: %w", result.Error)
		}
		profileUpdates := map[string]any{"version": gorm.Expr("version + 1"), "updated_at": now}
		if update.AvatarURL != nil {
			profileUpdates["avatar_url"] = *update.AvatarURL
		}
		if result := transaction.Table("agent_profiles").Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Updates(profileUpdates); result.Error != nil {
			return fmt.Errorf("advance agent profile identity: %w", result.Error)
		}
		if err := revokeActiveAgentFacts(transaction, principalID, agentID); err != nil {
			return err
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
			channels := make(pq.StringArray, 0, len(change.Policy.Channels))
			for _, value := range change.Policy.Channels {
				channels = append(channels, string(value))
			}
			row := disclosurePolicyRow{OwnerPrincipalID: principalID, AgentID: agentID, SubjectType: change.SubjectType, SubjectID: change.SubjectID, Visibility: change.Policy.Visibility, Channels: channels, Indexable: change.Policy.Indexable, Audiences: pq.StringArray(change.Policy.Audiences)}
			result := transaction.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "agent_id"}, {Name: "subject_type"}, {Name: "subject_id"}}, DoUpdates: clause.Assignments(map[string]any{"visibility": row.Visibility, "channels": row.Channels, "indexable": row.Indexable, "audiences": row.Audiences, "updated_at": time.Now().UTC()})}).Create(&row)
			if result.Error != nil {
				return fmt.Errorf("upsert agent profile disclosure policy: %w", result.Error)
			}
		}
		result := transaction.Table("agent_profiles").Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Updates(map[string]any{"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return fmt.Errorf("advance agent profile policy version: %w", result.Error)
		}
		if err := revokeActiveAgentFacts(transaction, principalID, agentID); err != nil {
			return err
		}
		return nil
	})
}

func (repository *AgentProfileRepository) ConfirmFact(ctx context.Context, principalID, agentID, candidateID string, expectedVersion, expectedCandidateVersion int64, update agent.ConfirmFactUpdate) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := lockProfileVersion(transaction, principalID, agentID, expectedVersion); err != nil {
			return err
		}
		var candidate factCandidateModel
		result := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_principal_id = ? AND agent_id = ?", candidateID, principalID, agentID).Take(&candidate)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return agent.ErrProfileSubject
		}
		if result.Error != nil {
			return fmt.Errorf("lock Fact candidate: %w", result.Error)
		}
		if candidate.Status != "pending" || candidate.CandidateVersion != expectedCandidateVersion {
			return agent.ErrProfileConflict
		}
		subject := agent.FactSubject(candidate.SubjectKind)
		namespace := candidate.Namespace
		key := candidate.FactKey
		value := candidate.Value
		if update.Subject != nil {
			subject = *update.Subject
		}
		if update.Namespace != nil {
			namespace = *update.Namespace
		}
		if update.Key != nil {
			key = *update.Key
		}
		if update.Value != nil {
			value = update.Value
		}
		now := time.Now().UTC()
		fact := confirmedFactModel{
			ID: ulid.Make().String(), OwnerPrincipalID: principalID, AgentID: agentID, CandidateID: &candidate.ID,
			SubjectKind: subject, Namespace: namespace, FactKey: key, Value: value,
			Confidence: candidate.Confidence, ConfirmationMethod: "owner-confirmed", ConfirmedBy: &principalID, ConfirmedAt: now,
		}
		if result := transaction.Create(&fact); result.Error != nil {
			return fmt.Errorf("create confirmed Fact: %w", result.Error)
		}
		if result := transaction.Model(&candidate).Updates(map[string]any{
			"status": "promoted", "candidate_version": gorm.Expr("candidate_version + 1"),
			"reviewed_at": now, "updated_at": now,
		}); result.Error != nil {
			return fmt.Errorf("promote Fact candidate: %w", result.Error)
		}
		if result := transaction.Table("agent_profiles").Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).
			Updates(map[string]any{"version": gorm.Expr("version + 1"), "context_revision": gorm.Expr("context_revision + 1"), "updated_at": now}); result.Error != nil {
			return fmt.Errorf("advance Profile after Fact confirmation: %w", result.Error)
		}
		if err := revokeActiveAgentFacts(transaction, principalID, agentID); err != nil {
			return err
		}
		return nil
	})
}

func (repository *AgentProfileRepository) RevokeFact(ctx context.Context, principalID, agentID, factID string, expectedVersion int64) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := lockProfileVersion(transaction, principalID, agentID, expectedVersion); err != nil {
			return err
		}
		now := time.Now().UTC()
		result := transaction.Model(&confirmedFactModel{}).
			Where("id = ? AND owner_principal_id = ? AND agent_id = ? AND revoked_at IS NULL", factID, principalID, agentID).
			Updates(map[string]any{"revoked_at": now, "updated_at": now})
		if result.Error != nil {
			return fmt.Errorf("revoke confirmed Fact: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return agent.ErrProfileSubject
		}
		if result := transaction.Where("owner_principal_id = ? AND agent_id = ? AND subject_type = ? AND subject_id = ?", principalID, agentID, agent.SubjectConfirmedFact, factID).Delete(&disclosurePolicyRow{}); result.Error != nil {
			return fmt.Errorf("remove revoked Fact disclosure: %w", result.Error)
		}
		if result := transaction.Table("agent_profiles").Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).
			Updates(map[string]any{"version": gorm.Expr("version + 1"), "updated_at": now}); result.Error != nil {
			return fmt.Errorf("advance Profile after Fact revocation: %w", result.Error)
		}
		return revokeActiveAgentFacts(transaction, principalID, agentID)
	})
}

func revokeActiveAgentFacts(transaction *gorm.DB, principalID, agentID string) error {
	now := time.Now().UTC()
	result := transaction.Table("agent_facts_publications").
		Where("owner_principal_id = ? AND agent_id = ? AND status = ?", principalID, agentID, "active").
		Updates(map[string]any{"status": "revoked", "revoked_at": now})
	if result.Error != nil {
		return fmt.Errorf("revoke stale AgentFacts publication: %w", result.Error)
	}
	return nil
}

func lockProfileVersion(transaction *gorm.DB, principalID, agentID string, expectedVersion int64) error {
	var row struct{ Version int64 }
	result := transaction.Table("agent_profiles").Select("version").Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return agent.ErrNotFound
	}
	if result.Error != nil {
		return fmt.Errorf("lock agent profile: %w", result.Error)
	}
	if row.Version != expectedVersion {
		return agent.ErrProfileConflict
	}
	return nil
}
