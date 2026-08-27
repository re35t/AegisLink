package discovery

import (
	"context"

	"github.com/re35t/AegisLink/index/internal/registry"
)

// These assertions keep persistence, inverted, vector, hash, and ranking
// implementations independently replaceable as the MVP evolves.
var (
	_ registry.Repository = registryAdapter{}
	_ RepresentationStore = representationAdapter{}
	_ StructuredIndex     = structuredAdapter{}
	_ SemanticIndex       = semanticAdapter{}
	_ HashIndex           = hashAdapter{}
	_ Ranker              = rankAdapter{}
)

type registryAdapter struct{}

func (registryAdapter) Create(context.Context, registry.Record) (registry.Record, bool, error) {
	return registry.Record{}, false, nil
}
func (registryAdapter) Exists(context.Context, registry.AgentAddr) (bool, error) {
	return false, nil
}

type representationAdapter struct{}

func (representationAdapter) Upsert(context.Context, Representation) error { return nil }
func (representationAdapter) Remove(context.Context, string) error         { return nil }
func (representationAdapter) Get(context.Context, string) (Representation, error) {
	return Representation{}, nil
}

type structuredAdapter struct{}

func (structuredAdapter) SearchStructured(context.Context, []RoutingKey, []WeightedRoutingKey, int) ([]CandidateSignal, error) {
	return nil, nil
}

type semanticAdapter struct{}

func (semanticAdapter) SearchSemantic(context.Context, []SemanticVector, []string, int) ([]CandidateSignal, error) {
	return nil, nil
}

type hashAdapter struct{}

func (hashAdapter) SearchHashes(context.Context, []HashSignature, int) ([]CandidateSignal, error) {
	return nil, nil
}

type rankAdapter struct{}

func (rankAdapter) Rank(context.Context, QueryPlan, []CandidateSignal) ([]Candidate, error) {
	return nil, nil
}
