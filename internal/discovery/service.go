package discovery

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/agent"
)

const (
	draftSchemaVersion   = "aegislink.agent-facts/1.0-draft"
	queryScope           = "agent-facts:query"
	defaultTokenLifetime = 90 * 24 * time.Hour
	maximumTokenLifetime = 365 * 24 * time.Hour
)

type Service struct {
	repository Repository
	profiles   ProfileReader
	cards      CardPreviewer
	resolver   TXTResolver
	keys       *keyCipher
	now        func() time.Time
}

func NewService(repository Repository, profiles ProfileReader, cards CardPreviewer, resolver TXTResolver, masterKey string) (*Service, error) {
	cipherState, err := newKeyCipher(strings.TrimSpace(masterKey))
	if err != nil {
		return nil, err
	}
	return &Service{repository: repository, profiles: profiles, cards: cards, resolver: resolver, keys: cipherState, now: func() time.Time { return time.Now().UTC() }}, nil
}

type NetResolver struct{ Resolver *net.Resolver }

func (resolver NetResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	active := resolver.Resolver
	if active == nil {
		active = net.DefaultResolver
	}
	return active.LookupTXT(ctx, name)
}

func (service *Service) Get(ctx context.Context, principalID, agentID string) (Publication, error) {
	settings, err := service.repository.GetSettings(ctx, principalID, agentID)
	if err != nil {
		return Publication{}, err
	}
	card, err := service.cards.Preview(ctx, principalID, agentID)
	if err != nil {
		return Publication{}, err
	}
	result := Publication{AgentID: agentID, Revision: settings.Revision, Enabled: settings.Enabled, Hostname: settings.Hostname, HostnameStatus: settings.HostnameStatus, DNSChallenge: settings.DNSChallenge, TTLSeconds: settings.TTLSeconds, AgentCard: card}
	key, keyErr := service.repository.ActiveKey(ctx, principalID, agentID)
	if keyErr == nil {
		result.SigningKey = SigningKeySummary{Available: service.keys.available(), KeyID: key.ID, Fingerprint: key.Fingerprint, Status: key.Status}
	} else if errors.Is(keyErr, ErrNotFound) {
		result.SigningKey = SigningKeySummary{Available: service.keys.available(), Status: map[bool]string{true: "not-generated", false: "unavailable"}[service.keys.available()]}
	} else {
		return Publication{}, keyErr
	}
	tokens, err := service.repository.ListTokens(ctx, principalID, agentID)
	if err != nil {
		return Publication{}, err
	}
	result.Tokens = tokens
	if publication, pubErr := service.repository.ActivePublication(ctx, principalID, agentID); pubErr == nil {
		result.LastPublishedAt = &publication.IssuedAt
		result.AgentFacts = &publication.Payload
		result.AgentFactsReady = publication.ExpiresAt.After(service.now())
	} else if !errors.Is(pubErr, ErrNotFound) {
		return Publication{}, pubErr
	}
	return result, nil
}

func (service *Service) Update(ctx context.Context, principalID, agentID string, update SettingsUpdate) (Publication, error) {
	settings, err := service.repository.GetSettings(ctx, principalID, agentID)
	if err != nil {
		return Publication{}, err
	}
	if update.ExpectedRevision < 1 {
		return Publication{}, ErrInvalid
	}
	if update.Hostname != nil {
		hostname, valid := normalizeHostname(*update.Hostname)
		if !valid {
			return Publication{}, ErrInvalid
		}
		if hostname != settings.Hostname {
			settings.Hostname = hostname
			settings.Enabled = false
			settings.HostnameVerifiedAt = nil
			if hostname == "" {
				settings.HostnameStatus = HostnameUnconfigured
				settings.DNSChallenge = ""
			} else {
				settings.HostnameStatus = HostnamePending
				challenge, randomErr := secureRandom(24)
				if randomErr != nil {
					return Publication{}, randomErr
				}
				settings.DNSChallenge = challenge
			}
		}
	}
	if update.TTLSeconds != nil {
		if *update.TTLSeconds < 3600 || *update.TTLSeconds > 2592000 {
			return Publication{}, ErrInvalid
		}
		settings.TTLSeconds = *update.TTLSeconds
	}
	if update.Enabled != nil {
		if *update.Enabled && (settings.HostnameStatus != HostnameVerified || !service.keys.available()) {
			return Publication{}, ErrUnavailable
		}
		settings.Enabled = *update.Enabled
	}
	if err := service.repository.UpdateSettings(ctx, settings, update.ExpectedRevision); err != nil {
		return Publication{}, err
	}
	if !settings.Enabled {
		_ = service.repository.RevokePublication(ctx, principalID, agentID)
	} else {
		if _, err = service.ensureKey(ctx, principalID, agentID); err != nil {
			return Publication{}, err
		}
		if _, err = service.publish(ctx, settings, true); err != nil {
			return Publication{}, err
		}
	}
	return service.Get(ctx, principalID, agentID)
}

