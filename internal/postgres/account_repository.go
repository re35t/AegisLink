package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"gorm.io/gorm"
)

type AccountRepository struct {
	database *gorm.DB
}

var _ account.Repository = (*AccountRepository)(nil)

func NewAccountRepository(database *gorm.DB) *AccountRepository {
	return &AccountRepository{database: database}
}

func (repository *AccountRepository) CreateAccountWithAgent(ctx context.Context, registration account.Registration) (account.Identity, error) {
	agentRecord := registration.Agent
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if result := exec(transaction, `
			INSERT INTO human_principals (id, display_name, status)
			VALUES ($1, $2, 'active')`, registration.User.ID, registration.User.DisplayName); result.Error != nil {
			return fmt.Errorf("create human principal: %w", result.Error)
		}
		if result := exec(transaction, `
			INSERT INTO user_accounts (id, principal_id, email, password_hash, status)
			VALUES ($1, $2, $3, $4, 'active')`,
			registration.Account.ID,
			registration.Account.PrincipalID,
			registration.Account.Email,
			registration.Account.PasswordHash,
		); result.Error != nil {
			if isUniqueViolation(result.Error) {
				return account.ErrEmailTaken
			}
			return fmt.Errorf("create user account: %w", result.Error)
		}
		language := registration.Preferences.Language
		if language == "" {
			language = account.LanguageSystem
		}
		theme := registration.Preferences.Theme
		if theme == "" {
			theme = account.ThemeSystem
		}
		if result := exec(transaction, `
			INSERT INTO user_preferences (principal_id, language, theme)
			VALUES ($1, $2, $3)`, registration.User.ID, language, theme); result.Error != nil {
			return fmt.Errorf("create user preferences: %w", result.Error)
		}
		var timestamps struct {
			CreatedAt time.Time
			UpdatedAt time.Time
		}
		if err := scanOne(transaction, &timestamps, `
			INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING created_at, updated_at`,
			agentRecord.ID,
			agentRecord.OwnerPrincipalID,
			agentRecord.Name,
			agentRecord.Description,
			agentRecord.SystemPrompt,
		); err != nil {
			return fmt.Errorf("create personal agent: %w", err)
		}
		agentRecord.CreatedAt = timestamps.CreatedAt
		agentRecord.UpdatedAt = timestamps.UpdatedAt
		if result := exec(transaction, `
			INSERT INTO agent_profiles (agent_id, owner_principal_id)
			VALUES ($1, $2)`, agentRecord.ID, agentRecord.OwnerPrincipalID); result.Error != nil {
			return fmt.Errorf("create personal agent profile: %w", result.Error)
		}
		return nil
	})
	if err != nil {
		return account.Identity{}, fmt.Errorf("register account: %w", err)
	}
	return account.Identity{Account: registration.Account, User: registration.User, Agent: agentRecord}, nil
}

type accountIdentityRow struct {
	AccountID          string    `gorm:"column:account_id"`
	AccountPrincipalID string    `gorm:"column:account_principal_id"`
	Email              string    `gorm:"column:email"`
	PasswordHash       string    `gorm:"column:password_hash"`
	AccountStatus      string    `gorm:"column:account_status"`
	PrincipalID        string    `gorm:"column:principal_id"`
	DisplayName        string    `gorm:"column:display_name"`
	AgentID            string    `gorm:"column:agent_id"`
	AgentPrincipalID   string    `gorm:"column:agent_principal_id"`
	AgentName          string    `gorm:"column:agent_name"`
	AgentDescription   string    `gorm:"column:agent_description"`
	SystemPrompt       string    `gorm:"column:system_prompt"`
	AgentCreatedAt     time.Time `gorm:"column:agent_created_at"`
	AgentUpdatedAt     time.Time `gorm:"column:agent_updated_at"`
}

