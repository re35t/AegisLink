package postgres

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/re35t/AegisLink/index/internal/discovery"
	"github.com/re35t/AegisLink/index/internal/registry"
)

var (
	_ discovery.RepresentationStore = (*DiscoveryRepository)(nil)
	_ discovery.VectorSearch        = (*DiscoveryRepository)(nil)
)

func TestDiscoverySnapshotReplacementAndExactSearch(t *testing.T) {
	database := openCleanIndexDatabase(t)
	repository := NewDiscoveryRepository(database)
	service, err := discovery.NewService(repository, repository)
	if err != nil {
		t.Fatal(err)
	}
	first := registerTestAddress(t, database, "discovery-first")
	second := registerTestAddress(t, database, "discovery-second")

	firstSnapshot := integrationSnapshot(1,
		integrationVector("fact_go", 0, 1),
		integrationVector("fact_security", 1, 1),
	)
	if err := service.Publish(t.Context(), first, firstSnapshot); err != nil {
		t.Fatal(err)
	}
	// Exact same-revision retries are idempotent.
	if err := service.Publish(t.Context(), first, firstSnapshot); err != nil {
		t.Fatalf("idempotent publish: %v", err)
	}
	conflict := integrationSnapshot(1, integrationVector("fact_other", 2, 1))
	if err := service.Publish(t.Context(), first, conflict); !errors.Is(err, discovery.ErrStaleRevision) {
		t.Fatalf("same revision conflict = %v", err)
	}
	if err := service.Publish(t.Context(), second, integrationSnapshot(1,
		integrationVector("fact_design", 2, 1),
	)); err != nil {
		t.Fatal(err)
	}

	result, err := service.Search(t.Context(), integrationQuery(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 2 || result.Candidates[0].AgentAddr != first ||
		result.Candidates[0].MatchedVectorID != "fact_security" || result.Candidates[0].Score < .999 {
		t.Fatalf("search result = %#v", result)
	}

	// A replacement deletes every old Fact Vector before inserting the new set.
	if err := service.Publish(t.Context(), first, integrationSnapshot(2,
		integrationVector("fact_replacement", 3, 1),
	)); err != nil {
		t.Fatal(err)
	}
	var oldVectorCount int64
	if err := database.connection.WithContext(t.Context()).Table("discovery_fact_vectors").
		Where("agent_addr = ? AND vector_id IN ?", string(first), []string{"fact_go", "fact_security"}).
		Count(&oldVectorCount).Error; err != nil || oldVectorCount != 0 {
		t.Fatalf("old vector count=%d error=%v", oldVectorCount, err)
	}

	// Empty snapshots are valid complete replacements and remove the Agent from Search.
	if err := service.Publish(t.Context(), first, integrationSnapshot(3)); err != nil {
		t.Fatal(err)
	}
	result, err = service.Search(t.Context(), integrationQuery(1, 5))
	if err != nil || len(result.Candidates) != 1 || result.Candidates[0].AgentAddr != second {
		t.Fatalf("search after empty replacement = %#v error=%v", result, err)
	}
	if err := database.connection.WithContext(t.Context()).Model(&agentRegistryModel{}).
		Where("agent_addr = ?", string(second)).Update("status", "disabled").Error; err != nil {
		t.Fatal(err)
	}
	result, err = service.Search(t.Context(), integrationQuery(2, 5))
	if err != nil || len(result.Candidates) != 0 {
		t.Fatalf("disabled AgentAddr search = %#v error=%v", result, err)
	}
	if err := service.Publish(t.Context(), registry.AgentAddr("agent_01ARZ3NDEKTSV4RRFFQ69G5FAA"), integrationSnapshot(1)); !errors.Is(err, discovery.ErrAgentNotFound) {
		t.Fatalf("unknown AgentAddr publish = %v", err)
	}
}

func TestConcurrentRepresentationRevisionsNeverRegress(t *testing.T) {
	database := openCleanIndexDatabase(t)
	repository := NewDiscoveryRepository(database)
	service, _ := discovery.NewService(repository, repository)
	address := registerTestAddress(t, database, "concurrent-revisions")
	if err := service.Publish(t.Context(), address, integrationSnapshot(1)); err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	errorsSeen := make(chan error, 2)
	for _, revision := range []int64{7, 8} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsSeen <- service.Publish(t.Context(), address, integrationSnapshot(revision,
				integrationVector(fmt.Sprintf("fact_%d", revision), int(revision), 1),
			))
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil && !errors.Is(err, discovery.ErrStaleRevision) {
			t.Fatalf("concurrent publish error = %v", err)
		}
	}
	var stored discoveryRepresentationModel
	if err := database.connection.WithContext(t.Context()).Where("agent_addr = ?", string(address)).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 8 {
		t.Fatalf("stored revision = %d, want 8", stored.Revision)
	}
}

func openCleanIndexDatabase(t *testing.T) *Database {
	t.Helper()
	database, err := Open(t.Context(), indexTestDatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close Index database: %v", err)
		}
	})
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := database.connection.WithContext(t.Context()).Exec(
		"TRUNCATE discovery_fact_vectors, discovery_representations, agent_registry, agent_addresses",
	).Error; err != nil {
		t.Fatal(err)
	}
	return database
}

func registerTestAddress(t *testing.T, database *Database, key string) registry.AgentAddr {
	t.Helper()
	service, err := registry.NewService(NewRegistryRepository(database))
	if err != nil {
		t.Fatal(err)
	}
	registered, replayed, err := service.Register(t.Context(), registry.HashIdempotencyKey(
		"0123456789abcdef0123456789abcdef", key,
	))
	if err != nil || replayed {
		t.Fatalf("register address replayed=%v error=%v", replayed, err)
	}
	return registered.AgentAddr
}

func integrationSnapshot(revision int64, vectors ...discovery.FactVector) discovery.Snapshot {
	snapshot := discovery.Snapshot{
		SchemaVersion: discovery.RepresentationSchemaVersion, Revision: revision,
		EncoderProfile: discovery.EncoderProfile, Vectors: vectors,
	}
	snapshot.SourceSetDigest = discovery.SourceSetDigest(vectors)
	return snapshot
}

func integrationVector(id string, dimension int, value float32) discovery.FactVector {
	embedding := make([]float32, discovery.EmbeddingDimensions)
	embedding[dimension] = value
	digest := sha256.Sum256([]byte(id))
	return discovery.FactVector{
		VectorID: id, SourceDigest: "sha256:" + hex.EncodeToString(digest[:]), Embedding: embedding,
	}
}

func integrationQuery(dimension, topK int) discovery.Query {
	embedding := make([]float32, discovery.EmbeddingDimensions)
	embedding[dimension] = 1
	return discovery.Query{
		SchemaVersion: discovery.QuerySchemaVersion, EncoderProfile: discovery.EncoderProfile,
		Embedding: embedding, TopK: topK,
	}
}
