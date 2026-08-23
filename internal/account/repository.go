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
}
