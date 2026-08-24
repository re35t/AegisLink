package account

import (
	"context"
	"time"
)

type Repository interface {
	CreateAccountWithAgent(context.Context, Registration) (Identity, error)
	FindIdentityByEmail(context.Context, string) (Identity, error)
	CreateSession(context.Context, Session) error
	AuthenticateSession(context.Context, []byte, time.Time) (Actor, error)
	RevokeSession(context.Context, []byte) error
	GetSettings(context.Context, string) (Settings, error)
	UpdateSettings(context.Context, string, string, SettingsUpdate) (Settings, error)
	PasswordHash(context.Context, string) (string, error)
	ChangePassword(context.Context, string, string, string) error
}
