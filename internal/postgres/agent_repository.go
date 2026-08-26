package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/re35t/AegisLink/internal/agent"
	"gorm.io/gorm"
)

type AgentRepository struct {
	database *gorm.DB
}

var _ agent.Repository = (*AgentRepository)(nil)

func NewAgentRepository(database *Database) *AgentRepository {
	return &AgentRepository{database: database.connection}
}

func (repository *AgentRepository) List(ctx context.Context, ownerID string) ([]agent.Agent, error) {
	items := make([]agent.Agent, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT id, owner_principal_id, name, description, system_prompt, created_at, updated_at
		FROM agents WHERE owner_principal_id=@p1 ORDER BY created_at`, ownerID).Scan(&items)
	if result.Error != nil {
		return nil, fmt.Errorf("list agents: %w", result.Error)
	}
	return items, nil
}

func (repository *AgentRepository) Default(ctx context.Context, ownerID string) (agent.Agent, error) {
	var item agent.Agent
	err := scanOne(repository.database.WithContext(ctx), &item, `
		SELECT id, owner_principal_id, name, description, system_prompt, created_at, updated_at
		FROM agents WHERE owner_principal_id=@p1 ORDER BY created_at, id LIMIT 1`, ownerID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent.Agent{}, agent.ErrNotFound
	}
	if err != nil {
		return agent.Agent{}, fmt.Errorf("get default agent: %w", err)
	}
	return item, nil
}

func (repository *AgentRepository) Get(ctx context.Context, ownerID, id string) (agent.Agent, error) {
	var item agent.Agent
	err := scanOne(repository.database.WithContext(ctx), &item, `
		SELECT id, owner_principal_id, name, description, system_prompt, created_at, updated_at
		FROM agents WHERE id=@p1 AND owner_principal_id=@p2`, id, ownerID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent.Agent{}, agent.ErrNotFound
	}
	if err != nil {
		return agent.Agent{}, fmt.Errorf("get agent: %w", err)
	}
	return item, nil
}

func (repository *AgentRepository) GetInstructions(ctx context.Context, ownerID, id string) (agent.Instructions, error) {
	var item agent.Instructions
	err := scanOne(repository.database.WithContext(ctx), &item, `
		SELECT id AS agent_id, system_prompt, system_prompt_version AS version, updated_at
		FROM agents WHERE id=@p1 AND owner_principal_id=@p2`, id, ownerID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent.Instructions{}, agent.ErrNotFound
	}
	if err != nil {
		return agent.Instructions{}, fmt.Errorf("get agent instructions: %w", err)
	}
	return item, nil
}

func (repository *AgentRepository) UpdateInstructions(ctx context.Context, ownerID, id string, update agent.InstructionsUpdate) (agent.Instructions, error) {
	now := time.Now().UTC()
	result := repository.database.WithContext(ctx).Table("agents").
		Where("id = ? AND owner_principal_id = ? AND system_prompt_version = ?", id, ownerID, update.ExpectedVersion).
		Updates(map[string]any{
			"system_prompt":         update.SystemPrompt,
			"system_prompt_version": gorm.Expr("system_prompt_version + 1"),
			"updated_at":            now,
		})
	if result.Error != nil {
		return agent.Instructions{}, fmt.Errorf("update agent instructions: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		if _, err := repository.GetInstructions(ctx, ownerID, id); err != nil {
			return agent.Instructions{}, err
		}
		return agent.Instructions{}, agent.ErrInstructionsConflict
	}
	return repository.GetInstructions(ctx, ownerID, id)
}