func (service *Service) VerifyHostname(ctx context.Context, principalID, agentID string) (Publication, error) {
	settings, err := service.repository.GetSettings(ctx, principalID, agentID)
	if err != nil {
		return Publication{}, err
	}
	if settings.Hostname == "" || settings.DNSChallenge == "" || service.resolver == nil {
		return Publication{}, ErrInvalid
	}
	records, err := service.resolver.LookupTXT(ctx, "_aegislink."+settings.Hostname)
	if err != nil {
		return Publication{}, ErrInvalid
	}
	expected := "aegislink-verification=" + settings.DNSChallenge
	found := false
	for _, record := range records {
		if strings.TrimSpace(record) == expected {
			found = true
			break
		}
	}
	if !found {
		return Publication{}, ErrInvalid
	}
	now := service.now()
	settings.HostnameStatus = HostnameVerified
	settings.HostnameVerifiedAt = &now
	if err := service.repository.UpdateSettings(ctx, settings, settings.Revision); err != nil {
		return Publication{}, err
	}
	return service.Get(ctx, principalID, agentID)
}

func (service *Service) RotateKey(ctx context.Context, principalID, agentID string) (Publication, error) {
	if !service.keys.available() {
		return Publication{}, ErrUnavailable
	}
	if _, err := service.newKey(ctx, principalID, agentID); err != nil {
		return Publication{}, err
	}
	settings, err := service.repository.GetSettings(ctx, principalID, agentID)
	if err != nil {
		return Publication{}, err
	}
	if settings.Enabled {
		if _, err = service.publish(ctx, settings, true); err != nil {
			return Publication{}, err
		}
	}
	return service.Get(ctx, principalID, agentID)
}

func (service *Service) CreateToken(ctx context.Context, principalID, agentID string, request TokenRequest) (CreatedAccessToken, error) {
	request.Label = strings.TrimSpace(request.Label)
	request.Audience = strings.TrimSpace(request.Audience)
	if request.Label == "" || utf8.RuneCountInString(request.Label) > 80 || request.Audience == "" || utf8.RuneCountInString(request.Audience) > 120 {
		return CreatedAccessToken{}, ErrInvalid
	}
	if _, err := service.repository.GetSettings(ctx, principalID, agentID); err != nil {
		return CreatedAccessToken{}, err
	}
	now := service.now()
	expires := now.Add(defaultTokenLifetime)
	if request.ExpiresAt != nil {
		expires = request.ExpiresAt.UTC()
	}
	if !expires.After(now) || expires.After(now.Add(maximumTokenLifetime)) {
		return CreatedAccessToken{}, ErrInvalid
	}
	randomValue, err := secureRandom(32)
	if err != nil {
		return CreatedAccessToken{}, err
	}
	secret := "aft_" + randomValue
	digest := sha256.Sum256([]byte(secret))
	token := StoredAccessToken{AccessToken: AccessToken{ID: ulid.Make().String(), Label: request.Label, Audience: request.Audience, Scopes: []string{queryScope}, CreatedAt: now, ExpiresAt: expires}, OwnerPrincipalID: principalID, AgentID: agentID, TokenHash: digest[:]}
	if err := service.repository.CreateToken(ctx, token); err != nil {
		return CreatedAccessToken{}, err
	}
	return CreatedAccessToken{Token: token.AccessToken, Secret: secret}, nil
}
func (service *Service) RevokeToken(ctx context.Context, principalID, agentID, tokenID string) error {
	return service.repository.RevokeToken(ctx, principalID, agentID, tokenID)
}

