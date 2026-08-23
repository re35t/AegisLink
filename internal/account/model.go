package account

import (
	"time"

	"github.com/re35t/AegisLink/internal/agent"
)

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type Account struct {
	ID           string
	PrincipalID  string
	Email        string
	PasswordHash string
	Status       string
}

type Identity struct {
	Account Account
	User    User
	Agent   agent.Agent
}

type Actor struct {
	AccountID string
	SessionID string
	Email     string
	User      User
	ExpiresAt time.Time
}

type Registration struct {
	Account Account
	User    User
	Agent   agent.Agent
}

type Session struct {
	ID        string
	AccountID string
	TokenHash []byte
	ExpiresAt time.Time
}

type AuthResult struct {
	User         User
	Agent        agent.Agent
	SessionToken string
	ExpiresAt    time.Time
}
