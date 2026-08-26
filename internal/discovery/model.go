package discovery

import (
	"context"
	"errors"
	"time"

	internala2a "github.com/re35t/AegisLink/internal/a2a"
	"github.com/re35t/AegisLink/internal/agent"
)

var (
	ErrNotFound     = errors.New("Agent publication not found")
	ErrInvalid      = errors.New("invalid Agent publication")
	ErrConflict     = errors.New("Agent publication revision conflict")
	ErrUnavailable  = errors.New("Agent publication unavailable")
	ErrUnauthorized = errors.New("AgentFacts token unauthorized")
)

type HostnameStatus string

const (
	HostnameUnconfigured HostnameStatus = "unconfigured"
	HostnamePending      HostnameStatus = "pending"
	HostnameVerified     HostnameStatus = "verified"
)

type Settings struct {
	AgentID            string
	OwnerPrincipalID   string
	PublicID           string
	Revision           int64
	Enabled            bool
	Hostname           string
	HostnameStatus     HostnameStatus
	DNSChallenge       string
	HostnameVerifiedAt *time.Time
	TTLSeconds         int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type SigningKey struct {
	ID                  string
	OwnerPrincipalID    string
	AgentID             string
	PublicKey           []byte
	EncryptedPrivateKey []byte
	Nonce               []byte
	Fingerprint         string
	Status              string
	CreatedAt           time.Time
	RetiredAt           *time.Time
	RevokedAt           *time.Time
}

type SigningKeySummary struct {
	Available   bool   `json:"available"`
	KeyID       string `json:"keyId,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Status      string `json:"status"`
}

type AccessToken struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Audience   string     `json:"audience"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	Revoked    bool       `json:"revoked"`
}

type StoredAccessToken struct {
	AccessToken
	OwnerPrincipalID string
	AgentID          string
	TokenHash        []byte
	RevokedAt        *time.Time
}

type CreatedAccessToken struct {
	Token  AccessToken `json:"token"`
	Secret string      `json:"secret"`
}

type Claim struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Subject    string         `json:"subject"`
	Namespace  string         `json:"namespace,omitempty"`
	Key        string         `json:"key,omitempty"`
	Value      map[string]any `json:"value"`
	Issuer     string         `json:"issuer"`
	ValidFrom  time.Time      `json:"validFrom"`
	ValidUntil *time.Time     `json:"validUntil,omitempty"`
	ProofRefs  []string       `json:"proofRefs"`
}

type Document struct {
	SchemaVersion  string           `json:"schemaVersion"`
	PublicationID  string           `json:"publicationId"`
	ProfileVersion int64            `json:"profileVersion"`
	Subject        map[string]any   `json:"subject"`
	Services       []map[string]any `json:"services"`
	Claims         []Claim          `json:"claims"`
	Privacy        map[string]any   `json:"privacy"`
	ValidFrom      time.Time        `json:"validFrom"`
	ValidUntil     time.Time        `json:"validUntil"`
	Revocation     map[string]any   `json:"revocation"`
	Proof          map[string]any   `json:"proof"`
}

type StoredPublication struct {
	ID               string
	OwnerPrincipalID string
	AgentID          string
	SigningKeyID     string
	ProfileVersion   int64
	Payload          Document
	Digest           string
	ProtectedHeader  string
	Signature        string
	IssuedAt         time.Time
	ExpiresAt        time.Time
	Status           string
	RevokedAt        *time.Time
}

type Publication struct {
	AgentID         string              `json:"agentId"`
	Revision        int64               `json:"revision"`
	Enabled         bool                `json:"enabled"`
	Hostname        string              `json:"hostname"`
	HostnameStatus  HostnameStatus      `json:"hostnameStatus"`
	DNSChallenge    string              `json:"dnsChallenge"`
	TTLSeconds      int64               `json:"ttlSeconds"`
	SigningKey      SigningKeySummary   `json:"signingKey"`
	AgentFactsReady bool                `json:"agentFactsReady"`
	LastPublishedAt *time.Time          `json:"lastPublishedAt,omitempty"`
	AgentFacts      *Document           `json:"agentFacts,omitempty"`
	AgentCard       internala2a.Preview `json:"agentCard"`
	Tokens          []AccessToken       `json:"tokens"`
}

type SettingsUpdate struct {
	ExpectedRevision int64
	Hostname         *string
	Enabled          *bool
	TTLSeconds       *int64
}

type TokenRequest struct {
	Label     string     `json:"label"`
	Audience  string     `json:"audience"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

type Selector struct {
	ClaimID   string `json:"claimId"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

type QueryRequest struct {
	Nonce     string     `json:"nonce"`
	Selectors []Selector `json:"selectors"`
}

type ProfileReader interface {
	Get(context.Context, string, string) (agent.Profile, error)
}

type CardPreviewer interface {
	Preview(context.Context, string, string) (internala2a.Preview, error)
}

type TXTResolver interface {
	LookupTXT(context.Context, string) ([]string, error)
}

type Repository interface {
	GetSettings(context.Context, string, string) (Settings, error)
	GetSettingsByHost(context.Context, string) (Settings, error)
	UpdateSettings(context.Context, Settings, int64) error
	ActiveKey(context.Context, string, string) (SigningKey, error)
	ListKeys(context.Context, string, string) ([]SigningKey, error)
	RotateKey(context.Context, SigningKey) error
	ListTokens(context.Context, string, string) ([]AccessToken, error)
	CreateToken(context.Context, StoredAccessToken) error
	RevokeToken(context.Context, string, string, string) error
	AuthenticateToken(context.Context, []byte, time.Time) (StoredAccessToken, error)
	ActivePublication(context.Context, string, string) (StoredPublication, error)
	SavePublication(context.Context, StoredPublication) error
	RevokePublication(context.Context, string, string) error
	Revocations(context.Context, string, string) ([]string, []string, error)
}
