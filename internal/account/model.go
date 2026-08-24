package account

import (
	"time"

	"github.com/re35t/AegisLink/internal/agent"
)

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type Language string

const (
	LanguageSystem  Language = "system"
	LanguageEnglish Language = "en"
	LanguageChinese Language = "zh-CN"
)

type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

type Preferences struct {
	Language Language `json:"language"`
	Theme    Theme    `json:"theme"`
}

type AccountSummary struct {
	Email  string `json:"email"`
	Status string `json:"status"`
}

type Settings struct {
	Account     AccountSummary `json:"account"`
	User        User           `json:"user"`
	Preferences Preferences    `json:"preferences"`
}

type SettingsUpdate struct {
	DisplayName *string   `json:"displayName"`
	Language    *Language `json:"language"`
	Theme       *Theme    `json:"theme"`
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
	Account     Account
	User        User
	Agent       agent.Agent
	Preferences Preferences
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
