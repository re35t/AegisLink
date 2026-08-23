package account

import (
	"context"
	"testing"
	"time"
)

func TestRegisterCreatesAccountPrincipalAgentAndHashedSession(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Register(t.Context(), " Test User ", " TEST@Example.com ", "long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	registration := repository.registration
	if registration.User.DisplayName != "Test User" || registration.Account.Email != "test@example.com" {
		t.Fatalf("registration was not normalized: %#v", registration)
	}
	if registration.Account.PrincipalID != registration.User.ID || registration.Agent.OwnerPrincipalID != registration.User.ID {
		t.Fatalf("principal ownership was not preserved: %#v", registration)
	}
	if registration.Account.PasswordHash == "long-enough-password" || !service.hasher.Verify("long-enough-password", registration.Account.PasswordHash) {
		t.Fatal("password was not securely hashed")
	}
	if result.SessionToken == "" || len(repository.session.TokenHash) != 32 {
		t.Fatalf("session was not issued correctly: %#v", repository.session)
	}
	if string(repository.session.TokenHash) == result.SessionToken {
		t.Fatal("raw session token was persisted")
	}
	if string(repository.session.TokenHash) != string(sessionTokenHash(result.SessionToken)) {
		t.Fatal("persisted session hash does not match the issued token")
	}
}

func TestRegisterRejectsInvalidInputBeforePersistence(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Register(t.Context(), "", "not-an-email", "short"); err != ErrInvalidRegistration {
		t.Fatalf("expected invalid registration, got %v", err)
	}
	if repository.registration.User.ID != "" {
		t.Fatal("invalid registration reached the repository")
	}
}

func TestLoginVerifiesPasswordAndReturnsGenericCredentialError(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	passwordHash, err := service.hasher.Hash("long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	repository.identity = Identity{
		Account: Account{ID: "account-1", PrincipalID: "principal-1", Email: "test@example.com", PasswordHash: passwordHash, Status: "active"},
		User:    User{ID: "principal-1", DisplayName: "Test user"},
	}
	if _, err := service.Login(t.Context(), "TEST@example.com", "wrong-password"); err != ErrInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	if _, err := service.Login(t.Context(), "TEST@example.com", "long-enough-password"); err != nil {
		t.Fatalf("valid login failed: %v", err)
	}
}

type fakeRepository struct {
	registration Registration
	identity     Identity
	session      Session
	actor        Actor
}

func (repository *fakeRepository) CreateAccountWithAgent(_ context.Context, registration Registration) (Identity, error) {
	repository.registration = registration
	return Identity{Account: registration.Account, User: registration.User, Agent: registration.Agent}, nil
}

func (repository *fakeRepository) FindIdentityByEmail(context.Context, string) (Identity, error) {
	if repository.identity.Account.ID == "" {
		return Identity{}, ErrInvalidCredentials
	}
	return repository.identity, nil
}

func (repository *fakeRepository) CreateSession(_ context.Context, session Session) error {
	repository.session = session
	return nil
}

func (repository *fakeRepository) AuthenticateSession(context.Context, []byte, time.Time) (Actor, error) {
	if repository.actor.AccountID == "" {
		return Actor{}, ErrUnauthenticated
	}
	return repository.actor, nil
}

func (repository *fakeRepository) RevokeSession(context.Context, []byte) error { return nil }
