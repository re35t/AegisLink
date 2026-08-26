package agentcontext

import (
	"context"
	"testing"

	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/impression"
)

func TestResolveInjectsFactsAndRanksAtMostTwelveImpressions(t *testing.T) {
	items := make([]impression.Impression, 14)
	for index := range items {
		items[index] = impression.Impression{ID: string(rune('a' + index)), Kind: impression.KindCurrentTask, Summary: "unrelated", Confidence: .8, Salience: .8, Freshness: .8}
	}
	items[13].Summary = "current security task"
	relevantID := items[13].ID
	service := NewService(selectionMemoryReader{}, selectionSkillReader{}, selectionMCPRuntime{}).WithProfileContext(factReaderStub{}, impressionReaderStub{items: items})
	resolved, err := service.Resolve(t.Context(), "owner", "agent", conversation.ContextRequest{CurrentMessage: "continue security work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Facts) != 1 || len(resolved.Impressions) != 12 {
		t.Fatalf("context = %#v", resolved)
	}
	if resolved.Impressions[0].ID != relevantID {
		t.Fatalf("relevant Impression was not ranked first: %#v", resolved.Impressions[0])
	}
}

type factReaderStub struct{}

func (factReaderStub) RuntimeFacts(context.Context, string, string) ([]agent.ConfirmedFact, error) {
	return []agent.ConfirmedFact{{ID: "fact-one", Subject: agent.FactSubjectUser, Namespace: "preference", Key: "language", Value: map[string]any{"value": "Go"}}}, nil
}

type impressionReaderStub struct{ items []impression.Impression }

func (stub impressionReaderStub) List(context.Context, string, string, string) ([]impression.Impression, error) {
	return stub.items, nil
}