func (service *Service) PublicDocument(ctx context.Context, host string) (Document, string, int64, error) {
	settings, err := service.settingsForHost(ctx, host)
	if err != nil {
		return Document{}, "", 0, err
	}
	publication, err := service.ensurePublication(ctx, settings, true)
	if err != nil {
		return Document{}, "", 0, err
	}
	return publication.Payload, publication.Digest, settings.TTLSeconds, nil
}

func (service *Service) Query(ctx context.Context, host, bearer string, request QueryRequest) (Document, error) {
	if len(request.Nonce) > 512 || len(request.Selectors) > 100 {
		return Document{}, ErrInvalid
	}
	for _, selector := range request.Selectors {
		if len(selector.ClaimID) > 200 || len(selector.Namespace) > 80 || len(selector.Key) > 120 {
			return Document{}, ErrInvalid
		}
	}
	settings, err := service.settingsForHost(ctx, host)
	if err != nil {
		return Document{}, err
	}
	profile, err := service.profiles.Get(ctx, settings.OwnerPrincipalID, settings.AgentID)
	if err != nil {
		return Document{}, ErrNotFound
	}
	audience := ""
	authenticated := false
	if strings.TrimSpace(bearer) != "" {
		digest := sha256.Sum256([]byte(strings.TrimSpace(bearer)))
		token, tokenErr := service.repository.AuthenticateToken(ctx, digest[:], service.now())
		if tokenErr != nil || token.AgentID != settings.AgentID || !contains(token.Scopes, queryScope) {
			return Document{}, ErrUnauthorized
		}
		audience = token.Audience
		authenticated = true
	}
	document, err := service.buildDocument(ctx, settings, profile, false, audience, authenticated)
	if err != nil {
		return Document{}, err
	}
	if len(request.Selectors) > 0 {
		allowed := make(map[string]struct{}, len(request.Selectors))
		for _, selector := range request.Selectors {
			allowed[selector.ClaimID+"\x00"+selector.Namespace+"\x00"+selector.Key] = struct{}{}
		}
		filtered := document.Claims[:0]
		for _, claim := range document.Claims {
			if _, ok := allowed[claim.ID+"\x00"+claim.Namespace+"\x00"+claim.Key]; ok {
				filtered = append(filtered, claim)
			}
		}
		document.Claims = filtered
	}
	key, err := service.ensureKey(ctx, settings.OwnerPrincipalID, settings.AgentID)
	if err != nil {
		return Document{}, err
	}
	privateKey, err := service.keys.decrypt(key)
	if err != nil {
		return Document{}, err
	}
	document.PublicationID = ulid.Make().String()
	document.ValidUntil = service.now().Add(5 * time.Minute)
	signed, _, _, _, err := signDocument(document, key, privateKey)
	return signed, err
}