func (repository *AccountRepository) FindIdentityByEmail(ctx context.Context, email string) (account.Identity, error) {
	var row accountIdentityRow
	err := scanOne(repository.database.WithContext(ctx), &row, `
		SELECT accounts.id AS account_id, accounts.principal_id AS account_principal_id,
		       accounts.email, accounts.password_hash, accounts.status AS account_status,
		       principals.id AS principal_id, principals.display_name,
		       agents.id AS agent_id, agents.owner_principal_id AS agent_principal_id,
		       agents.name AS agent_name, agents.description AS agent_description, agents.system_prompt,
		       agents.created_at AS agent_created_at, agents.updated_at AS agent_updated_at
		FROM user_accounts accounts
		JOIN human_principals principals ON principals.id=accounts.principal_id
		JOIN LATERAL (
			SELECT id, owner_principal_id, name, description, system_prompt, created_at, updated_at
			FROM agents
			WHERE owner_principal_id=principals.id
			ORDER BY created_at, id
			LIMIT 1
		) agents ON true
		WHERE accounts.email=$1 AND principals.status='active'`, email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return account.Identity{}, account.ErrInvalidCredentials
	}
	if err != nil {
		return account.Identity{}, fmt.Errorf("find account identity: %w", err)
	}
	return account.Identity{
		Account: account.Account{ID: row.AccountID, PrincipalID: row.AccountPrincipalID, Email: row.Email, PasswordHash: row.PasswordHash, Status: row.AccountStatus},
		User:    account.User{ID: row.PrincipalID, DisplayName: row.DisplayName},
		Agent:   accountAgent(row),
	}, nil
}

func accountAgent(row accountIdentityRow) (item agent.Agent) {
	item.ID = row.AgentID
	item.OwnerPrincipalID = row.AgentPrincipalID
	item.Name = row.AgentName
	item.Description = row.AgentDescription
	item.SystemPrompt = row.SystemPrompt
	item.CreatedAt = row.AgentCreatedAt
	item.UpdatedAt = row.AgentUpdatedAt
	return item
}

