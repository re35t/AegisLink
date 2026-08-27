package registry

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

var emptyRegistrationRequestDigest = sha256.Sum256([]byte("{}"))

type Service struct {
	repository Repository
	now        func() time.Time
	newAddr    func() AgentAddr
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errorsJoin(ErrInvalid, "registry repository is required")
	}
	return &Service{
		repository: repository,
		now:        func() time.Time { return time.Now().UTC() },
		newAddr:    func() AgentAddr { return AgentAddr("agent_" + ulid.Make().String()) },
	}, nil
}

func (service *Service) Register(ctx context.Context, idempotencyKeyHash []byte) (Registration, bool, error) {
	if len(idempotencyKeyHash) != sha256.Size {
		return Registration{}, false, errorsJoin(ErrInvalid, "invalid idempotency key digest")
	}
	address := service.newAddr()
	if !ValidAgentAddr(address) {
		return Registration{}, false, errorsJoin(ErrInvalid, "generated AgentAddr is invalid")
	}
	// PostgreSQL timestamptz stores microseconds. Normalize before the first
	// response so an idempotent replay returns byte-equivalent timestamps.
	now := service.now().UTC().Truncate(time.Microsecond)
	record := Record{
		SchemaVersion:      DraftSchemaVersion,
		AgentAddr:          address,
		Status:             StatusActive,
		IdempotencyKeyHash: append([]byte(nil), idempotencyKeyHash...),
		RequestDigest:      append([]byte(nil), emptyRegistrationRequestDigest[:]...),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	stored, replayed, err := service.repository.Create(ctx, record)
	if err != nil {
		return Registration{}, false, err
	}
	return stored.Registration(), replayed, nil
}

func HashIdempotencyKey(registrationToken, key string) []byte {
	digest := sha256.Sum256([]byte(registrationToken + "\x00" + key))
	return digest[:]
}

func ValidateIdempotencyKey(key string) error {
	if len(key) < 1 || len(key) > 128 {
		return errorsJoin(ErrInvalid, "Idempotency-Key must contain 1 to 128 bytes")
	}
	for _, character := range key {
		if character < 0x20 || character == 0x7f {
			return errorsJoin(ErrInvalid, "Idempotency-Key must not contain control characters")
		}
	}
	return nil
}

func ValidAgentAddr(address AgentAddr) bool {
	value := string(address)
	if !strings.HasPrefix(value, "agent_") || len(value) != len("agent_")+26 {
		return false
	}
	_, err := ulid.ParseStrict(strings.TrimPrefix(value, "agent_"))
	return err == nil
}

func errorsJoin(base error, detail string) error {
	return fmt.Errorf("%w: %s", base, detail)
}