func (service *Service) JWKS(ctx context.Context, host string) (map[string]any, error) {
	settings, err := service.settingsForHost(ctx, host)
	if err != nil {
		return nil, err
	}
	keys, err := service.repository.ListKeys(ctx, settings.OwnerPrincipalID, settings.AgentID)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		if key.Status == "revoked" {
			continue
		}
		items = append(items, map[string]any{"kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "use": "sig", "kid": key.ID, "x": base64.RawURLEncoding.EncodeToString(key.PublicKey)})
	}
	return map[string]any{"keys": items}, nil
}
func (service *Service) Revocations(ctx context.Context, host string) (map[string]any, error) {
	settings, err := service.settingsForHost(ctx, host)
	if err != nil {
		return nil, err
	}
	publications, keys, err := service.repository.Revocations(ctx, settings.OwnerPrincipalID, settings.AgentID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"schemaVersion": draftSchemaVersion, "publicationIds": publications, "keyIds": keys, "updatedAt": service.now()}, nil
}

func (service *Service) settingsForHost(ctx context.Context, host string) (Settings, error) {
	hostname, valid := normalizeHostname(host)
	if !valid || hostname == "" {
		return Settings{}, ErrNotFound
	}
	return service.repository.GetSettingsByHost(ctx, hostname)
}
func (service *Service) ensureKey(ctx context.Context, principalID, agentID string) (SigningKey, error) {
	key, err := service.repository.ActiveKey(ctx, principalID, agentID)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return SigningKey{}, err
	}
	return service.newKey(ctx, principalID, agentID)
}
func (service *Service) newKey(ctx context.Context, principalID, agentID string) (SigningKey, error) {
	if !service.keys.available() {
		return SigningKey{}, ErrUnavailable
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return SigningKey{}, err
	}
	key := SigningKey{ID: "key_" + ulid.Make().String(), OwnerPrincipalID: principalID, AgentID: agentID, PublicKey: publicKey, Fingerprint: keyFingerprint(publicKey), Status: "active", CreatedAt: service.now()}
	encrypted, nonce, err := service.keys.encrypt(agentID, key.ID, privateKey)
	if err != nil {
		return SigningKey{}, err
	}
	key.EncryptedPrivateKey = encrypted
	key.Nonce = nonce
	if err = service.repository.RotateKey(ctx, key); err != nil {
		return SigningKey{}, err
	}
	return key, nil
}
func (service *Service) ensurePublication(ctx context.Context, settings Settings, indexedOnly bool) (StoredPublication, error) {
	publication, err := service.repository.ActivePublication(ctx, settings.OwnerPrincipalID, settings.AgentID)
	if err == nil && publication.ExpiresAt.After(service.now()) {
		return publication, nil
	}
	return service.publish(ctx, settings, indexedOnly)
}
func (service *Service) publish(ctx context.Context, settings Settings, indexedOnly bool) (StoredPublication, error) {
	if !settings.Enabled || settings.HostnameStatus != HostnameVerified {
		return StoredPublication{}, ErrNotFound
	}
	profile, err := service.profiles.Get(ctx, settings.OwnerPrincipalID, settings.AgentID)
	if err != nil {
		return StoredPublication{}, err
	}
	document, err := service.buildDocument(ctx, settings, profile, indexedOnly, "", false)
	if err != nil {
		return StoredPublication{}, err
	}
	key, err := service.ensureKey(ctx, settings.OwnerPrincipalID, settings.AgentID)
	if err != nil {
		return StoredPublication{}, err
	}
	privateKey, err := service.keys.decrypt(key)
	if err != nil {
		return StoredPublication{}, err
	}
	signed, digest, protected, signature, err := signDocument(document, key, privateKey)
	if err != nil {
		return StoredPublication{}, err
	}
	publication := StoredPublication{ID: document.PublicationID, OwnerPrincipalID: settings.OwnerPrincipalID, AgentID: settings.AgentID, SigningKeyID: key.ID, ProfileVersion: profile.Version, Payload: signed, Digest: digest, ProtectedHeader: protected, Signature: signature, IssuedAt: document.ValidFrom, ExpiresAt: document.ValidUntil, Status: "active"}
	if err = service.repository.SavePublication(ctx, publication); err != nil {
		return StoredPublication{}, err
	}
	return publication, nil
}

