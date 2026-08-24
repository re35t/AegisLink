package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/re35t/AegisLink/internal/memory"
)

type MemoryRepository struct {
	database *sql.DB
}

var _ memory.Repository = (*MemoryRepository)(nil)

func NewMemoryRepository(database *sql.DB) *MemoryRepository {
	return &MemoryRepository{database: database}
}

func (repository *MemoryRepository) List(ctx context.Context, principalID, agentID string) ([]memory.Memory, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, owner_principal_id, agent_id, kind, content, confidence, source_uri, status,
		       last_confirmed_at, created_at, updated_at
		FROM memories
		WHERE owner_principal_id=$1 AND agent_id=$2 AND status='active'
		ORDER BY updated_at DESC, id`, principalID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

func (repository *MemoryRepository) Create(ctx context.Context, item memory.Memory) (memory.Memory, error) {
	err := repository.database.QueryRowContext(ctx, `
		INSERT INTO memories (id, owner_principal_id, agent_id, kind, content, confidence, source_uri, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
		RETURNING created_at, updated_at`,
		item.ID, item.OwnerPrincipalID, item.AgentID, item.Kind, item.Content, item.Confidence, item.SourceURI,
	).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return memory.Memory{}, fmt.Errorf("create memory: %w", err)
	}
	return item, nil
}

func (repository *MemoryRepository) Update(ctx context.Context, principalID, agentID, memoryID string, update memory.Update) (memory.Memory, error) {
	var kind *string
	if update.Kind != nil {
		value := string(*update.Kind)
		kind = &value
	}
	var item memory.Memory
	err := repository.database.QueryRowContext(ctx, `
		UPDATE memories
		SET kind=COALESCE($4, kind),
		    content=COALESCE($5, content),
		    confidence=COALESCE($6, confidence),
		    last_confirmed_at=CASE WHEN $7 THEN now() ELSE last_confirmed_at END,
		    updated_at=now()
		WHERE owner_principal_id=$1 AND agent_id=$2 AND id=$3 AND status='active'
		RETURNING id, owner_principal_id, agent_id, kind, content, confidence, source_uri, status,
		          last_confirmed_at, created_at, updated_at`,
		principalID, agentID, memoryID, kind, update.Content, update.Confidence, update.Confirmed,
	).Scan(memoryScanTargets(&item)...)
	if errors.Is(err, sql.ErrNoRows) {
		return memory.Memory{}, memory.ErrNotFound
	}
	if err != nil {
		return memory.Memory{}, fmt.Errorf("update memory: %w", err)
	}
	return item, nil
}

func (repository *MemoryRepository) Forget(ctx context.Context, principalID, agentID, memoryID string) error {
	result, err := repository.database.ExecContext(ctx, `
		UPDATE memories SET status='forgotten', updated_at=now()
		WHERE owner_principal_id=$1 AND agent_id=$2 AND id=$3 AND status='active'`, principalID, agentID, memoryID)
	if err != nil {
		return fmt.Errorf("forget memory: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read forgotten memory rows: %w", err)
	}
	if changed == 0 {
		return memory.ErrNotFound
	}
	return nil
}

func (repository *MemoryRepository) Context(ctx context.Context, principalID, agentID string, limit int) ([]memory.Memory, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, owner_principal_id, agent_id, kind, content, confidence, source_uri, status,
		       last_confirmed_at, created_at, updated_at
		FROM memories
		WHERE owner_principal_id=$1 AND agent_id=$2 AND status='active'
		ORDER BY CASE kind WHEN 'semantic' THEN 0 ELSE 1 END,
		         last_confirmed_at DESC NULLS LAST, confidence DESC, updated_at DESC
		LIMIT $3`, principalID, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("load memory context: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

func scanMemories(rows *sql.Rows) ([]memory.Memory, error) {
	items := make([]memory.Memory, 0)
	for rows.Next() {
		var item memory.Memory
		if err := rows.Scan(memoryScanTargets(&item)...); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func memoryScanTargets(item *memory.Memory) []any {
	return []any{
		&item.ID, &item.OwnerPrincipalID, &item.AgentID, &item.Kind, &item.Content, &item.Confidence,
		&item.SourceURI, &item.Status, &item.LastConfirmedAt, &item.CreatedAt, &item.UpdatedAt,
	}
}
