package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/re35t/AegisLink/internal/account"
)

type AccountRepository struct {
	database *sql.DB
}

var _ account.Repository = (*AccountRepository)(nil)

func NewAccountRepository(database *sql.DB) *AccountRepository {
	return &AccountRepository{database: database}
}

func (repository *AccountRepository) CreateAccountWithAgent(ctx context.Context, registration account.Registration) (account.Identity, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return account.Identity{}, fmt.Errorf("begin account registration: %w", err)
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO human_principals (id, display_name, status)
		VALUES ($1, $2, 'active')`, registration.User.ID, registration.User.DisplayName); err != nil {
		return account.Identity{}, fmt.Errorf("create human principal: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO user_accounts (id, principal_id, email, password_hash, status)
		VALUES ($1, $2, $3, $4, 'active')`,
		registration.Account.ID,
		registration.Account.PrincipalID,
		registration.Account.Email,
		registration.Account.PasswordHash,
	); err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.ConstraintName == "user_accounts_email_key" {
			return account.Identity{}, account.ErrEmailTaken
		}
		return account.Identity{}, fmt.Errorf("create user account: %w", err)
	}
	language := registration.Preferences.Language
	if language == "" {
		language = account.LanguageSystem
	}
	theme := registration.Preferences.Theme
	if theme == "" {
		theme = account.ThemeSystem
	}
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO user_preferences (principal_id, language, theme)
		VALUES ($1, $2, $3)`, registration.User.ID, language, theme); err != nil {
		return account.Identity{}, fmt.Errorf("create user preferences: %w", err)
	}
	agentRecord := registration.Agent
	err = transaction.QueryRowContext(ctx, `
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at`,
		agentRecord.ID,
		agentRecord.OwnerPrincipalID,
		agentRecord.Name,
		agentRecord.Description,
		agentRecord.SystemPrompt,
	).Scan(&agentRecord.CreatedAt, &agentRecord.UpdatedAt)
	if err != nil {
		return account.Identity{}, fmt.Errorf("create personal agent: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return account.Identity{}, fmt.Errorf("commit account registration: %w", err)
	}
	return account.Identity{Account: registration.Account, User: registration.User, Agent: agentRecord}, nil
}

func (repository *AccountRepository) FindIdentityByEmail(ctx context.Context, email string) (account.Identity, error) {
	var identity account.Identity
	err := repository.database.QueryRowContext(ctx, `
		SELECT accounts.id, accounts.principal_id, accounts.email, accounts.password_hash, accounts.status,
		       principals.id, principals.display_name,
		       agents.id, agents.owner_principal_id, agents.name, agents.description, agents.system_prompt,
		       agents.created_at, agents.updated_at
		FROM user_accounts accounts
		JOIN human_principals principals ON principals.id=accounts.principal_id
		JOIN LATERAL (
			SELECT id, owner_principal_id, name, description, system_prompt, created_at, updated_at
			FROM agents
			WHERE owner_principal_id=principals.id
			ORDER BY created_at, id
			LIMIT 1
		) agents ON true
		WHERE accounts.email=$1 AND principals.status='active'`, email).Scan(
		&identity.Account.ID,
		&identity.Account.PrincipalID,
		&identity.Account.Email,
		&identity.Account.PasswordHash,
		&identity.Account.Status,
		&identity.User.ID,
		&identity.User.DisplayName,
		&identity.Agent.ID,
		&identity.Agent.OwnerPrincipalID,
		&identity.Agent.Name,
		&identity.Agent.Description,
		&identity.Agent.SystemPrompt,
		&identity.Agent.CreatedAt,
		&identity.Agent.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return account.Identity{}, account.ErrInvalidCredentials
	}
	if err != nil {
		return account.Identity{}, fmt.Errorf("find account identity: %w", err)
	}
	return identity, nil
}

func (repository *AccountRepository) CreateSession(ctx context.Context, session account.Session) error {
	if _, err := repository.database.ExecContext(ctx, `
		INSERT INTO account_sessions (id, account_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, session.ID, session.AccountID, session.TokenHash, session.ExpiresAt); err != nil {
		return fmt.Errorf("create account session: %w", err)
	}
	return nil
}

func (repository *AccountRepository) AuthenticateSession(ctx context.Context, tokenHash []byte, now time.Time) (account.Actor, error) {
	var actor account.Actor
	err := repository.database.QueryRowContext(ctx, `
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
			RETURNING sessions.id, sessions.account_id, sessions.expires_at,
			          accounts.email, principals.id AS principal_id, principals.display_name
		)
		SELECT id, account_id, email, principal_id, display_name, expires_at
		FROM valid_session`, tokenHash, now).Scan(
		&actor.SessionID,
		&actor.AccountID,
		&actor.Email,
		&actor.User.ID,
		&actor.User.DisplayName,
		&actor.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return account.Actor{}, account.ErrUnauthenticated
	}
	if err != nil {
		return account.Actor{}, fmt.Errorf("authenticate account session: %w", err)
	}
	return actor, nil
}

