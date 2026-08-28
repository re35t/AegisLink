package agentindex

import (
	"context"
	"errors"
	"time"
)

const (
	RepresentationSchema = "aegislink.discovery-representation/0.2-draft"
	QuerySchema          = "aegislink.discovery-query/0.2-draft"
	EncoderProfile       = "aegislink-discovery-v1:text-embedding-model:1536:cosine"
	EmbeddingDimensions  = 1536
	MaximumVectors       = 64
)

var (
	ErrInvalid     = errors.New("invalid Agent discovery request")
	ErrUnavailable = errors.New("Agent Index is unavailable")
)

type State struct {
	AgentID           string
	OwnerPrincipalID  string
	AgentAddr         string
	NextRevision      int64
	PublishedRevision int64
	LastError         string
	RegisteredAt      *time.Time
	PublishedAt       *time.Time
	UpdatedAt         time.Time
}

type Repository interface {
	Get(context.Context, string, string) (State, error)
	SaveRegistration(context.Context, string, string, string) error
	ReserveRevision(context.Context, string, string) (int64, error)
	MarkPublished(context.Context, string, string, int64) error
	MarkFailed(context.Context, string, string, error) error
}

type FactVector struct {
	VectorID     string    `json:"vectorId"`
	SourceDigest string    `json:"sourceDigest"`
	Embedding    []float32 `json:"embedding"`
}

type Snapshot struct {
	SchemaVersion   string       `json:"schemaVersion"`
	Revision        int64        `json:"revision"`
	EncoderProfile  string       `json:"encoderProfile"`
	SourceSetDigest string       `json:"sourceSetDigest"`
	Vectors         []FactVector `json:"vectors"`
}

type Candidate struct {
	AgentAddr              string  `json:"agentAddr"`
	Score                  float64 `json:"score"`
	MatchedVectorID        string  `json:"matchedVectorId"`
	RepresentationRevision int64   `json:"representationRevision"`
}

type Client interface {
	Register(context.Context, string) (string, error)
	Publish(context.Context, string, Snapshot) error
	Search(context.Context, []float32, int) ([]Candidate, error)
}

type Embedder interface {
	Embed(context.Context, []string) ([][]float32, error)
}
