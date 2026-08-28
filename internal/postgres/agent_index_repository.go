package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/re35t/AegisLink/internal/agentindex"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AgentIndexRepository struct {
	database *gorm.DB
}

func NewAgentIndexRepository(database *Database) *AgentIndexRepository {
	return &AgentIndexRepository{database: database.connection}
}

func (repository *AgentIndexRepository) Get(ctx context.Context, principalID, agentID string) (agentindex.State, error) {
	var state agentindex.State
	result := repository.database.WithContext(ctx).Table("agent_index_states").
		Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Take(&state)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return agentindex.State{AgentID: agentID, OwnerPrincipalID: principalID}, nil
	}
	if result.Error != nil {
		return agentindex.State{}, fmt.Errorf("get Agent Index state: %w", result.Error)
	}
	return state, nil
}

func (repository *AgentIndexRepository) SaveRegistration(ctx context.Context, principalID, agentID, agentAddr string) error {
	now := time.Now().UTC()
	state := agentindex.State{AgentID: agentID, OwnerPrincipalID: principalID, AgentAddr: agentAddr, RegisteredAt: &now, UpdatedAt: now}
	result := repository.database.WithContext(ctx).Table("agent_index_states").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "agent_id"}},
		DoUpdates: clause.Assignments(map[string]any{"agent_addr": agentAddr, "registered_at": now, "last_error": "", "updated_at": now}),
	}).Create(&state)
	if result.Error != nil {
		return fmt.Errorf("save AgentAddr registration: %w", result.Error)
	}
	return nil
}

func (repository *AgentIndexRepository) ReserveRevision(ctx context.Context, principalID, agentID string) (int64, error) {
	var revision int64
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		now := time.Now().UTC()
		state := agentindex.State{AgentID: agentID, OwnerPrincipalID: principalID, UpdatedAt: now}
		if result := transaction.Table("agent_index_states").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "agent_id"}}, DoNothing: true}).Create(&state); result.Error != nil {
			return fmt.Errorf("ensure Agent Index state: %w", result.Error)
		}
		var locked agentindex.State
		result := transaction.Table("agent_index_states").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Take(&locked)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return agentindex.ErrInvalid
		}
		if result.Error != nil {
			return fmt.Errorf("lock Agent Index revision: %w", result.Error)
		}
		revision = locked.NextRevision + 1
		if result := transaction.Table("agent_index_states").Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).
			Updates(map[string]any{"next_revision": revision, "updated_at": now}); result.Error != nil {
			return fmt.Errorf("reserve Agent Index revision: %w", result.Error)
		}
		return nil
	})
	return revision, err
}

func (repository *AgentIndexRepository) MarkPublished(ctx context.Context, principalID, agentID string, revision int64) error {
	now := time.Now().UTC()
	result := repository.database.WithContext(ctx).Table("agent_index_states").
		Where("owner_principal_id = ? AND agent_id = ? AND published_revision < ?", principalID, agentID, revision).
		Updates(map[string]any{"published_revision": revision, "published_at": now, "last_error": "", "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("mark Agent Index publication: %w", result.Error)
	}
	return nil
}

func (repository *AgentIndexRepository) MarkFailed(ctx context.Context, principalID, agentID string, cause error) error {
	message := cause.Error()
	if len(message) > 2000 {
		message = message[:2000]
	}
	now := time.Now().UTC()
	state := agentindex.State{AgentID: agentID, OwnerPrincipalID: principalID, LastError: message, UpdatedAt: now}
	result := repository.database.WithContext(ctx).Table("agent_index_states").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "agent_id"}},
		DoUpdates: clause.Assignments(map[string]any{"last_error": message, "updated_at": now}),
	}).Create(&state)
	if result.Error != nil {
		return fmt.Errorf("mark Agent Index failure: %w", result.Error)
	}
	return nil
}