func (repository *AccountRepository) RevokeSession(ctx context.Context, tokenHash []byte) error {
	if _, err := repository.database.ExecContext(ctx, `
		UPDATE account_sessions SET revoked_at=COALESCE(revoked_at, now())
		WHERE token_hash=$1`, tokenHash); err != nil {
		return fmt.Errorf("revoke account session: %w", err)
	}
	return nil
}

func (repository *AccountRepository) GetSettings(ctx context.Context, accountID string) (account.Settings, error) {
	var settings account.Settings
	err := repository.database.QueryRowContext(ctx, `
		SELECT accounts.email, accounts.status,
		       principals.id, principals.display_name,
		       preferences.language, preferences.theme
		FROM user_accounts accounts
		JOIN human_principals principals ON principals.id=accounts.principal_id
		JOIN user_preferences preferences ON preferences.principal_id=principals.id
		WHERE accounts.id=$1 AND accounts.status='active' AND principals.status='active'`, accountID).Scan(
		&settings.Account.Email,
		&settings.Account.Status,
		&settings.User.ID,
		&settings.User.DisplayName,
		&settings.Preferences.Language,
		&settings.Preferences.Theme,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return account.Settings{}, account.ErrUnauthenticated
	}
	if err != nil {
		return account.Settings{}, fmt.Errorf("get account settings: %w", err)
	}
	return settings, nil
}

func (repository *AccountRepository) UpdateSettings(ctx context.Context, accountID, principalID string, update account.SettingsUpdate) (account.Settings, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return account.Settings{}, fmt.Errorf("begin account settings update: %w", err)
	}
	defer transaction.Rollback()
	var displayName any
	if update.DisplayName != nil {
		displayName = *update.DisplayName
	}
	result, err := transaction.ExecContext(ctx, `
		UPDATE human_principals principals
		SET display_name=COALESCE($3::text, principals.display_name), updated_at=now()
		FROM user_accounts accounts
		WHERE accounts.id=$1 AND accounts.principal_id=$2
		  AND principals.id=accounts.principal_id
		  AND accounts.status='active' AND principals.status='active'`, accountID, principalID, displayName)
	if err != nil {
		return account.Settings{}, fmt.Errorf("update account profile: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return account.Settings{}, fmt.Errorf("count updated account profile: %w", err)
	}
	if updated != 1 {
		return account.Settings{}, account.ErrUnauthenticated
	}
	var language, theme any
	if update.Language != nil {
		language = string(*update.Language)
	}
	if update.Theme != nil {
		theme = string(*update.Theme)
	}
	if _, err := transaction.ExecContext(ctx, `
		UPDATE user_preferences
		SET language=COALESCE($2::text, language),
		    theme=COALESCE($3::text, theme),
		    updated_at=now()
		WHERE principal_id=$1`, principalID, language, theme); err != nil {
		return account.Settings{}, fmt.Errorf("update user preferences: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return account.Settings{}, fmt.Errorf("commit account settings update: %w", err)
	}
	return repository.GetSettings(ctx, accountID)
}

func (repository *AccountRepository) PasswordHash(ctx context.Context, accountID string) (string, error) {
	var passwordHash string
	err := repository.database.QueryRowContext(ctx, `
		SELECT password_hash FROM user_accounts
		WHERE id=$1 AND status='active'`, accountID).Scan(&passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", account.ErrUnauthenticated
	}
	if err != nil {
		return "", fmt.Errorf("get account password hash: %w", err)
	}
	return passwordHash, nil
}

func (repository *AccountRepository) ChangePassword(ctx context.Context, accountID, currentSessionID, passwordHash string) error {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin password change: %w", err)
	}
	defer transaction.Rollback()
	result, err := transaction.ExecContext(ctx, `
		UPDATE user_accounts
		SET password_hash=$2, updated_at=now()
		WHERE id=$1 AND status='active'`, accountID, passwordHash)
	if err != nil {
		return fmt.Errorf("update account password: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated account password: %w", err)
	}
	if updated != 1 {
		return account.ErrUnauthenticated
	}
	if _, err := transaction.ExecContext(ctx, `
		UPDATE account_sessions
		SET revoked_at=COALESCE(revoked_at, now())
		WHERE account_id=$1 AND id<>$2 AND revoked_at IS NULL`, accountID, currentSessionID); err != nil {
		return fmt.Errorf("revoke other account sessions: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}
