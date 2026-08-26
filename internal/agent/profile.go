package agent

import (
	"context"
	"errors"
	"time"

	"github.com/re35t/AegisLink/internal/impression"
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
	ContextRevision  int64
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

type ProfileEndpoint struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Disclosure  DisclosurePolicy `json:"disclosure"`
}

type FactSubject string

const (
	FactSubjectAgent   FactSubject = "agent"
	FactSubjectUser    FactSubject = "user"
	FactSubjectProject FactSubject = "project"
	FactSubjectTask    FactSubject = "task"
)

type FactConfirmation struct {
	ConfirmedAt time.Time `json:"confirmedAt"`
	Method      string    `json:"method"`
}

type ConfirmedFact struct {
	ID           string           `json:"id"`
	Subject      FactSubject      `json:"subject"`
	Namespace    string           `json:"namespace"`
	Key          string           `json:"key"`
	Value        map[string]any   `json:"value"`
	CandidateID  *string          `json:"candidateId,omitempty"`
	Confidence   float64          `json:"confidence"`
	Confirmation FactConfirmation `json:"confirmation"`
	ValidFrom    *time.Time       `json:"validFrom,omitempty"`
	ValidUntil   *time.Time       `json:"validUntil,omitempty"`
	RevokedAt    *time.Time       `json:"revokedAt,omitempty"`
	Disclosure   DisclosurePolicy `json:"disclosure"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

type Profile struct {
	AgentID          string                  `json:"agentId"`
	Version          int64                   `json:"version"`
	ContextRevision  int64                   `json:"contextRevision"`
	Identity         ProfileIdentity         `json:"identity"`
	Capabilities     []ProfileCapability     `json:"capabilities"`
	Endpoints        []ProfileEndpoint       `json:"endpoints"`
	ConfirmedFacts   []ConfirmedFact         `json:"confirmedFacts"`
	Impressions      []impression.Impression `json:"impressions"`
	PendingFactCount int                     `json:"pendingFactCount"`
	CreatedAt        time.Time               `json:"createdAt"`
	UpdatedAt        time.Time               `json:"updatedAt"`
}

type ProfileSubjectType string

const (
	SubjectIdentity      ProfileSubjectType = "identity"
	SubjectCapability    ProfileSubjectType = "capability"
	SubjectConfirmedFact ProfileSubjectType = "confirmed-fact"
	SubjectEndpoint      ProfileSubjectType = "endpoint"
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
	ListConfirmedFacts(context.Context, string, string, bool) ([]ConfirmedFact, error)
	ListDisclosurePolicies(context.Context, string, string) (map[PolicyKey]DisclosurePolicy, error)
	UpdateProfileIdentity(context.Context, string, string, ProfileUpdate) error
	UpdateDisclosurePolicies(context.Context, string, string, int64, []PolicyChange) error
	ConfirmFact(context.Context, string, string, string, int64, int64, ConfirmFactUpdate) error
	RevokeFact(context.Context, string, string, string, int64) error
}

type ImpressionReader interface {
	List(context.Context, string, string, string) ([]impression.Impression, error)
	ListCandidates(context.Context, string, string, string) ([]impression.FactCandidate, error)
}

type ConfirmFactUpdate struct {
	Subject   *FactSubject   `json:"subject,omitempty"`
	Namespace *string        `json:"namespace,omitempty"`
	Key       *string        `json:"key,omitempty"`
	Value     map[string]any `json:"value,omitempty"`
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
