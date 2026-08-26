package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/re35t/AegisLink/internal/discovery"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DiscoveryRepository struct{ database *gorm.DB }

func NewDiscoveryRepository(database *Database) *DiscoveryRepository {
	return &DiscoveryRepository{database: database.connection}
}

type publicationSettingsModel struct {
	AgentID            string `gorm:"primaryKey"`
	OwnerPrincipalID   string
	PublicID           string
	Revision           int64
	Enabled            bool
	Hostname           string
	HostnameStatus     discovery.HostnameStatus
	DNSChallenge       string
	HostnameVerifiedAt *time.Time
	TTLSeconds         int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (publicationSettingsModel) TableName() string { return "agent_publication_settings" }

type signingKeyModel struct {
	ID                  string `gorm:"primaryKey"`
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

func (signingKeyModel) TableName() string { return "agent_signing_keys" }

type accessTokenModel struct {
	ID               string `gorm:"primaryKey"`
	OwnerPrincipalID string
	AgentID          string
	TokenHash        []byte
	Label            string
	Audience         string
	Scopes           []string `gorm:"serializer:json;type:jsonb"`
	ExpiresAt        time.Time
	LastUsedAt       *time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
}

func (accessTokenModel) TableName() string { return "agent_access_tokens" }

type factsPublicationModel struct {
	ID               string `gorm:"primaryKey"`
	OwnerPrincipalID string
	AgentID          string
	SigningKeyID     string
	ProfileVersion   int64
	Payload          discovery.Document `gorm:"column:payload_json;serializer:json;type:jsonb"`
	Digest           string
	ProtectedHeader  string
	Signature        string
	IssuedAt         time.Time
	ExpiresAt        time.Time
	Status           string
	RevokedAt        *time.Time
	CreatedAt        time.Time
}

func (factsPublicationModel) TableName() string { return "agent_facts_publications" }

func settingsFromModel(row publicationSettingsModel) discovery.Settings {
	return discovery.Settings{AgentID: row.AgentID, OwnerPrincipalID: row.OwnerPrincipalID, PublicID: row.PublicID,
		Revision: row.Revision, Enabled: row.Enabled, Hostname: row.Hostname, HostnameStatus: row.HostnameStatus,
		DNSChallenge: row.DNSChallenge, HostnameVerifiedAt: row.HostnameVerifiedAt, TTLSeconds: row.TTLSeconds,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func (repository *DiscoveryRepository) GetSettings(ctx context.Context, principalID, agentID string) (discovery.Settings, error) {
	var row publicationSettingsModel
	result := repository.database.WithContext(ctx).Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return discovery.Settings{}, discovery.ErrNotFound
	}
	if result.Error != nil {
		return discovery.Settings{}, fmt.Errorf("get publication settings: %w", result.Error)
	}
	return settingsFromModel(row), nil
}

func (repository *DiscoveryRepository) GetSettingsByHost(ctx context.Context, hostname string) (discovery.Settings, error) {
	var row publicationSettingsModel
	result := repository.database.WithContext(ctx).Where("hostname = ? AND hostname_status = ? AND enabled = true", hostname, discovery.HostnameVerified).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return discovery.Settings{}, discovery.ErrNotFound
	}
	if result.Error != nil {
		return discovery.Settings{}, fmt.Errorf("get publication by host: %w", result.Error)
	}
	return settingsFromModel(row), nil
}

func (repository *DiscoveryRepository) UpdateSettings(ctx context.Context, settings discovery.Settings, expectedRevision int64) error {
	updates := map[string]any{"enabled": settings.Enabled, "hostname": settings.Hostname, "hostname_status": settings.HostnameStatus,
		"dns_challenge": settings.DNSChallenge, "hostname_verified_at": settings.HostnameVerifiedAt, "ttl_seconds": settings.TTLSeconds,
		"revision": gorm.Expr("revision + 1"), "updated_at": time.Now().UTC()}
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		result := transaction.Model(&publicationSettingsModel{}).
			Where("owner_principal_id = ? AND agent_id = ? AND revision = ?", settings.OwnerPrincipalID, settings.AgentID, expectedRevision).Updates(updates)
		if isUniqueViolation(result.Error) {
			return discovery.ErrInvalid
		}
		if result.Error != nil {
			return fmt.Errorf("update publication settings: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return discovery.ErrConflict
		}
		if !settings.Enabled {
			if err := revokePublicationTx(transaction, settings.OwnerPrincipalID, settings.AgentID); err != nil {
				return err
			}
		}
		return nil
	})
}

func keyFromModel(row signingKeyModel) discovery.SigningKey {
	return discovery.SigningKey{ID: row.ID, OwnerPrincipalID: row.OwnerPrincipalID, AgentID: row.AgentID,
		PublicKey: row.PublicKey, EncryptedPrivateKey: row.EncryptedPrivateKey, Nonce: row.Nonce,
		Fingerprint: row.Fingerprint, Status: row.Status, CreatedAt: row.CreatedAt, RetiredAt: row.RetiredAt, RevokedAt: row.RevokedAt}
}

func (repository *DiscoveryRepository) ActiveKey(ctx context.Context, principalID, agentID string) (discovery.SigningKey, error) {
	var row signingKeyModel
	result := repository.database.WithContext(ctx).Where("owner_principal_id = ? AND agent_id = ? AND status = 'active'", principalID, agentID).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return discovery.SigningKey{}, discovery.ErrNotFound
	}
	if result.Error != nil {
		return discovery.SigningKey{}, fmt.Errorf("get active signing key: %w", result.Error)
	}
	return keyFromModel(row), nil
}

