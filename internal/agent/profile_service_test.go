package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProfileServiceAggregatesCapabilitiesAndPolicies(t *testing.T) {
	now := time.Now()
	repository := &profileRepositoryStub{
		record:      ProfileRecord{AgentID: "agent-1", OwnerPrincipalID: "owner-1", Version: 3, CreatedAt: now, UpdatedAt: now},
		facts:       []ProfileFact{{ID: "fact-1", Namespace: "interest", Key: "topic", Value: map[string]any{"name": "security"}}},
		projections: []MemoryProjection{{ID: "projection-1", Type: "interest", Summary: "Security research", Status: "candidate"}},
		policies: map[PolicyKey]DisclosurePolicy{
			{SubjectType: SubjectCapability, SubjectID: "skill:one"}: {
				Visibility: VisibilityPublic, Channels: []DisclosureChannel{ChannelAgentFacts}, Indexable: true, Audiences: []string{},
			},
		},
	}
	service := NewProfileService(repository, profileAgentStub{}, capabilityProviderStub{items: []ProfileCapability{{
		ID: "skill:one", Name: "Review", Kind: CapabilitySkill, Source: CapabilitySourceRuntime, Confidence: 1, Callable: true,
	}}})

	profile, err := service.Get(t.Context(), "owner-1", "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Version != 3 || profile.Identity.Name != "Aegis" || len(profile.Capabilities) != 1 {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	if profile.Capabilities[0].Disclosure.Visibility != VisibilityPublic || !profile.Capabilities[0].Disclosure.Indexable {
		t.Fatalf("capability policy not applied: %#v", profile.Capabilities[0].Disclosure)
	}
	if profile.Capabilities[0].Disclosure.Audiences == nil {
		t.Fatal("empty disclosure audiences must serialize as an array")
	}
	if profile.Facts[0].Disclosure.Visibility != VisibilityPrivate || len(profile.Facts[0].Disclosure.Channels) != 1 {
		t.Fatalf("default policy not applied: %#v", profile.Facts[0].Disclosure)
	}
}

func TestProfileServiceUpdatesIdentityAndRejectsUnsafeAvatar(t *testing.T) {
	repository := newProfileRepositoryStub()
	service := NewProfileService(repository, profileAgentStub{})
	name := "  Research Agent  "
	avatar := "https://images.example.test/aegis.png"
	profile, err := service.Update(t.Context(), "owner-1", "agent-1", ProfileUpdate{ExpectedVersion: 1, Name: &name, AvatarURL: &avatar})
	if err != nil {
		t.Fatal(err)
	}
	if repository.identityUpdate.Name == nil || *repository.identityUpdate.Name != "Research Agent" || profile.Version != 2 {
		t.Fatalf("identity update not normalized: update=%#v profile=%#v", repository.identityUpdate, profile)
	}

	unsafe := "http://example.test/avatar.png"
	_, err = service.Update(t.Context(), "owner-1", "agent-1", ProfileUpdate{ExpectedVersion: 2, AvatarURL: &unsafe})
	if !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("unsafe avatar error = %v", err)
	}
}

