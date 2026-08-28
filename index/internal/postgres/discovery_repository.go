package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/re35t/AegisLink/index/internal/discovery"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DiscoveryRepository struct {
	database *Database
}

type discoveryRepresentationModel struct {
	AgentAddr       string    `gorm:"column:agent_addr;primaryKey"`
	Revision        int64     `gorm:"column:revision"`
	EncoderProfile  string    `gorm:"column:encoder_profile"`
	SourceSetDigest string    `gorm:"column:source_set_digest"`
	VectorCount     int       `gorm:"column:vector_count"`
	RequestDigest   []byte    `gorm:"column:request_digest"`
	PublishedAt     time.Time `gorm:"column:published_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (discoveryRepresentationModel) TableName() string { return "discovery_representations" }

func NewDiscoveryRepository(database *Database) *DiscoveryRepository {
	return &DiscoveryRepository{database: database}
}

func (repository *DiscoveryRepository) Replace(ctx context.Context, representation discovery.Representation) error {
	return repository.database.connection.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var registered agentRegistryModel
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("agent_addr = ?", string(representation.AgentAddr)).Take(&registered).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return discovery.ErrAgentNotFound
			}
			return fmt.Errorf("lock AgentAddr for representation replacement: %w", err)
		}

		var current discoveryRepresentationModel
		err := transaction.Where("agent_addr = ?", string(representation.AgentAddr)).Take(&current).Error
		switch {
		case err == nil && representation.Revision == current.Revision && bytes.Equal(representation.RequestDigest, current.RequestDigest):
			return nil
		case err == nil && representation.Revision <= current.Revision:
			return discovery.ErrStaleRevision
		case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
			return fmt.Errorf("load current representation: %w", err)
		}

		model := discoveryRepresentationModel{
			AgentAddr: string(representation.AgentAddr), Revision: representation.Revision,
			EncoderProfile: representation.EncoderProfile, SourceSetDigest: representation.SourceSetDigest,
			VectorCount: len(representation.Vectors), RequestDigest: representation.RequestDigest,
			PublishedAt: representation.PublishedAt, UpdatedAt: representation.PublishedAt,
		}
		if err := transaction.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "agent_addr"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"revision", "encoder_profile", "source_set_digest", "vector_count",
				"request_digest", "published_at", "updated_at",
			}),
		}).Create(&model).Error; err != nil {
			return fmt.Errorf("store representation metadata: %w", err)
		}
		if err := transaction.Exec(
			"DELETE FROM discovery_fact_vectors WHERE agent_addr = ?",
			string(representation.AgentAddr),
		).Error; err != nil {
			return fmt.Errorf("remove previous representation vectors: %w", err)
		}
		for _, vector := range representation.Vectors {
			if err := transaction.Exec(`
				INSERT INTO discovery_fact_vectors (
					agent_addr, vector_id, source_digest, embedding, created_at
				) VALUES (?, ?, ?, CAST(? AS vector), ?)
			`, string(representation.AgentAddr), vector.VectorID, vector.SourceDigest,
				vectorLiteral(vector.Embedding), representation.PublishedAt,
			).Error; err != nil {
				return fmt.Errorf("store representation vector %q: %w", vector.VectorID, err)
			}
		}
		return nil
	})
}

func (repository *DiscoveryRepository) Search(
	ctx context.Context,
	encoderProfile string,
	embedding []float32,
	topK int,
) ([]discovery.Candidate, error) {
	var candidates []discovery.Candidate
	result := repository.database.connection.WithContext(ctx).Raw(`
		WITH query_vector AS (
			SELECT CAST(? AS vector) AS embedding
		), vector_scores AS (
			SELECT
				v.agent_addr,
				v.vector_id AS matched_vector_id,
				r.revision AS representation_revision,
				1 - (v.embedding <=> q.embedding) AS score,
				ROW_NUMBER() OVER (
					PARTITION BY v.agent_addr
					ORDER BY (v.embedding <=> q.embedding) ASC, v.vector_id ASC
				) AS per_agent_rank
			FROM discovery_fact_vectors v
			JOIN discovery_representations r USING (agent_addr)
			JOIN agent_registry a USING (agent_addr)
			CROSS JOIN query_vector q
			WHERE a.status = 'active' AND r.encoder_profile = ?
		)
		SELECT agent_addr, score, matched_vector_id, representation_revision
		FROM vector_scores
		WHERE per_agent_rank = 1
		ORDER BY score DESC, representation_revision DESC, agent_addr ASC
		LIMIT ?
	`, vectorLiteral(embedding), encoderProfile, topK).Scan(&candidates)
	if result.Error != nil {
		return nil, fmt.Errorf("search discovery vectors: %w", result.Error)
	}
	return candidates, nil
}

func vectorLiteral(values []float32) string {
	var builder strings.Builder
	builder.Grow(len(values) * 12)
	builder.WriteByte('[')
	for index, value := range values {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(float64(value), 'g', -1, 32))
	}
	builder.WriteByte(']')
	return builder.String()
}
