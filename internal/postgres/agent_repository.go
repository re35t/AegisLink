package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/re35t/AegisLink/internal/agent"
)

type AgentRepository struct {
	database *sql.DB
}

var _ agent.Repository = (*AgentRepository)(nil)

func NewAgentRepository(database *sql.DB) *AgentRepository {
	return &AgentRepository{database: database}
}

func (repository *AgentRepository) List(ctx context.Context, ownerID string) ([]agent.Agent, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, owner_principal_id, name, description, system_prompt, created_at, updated_at
		FROM agents WHERE owner_principal_id=$1 ORDER BY created_at`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()
	agents := make([]agent.Agent, 0)
	for rows.Next() {
		var item agent.Agent
		if err := rows.Scan(&item.ID, &item.OwnerPrincipalID, &item.Name, &item.Description, &item.SystemPrompt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, item)
	}
	return agents, rows.Err()
}

func (repository *AgentRepository) Default(ctx context.Context, ownerID string) (agent.Agent, error) {
	var item agent.Agent
	err := repository.database.QueryRowContext(ctx, `
		SELECT id, owner_principal_id, name, description, system_prompt, created_at, updated_at
		FROM agents WHERE owner_principal_id=$1 ORDER BY created_at, id LIMIT 1`, ownerID).Scan(
		&item.ID, &item.OwnerPrincipalID, &item.Name, &item.Description, &item.SystemPrompt, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return agent.Agent{}, agent.ErrNotFound
	}
	if err != nil {
		return agent.Agent{}, fmt.Errorf("get default agent: %w", err)
	}
	return item, nil
}

func (repository *AgentRepository) Get(ctx context.Context, ownerID, id string) (agent.Agent, error) {
	var item agent.Agent
	err := repository.database.QueryRowContext(ctx, `
		SELECT id, owner_principal_id, name, description, system_prompt, created_at, updated_at
		FROM agents WHERE id=$1 AND owner_principal_id=$2`, id, ownerID).Scan(
		&item.ID, &item.OwnerPrincipalID, &item.Name, &item.Description, &item.SystemPrompt, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return agent.Agent{}, agent.ErrNotFound
	}
	if err != nil {
		return agent.Agent{}, fmt.Errorf("get agent: %w", err)
	}
	return item, nil
}