func (repository *DiscoveryRepository) ListKeys(ctx context.Context, principalID, agentID string) ([]discovery.SigningKey, error) {
	var rows []signingKeyModel
	if result := repository.database.WithContext(ctx).Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Order("created_at DESC").Find(&rows); result.Error != nil {
		return nil, result.Error
	}
	items := make([]discovery.SigningKey, 0, len(rows))
	for _, row := range rows {
		items = append(items, keyFromModel(row))
	}
	return items, nil
}

func (repository *DiscoveryRepository) RotateKey(ctx context.Context, key discovery.SigningKey) error {
	return repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if result := tx.Model(&signingKeyModel{}).Where("owner_principal_id = ? AND agent_id = ? AND status = 'active'", key.OwnerPrincipalID, key.AgentID).Updates(map[string]any{"status": "retired", "retired_at": now}); result.Error != nil {
			return result.Error
		}
		row := signingKeyModel{ID: key.ID, OwnerPrincipalID: key.OwnerPrincipalID, AgentID: key.AgentID, PublicKey: key.PublicKey,
			EncryptedPrivateKey: key.EncryptedPrivateKey, Nonce: key.Nonce, Fingerprint: key.Fingerprint, Status: "active", CreatedAt: key.CreatedAt}
		if result := tx.Create(&row); result.Error != nil {
			return result.Error
		}
		return revokePublicationTx(tx, key.OwnerPrincipalID, key.AgentID)
	})
}

func tokenSummary(row accessTokenModel) discovery.AccessToken {
	return discovery.AccessToken{ID: row.ID, Label: row.Label, Audience: row.Audience, Scopes: row.Scopes, CreatedAt: row.CreatedAt,
		ExpiresAt: row.ExpiresAt, LastUsedAt: row.LastUsedAt, Revoked: row.RevokedAt != nil}
}
func (repository *DiscoveryRepository) ListTokens(ctx context.Context, principalID, agentID string) ([]discovery.AccessToken, error) {
	var rows []accessTokenModel
	if result := repository.database.WithContext(ctx).Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Order("created_at DESC").Find(&rows); result.Error != nil {
		return nil, result.Error
	}
	items := make([]discovery.AccessToken, 0, len(rows))
	for _, row := range rows {
		items = append(items, tokenSummary(row))
	}
	return items, nil
}
func (repository *DiscoveryRepository) CreateToken(ctx context.Context, token discovery.StoredAccessToken) error {
	row := accessTokenModel{ID: token.ID, OwnerPrincipalID: token.OwnerPrincipalID, AgentID: token.AgentID, TokenHash: token.TokenHash, Label: token.Label, Audience: token.Audience, Scopes: token.Scopes, ExpiresAt: token.ExpiresAt, CreatedAt: token.CreatedAt}
	if result := repository.database.WithContext(ctx).Create(&row); result.Error != nil {
		return fmt.Errorf("create AgentFacts token: %w", result.Error)
	}
	return nil
}
func (repository *DiscoveryRepository) RevokeToken(ctx context.Context, principalID, agentID, tokenID string) error {
	result := repository.database.WithContext(ctx).Model(&accessTokenModel{}).Where("owner_principal_id = ? AND agent_id = ? AND id = ? AND revoked_at IS NULL", principalID, agentID, tokenID).Update("revoked_at", time.Now().UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return discovery.ErrNotFound
	}
	return nil
}
func (repository *DiscoveryRepository) AuthenticateToken(ctx context.Context, hash []byte, now time.Time) (discovery.StoredAccessToken, error) {
	var row accessTokenModel
	result := repository.database.WithContext(ctx).Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", hash, now).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return discovery.StoredAccessToken{}, discovery.ErrUnauthorized
	}
	if result.Error != nil {
		return discovery.StoredAccessToken{}, result.Error
	}
	if result := repository.database.WithContext(ctx).Model(&row).Update("last_used_at", now); result.Error != nil {
		return discovery.StoredAccessToken{}, fmt.Errorf("record AgentFacts token use: %w", result.Error)
	}
	return discovery.StoredAccessToken{AccessToken: tokenSummary(row), OwnerPrincipalID: row.OwnerPrincipalID, AgentID: row.AgentID, TokenHash: row.TokenHash, RevokedAt: row.RevokedAt}, nil
}