func (service *Service) buildDocument(ctx context.Context, settings Settings, profile agent.Profile, indexedOnly bool, audience string, authenticated bool) (Document, error) {
	now := service.now()
	key, err := service.ensureKey(ctx, settings.OwnerPrincipalID, settings.AgentID)
	if err != nil {
		return Document{}, err
	}
	issuer := "https://" + settings.Hostname
	document := Document{SchemaVersion: draftSchemaVersion, PublicationID: ulid.Make().String(), ProfileVersion: profile.Version, Subject: map[string]any{"id": settings.PublicID, "issuer": issuer, "jwk": map[string]any{"kty": "OKP", "crv": "Ed25519", "kid": key.ID, "x": base64.RawURLEncoding.EncodeToString(key.PublicKey)}}, Services: []map[string]any{}, Claims: []Claim{}, Privacy: map[string]any{"queryMethod": "POST", "anonymous": "public claims only", "authenticatedScope": queryScope}, ValidFrom: now, ValidUntil: now.Add(time.Duration(settings.TTLSeconds) * time.Second), Revocation: map[string]any{"endpoint": issuer + "/.well-known/agentfacts-revocations.json"}}
	include := func(policy agent.DisclosurePolicy) bool {
		if indexedOnly {
			return policy.Visibility == agent.VisibilityPublic && policy.Indexable && policy.Allows(agent.ChannelAgentFacts, "", false)
		}
		return policy.Allows(agent.ChannelAgentFacts, audience, authenticated)
	}
	if include(profile.Identity.Disclosure) {
		document.Claims = append(document.Claims, Claim{ID: "identity:" + profile.Identity.ID, Kind: "identity", Subject: settings.PublicID, Value: map[string]any{"name": profile.Identity.Name, "description": profile.Identity.Description, "avatarUrl": profile.Identity.AvatarURL}, Issuer: issuer, ValidFrom: now, ValidUntil: &document.ValidUntil, ProofRefs: []string{key.ID}})
	}
	for _, endpoint := range profile.Endpoints {
		if include(endpoint.Disclosure) {
			value := map[string]any{"type": "AgentFactsQuery", "endpoint": issuer + "/agentfacts/query"}
			document.Services = append(document.Services, map[string]any{"id": endpoint.ID, "type": "AgentFactsQuery", "endpoint": issuer + "/agentfacts/query"})
			document.Claims = append(document.Claims, Claim{ID: endpoint.ID, Kind: "endpoint", Subject: settings.PublicID, Value: value, Issuer: issuer, ValidFrom: now, ValidUntil: &document.ValidUntil, ProofRefs: []string{key.ID}})
		}
	}
	for _, capability := range profile.Capabilities {
		if include(capability.Disclosure) {
			document.Claims = append(document.Claims, Claim{ID: capability.ID, Kind: "capability", Subject: settings.PublicID, Value: map[string]any{"name": capability.Name, "description": capability.Description, "kind": capability.Kind, "tags": capability.Tags, "callable": capability.Callable}, Issuer: issuer, ValidFrom: now, ValidUntil: &document.ValidUntil, ProofRefs: []string{key.ID}})
		}
	}
	for _, fact := range profile.ConfirmedFacts {
		if fact.Subject != agent.FactSubjectAgent || !include(fact.Disclosure) {
			continue
		}
		validFrom := fact.Confirmation.ConfirmedAt
		if fact.ValidFrom != nil {
			validFrom = *fact.ValidFrom
		}
		validUntil := fact.ValidUntil
		if validUntil == nil || validUntil.After(document.ValidUntil) {
			validUntil = &document.ValidUntil
		}
		document.Claims = append(document.Claims, Claim{ID: fact.ID, Kind: "confirmed-fact", Subject: settings.PublicID, Namespace: fact.Namespace, Key: fact.Key, Value: fact.Value, Issuer: issuer, ValidFrom: validFrom, ValidUntil: validUntil, ProofRefs: []string{key.ID}})
	}
	return document, nil
}

func normalizeHostname(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
	if value == "" {
		return "", true
	}
	if strings.ContainsAny(value, "/:[]") || value == "localhost" || strings.HasSuffix(value, ".localhost") || net.ParseIP(value) != nil || len(value) > 253 {
		return "", false
	}
	labels := strings.Split(value, ".")
	if len(labels) < 2 {
		return "", false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", false
		}
		for _, character := range label {
			if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-') {
				return "", false
			}
		}
	}
	return value, true
}
func secureRandom(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
