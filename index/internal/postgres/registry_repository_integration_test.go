package postgres

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/re35t/AegisLink/index/internal/registry"
	indexmigrations "github.com/re35t/AegisLink/index/migrations"
)

var _ registry.Repository = (*RegistryRepository)(nil)

func TestRegistryPersistenceAndIdempotency(t *testing.T) {
	databaseURL := indexTestDatabaseURL(t)
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("close Index database: %v", err)
		}
	}()
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := database.connection.WithContext(t.Context()).Exec(
		"TRUNCATE discovery_fact_vectors, discovery_representations, agent_registry, agent_addresses",
	).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewRegistryRepository(database)
	service, err := registry.NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	keyHash := registry.HashIdempotencyKey("0123456789abcdef0123456789abcdef", "persistent-registration")
	created, replayed, err := service.Register(t.Context(), keyHash)
	if err != nil || replayed {
		t.Fatalf("create registration=%#v replayed=%v error=%v", created, replayed, err)
	}
	if !registry.ValidAgentAddr(created.AgentAddr) || created.SchemaVersion != registry.DraftSchemaVersion {
		t.Fatalf("created registration = %#v", created)
	}

	var storedCount int64
	if err := database.connection.WithContext(t.Context()).
		Table("agent_registry").
		Where("agent_addr = ?", string(created.AgentAddr)).
		Count(&storedCount).Error; err != nil || storedCount != 1 {
		t.Fatalf("stored count=%d error=%v", storedCount, err)
	}
	exists, err := repository.Exists(t.Context(), created.AgentAddr)
	if err != nil || !exists {
		t.Fatalf("repository exists=%v error=%v", exists, err)
	}

	secondDatabase, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := secondDatabase.Close(); err != nil {
			t.Errorf("close second Index database: %v", err)
		}
	}()
	if err := secondDatabase.Migrate(); err != nil {
		t.Fatal(err)
	}
	restartedRepository := NewRegistryRepository(secondDatabase)
	restartedService, err := registry.NewService(restartedRepository)
	if err != nil {
		t.Fatal(err)
	}
	replayedRegistration, replayed, err := restartedService.Register(t.Context(), keyHash)
	if err != nil || !replayed || replayedRegistration != created {
		t.Fatalf("replay registration=%#v replayed=%v error=%v", replayedRegistration, replayed, err)
	}
	exists, err = restartedRepository.Exists(t.Context(), created.AgentAddr)
	if err != nil || !exists {
		t.Fatalf("restarted repository exists=%v error=%v", exists, err)
	}

	conflicting := registry.Record{
		SchemaVersion:      registry.DraftSchemaVersion,
		AgentAddr:          "agent_01ARZ3NDEKTSV4RRFFQ69G5FAA",
		Status:             registry.StatusActive,
		IdempotencyKeyHash: append([]byte(nil), keyHash...),
		RequestDigest:      make([]byte, 32),
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
	if _, _, err := restartedRepository.Create(t.Context(), conflicting); !errors.Is(err, registry.ErrIdempotencyConflict) {
		t.Fatalf("conflicting replay error = %v", err)
	}

	second, replayed, err := restartedService.Register(
		t.Context(),
		registry.HashIdempotencyKey("0123456789abcdef0123456789abcdef", "second-registration"),
	)
	if err != nil || replayed || second.AgentAddr == created.AgentAddr {
		t.Fatalf("second registration=%#v replayed=%v error=%v", second, replayed, err)
	}
}

func TestAgentRegistryMigrationPreservesAllocatedLegacyAddress(t *testing.T) {
	databaseURL := indexTestDatabaseURL(t)
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("close Index database: %v", err)
		}
	}()
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	// Restore the latest migration even when an assertion aborts the test.
	defer func() {
		if err := database.Migrate(); err != nil {
			t.Errorf("restore Index migrations: %v", err)
		}
	}()

	sqlDatabase, err := database.connection.DB()
	if err != nil {
		t.Fatal(err)
	}
	goose.SetBaseFS(indexmigrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(sqlDatabase, ".", 1); err != nil {
		t.Fatal(err)
	}
	if err := database.connection.WithContext(t.Context()).Exec("TRUNCATE agent_addresses").Error; err != nil {
		t.Fatal(err)
	}

	address := registry.AgentAddr("agent_01ARZ3NDEKTSV4RRFFQ69G5FAV")
	keyHash := registry.HashIdempotencyKey("0123456789abcdef0123456789abcdef", "legacy-registration")
	legacyRequestDigest := make([]byte, 32)
	createdAt := time.Date(2026, 8, 26, 1, 2, 3, 0, time.UTC)
	if err := database.connection.WithContext(t.Context()).Exec(`
		INSERT INTO agent_addresses (
			agent_id, schema_version, agent_name, facts_url, ttl_seconds, lsh_json,
			revision, idempotency_key_hash, request_digest, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, CAST(? AS jsonb), ?, ?, ?, ?, ?)
	`, string(address), "aegislink.agent-addr/0.1-draft", "Legacy Agent",
		"https://legacy.example/agentfacts", 3600, `{}`, 1, keyHash,
		legacyRequestDigest, createdAt, createdAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}

	repository := NewRegistryRepository(database)
	exists, err := repository.Exists(t.Context(), address)
	if err != nil || !exists {
		t.Fatalf("migrated address exists=%v error=%v", exists, err)
	}
	service, err := registry.NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	replayed, wasReplay, err := service.Register(t.Context(), keyHash)
	if err != nil || !wasReplay || replayed.AgentAddr != address || !replayed.CreatedAt.Equal(createdAt) {
		t.Fatalf("migrated replay=%#v replayed=%v error=%v", replayed, wasReplay, err)
	}
}

func indexTestDatabaseURL(t *testing.T) string {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("INDEX_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("INDEX_TEST_DATABASE_URL is not set")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	databaseName, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/"))
	if err != nil {
		t.Fatal(err)
	}
	if databaseName == "" || strings.Contains(databaseName, "/") || !strings.HasSuffix(databaseName, "_test") {
		t.Fatal(fmt.Errorf("refusing destructive Index tests against database %q: INDEX_TEST_DATABASE_URL must name a dedicated *_test database", databaseName))
	}
	return databaseURL
}
