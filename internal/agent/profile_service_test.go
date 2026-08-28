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
		record: ProfileRecord{AgentID: "agent-1", OwnerPrincipalID: "owner-1", Version: 3, CreatedAt: now, UpdatedAt: now},
		facts:  []ConfirmedFact{{ID: "fact-1", Subject: FactSubjectUser, Namespace: "interest", Key: "topic", Value: map[string]any{"name": "security"}}},
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
	if profile.ConfirmedFacts[0].Disclosure.Visibility != VisibilityPrivate || len(profile.ConfirmedFacts[0].Disclosure.Channels) != 1 {
		t.Fatalf("default policy not applied: %#v", profile.ConfirmedFacts[0].Disclosure)
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

func TestProfileServiceConfiguresAndSyncsBeforeCompletingSetup(t *testing.T) {
	repository := newProfileRepositoryStub()
	synchronizer := &profileSynchronizerStub{}
	service := NewProfileService(repository, profileAgentStub{}).WithSynchronizer(synchronizer)

	profile, err := service.Configure(t.Context(), "owner-1", "agent-1", "  Atlas  ", "  Go systems and API design  ")
	if err != nil {
		t.Fatal(err)
	}
	if repository.configuredName != "Atlas" || repository.configuredFocus != "Go systems and API design" || !repository.setupCompleted {
		t.Fatalf("setup was not persisted: %#v", repository)
	}
	if synchronizer.calls != 1 || profile.AgentID != "agent-1" {
		t.Fatalf("setup was not synchronized: calls=%d profile=%#v", synchronizer.calls, profile)
	}
}

func TestProfileAndFactMutationsSynchronizeDiscoveryVectors(t *testing.T) {
	repository := newProfileRepositoryStub()
	synchronizer := &profileSynchronizerStub{}
	service := NewProfileService(repository, profileAgentStub{}).WithSynchronizer(synchronizer)

	description := "Go systems"
	if _, err := service.Update(t.Context(), "owner-1", "agent-1", ProfileUpdate{ExpectedVersion: 1, Description: &description}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdatePolicies(t.Context(), "owner-1", "agent-1", 2, []PolicyChange{{
		SubjectType: SubjectIdentity,
		SubjectID:   "agent-1",
		Policy:      DisclosurePolicy{Visibility: VisibilityPublic, Channels: []DisclosureChannel{ChannelAgentFacts}, Indexable: true},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmCandidate(t.Context(), "owner-1", "agent-1", "candidate-1", 3, 1, ConfirmFactUpdate{}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RevokeFact(t.Context(), "owner-1", "agent-1", "fact-1", 3); err != nil {
		t.Fatal(err)
	}
	if synchronizer.calls != 4 {
		t.Fatalf("discovery synchronizer calls = %d, want 4", synchronizer.calls)
	}
}

func TestProfileMutationReportsIndexFailureAfterLocalSave(t *testing.T) {
	repository := newProfileRepositoryStub()
	synchronizer := &profileSynchronizerStub{err: errors.New("encoder offline")}
	service := NewProfileService(repository, profileAgentStub{}).WithSynchronizer(synchronizer)
	description := "Go systems"

	_, err := service.Update(t.Context(), "owner-1", "agent-1", ProfileUpdate{ExpectedVersion: 1, Description: &description})
	if !errors.Is(err, ErrIndexSync) || repository.identityUpdate.Description == nil {
		t.Fatalf("update error=%v identityUpdate=%#v", err, repository.identityUpdate)
	}
}

func TestDisclosurePolicyFiltering(t *testing.T) {
	public := DisclosurePolicy{Visibility: VisibilityPublic, Channels: []DisclosureChannel{ChannelAgentFacts}}
	if !public.Allows(ChannelAgentFacts, "", false) || public.Allows(ChannelAgentCard, "", false) {
		t.Fatal("public channel filtering is incorrect")
	}
	restricted := DisclosurePolicy{Visibility: VisibilityRestricted, Channels: []DisclosureChannel{ChannelAgentCard}, Audiences: []string{"team-1"}}
	if !restricted.Allows(ChannelAgentCard, "team-1", true) || restricted.Allows(ChannelAgentCard, "team-2", true) {
		t.Fatal("restricted audience filtering is incorrect")
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
	record          ProfileRecord
	facts           []ConfirmedFact
	policies        map[PolicyKey]DisclosurePolicy
	identityUpdate  ProfileUpdate
	policyChanges   []PolicyChange
	configuredName  string
	configuredFocus string
	setupCompleted  bool
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
func (repository *profileRepositoryStub) ListConfirmedFacts(context.Context, string, string, bool) ([]ConfirmedFact, error) {
	return repository.facts, nil
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
func (repository *profileRepositoryStub) ConfirmFact(context.Context, string, string, string, int64, int64, ConfirmFactUpdate) error {
	return nil
}
func (repository *profileRepositoryStub) RevokeFact(context.Context, string, string, string, int64) error {
	return nil
}
func (repository *profileRepositoryStub) ConfigureBasic(_ context.Context, _, _ string, name, focus string) error {
	repository.configuredName = name
	repository.configuredFocus = focus
	repository.record.Version++
	return nil
}
func (repository *profileRepositoryStub) CompleteSetup(context.Context, string, string) error {
	repository.setupCompleted = true
	now := time.Now()
	repository.record.ConfiguredAt = &now
	return nil
}

type profileSynchronizerStub struct {
	calls int
	err   error
}

func (synchronizer *profileSynchronizerStub) Sync(context.Context, string, string) error {
	synchronizer.calls++
	return synchronizer.err
}
