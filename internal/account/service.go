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
			SystemPrompt:     "Be concise and reliable. Answer in the language used by the user.",
		},
		Preferences: Preferences{Language: LanguageSystem, Theme: ThemeSystem},
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

func (service *Service) GetSettings(ctx context.Context, actor Actor) (Settings, error) {
	return service.repository.GetSettings(ctx, actor.AccountID)
}

func (service *Service) UpdateSettings(ctx context.Context, actor Actor, update SettingsUpdate) (Settings, error) {
	if update.DisplayName == nil && update.Language == nil && update.Theme == nil {
		return Settings{}, ErrInvalidSettings
	}
	if update.DisplayName != nil {
		displayName := strings.TrimSpace(*update.DisplayName)
		if displayName == "" || utf8.RuneCountInString(displayName) > maximumDisplayName {
			return Settings{}, ErrInvalidSettings
		}
		update.DisplayName = &displayName
	}
	if update.Language != nil && !validLanguage(*update.Language) {
		return Settings{}, ErrInvalidSettings
	}
	if update.Theme != nil && !validTheme(*update.Theme) {
		return Settings{}, ErrInvalidSettings
	}
	return service.repository.UpdateSettings(ctx, actor.AccountID, actor.User.ID, update)
}

func (service *Service) ChangePassword(ctx context.Context, actor Actor, currentPassword, newPassword string) error {
	if currentPassword == "" || len(currentPassword) > maximumPasswordLength || !validNewPassword(newPassword) || currentPassword == newPassword {
		return ErrInvalidPassword
	}
	passwordHash, err := service.repository.PasswordHash(ctx, actor.AccountID)
	if err != nil {
		return err
	}
	if !service.hasher.Verify(currentPassword, passwordHash) {
		return ErrCurrentPassword
	}
	newHash, err := service.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	return service.repository.ChangePassword(ctx, actor.AccountID, actor.SessionID, newHash)
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

func validLanguage(language Language) bool {
	return language == LanguageSystem || language == LanguageEnglish || language == LanguageChinese
}

func validTheme(theme Theme) bool {
	return theme == ThemeSystem || theme == ThemeLight || theme == ThemeDark
}