func TestProfileServiceValidatesAndSavesDisclosureChanges(t *testing.T) {
	repository := newProfileRepositoryStub()
	service := NewProfileService(repository, profileAgentStub{}, capabilityProviderStub{items: []ProfileCapability{{
		ID: "skill:one", Name: "Review", Kind: CapabilitySkill, Source: CapabilitySourceRuntime, Confidence: 1, Callable: true,
	}}})
	profile, err := service.UpdatePolicies(t.Context(), "owner-1", "agent-1", 1, []PolicyChange{{
		SubjectType: SubjectCapability,
		SubjectID:   "skill:one",
		Policy: DisclosurePolicy{
			Visibility: VisibilityRestricted,
			Channels:   []DisclosureChannel{ChannelAgentCard},
			Audiences:  []string{"  team-security  "},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Version != 2 || len(repository.policyChanges) != 1 || repository.policyChanges[0].Policy.Audiences[0] != "team-security" {
		t.Fatalf("policy update not saved: profile=%#v changes=%#v", profile, repository.policyChanges)
	}

	_, err = service.UpdatePolicies(t.Context(), "owner-1", "agent-1", 2, []PolicyChange{{
		SubjectType: SubjectCapability, SubjectID: "missing",
		Policy: DefaultDisclosurePolicy(),
	}})
	if !errors.Is(err, ErrProfileSubject) {
		t.Fatalf("missing subject error = %v", err)
	}

	_, err = service.UpdatePolicies(t.Context(), "owner-1", "agent-1", 2, []PolicyChange{{
		SubjectType: SubjectIdentity, SubjectID: "agent-1",
		Policy: DisclosurePolicy{Visibility: VisibilityPrivate, Channels: []DisclosureChannel{ChannelAgentCard}},
	}})
	if !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("invalid private policy error = %v", err)
	}
}

func TestDisclosurePolicyAndProjectionFiltering(t *testing.T) {
	public := DisclosurePolicy{Visibility: VisibilityPublic, Channels: []DisclosureChannel{ChannelAgentFacts}}
	if !public.Allows(ChannelAgentFacts, "", false) || public.Allows(ChannelAgentCard, "", false) {
		t.Fatal("public channel filtering is incorrect")
	}
	restricted := DisclosurePolicy{Visibility: VisibilityRestricted, Channels: []DisclosureChannel{ChannelAgentCard}, Audiences: []string{"team-1"}}
	if !restricted.Allows(ChannelAgentCard, "team-1", true) || restricted.Allows(ChannelAgentCard, "team-2", true) {
		t.Fatal("restricted audience filtering is incorrect")
	}
	projection := MemoryProjection{Status: "candidate", Disclosure: public}
	if projection.Disclosable(ChannelAgentFacts, "", false) {
		t.Fatal("candidate projection must not be externally disclosable")
	}
	projection.Status = "accepted"
	if !projection.Disclosable(ChannelAgentFacts, "", false) {
		t.Fatal("accepted projection should follow its disclosure policy")
	}
}

type profileAgentStub struct{}

func (profileAgentStub) Get(_ context.Context, principalID, agentID string) (Agent, error) {
	if principalID != "owner-1" || agentID != "agent-1" {
		return Agent{}, ErrNotFound
	}
	return Agent{ID: agentID, OwnerPrincipalID: principalID, Name: "Aegis", Description: "Personal Agent"}, nil
}

type capabilityProviderStub struct {
	items []ProfileCapability
	err   error
}

func (provider capabilityProviderStub) ProfileCapabilities(context.Context, string, string) ([]ProfileCapability, error) {
	return provider.items, provider.err
}

type profileRepositoryStub struct {
	record         ProfileRecord
	facts          []ProfileFact
	projections    []MemoryProjection
	policies       map[PolicyKey]DisclosurePolicy
	identityUpdate ProfileUpdate
	policyChanges  []PolicyChange
}

func newProfileRepositoryStub() *profileRepositoryStub {
	now := time.Now()
	return &profileRepositoryStub{
		record:   ProfileRecord{AgentID: "agent-1", OwnerPrincipalID: "owner-1", Version: 1, CreatedAt: now, UpdatedAt: now},
		policies: make(map[PolicyKey]DisclosurePolicy),
	}
}

func (repository *profileRepositoryStub) GetProfile(context.Context, string, string) (ProfileRecord, error) {
	return repository.record, nil
}
func (repository *profileRepositoryStub) ListProfileFacts(context.Context, string, string) ([]ProfileFact, error) {
	return repository.facts, nil
}
func (repository *profileRepositoryStub) ListMemoryProjections(context.Context, string, string) ([]MemoryProjection, error) {
	return repository.projections, nil
}
func (repository *profileRepositoryStub) ListDisclosurePolicies(context.Context, string, string) (map[PolicyKey]DisclosurePolicy, error) {
	return repository.policies, nil
}
func (repository *profileRepositoryStub) UpdateProfileIdentity(_ context.Context, _, _ string, update ProfileUpdate) error {
	repository.identityUpdate = update
	repository.record.Version++
	return nil
}
func (repository *profileRepositoryStub) UpdateDisclosurePolicies(_ context.Context, _, _ string, _ int64, changes []PolicyChange) error {
	repository.policyChanges = changes
	for _, change := range changes {
		repository.policies[PolicyKey{SubjectType: change.SubjectType, SubjectID: change.SubjectID}] = change.Policy
	}
	repository.record.Version++
	return nil
}
