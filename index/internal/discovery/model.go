package discovery

import (
	"context"
	"errors"
	"time"

	"github.com/re35t/AegisLink/index/internal/registry"
)

const (
	RepresentationSchemaVersion = "aegislink.discovery-representation/0.2-draft"
	QuerySchemaVersion          = "aegislink.discovery-query/0.2-draft"
	ResultSchemaVersion         = "aegislink.discovery-result/0.2-draft"
	EncoderProfile              = "aegislink-discovery-v1:text-embedding-model:1536:cosine"
	EmbeddingDimensions         = 1536
	MaxVectorsPerAgent          = 64
	DefaultTopK                 = 5
	MaxTopK                     = 50
)

var (
	ErrAgentNotFound      = errors.New("AgentAddr is not registered")
	ErrInvalidSnapshot    = errors.New("invalid vector snapshot")
	ErrInvalidQuery       = errors.New("invalid query vector")
	ErrUnsupportedProfile = errors.New("encoder profile is unsupported")
	ErrStaleRevision      = errors.New("representation revision is stale")
)

// FactVector contains only a publisher-chosen identifier, a source digest, and
// an embedding. The Index never receives the source AgentFact text.
type FactVector struct {
	VectorID     string    `json:"vectorId"`
	SourceDigest string    `json:"sourceDigest"`
	Embedding    []float32 `json:"embedding"`
}

// Snapshot is a complete replacement, not a patch. An empty Vectors slice
// deliberately removes the AgentAddr from discovery results.
type Snapshot struct {
	SchemaVersion   string       `json:"schemaVersion"`
	Revision        int64        `json:"revision"`
	EncoderProfile  string       `json:"encoderProfile"`
	SourceSetDigest string       `json:"sourceSetDigest"`
	Vectors         []FactVector `json:"vectors"`
}

// Representation is the validated form passed to persistence.
type Representation struct {
	AgentAddr       registry.AgentAddr
	Revision        int64
	EncoderProfile  string
	SourceSetDigest string
	Vectors         []FactVector
	RequestDigest   []byte
	PublishedAt     time.Time
}

type Query struct {
	SchemaVersion  string    `json:"schemaVersion"`
	EncoderProfile string    `json:"encoderProfile"`
	Embedding      []float32 `json:"embedding"`
	TopK           int       `json:"topK,omitempty"`
}

type Candidate struct {
	AgentAddr              registry.AgentAddr `json:"agentAddr"`
	Score                  float64            `json:"score"`
	MatchedVectorID        string             `json:"matchedVectorId"`
	RepresentationRevision int64              `json:"representationRevision"`
}

type Result struct {
	SchemaVersion  string      `json:"schemaVersion"`
	EncoderProfile string      `json:"encoderProfile"`
	Candidates     []Candidate `json:"candidates"`
}

type RepresentationStore interface {
	Replace(context.Context, Representation) error
}

type VectorSearch interface {
	Search(context.Context, string, []float32, int) ([]Candidate, error)
}
