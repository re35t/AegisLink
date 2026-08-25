package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/re35t/AegisLink/internal/memory"
	"gorm.io/gorm"
)

type MemoryRepository struct {
	database *gorm.DB
}

var _ memory.Repository = (*MemoryRepository)(nil)

func NewMemoryRepository(database *gorm.DB) *MemoryRepository {
	return &MemoryRepository{database: database}
}

func (repository *MemoryRepository) List(ctx context.Context, principalID, agentID string) ([]memory.Memory, error) {
	items := make([]memory.Memory, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT id, owner_principal_id, agent_id, kind, content, confidence, source_uri, status,
		       last_confirmed_at, created_at, updated_at
		FROM memories
		WHERE owner_principal_id=$1 AND agent_id=$2 AND status='active'
		ORDER BY updated_at DESC, id`, principalID, agentID).Scan(&items)
	if result.Error != nil {
		return nil, fmt.Errorf("list memories: %w", result.Error)
	}
	return items, nil
}

func (repository *MemoryRepository) Create(ctx context.Context, item memory.Memory) (memory.Memory, error) {
	var timestamps struct {
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	err := scanOne(repository.database.WithContext(ctx), &timestamps, `
		INSERT INTO memories (id, owner_principal_id, agent_id, kind, content, confidence, source_uri, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
		RETURNING created_at, updated_at`,
		item.ID, item.OwnerPrincipalID, item.AgentID, item.Kind, item.Content, item.Confidence, item.SourceURI,
	)
	if err != nil {
		return memory.Memory{}, fmt.Errorf("create memory: %w", err)
	}
	item.CreatedAt = timestamps.CreatedAt
	item.UpdatedAt = timestamps.UpdatedAt
	return item, nil
}

func (repository *MemoryRepository) Update(ctx context.Context, principalID, agentID, memoryID string, update memory.Update) (memory.Memory, error) {
	var kind *string
	if update.Kind != nil {
		value := string(*update.Kind)
		kind = &value
	}
	var item memory.Memory
	err := scanOne(repository.database.WithContext(ctx), &item, `
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
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return memory.Memory{}, memory.ErrNotFound
	}
	if err != nil {
		return memory.Memory{}, fmt.Errorf("update memory: %w", err)
	}
	return item, nil
}

func (repository *MemoryRepository) Forget(ctx context.Context, principalID, agentID, memoryID string) error {
	result := exec(repository.database.WithContext(ctx), `
		UPDATE memories SET status='forgotten', updated_at=now()
		WHERE owner_principal_id=$1 AND agent_id=$2 AND id=$3 AND status='active'`, principalID, agentID, memoryID)
	if result.Error != nil {
		return fmt.Errorf("forget memory: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return memory.ErrNotFound
	}
	return nil
}

func (repository *MemoryRepository) Context(ctx context.Context, principalID, agentID string, limit int) ([]memory.Memory, error) {
	items := make([]memory.Memory, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT id, owner_principal_id, agent_id, kind, content, confidence, source_uri, status,
		       last_confirmed_at, created_at, updated_at
		FROM memories
		WHERE owner_principal_id=$1 AND agent_id=$2 AND status='active'
		ORDER BY CASE kind WHEN 'semantic' THEN 0 ELSE 1 END,
		         last_confirmed_at DESC NULLS LAST, confidence DESC, updated_at DESC
		LIMIT $3`, principalID, agentID, limit).Scan(&items)
	if result.Error != nil {
		return nil, fmt.Errorf("load memory context: %w", result.Error)
	}
	return items, nil
}
