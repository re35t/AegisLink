package registry

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceAllocatesAndStoresAgentAddr(t *testing.T) {
	repository := &memoryRepository{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 8, 27, 1, 2, 3, 123456789, time.UTC) }
	service.newAddr = func() AgentAddr { return "agent_01ARZ3NDEKTSV4RRFFQ69G5FAV" }
	keyHash := HashIdempotencyKey("0123456789abcdef0123456789abcdef", "registration-one")

	registration, replayed, err := service.Register(t.Context(), keyHash)
	if err != nil {
		t.Fatal(err)
	}
	if replayed {
		t.Fatal("first registration was reported as replayed")
	}
	if registration.SchemaVersion != DraftSchemaVersion || registration.AgentAddr != "agent_01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("registration = %#v", registration)
	}
	wantTime := time.Date(2026, 8, 27, 1, 2, 3, 123456000, time.UTC)
	if !registration.CreatedAt.Equal(wantTime) {
		t.Fatalf("createdAt = %s want %s", registration.CreatedAt, wantTime)
	}
	exists, err := repository.Exists(t.Context(), registration.AgentAddr)
	if err != nil || !exists {
		t.Fatalf("stored address exists=%v error=%v", exists, err)
	}
	if repository.record.Status != StatusActive || !bytes.Equal(repository.record.RequestDigest, emptyRegistrationRequestDigest[:]) {
		t.Fatalf("stored record = %#v", repository.record)
	}

	replayedRegistration, replayed, err := service.Register(t.Context(), keyHash)
	if err != nil || !replayed || replayedRegistration != registration {
		t.Fatalf("replay registration=%#v replayed=%v error=%v", replayedRegistration, replayed, err)
	}
}

func TestRegistrationRejectsInvalidInternalInputs(t *testing.T) {
	service, err := NewService(&memoryRepository{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Register(t.Context(), []byte("short")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short digest error = %v", err)
	}
	service.newAddr = func() AgentAddr { return "not-an-agent-address" }
	if _, _, err := service.Register(t.Context(), make([]byte, 32)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid generated address error = %v", err)
	}
	if _, err := NewService(nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil repository error = %v", err)
	}
}

func TestAgentAddrAndIdempotencyKeyValidation(t *testing.T) {
	if !ValidAgentAddr("agent_01ARZ3NDEKTSV4RRFFQ69G5FAV") {
		t.Fatal("valid AgentAddr was rejected")
	}
	for _, invalid := range []AgentAddr{"", "agent_short", "other_01ARZ3NDEKTSV4RRFFQ69G5FAV", "agent_01ARZ3NDEKTSV4RRFFQ69G5FAI"} {
		if ValidAgentAddr(invalid) {
			t.Fatalf("invalid AgentAddr %q was accepted", invalid)
		}
	}

	if err := ValidateIdempotencyKey("retry-key"); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", strings.Repeat("a", 129), "line\nbreak"} {
		if err := ValidateIdempotencyKey(invalid); !errors.Is(err, ErrInvalid) {
			t.Fatalf("key %q error = %v", invalid, err)
		}
	}
	one := HashIdempotencyKey("token-one", "same-key")
	two := HashIdempotencyKey("token-two", "same-key")
	if bytes.Equal(one, two) || len(one) != 32 {
		t.Fatalf("unexpected scoped digests: %x %x", one, two)
	}
}

type memoryRepository struct {
	record Record
}

func (repository *memoryRepository) Create(_ context.Context, record Record) (Record, bool, error) {
	if repository.record.AgentAddr != "" {
		if bytes.Equal(repository.record.IdempotencyKeyHash, record.IdempotencyKeyHash) && bytes.Equal(repository.record.RequestDigest, record.RequestDigest) {
			return repository.record, true, nil
		}
		return Record{}, false, ErrIdempotencyConflict
	}
	repository.record = record
	return record, false, nil
}

func (repository *memoryRepository) Exists(_ context.Context, address AgentAddr) (bool, error) {
	return repository.record.AgentAddr == address, nil
}
