package agent

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/re35t/AegisLink/internal/impression"
)

const (
	maximumProfileName        = 80
	maximumProfileDescription = 1000
	maximumAvatarURL          = 2048
	maximumAudience           = 120
	maximumPolicyChanges      = 200
)

type ProfileAgentReader interface {
	Get(context.Context, string, string) (Agent, error)
}

type ProfileService struct {
	repository  ProfileRepository
	agents      ProfileAgentReader
	providers   []CapabilityProvider
	impressions ImpressionReader
}

func NewProfileService(repository ProfileRepository, agents ProfileAgentReader, providers ...CapabilityProvider) *ProfileService {
	return &ProfileService{repository: repository, agents: agents, providers: providers}
}

func (service *ProfileService) WithImpressions(reader ImpressionReader) *ProfileService {
	service.impressions = reader
	return service
}

func (service *ProfileService) Get(ctx context.Context, principalID, agentID string) (Profile, error) {
	agentRecord, err := service.agents.Get(ctx, principalID, agentID)
	if err != nil {
		return Profile{}, err
	}
	record, err := service.repository.GetProfile(ctx, principalID, agentID)
	if err != nil {
		return Profile{}, err
	}
	facts, err := service.repository.ListConfirmedFacts(ctx, principalID, agentID, false)
	if err != nil {
		return Profile{}, err
	}
	activeImpressions := []impression.Impression{}
	pendingFactCount := 0
	if service.impressions != nil {
		activeImpressions, err = service.impressions.List(ctx, principalID, agentID, string(impression.StatusActive))
		if err != nil {
			return Profile{}, err
		}
		candidates, candidateErr := service.impressions.ListCandidates(ctx, principalID, agentID, "pending")
		if candidateErr != nil {
			return Profile{}, candidateErr
		}
		pendingFactCount = len(candidates)
	}
	policies, err := service.repository.ListDisclosurePolicies(ctx, principalID, agentID)
	if err != nil {
		return Profile{}, err
	}

	capabilities := make([]ProfileCapability, 0)
	seenCapabilities := make(map[string]struct{})
	for _, provider := range service.providers {
		items, err := provider.ProfileCapabilities(ctx, principalID, agentID)
		if err != nil {
			return Profile{}, err
		}
		for _, item := range items {
			if item.ID == "" {
				return Profile{}, ErrInvalidProfile
			}
			if _, exists := seenCapabilities[item.ID]; exists {
				continue
			}
			seenCapabilities[item.ID] = struct{}{}
			item.Tags = nonNilStrings(item.Tags)
			item.Disclosure = policyFor(policies, SubjectCapability, item.ID)
			capabilities = append(capabilities, item)
		}
	}
	sort.Slice(capabilities, func(i, j int) bool {
		if capabilities[i].Kind == capabilities[j].Kind {
			return capabilities[i].Name < capabilities[j].Name
		}
		return capabilities[i].Kind < capabilities[j].Kind
	})

	for index := range facts {
		facts[index].Disclosure = policyFor(policies, SubjectConfirmedFact, facts[index].ID)
	}
	endpoints := []ProfileEndpoint{{ID: "agent-facts-query", Name: "AgentFacts query", Description: "Privacy-preserving POST query endpoint.", Disclosure: policyFor(policies, SubjectEndpoint, "agent-facts-query")}}

	return Profile{
		AgentID:         agentID,
		Version:         record.Version,
		ContextRevision: record.ContextRevision,
		Identity: ProfileIdentity{
			ID:          agentRecord.ID,
			Name:        agentRecord.Name,
			Description: agentRecord.Description,
			AvatarURL:   record.AvatarURL,
			HumanLinked: true,
			Disclosure:  policyFor(policies, SubjectIdentity, agentRecord.ID),
		},
		Capabilities:     capabilities,
		Endpoints:        endpoints,
		ConfirmedFacts:   facts,
		Impressions:      activeImpressions,
		PendingFactCount: pendingFactCount,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}, nil
}