func (repository *AccountRepository) CreateSession(ctx context.Context, session account.Session) error {
	result := exec(repository.database.WithContext(ctx), `
		INSERT INTO account_sessions (id, account_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, session.ID, session.AccountID, session.TokenHash, session.ExpiresAt)
	if result.Error != nil {
		return fmt.Errorf("create account session: %w", result.Error)
	}
	return nil
}

type actorRow struct {
	SessionID   string
	AccountID   string
	Email       string
	PrincipalID string
	DisplayName string
	ExpiresAt   time.Time
}

func (repository *AccountRepository) AuthenticateSession(ctx context.Context, tokenHash []byte, now time.Time) (account.Actor, error) {
	var row actorRow
	err := scanOne(repository.database.WithContext(ctx), &row, `
		WITH valid_session AS (
			UPDATE account_sessions sessions
			SET last_seen_at=$2
			FROM user_accounts accounts, human_principals principals
			WHERE sessions.token_hash=$1
			  AND sessions.account_id=accounts.id
			  AND accounts.principal_id=principals.id
			  AND sessions.revoked_at IS NULL
			  AND sessions.expires_at>$2
			  AND accounts.status='active'
			  AND principals.status='active'
			RETURNING sessions.id AS session_id, sessions.account_id, sessions.expires_at,
			          accounts.email, principals.id AS principal_id, principals.display_name
		)
		SELECT session_id, account_id, email, principal_id, display_name, expires_at
		FROM valid_session`, tokenHash, now)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return account.Actor{}, account.ErrUnauthenticated
	}
	if err != nil {
		return account.Actor{}, fmt.Errorf("authenticate account session: %w", err)
	}
	return account.Actor{
		SessionID: row.SessionID, AccountID: row.AccountID, Email: row.Email,
		User: account.User{ID: row.PrincipalID, DisplayName: row.DisplayName}, ExpiresAt: row.ExpiresAt,
	}, nil
}

func (repository *AccountRepository) RevokeSession(ctx context.Context, tokenHash []byte) error {
	result := exec(repository.database.WithContext(ctx), `
		UPDATE account_sessions SET revoked_at=COALESCE(revoked_at, now())
		WHERE token_hash=$1`, tokenHash)
	if result.Error != nil {
		return fmt.Errorf("revoke account session: %w", result.Error)
	}
	return nil
}

type settingsRow struct {
	Email       string
	Status      string
	PrincipalID string
	DisplayName string
	Language    account.Language
	Theme       account.Theme
}

func (repository *AccountRepository) GetSettings(ctx context.Context, accountID string) (account.Settings, error) {
	var row settingsRow
	err := scanOne(repository.database.WithContext(ctx), &row, `
		SELECT accounts.email, accounts.status,
		       principals.id AS principal_id, principals.display_name,
		       preferences.language, preferences.theme
		FROM user_accounts accounts
		JOIN human_principals principals ON principals.id=accounts.principal_id
		JOIN user_preferences preferences ON preferences.principal_id=principals.id
		WHERE accounts.id=$1 AND accounts.status='active' AND principals.status='active'`, accountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return account.Settings{}, account.ErrUnauthenticated
	}
	if err != nil {
		return account.Settings{}, fmt.Errorf("get account settings: %w", err)
	}
	return account.Settings{
		Account:     account.AccountSummary{Email: row.Email, Status: row.Status},
		User:        account.User{ID: row.PrincipalID, DisplayName: row.DisplayName},
		Preferences: account.Preferences{Language: row.Language, Theme: row.Theme},
	}, nil
}

func (repository *AccountRepository) UpdateSettings(ctx context.Context, accountID, principalID string, update account.SettingsUpdate) (account.Settings, error) {
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var displayName any
		if update.DisplayName != nil {
			displayName = *update.DisplayName
		}
		result := exec(transaction, `
			UPDATE human_principals principals
			SET display_name=COALESCE($3::text, principals.display_name), updated_at=now()
			FROM user_accounts accounts
			WHERE accounts.id=$1 AND accounts.principal_id=$2
			  AND principals.id=accounts.principal_id
			  AND accounts.status='active' AND principals.status='active'`, accountID, principalID, displayName)
		if result.Error != nil {
			return fmt.Errorf("update account profile: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return account.ErrUnauthenticated
		}
		var language, theme any
		if update.Language != nil {
			language = string(*update.Language)
		}
		if update.Theme != nil {
			theme = string(*update.Theme)
		}
		result = exec(transaction, `
			UPDATE user_preferences
			SET language=COALESCE($2::text, language),
			    theme=COALESCE($3::text, theme),
			    updated_at=now()
			WHERE principal_id=$1`, principalID, language, theme)
		if result.Error != nil {
			return fmt.Errorf("update user preferences: %w", result.Error)
		}
		return nil
	})
	if err != nil {
		return account.Settings{}, err
	}
	return repository.GetSettings(ctx, accountID)
}

func (repository *AccountRepository) PasswordHash(ctx context.Context, accountID string) (string, error) {
	var row struct{ PasswordHash string }
	err := scanOne(repository.database.WithContext(ctx), &row, `
		SELECT password_hash FROM user_accounts
		WHERE id=$1 AND status='active'`, accountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", account.ErrUnauthenticated
	}
	if err != nil {
		return "", fmt.Errorf("get account password hash: %w", err)
	}
	return row.PasswordHash, nil
}

func (repository *AccountRepository) ChangePassword(ctx context.Context, accountID, currentSessionID, passwordHash string) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		result := exec(transaction, `
			UPDATE user_accounts
			SET password_hash=$2, updated_at=now()
			WHERE id=$1 AND status='active'`, accountID, passwordHash)
		if result.Error != nil {
			return fmt.Errorf("update account password: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return account.ErrUnauthenticated
		}
		result = exec(transaction, `
			UPDATE account_sessions
			SET revoked_at=COALESCE(revoked_at, now())
			WHERE account_id=$1 AND id<>$2 AND revoked_at IS NULL`, accountID, currentSessionID)
		if result.Error != nil {
			return fmt.Errorf("revoke other account sessions: %w", result.Error)
		}
		return nil
	})
}
