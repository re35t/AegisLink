package discovery

import (
	"context"
	"time"

	"github.com/re35t/AegisLink/index/internal/registry"
)

const DraftSchemaVersion = "aegislink.discovery-representation/0.1-draft"

type RoutingKey struct {
	Facet string `json:"facet"`
	Value string `json:"value"`
}

type WeightedRoutingKey struct {
	RoutingKey
	Weight float64 `json:"weight"`
}

type SemanticVector struct {
	Name           string    `json:"name"`
	EncoderProfile string    `json:"encoderProfile"`
	Values         []float32 `json:"values"`
}

type HashSignature struct {
	Family string `json:"family"`
	Table  uint16 `json:"table"`
	Bucket string `json:"bucket"`
}

// Representation is a lossy routing projection. It deliberately contains no
// private Profile data or complete AgentFacts document.
type Representation struct {
	SchemaVersion   string             `json:"schemaVersion"`
	AgentAddr       registry.AgentAddr `json:"agentAddr"`
	Revision        int64              `json:"revision"`
	EncoderProfile  string             `json:"encoderProfile"`
	RoutingKeys     []RoutingKey       `json:"routingKeys"`
	SemanticVectors []SemanticVector   `json:"semanticVectors"`
	HashSignatures  []HashSignature    `json:"hashSignatures"`
	FactsDigest     string             `json:"factsDigest"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}

type QueryPlan struct {
	Required       []RoutingKey         `json:"required"`
	Preferred      []WeightedRoutingKey `json:"preferred"`
	Semantic       []SemanticVector     `json:"semantic"`
	HashSignatures []HashSignature      `json:"hashSignatures"`
	TopK           int                  `json:"topK"`
}

type CandidateSignal struct {
	AgentID        string  `json:"agentId"`
	PreferredScore float64 `json:"preferredScore"`
	SemanticScore  float64 `json:"semanticScore"`
	FreshnessScore float64 `json:"freshnessScore"`
	MatchedBuckets int     `json:"matchedBuckets"`
}

type Candidate struct {
	AgentID     string  `json:"agentId"`
	Score       float64 `json:"score"`
	FactsDigest string  `json:"factsDigest"`
	Revision    int64   `json:"revision"`
}

type RepresentationStore interface {
	Upsert(context.Context, Representation) error
	Remove(context.Context, string) error
	Get(context.Context, string) (Representation, error)
}

type StructuredIndex interface {
	SearchStructured(context.Context, []RoutingKey, []WeightedRoutingKey, int) ([]CandidateSignal, error)
}

type SemanticIndex interface {
	SearchSemantic(context.Context, []SemanticVector, []string, int) ([]CandidateSignal, error)
}

type HashIndex interface {
	SearchHashes(context.Context, []HashSignature, int) ([]CandidateSignal, error)
}

type Ranker interface {
	Rank(context.Context, QueryPlan, []CandidateSignal) ([]Candidate, error)
}
