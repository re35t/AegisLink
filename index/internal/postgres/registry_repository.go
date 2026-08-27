package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/re35t/AegisLink/index/internal/registry"
	"gorm.io/gorm"
)

type RegistryRepository struct {
	database *Database
}

type agentRegistryModel struct {
	AgentAddr          string    `gorm:"column:agent_addr;primaryKey"`
	SchemaVersion      string    `gorm:"column:schema_version"`
	Status             string    `gorm:"column:status"`
	IdempotencyKeyHash []byte    `gorm:"column:idempotency_key_hash"`
	RequestDigest      []byte    `gorm:"column:request_digest"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (agentRegistryModel) TableName() string { return "agent_registry" }

func NewRegistryRepository(database *Database) *RegistryRepository {
	return &RegistryRepository{database: database}
}

func (repository *RegistryRepository) Create(ctx context.Context, record registry.Record) (registry.Record, bool, error) {
	model := modelFromRecord(record)
	result := repository.database.connection.WithContext(ctx).Create(&model)
	if result.Error == nil {
		return recordFromModel(model), false, nil
	}
	if !errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return registry.Record{}, false, fmt.Errorf("create AgentAddr: %w", result.Error)
	}

	var existing agentRegistryModel
	query := repository.database.connection.WithContext(ctx).
		Where("idempotency_key_hash = ?", record.IdempotencyKeyHash).
		Take(&existing)
	if query.Error == nil {
		if bytes.Equal(existing.RequestDigest, record.RequestDigest) {
			return recordFromModel(existing), true, nil
		}
		return registry.Record{}, false, registry.ErrIdempotencyConflict
	}
	if !errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return registry.Record{}, false, fmt.Errorf("resolve idempotent AgentAddr: %w", query.Error)
	}
	return registry.Record{}, false, fmt.Errorf("create AgentAddr: %w", result.Error)
}

func (repository *RegistryRepository) Exists(ctx context.Context, address registry.AgentAddr) (bool, error) {
	var count int64
	result := repository.database.connection.WithContext(ctx).
		Model(&agentRegistryModel{}).
		Where("agent_addr = ?", string(address)).
		Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check AgentAddr existence: %w", result.Error)
	}
	return count == 1, nil
}

func modelFromRecord(record registry.Record) agentRegistryModel {
	return agentRegistryModel{
		AgentAddr: string(record.AgentAddr), SchemaVersion: record.SchemaVersion,
		Status: record.Status, IdempotencyKeyHash: record.IdempotencyKeyHash,
		RequestDigest: record.RequestDigest, CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func recordFromModel(model agentRegistryModel) registry.Record {
	return registry.Record{
		SchemaVersion: model.SchemaVersion, AgentAddr: registry.AgentAddr(model.AgentAddr),
		Status: model.Status, IdempotencyKeyHash: model.IdempotencyKeyHash,
		RequestDigest: model.RequestDigest, CreatedAt: model.CreatedAt.UTC(),
		UpdatedAt: model.UpdatedAt.UTC(),
	}
}
