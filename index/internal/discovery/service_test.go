package discovery

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/re35t/AegisLink/index/internal/registry"
)

func TestPublishValidatesAndBuildsCompleteRepresentation(t *testing.T) {
	adapter := &serviceAdapter{}
	service, err := NewService(adapter, adapter)
	if err != nil {
		t.Fatal(err)
	}
	vector := testFactVector("fact_one", 1)
	snapshot := Snapshot{
		SchemaVersion: RepresentationSchemaVersion, Revision: 2,
		EncoderProfile: EncoderProfile, Vectors: []FactVector{vector},
	}
	snapshot.SourceSetDigest = SourceSetDigest(snapshot.Vectors)
	if err := service.Publish(t.Context(), registry.AgentAddr("agent_01ARZ3NDEKTSV4RRFFQ69G5FAV"), snapshot); err != nil {
		t.Fatal(err)
	}
	if adapter.representation.Revision != 2 || len(adapter.representation.RequestDigest) != 32 || len(adapter.representation.Vectors) != 1 {
		t.Fatalf("stored representation = %#v", adapter.representation)
	}
}

func TestPublishRejectsInvalidSnapshots(t *testing.T) {
	adapter := &serviceAdapter{}
	service, _ := NewService(adapter, adapter)
	validVector := testFactVector("fact_one", 1)
	for _, test := range []struct {
		name     string
		address  registry.AgentAddr
		snapshot Snapshot
		want     error
	}{
		{name: "unknown address shape", address: "bad", snapshot: validSnapshot(validVector), want: ErrAgentNotFound},
		{name: "unsupported profile", address: testAddress, snapshot: mutateSnapshot(validSnapshot(validVector), func(value *Snapshot) { value.EncoderProfile = "other" }), want: ErrUnsupportedProfile},
		{name: "duplicate vector", address: testAddress, snapshot: validSnapshot(validVector, validVector), want: ErrInvalidSnapshot},
		{name: "wrong dimension", address: testAddress, snapshot: validSnapshot(testFactVector("fact_one", 0)), want: ErrInvalidSnapshot},
		{name: "wrong source set", address: testAddress, snapshot: mutateSnapshot(validSnapshot(validVector), func(value *Snapshot) { value.SourceSetDigest = digest("wrong") }), want: ErrInvalidSnapshot},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := service.Publish(t.Context(), test.address, test.snapshot); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestSearchDefaultsTopKAndReturnsStableEnvelope(t *testing.T) {
	adapter := &serviceAdapter{candidates: []Candidate{{AgentAddr: testAddress, Score: .8, MatchedVectorID: "fact_one", RepresentationRevision: 3}}}
	service, _ := NewService(adapter, adapter)
	result, err := service.Search(t.Context(), Query{
		SchemaVersion: QuerySchemaVersion, EncoderProfile: EncoderProfile,
		Embedding: testEmbedding(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if adapter.topK != DefaultTopK || result.SchemaVersion != ResultSchemaVersion || len(result.Candidates) != 1 {
		t.Fatalf("topK=%d result=%#v", adapter.topK, result)
	}
}

func TestSnapshotAndQueryBounds(t *testing.T) {
	adapter := &serviceAdapter{}
	service, _ := NewService(adapter, adapter)

	tooMany := make([]FactVector, MaxVectorsPerAgent+1)
	for index := range tooMany {
		tooMany[index] = testFactVector("fact_"+strconv.Itoa(index), 1)
	}
	nonFinite := testFactVector("fact_nan", 1)
	nonFinite.Embedding[0] = float32(math.NaN())
	for _, snapshot := range []Snapshot{
		validSnapshot(tooMany...),
		validSnapshot(nonFinite),
	} {
		if err := service.Publish(t.Context(), testAddress, snapshot); !errors.Is(err, ErrInvalidSnapshot) {
			t.Fatalf("snapshot error = %v", err)
		}
	}

	for _, query := range []Query{
		{SchemaVersion: QuerySchemaVersion, EncoderProfile: EncoderProfile, Embedding: testEmbedding(1), TopK: MaxTopK + 1},
		{SchemaVersion: QuerySchemaVersion, EncoderProfile: EncoderProfile, Embedding: testEmbedding(0)},
	} {
		if _, err := service.Search(t.Context(), query); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("query error = %v", err)
		}
	}
}

func TestSourceSetDigestIsIndependentOfVectorOrder(t *testing.T) {
	first := testFactVector("fact_a", 1)
	second := testFactVector("fact_b", 2)
	if SourceSetDigest([]FactVector{first, second}) != SourceSetDigest([]FactVector{second, first}) {
		t.Fatal("source set digest changed with vector order")
	}
}

const testAddress = registry.AgentAddr("agent_01ARZ3NDEKTSV4RRFFQ69G5FAV")

type serviceAdapter struct {
	representation Representation
	candidates     []Candidate
	topK           int
}

func (adapter *serviceAdapter) Replace(_ context.Context, representation Representation) error {
	adapter.representation = representation
	return nil
}

func (adapter *serviceAdapter) Search(_ context.Context, _ string, _ []float32, topK int) ([]Candidate, error) {
	adapter.topK = topK
	return adapter.candidates, nil
}

func validSnapshot(vectors ...FactVector) Snapshot {
	snapshot := Snapshot{
		SchemaVersion: RepresentationSchemaVersion, Revision: 1,
		EncoderProfile: EncoderProfile, Vectors: vectors,
	}
	snapshot.SourceSetDigest = SourceSetDigest(vectors)
	return snapshot
}

func mutateSnapshot(snapshot Snapshot, mutate func(*Snapshot)) Snapshot {
	mutate(&snapshot)
	return snapshot
}

func testFactVector(id string, first float32) FactVector {
	return FactVector{VectorID: id, SourceDigest: digest(id), Embedding: testEmbedding(first)}
}

func testEmbedding(first float32) []float32 {
	values := make([]float32, EmbeddingDimensions)
	values[0] = first
	return values
}

func digest(seed string) string {
	const hex = "0123456789abcdef"
	value := make([]byte, 64)
	for index := range value {
		value[index] = hex[(index+len(seed))%len(hex)]
	}
	return "sha256:" + string(value)
}