func publicationFromModel(row factsPublicationModel) discovery.StoredPublication {
	return discovery.StoredPublication{ID: row.ID, OwnerPrincipalID: row.OwnerPrincipalID, AgentID: row.AgentID, SigningKeyID: row.SigningKeyID, ProfileVersion: row.ProfileVersion, Payload: row.Payload, Digest: row.Digest, ProtectedHeader: row.ProtectedHeader, Signature: row.Signature, IssuedAt: row.IssuedAt, ExpiresAt: row.ExpiresAt, Status: row.Status, RevokedAt: row.RevokedAt}
}
func (repository *DiscoveryRepository) ActivePublication(ctx context.Context, principalID, agentID string) (discovery.StoredPublication, error) {
	var row factsPublicationModel
	result := repository.database.WithContext(ctx).Where("owner_principal_id = ? AND agent_id = ? AND status = 'active'", principalID, agentID).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return discovery.StoredPublication{}, discovery.ErrNotFound
	}
	if result.Error != nil {
		return discovery.StoredPublication{}, result.Error
	}
	return publicationFromModel(row), nil
}
func (repository *DiscoveryRepository) SavePublication(ctx context.Context, publication discovery.StoredPublication) error {
	return repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if result := tx.Model(&factsPublicationModel{}).Where("owner_principal_id = ? AND agent_id = ? AND status = 'active'", publication.OwnerPrincipalID, publication.AgentID).Updates(map[string]any{"status": "superseded", "revoked_at": now}); result.Error != nil {
			return result.Error
		}
		row := factsPublicationModel{ID: publication.ID, OwnerPrincipalID: publication.OwnerPrincipalID, AgentID: publication.AgentID, SigningKeyID: publication.SigningKeyID, ProfileVersion: publication.ProfileVersion, Payload: publication.Payload, Digest: publication.Digest, ProtectedHeader: publication.ProtectedHeader, Signature: publication.Signature, IssuedAt: publication.IssuedAt, ExpiresAt: publication.ExpiresAt, Status: "active"}
		return tx.Create(&row).Error
	})
}
func revokePublicationTx(tx *gorm.DB, principalID, agentID string) error {
	now := time.Now().UTC()
	return tx.Model(&factsPublicationModel{}).Where("owner_principal_id = ? AND agent_id = ? AND status = 'active'", principalID, agentID).Updates(map[string]any{"status": "revoked", "revoked_at": now}).Error
}
func (repository *DiscoveryRepository) RevokePublication(ctx context.Context, principalID, agentID string) error {
	return revokePublicationTx(repository.database.WithContext(ctx), principalID, agentID)
}
func (repository *DiscoveryRepository) Revocations(ctx context.Context, principalID, agentID string) ([]string, []string, error) {
	var publications []string
	if result := repository.database.WithContext(ctx).Model(&factsPublicationModel{}).Where("owner_principal_id = ? AND agent_id = ? AND status = 'revoked'", principalID, agentID).Pluck("id", &publications); result.Error != nil {
		return nil, nil, result.Error
	}
	var keys []string
	if result := repository.database.WithContext(ctx).Model(&signingKeyModel{}).Where("owner_principal_id = ? AND agent_id = ? AND status = 'revoked'", principalID, agentID).Pluck("id", &keys); result.Error != nil {
		return nil, nil, result.Error
	}
	return publications, keys, nil
}

var _ discovery.Repository = (*DiscoveryRepository)(nil)
var _ = clause.Locking{}
