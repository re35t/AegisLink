package catalog

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/skills"
)

func TestCatalogProjectsMCPSkillsAndDiscoveryAsDistinctKinds(t *testing.T) {
	service := NewService(catalogAgentReader{}, catalogMCPReader{}, catalogSkillReader{}, true)
	page, err := service.List(t.Context(), "principal-one", "agent-one", Query{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("items = %#v", page.Items)
	}
	want := []struct {
		kind, category, action, availability string
	}{
		{"mcp-tool", "mcp", "force-tool-once", "tool-disabled"},
		{"skill", "skills", "use-skill-once", "skill-disabled"},
		{"discovery", "discovery", "discover-once", "ready"},
	}
	for index, expected := range want {
		item := page.Items[index]
		if item.Kind != expected.kind || item.Category != expected.category || item.Action != expected.action || item.Availability != expected.availability || item.ResourceID == "" {
			t.Fatalf("item %d = %#v, expected %#v", index, item, expected)
		}
	}
	discovery := page.Items[2]
	if discovery.ID != "discovery:agent-search" || discovery.Label != "Find related Agents" || discovery.Group.Label != "AegisLink Index" {
		t.Fatalf("discovery item = %#v", discovery)
	}
}

func TestCatalogFiltersKindsAndQuery(t *testing.T) {
	service := NewService(catalogAgentReader{}, catalogMCPReader{}, catalogSkillReader{}, true)
	page, err := service.List(t.Context(), "principal-one", "agent-one", Query{
		Kinds: []string{"skill"}, Query: "review", Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Kind != "skill" {
		t.Fatalf("filtered items = %#v", page.Items)
	}
}

func TestCatalogMarksDiscoveryOfflineWhenIndexIsNotConfigured(t *testing.T) {
	service := NewService(catalogAgentReader{}, catalogMCPReader{}, catalogSkillReader{}, false)
	page, err := service.List(t.Context(), "principal-one", "agent-one", Query{Kinds: []string{"discovery"}, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Availability != "server-offline" || page.Items[0].DisabledReason == "" {
		t.Fatalf("offline discovery item = %#v", page.Items)
	}
}

func TestCatalogSearchesDiscoveryByAgentAddrAndIndexTerms(t *testing.T) {
	service := NewService(catalogAgentReader{}, catalogMCPReader{}, catalogSkillReader{}, true)
	for _, query := range []string{"related agents", "agentaddr", "index"} {
		page, err := service.List(t.Context(), "principal-one", "agent-one", Query{Kinds: []string{"discovery"}, Query: query, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 {
			t.Fatalf("query %q returned %#v", query, page.Items)
		}
	}
}

type catalogAgentReader struct{}

func (catalogAgentReader) Get(context.Context, string, string) (agent.Agent, error) {
	return agent.Agent{ID: "agent-one"}, nil
}

type catalogMCPReader struct{}

func (catalogMCPReader) List(context.Context, string, string) ([]mcp.Server, error) {
	return []mcp.Server{{
		ID: "server-one", Name: "GitHub", Bound: true, Enabled: true, Status: "connected",
		Tools: []mcp.Tool{{
			ID: "tool-one", ServerID: "server-one", Name: "read_file", Description: "Read a file",
			InputSchema: json.RawMessage(`{"type":"object"}`), RiskLevel: mcp.ReadOnly, Enabled: false,
		}},
	}}, nil
}

type catalogSkillReader struct{}

func (catalogSkillReader) List(context.Context, string, string) ([]skills.Skill, error) {
	return []skills.Skill{{
		ID: "skill-one", AgentID: "agent-one", Name: "go-review", Description: "Review Go code", Enabled: false,
	}}, nil
}