func (service *ProfileService) Update(ctx context.Context, principalID, agentID string, update ProfileUpdate) (Profile, error) {
	if _, err := service.agents.Get(ctx, principalID, agentID); err != nil {
		return Profile{}, err
	}
	if update.ExpectedVersion < 1 || (update.Name == nil && update.Description == nil && update.AvatarURL == nil) {
		return Profile{}, ErrInvalidProfile
	}
	if update.Name != nil {
		value := strings.TrimSpace(*update.Name)
		if value == "" || utf8.RuneCountInString(value) > maximumProfileName {
			return Profile{}, ErrInvalidProfile
		}
		update.Name = &value
	}
	if update.Description != nil {
		value := strings.TrimSpace(*update.Description)
		if utf8.RuneCountInString(value) > maximumProfileDescription {
			return Profile{}, ErrInvalidProfile
		}
		update.Description = &value
	}
	if update.AvatarURL != nil {
		value := strings.TrimSpace(*update.AvatarURL)
		if !validAvatarURL(value) {
			return Profile{}, ErrInvalidProfile
		}
		update.AvatarURL = &value
	}
	if err := service.repository.UpdateProfileIdentity(ctx, principalID, agentID, update); err != nil {
		return Profile{}, err
	}
	return service.Get(ctx, principalID, agentID)
}

func (service *ProfileService) UpdatePolicies(ctx context.Context, principalID, agentID string, expectedVersion int64, changes []PolicyChange) (Profile, error) {
	if expectedVersion < 1 || len(changes) == 0 || len(changes) > maximumPolicyChanges {
		return Profile{}, ErrInvalidProfile
	}
	profile, err := service.Get(ctx, principalID, agentID)
	if err != nil {
		return Profile{}, err
	}
	validSubjects := profileSubjects(profile)
	seen := make(map[PolicyKey]struct{}, len(changes))
	for index := range changes {
		changes[index].SubjectID = strings.TrimSpace(changes[index].SubjectID)
		key := PolicyKey{SubjectType: changes[index].SubjectType, SubjectID: changes[index].SubjectID}
		if _, ok := validSubjects[key]; !ok {
			return Profile{}, ErrProfileSubject
		}
		if _, duplicate := seen[key]; duplicate {
			return Profile{}, ErrInvalidProfile
		}
		seen[key] = struct{}{}
		policy, ok := normalizePolicy(changes[index].Policy)
		if !ok {
			return Profile{}, ErrInvalidProfile
		}
		changes[index].Policy = policy
		if key.SubjectType == SubjectConfirmedFact && hasExternalPolicy(policy) && !agentFactSubject(profile, key.SubjectID) {
			return Profile{}, ErrInvalidProfile
		}
	}
	if err := service.repository.UpdateDisclosurePolicies(ctx, principalID, agentID, expectedVersion, changes); err != nil {
		return Profile{}, err
	}
	return service.Get(ctx, principalID, agentID)
}

func profileSubjects(profile Profile) map[PolicyKey]struct{} {
	items := map[PolicyKey]struct{}{
		{SubjectType: SubjectIdentity, SubjectID: profile.Identity.ID}: {},
	}
	for _, capability := range profile.Capabilities {
		items[PolicyKey{SubjectType: SubjectCapability, SubjectID: capability.ID}] = struct{}{}
	}
	for _, fact := range profile.ConfirmedFacts {
		items[PolicyKey{SubjectType: SubjectConfirmedFact, SubjectID: fact.ID}] = struct{}{}
	}
	for _, endpoint := range profile.Endpoints {
		items[PolicyKey{SubjectType: SubjectEndpoint, SubjectID: endpoint.ID}] = struct{}{}
	}
	return items
}

func (service *ProfileService) ConfirmCandidate(ctx context.Context, principalID, agentID, candidateID string, expectedVersion, expectedCandidateVersion int64, update ConfirmFactUpdate) (Profile, error) {
	if expectedVersion < 1 || expectedCandidateVersion < 1 {
		return Profile{}, ErrInvalidProfile
	}
	if update.Namespace != nil {
		value := strings.TrimSpace(*update.Namespace)
		if value == "" || utf8.RuneCountInString(value) > 80 {
			return Profile{}, ErrInvalidProfile
		}
		update.Namespace = &value
	}
	if update.Key != nil {
		value := strings.TrimSpace(*update.Key)
		if value == "" || utf8.RuneCountInString(value) > 120 {
			return Profile{}, ErrInvalidProfile
		}
		update.Key = &value
	}
	if update.Subject != nil && *update.Subject != FactSubjectAgent && *update.Subject != FactSubjectUser && *update.Subject != FactSubjectProject && *update.Subject != FactSubjectTask {
		return Profile{}, ErrInvalidProfile
	}
	if err := service.repository.ConfirmFact(ctx, principalID, agentID, candidateID, expectedVersion, expectedCandidateVersion, update); err != nil {
		return Profile{}, err
	}
	return service.Get(ctx, principalID, agentID)
}

