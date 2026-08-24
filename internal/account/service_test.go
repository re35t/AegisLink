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
	if registration.Preferences.Language != LanguageSystem || registration.Preferences.Theme != ThemeSystem {
		t.Fatalf("registration defaults were not set: %#v", registration.Preferences)
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

func TestUpdateSettingsNormalizesAndValidatesPreferences(t *testing.T) {
	repository := &fakeRepository{settings: Settings{
		Account:     AccountSummary{Email: "test@example.com", Status: "active"},
		User:        User{ID: "principal-1", DisplayName: "Test user"},
		Preferences: Preferences{Language: LanguageSystem, Theme: ThemeSystem},
	}}
	service, err := NewService(repository, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	displayName := "  Updated user  "
	language := LanguageChinese
	theme := ThemeDark
	updated, err := service.UpdateSettings(t.Context(), Actor{AccountID: "account-1", User: User{ID: "principal-1"}}, SettingsUpdate{
		DisplayName: &displayName, Language: &language, Theme: &theme,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.User.DisplayName != "Updated user" || updated.Preferences.Language != LanguageChinese || updated.Preferences.Theme != ThemeDark {
		t.Fatalf("unexpected settings: %#v", updated)
	}
	invalidLanguage := Language("unsupported")
	if _, err := service.UpdateSettings(t.Context(), Actor{}, SettingsUpdate{Language: &invalidLanguage}); err != ErrInvalidSettings {
		t.Fatalf("expected invalid settings, got %v", err)
	}
}

func TestChangePasswordVerifiesCurrentPasswordAndRevokesOtherSessions(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	repository.passwordHash, err = service.hasher.Hash("current-password")
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{AccountID: "account-1", SessionID: "session-current"}
	if err := service.ChangePassword(t.Context(), actor, "wrong-password", "different-password"); err != ErrCurrentPassword {
		t.Fatalf("expected current password error, got %v", err)
	}
	if err := service.ChangePassword(t.Context(), actor, "current-password", "different-password"); err != nil {
		t.Fatal(err)
	}
	if repository.changedAccountID != actor.AccountID || repository.keptSessionID != actor.SessionID || !service.hasher.Verify("different-password", repository.changedPasswordHash) {
		t.Fatalf("password change was not persisted safely: %#v", repository)
	}
}

type fakeRepository struct {
	registration        Registration
	identity            Identity
	session             Session
	actor               Actor
	settings            Settings
	settingsUpdate      SettingsUpdate
	passwordHash        string
	changedAccountID    string
	keptSessionID       string
	changedPasswordHash string
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

func (repository *fakeRepository) GetSettings(context.Context, string) (Settings, error) {
	return repository.settings, nil
}

func (repository *fakeRepository) UpdateSettings(_ context.Context, _, _ string, update SettingsUpdate) (Settings, error) {
	repository.settingsUpdate = update
	if update.DisplayName != nil {
		repository.settings.User.DisplayName = *update.DisplayName
	}
	if update.Language != nil {
		repository.settings.Preferences.Language = *update.Language
	}
	if update.Theme != nil {
		repository.settings.Preferences.Theme = *update.Theme
	}
	return repository.settings, nil
}

func (repository *fakeRepository) PasswordHash(context.Context, string) (string, error) {
	return repository.passwordHash, nil
}

func (repository *fakeRepository) ChangePassword(_ context.Context, accountID, currentSessionID, passwordHash string) error {
	repository.changedAccountID = accountID
	repository.keptSessionID = currentSessionID
	repository.changedPasswordHash = passwordHash
	return nil
}
