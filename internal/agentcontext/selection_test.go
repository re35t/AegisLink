package agentcontext

import (
	"context"
	"errors"
	"testing"

	"github.com/re35t/AegisLink/internal/catalog"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/memory"
	"github.com/re35t/AegisLink/internal/skills"
)

func TestResolveSelectionUsesTypedPolicies(t *testing.T) {
	service := NewService(selectionMemoryReader{}, selectionSkillReader{}, selectionMCPRuntime{})
	for _, test := range []struct {
		selection conversation.RunSelection
		mode      string
		kind      string
		qualified string
	}{
		{conversation.RunSelection{MentionID: "mcp-tool:tool-one", Action: "force-tool-once"}, "force-tool-once", "mcp-tool", "mcp__demo__read"},
		{conversation.RunSelection{MentionID: "skill:skill-one", Action: "use-skill-once"}, "use-skill-once", "skill", "load_skill"},
		{conversation.RunSelection{MentionID: catalog.DiscoveryMentionID, Action: "discover-once"}, "discover-once", "discovery", "discover_capabilities"},
	} {
		policy, err := service.ResolveSelection(t.Context(), "principal-one", "agent-one", test.selection)
		if err != nil {
			t.Fatalf("resolve %#v: %v", test.selection, err)
		}
		if policy.Mode != test.mode || policy.Kind != test.kind || policy.QualifiedToolName != test.qualified || policy.ResourceID == "" {
			t.Fatalf("policy = %#v", policy)
		}
	}
}

func TestResolveSelectionRejectsDisabledSkill(t *testing.T) {
	service := NewService(selectionMemoryReader{}, disabledSelectionSkillReader{}, selectionMCPRuntime{})
	_, err := service.ResolveSelection(t.Context(), "principal-one", "agent-one", conversation.RunSelection{
		MentionID: "skill:skill-one", Action: "use-skill-once",
	})
	if !errors.Is(err, skills.ErrDisabled) {
		t.Fatalf("error = %v", err)
	}
}

type selectionMemoryReader struct{}

func (selectionMemoryReader) Context(context.Context, string, string) ([]memory.Memory, error) {
	return nil, nil
}

type selectionSkillReader struct{}

func (selectionSkillReader) List(context.Context, string, string) ([]skills.Skill, error) {
	return []skills.Skill{{ID: "skill-one", Name: "go-review", Enabled: true}}, nil
}
func (selectionSkillReader) Enabled(context.Context, string, string) ([]skills.Skill, error) {
	return nil, nil
}
func (selectionSkillReader) ReadFile(context.Context, string, string, string, string) (skills.File, error) {
	return skills.File{}, nil
}

type disabledSelectionSkillReader struct{ selectionSkillReader }

func (disabledSelectionSkillReader) List(context.Context, string, string) ([]skills.Skill, error) {
	return []skills.Skill{{ID: "skill-one", Name: "go-review", Enabled: false}}, nil
}

type selectionMCPRuntime struct{}

func (selectionMCPRuntime) RuntimeTools(context.Context, string, string) ([]mcp.RuntimeTool, error) {
	return nil, nil
}
func (selectionMCPRuntime) ResolveMention(context.Context, string, string, string) (mcp.RuntimeTool, error) {
	return mcp.RuntimeTool{ToolID: "tool-one", Name: "read", QualifiedName: "mcp__demo__read"}, nil
}
func (selectionMCPRuntime) Invoke(context.Context, string, string, string, string, string) (string, error) {
	return "", nil
}
