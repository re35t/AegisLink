package account

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/agent"
)

const (
	minimumPasswordLength = 10
	maximumPasswordLength = 128
	maximumDisplayName    = 80
)

type Service struct {
	repository Repository
	sessionTTL time.Duration
	hasher     passwordHasher
	dummyHash  string
	now        func() time.Time
}

func NewService(repository Repository, sessionTTL time.Duration) (*Service, error) {
	hasher := passwordHasher{}
	dummyHash, err := hasher.Hash("not-a-real-user-password")
	if err != nil {
		return nil, err
	}
	return &Service{
		repository: repository,
		sessionTTL: sessionTTL,
		hasher:     hasher,
		dummyHash:  dummyHash,
		now:        time.Now,
	}, nil
}

func (service *Service) Register(ctx context.Context, displayName, email, password string) (AuthResult, error) {
	displayName = strings.TrimSpace(displayName)
	normalizedEmail, emailOK := normalizeEmail(email)
	if displayName == "" || utf8.RuneCountInString(displayName) > maximumDisplayName || !emailOK || !validNewPassword(password) {
		return AuthResult{}, ErrInvalidRegistration
	}
	passwordHash, err := service.hasher.Hash(password)
	if err != nil {
		return AuthResult{}, err
	}
	principalID := ulid.Make().String()
	registration := Registration{
		User: User{ID: principalID, DisplayName: displayName},
		Account: Account{
			ID:           ulid.Make().String(),
			PrincipalID:  principalID,
			Email:        normalizedEmail,
			PasswordHash: passwordHash,
			Status:       "active",
		},
		Agent: agent.Agent{
			ID:               ulid.Make().String(),
			OwnerPrincipalID: principalID,
			Name:             "Aegis",
			Description:      "A private, focused personal agent.",
			SystemPrompt:     "You are Aegis, a concise and reliable personal assistant. Answer in the language used by the user.",
		},
	}
	identity, err := service.repository.CreateAccountWithAgent(ctx, registration)
	if err != nil {
		return AuthResult{}, err
	}
	return service.issueSession(ctx, identity)
}

func (service *Service) Login(ctx context.Context, email, password string) (AuthResult, error) {
	normalizedEmail, emailOK := normalizeEmail(email)
	if !emailOK || len(password) == 0 || len(password) > maximumPasswordLength {
		service.hasher.Verify(password, service.dummyHash)
		return AuthResult{}, ErrInvalidCredentials
	}
	identity, err := service.repository.FindIdentityByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			service.hasher.Verify(password, service.dummyHash)
		}
		return AuthResult{}, err
	}
	if identity.Account.Status != "active" || !service.hasher.Verify(password, identity.Account.PasswordHash) {
		return AuthResult{}, ErrInvalidCredentials
	}
	return service.issueSession(ctx, identity)
}

func (service *Service) Authenticate(ctx context.Context, token string) (Actor, error) {
	if strings.TrimSpace(token) == "" {
		return Actor{}, ErrUnauthenticated
	}
	return service.repository.AuthenticateSession(ctx, sessionTokenHash(token), service.now())
}

func (service *Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return service.repository.RevokeSession(ctx, sessionTokenHash(token))
}

func (service *Service) issueSession(ctx context.Context, identity Identity) (AuthResult, error) {
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return AuthResult{}, err
	}
	expiresAt := service.now().Add(service.sessionTTL)
	if err := service.repository.CreateSession(ctx, Session{
		ID: ulid.Make().String(), AccountID: identity.Account.ID, TokenHash: tokenHash, ExpiresAt: expiresAt,
	}); err != nil {
		return AuthResult{}, err
	}
	return AuthResult{User: identity.User, Agent: identity.Agent, SessionToken: token, ExpiresAt: expiresAt}, nil
}

func normalizeEmail(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || len(normalized) > 254 {
		return "", false
	}
	address, err := mail.ParseAddress(normalized)
	return normalized, err == nil && address.Address == normalized
}

func validNewPassword(password string) bool {
	return len(password) >= minimumPasswordLength && len(password) <= maximumPasswordLength && utf8.ValidString(password)
}
