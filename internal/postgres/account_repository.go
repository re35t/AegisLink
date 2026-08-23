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
