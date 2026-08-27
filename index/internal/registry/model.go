package registry

import (
	"context"
	"errors"
	"time"
)

const (
	DraftSchemaVersion = "aegislink.agent-addr/0.2-draft"
	StatusActive       = "active"
)

var (
	ErrInvalid             = errors.New("invalid AgentAddr registration")
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with an existing registration")
)

// AgentAddr is an opaque, Index-assigned logical address. It intentionally
// carries no AgentFacts, endpoint, cache, or retrieval-algorithm data.
type AgentAddr string

// Registration is the complete public response for the Registry stage.
type Registration struct {
	SchemaVersion string    `json:"schemaVersion"`
	AgentAddr     AgentAddr `json:"agentAddr"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Record contains the durable Registry state and idempotency metadata. Status
// and UpdatedAt are retained for later publication authorization/lifecycle work
// without expanding the stage-one public response.
type Record struct {
	SchemaVersion      string
	AgentAddr          AgentAddr
	Status             string
	IdempotencyKeyHash []byte
	RequestDigest      []byte
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (record Record) Registration() Registration {
	return Registration{
		SchemaVersion: record.SchemaVersion,
		AgentAddr:     record.AgentAddr,
		CreatedAt:     record.CreatedAt,
	}
}

// Repository owns atomic address allocation and durable address existence.
// Exists is intentionally internal; stage one exposes no public resolve route.
type Repository interface {
	Create(context.Context, Record) (Record, bool, error)
	Exists(context.Context, AgentAddr) (bool, error)
}