func (service *ProfileService) RevokeFact(ctx context.Context, principalID, agentID, factID string, expectedVersion int64) (Profile, error) {
	if expectedVersion < 1 {
		return Profile{}, ErrInvalidProfile
	}
	if err := service.repository.RevokeFact(ctx, principalID, agentID, factID, expectedVersion); err != nil {
		return Profile{}, err
	}
	return service.Get(ctx, principalID, agentID)
}

func (service *ProfileService) RuntimeFacts(ctx context.Context, principalID, agentID string) ([]ConfirmedFact, error) {
	facts, err := service.repository.ListConfirmedFacts(ctx, principalID, agentID, false)
	if err != nil {
		return nil, err
	}
	policies, err := service.repository.ListDisclosurePolicies(ctx, principalID, agentID)
	if err != nil {
		return nil, err
	}
	result := make([]ConfirmedFact, 0, len(facts))
	for _, fact := range facts {
		fact.Disclosure = policyFor(policies, SubjectConfirmedFact, fact.ID)
		if fact.Disclosure.Allows(ChannelRuntimeContext, "", true) {
			result = append(result, fact)
		}
	}
	return result, nil
}

func hasExternalPolicy(policy DisclosurePolicy) bool {
	return hasChannel(policy.Channels, ChannelAgentFacts) || hasChannel(policy.Channels, ChannelAgentCard)
}

func agentFactSubject(profile Profile, id string) bool {
	for _, fact := range profile.ConfirmedFacts {
		if fact.ID == id {
			return fact.Subject == FactSubjectAgent
		}
	}
	return false
}

func normalizePolicy(policy DisclosurePolicy) (DisclosurePolicy, bool) {
	channels := make([]DisclosureChannel, 0, len(policy.Channels))
	seenChannels := make(map[DisclosureChannel]struct{}, len(policy.Channels))
	for _, channel := range policy.Channels {
		if channel != ChannelRuntimeContext && channel != ChannelAgentFacts && channel != ChannelAgentCard {
			return DisclosurePolicy{}, false
		}
		if _, duplicate := seenChannels[channel]; duplicate {
			return DisclosurePolicy{}, false
		}
		seenChannels[channel] = struct{}{}
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i] < channels[j] })

	audiences := make([]string, 0, len(policy.Audiences))
	seenAudiences := make(map[string]struct{}, len(policy.Audiences))
	for _, raw := range policy.Audiences {
		audience := strings.TrimSpace(raw)
		if audience == "" || utf8.RuneCountInString(audience) > maximumAudience {
			return DisclosurePolicy{}, false
		}
		if _, duplicate := seenAudiences[audience]; duplicate {
			return DisclosurePolicy{}, false
		}
		seenAudiences[audience] = struct{}{}
		audiences = append(audiences, audience)
	}
	sort.Strings(audiences)

	hasExternal := hasChannel(channels, ChannelAgentFacts) || hasChannel(channels, ChannelAgentCard)
	switch policy.Visibility {
	case VisibilityPrivate:
		if hasExternal || policy.Indexable || len(audiences) != 0 {
			return DisclosurePolicy{}, false
		}
	case VisibilityAuthenticated:
		if policy.Indexable || len(audiences) != 0 {
			return DisclosurePolicy{}, false
		}
	case VisibilityRestricted:
		if policy.Indexable || len(audiences) == 0 {
			return DisclosurePolicy{}, false
		}
	case VisibilityPublic:
		if len(audiences) != 0 || (policy.Indexable && !hasChannel(channels, ChannelAgentFacts)) {
			return DisclosurePolicy{}, false
		}
	default:
		return DisclosurePolicy{}, false
	}
	return DisclosurePolicy{Visibility: policy.Visibility, Channels: channels, Indexable: policy.Indexable, Audiences: audiences}, true
}

func validAvatarURL(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > maximumAvatarURL || !utf8.ValidString(value) {
		return false
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func hasChannel(channels []DisclosureChannel, target DisclosureChannel) bool {
	for _, channel := range channels {
		if channel == target {
			return true
		}
	}
	return false
}

func policyFor(policies map[PolicyKey]DisclosurePolicy, subjectType ProfileSubjectType, subjectID string) DisclosurePolicy {
	if policy, ok := policies[PolicyKey{SubjectType: subjectType, SubjectID: subjectID}]; ok {
		policy.Channels = append([]DisclosureChannel{}, policy.Channels...)
		policy.Audiences = append([]string{}, policy.Audiences...)
		return policy
	}
	return DefaultDisclosurePolicy()
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
