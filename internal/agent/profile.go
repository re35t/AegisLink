package agent

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidProfile  = errors.New("invalid agent profile")
	ErrProfileConflict = errors.New("agent profile version conflict")
	ErrProfileSubject  = errors.New("agent profile subject not found")
)

type Visibility string

const (
	VisibilityPrivate       Visibility = "private"
	VisibilityAuthenticated Visibility = "authenticated"
	VisibilityRestricted    Visibility = "restricted"
	VisibilityPublic        Visibility = "public"
)

type DisclosureChannel string

const (
	ChannelRuntimeContext DisclosureChannel = "runtime-context"
	ChannelAgentFacts     DisclosureChannel = "agent-facts"
	ChannelAgentCard      DisclosureChannel = "agent-card"
)

type DisclosurePolicy struct {
	Visibility Visibility          `json:"visibility"`
	Channels   []DisclosureChannel `json:"channels"`
	Indexable  bool                `json:"indexable"`
	Audiences  []string            `json:"audiences"`
}

func (policy DisclosurePolicy) Allows(channel DisclosureChannel, audience string, authenticated bool) bool {
	allowedChannel := false
	for _, candidate := range policy.Channels {
		if candidate == channel {
			allowedChannel = true
			break
		}
	}
	if !allowedChannel {
		return false
	}
	switch policy.Visibility {
	case VisibilityPrivate:
		return channel == ChannelRuntimeContext
	case VisibilityAuthenticated:
		return authenticated
	case VisibilityRestricted:
		for _, candidate := range policy.Audiences {
			if candidate == audience {
				return true
			}
		}
		return false
	case VisibilityPublic:
		return true
	default:
		return false
	}
}

func DefaultDisclosurePolicy() DisclosurePolicy {
	return DisclosurePolicy{
		Visibility: VisibilityPrivate,
		Channels:   []DisclosureChannel{ChannelRuntimeContext},
		Audiences:  []string{},
	}
}

type ProfileRecord struct {
	AgentID          string
	OwnerPrincipalID string
	AvatarURL        string
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ProfileIdentity struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	AvatarURL   string           `json:"avatarUrl"`
	HumanLinked bool             `json:"humanLinked"`
	Disclosure  DisclosurePolicy `json:"disclosure"`
}

type CapabilityKind string

const (
	CapabilitySkill CapabilityKind = "skill"
	CapabilityTool  CapabilityKind = "tool"
	CapabilityModel CapabilityKind = "model_capability"
)

type CapabilitySource string

const (
	CapabilitySourceDeclared CapabilitySource = "declared"
	CapabilitySourceRuntime  CapabilitySource = "runtime"
	CapabilitySourceTool     CapabilitySource = "tool"
	CapabilitySourceInferred CapabilitySource = "inferred"
)

type ProfileCapability struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Kind        CapabilityKind   `json:"kind"`
	Tags        []string         `json:"tags"`
	Source      CapabilitySource `json:"source"`
	Confidence  float64          `json:"confidence"`
	Callable    bool             `json:"callable"`
	Disclosure  DisclosurePolicy `json:"disclosure"`
}

type ProfileFact struct {
	ID         string           `json:"id"`
	Namespace  string           `json:"namespace"`
	Key        string           `json:"key"`
	Value      map[string]any   `json:"value"`
	Source     string           `json:"source"`
	Confidence float64          `json:"confidence"`
	ValidFrom  *time.Time       `json:"validFrom,omitempty"`
	ValidUntil *time.Time       `json:"validUntil,omitempty"`
	Disclosure DisclosurePolicy `json:"disclosure"`
	CreatedAt  time.Time        `json:"createdAt"`
	UpdatedAt  time.Time        `json:"updatedAt"`
}

type MemoryProjection struct {
	ID              string           `json:"id"`
	Type            string           `json:"type"`
	Summary         string           `json:"summary"`
	SourceMemoryIDs []string         `json:"sourceMemoryIds"`
	Confidence      float64          `json:"confidence"`
	Freshness       float64          `json:"freshness"`
	GeneratedAt     time.Time        `json:"generatedAt"`
	ExpiresAt       *time.Time       `json:"expiresAt,omitempty"`
	Status          string           `json:"status"`
	Disclosure      DisclosurePolicy `json:"disclosure"`
}

func (projection MemoryProjection) Disclosable(channel DisclosureChannel, audience string, authenticated bool) bool {
	return projection.Status == "accepted" && projection.Disclosure.Allows(channel, audience, authenticated)
}

type Profile struct {
	AgentID           string              `json:"agentId"`
	Version           int64               `json:"version"`
	Identity          ProfileIdentity     `json:"identity"`
	Capabilities      []ProfileCapability `json:"capabilities"`
	Facts             []ProfileFact       `json:"facts"`
	MemoryProjections []MemoryProjection  `json:"memoryProjections"`
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         time.Time           `json:"updatedAt"`
}

type ProfileSubjectType string

const (
	SubjectIdentity   ProfileSubjectType = "identity"
	SubjectCapability ProfileSubjectType = "capability"
	SubjectFact       ProfileSubjectType = "fact"
	SubjectProjection ProfileSubjectType = "projection"
)

type PolicyKey struct {
	SubjectType ProfileSubjectType
	SubjectID   string
}

type PolicyChange struct {
	SubjectType ProfileSubjectType `json:"subjectType"`
	SubjectID   string             `json:"subjectId"`
	Policy      DisclosurePolicy   `json:"policy"`
}

type ProfileUpdate struct {
	ExpectedVersion int64   `json:"expectedVersion"`
	Name            *string `json:"name,omitempty"`
	Description     *string `json:"description,omitempty"`
	AvatarURL       *string `json:"avatarUrl,omitempty"`
}

type ProfileRepository interface {
	GetProfile(context.Context, string, string) (ProfileRecord, error)
	ListProfileFacts(context.Context, string, string) ([]ProfileFact, error)
	ListMemoryProjections(context.Context, string, string) ([]MemoryProjection, error)
	ListDisclosurePolicies(context.Context, string, string) (map[PolicyKey]DisclosurePolicy, error)
	UpdateProfileIdentity(context.Context, string, string, ProfileUpdate) error
	UpdateDisclosurePolicies(context.Context, string, string, int64, []PolicyChange) error
}

type CapabilityProvider interface {
	ProfileCapabilities(context.Context, string, string) ([]ProfileCapability, error)
}

type StaticCapabilityProvider struct {
	Capabilities []ProfileCapability
}

func (provider StaticCapabilityProvider) ProfileCapabilities(context.Context, string, string) ([]ProfileCapability, error) {
	items := make([]ProfileCapability, len(provider.Capabilities))
	copy(items, provider.Capabilities)
	return items, nil
}
