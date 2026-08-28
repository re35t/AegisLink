package agentindex

import (
	"context"
	"testing"
	"time"

	"github.com/re35t/AegisLink/internal/agent"
)

func TestSyncPublishesOnlyPublicIndexableAgentProfileUnits(t *testing.T) {
	repository := &repositoryStub{}
	encoder := &embedderStub{}
	index := &clientStub{}
	profile := agent.Profile{
		AgentID: "agent-1",
		Identity: agent.ProfileIdentity{
			ID: "agent-1", Name: "Atlas", Description: "Go systems",
			Disclosure: publicIndexPolicy(),
		},
		ConfirmedFacts: []agent.ConfirmedFact{
			{ID: "agent-fact", Subject: agent.FactSubjectAgent, Namespace: "expertise", Key: "language", Value: map[string]any{"name": "Go"}, Disclosure: publicIndexPolicy()},
			{ID: "user-fact", Subject: agent.FactSubjectUser, Namespace: "identity", Key: "email", Value: map[string]any{"value": "private@example.test"}, Disclosure: publicIndexPolicy()},
		},
	}
	service, err := NewService(repository, profileReaderStub{profile: profile}, encoder, index)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Sync(t.Context(), "owner-1", "agent-1"); err != nil {
		t.Fatal(err)
	}
	if index.registerKey != "server-agent-agent-1" || index.agentAddr != "agent_01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("registration/publication mismatch: %#v", index)
	}
	if len(encoder.input) != 2 || len(index.snapshot.Vectors) != 2 {
		t.Fatalf("expected identity and Agent Fact only: input=%q snapshot=%#v", encoder.input, index.snapshot)
	}
	if repository.published != 1 || index.snapshot.SourceSetDigest != sourceSetDigest(index.snapshot.Vectors) {
		t.Fatalf("publication state mismatch: repository=%#v snapshot=%#v", repository, index.snapshot)
	}
}

func TestSearchEncodesInputAndReturnsIndexCandidates(t *testing.T) {
	index := &clientStub{candidates: []Candidate{{AgentAddr: "agent_01ARZ3NDEKTSV4RRFFQ69G5FAV", Score: .8}}}
	encoder := &embedderStub{}
	service, err := NewService(&repositoryStub{}, profileReaderStub{profile: agent.Profile{AgentID: "agent-1"}}, encoder, index)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Search(t.Context(), "owner-1", "agent-1", "  Go backend help  ", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || len(encoder.input) != 1 || encoder.input[0] != "Go backend help" || index.topK != 3 {
		t.Fatalf("unexpected search: result=%#v input=%q topK=%d", result, encoder.input, index.topK)
	}
}

func publicIndexPolicy() agent.DisclosurePolicy {
	return agent.DisclosurePolicy{Visibility: agent.VisibilityPublic, Channels: []agent.DisclosureChannel{agent.ChannelAgentFacts}, Indexable: true}
}

type profileReaderStub struct{ profile agent.Profile }

func (reader profileReaderStub) Get(context.Context, string, string) (agent.Profile, error) {
	return reader.profile, nil
}

type embedderStub struct{ input []string }

func (embedder *embedderStub) Embed(_ context.Context, input []string) ([][]float32, error) {
	embedder.input = append([]string(nil), input...)
	result := make([][]float32, len(input))
	for index := range result {
		result[index] = make([]float32, EmbeddingDimensions)
		result[index][0] = 1
	}
	return result, nil
}

type clientStub struct {
	registerKey string
	agentAddr   string
	snapshot    Snapshot
	topK        int
	candidates  []Candidate
}

func (client *clientStub) Register(_ context.Context, key string) (string, error) {
	client.registerKey = key
	return "agent_01ARZ3NDEKTSV4RRFFQ69G5FAV", nil
}
func (client *clientStub) Publish(_ context.Context, address string, snapshot Snapshot) error {
	client.agentAddr = address
	client.snapshot = snapshot
	return nil
}
func (client *clientStub) Search(_ context.Context, _ []float32, topK int) ([]Candidate, error) {
	client.topK = topK
	return client.candidates, nil
}

type repositoryStub struct {
	state     State
	published int64
}

func (repository *repositoryStub) Get(context.Context, string, string) (State, error) {
	return repository.state, nil
}
func (repository *repositoryStub) SaveRegistration(_ context.Context, principalID, agentID, address string) error {
	repository.state = State{OwnerPrincipalID: principalID, AgentID: agentID, AgentAddr: address}
	return nil
}
func (repository *repositoryStub) ReserveRevision(context.Context, string, string) (int64, error) {
	repository.state.NextRevision++
	return repository.state.NextRevision, nil
}
func (repository *repositoryStub) MarkPublished(_ context.Context, _, _ string, revision int64) error {
	repository.published = revision
	return nil
}
func (repository *repositoryStub) MarkFailed(context.Context, string, string, error) error {
	repository.state.UpdatedAt = time.Now()
	return nil
}
