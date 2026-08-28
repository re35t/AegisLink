package discovery

import (
	"context"

	"github.com/re35t/AegisLink/index/internal/registry"
)

var (
	_ registry.Repository = registryAdapter{}
	_ RepresentationStore = discoveryAdapter{}
	_ VectorSearch        = discoveryAdapter{}
)

type registryAdapter struct{}

func (registryAdapter) Create(context.Context, registry.Record) (registry.Record, bool, error) {
	return registry.Record{}, false, nil
}
func (registryAdapter) Exists(context.Context, registry.AgentAddr) (bool, error) {
	return false, nil
}

type discoveryAdapter struct{}

func (discoveryAdapter) Replace(context.Context, Representation) error { return nil }
func (discoveryAdapter) Search(context.Context, string, []float32, int) ([]Candidate, error) {
	return nil, nil
}
