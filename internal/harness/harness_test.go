package harness

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/catalog"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/impression"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/memory"
	"github.com/re35t/AegisLink/internal/runtime"
	"github.com/re35t/AegisLink/internal/skills"
)

func TestResolveSelectionUsesTypedPolicies(t *testing.T) {
	harness := newTestHarness(&runtimeStub{})
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
		policy, err := harness.ResolveSelection(t.Context(), "principal-one", "agent-one", test.selection)
		if err != nil {
			t.Fatalf("resolve %#v: %v", test.selection, err)
		}
		if policy.Mode != test.mode || policy.Kind != test.kind || policy.QualifiedToolName != test.qualified || policy.ResourceID == "" {
			t.Fatalf("policy = %#v", policy)
		}
	}
}

func TestResolveSelectionRejectsDisabledSkill(t *testing.T) {
	harness := New(&runtimeStub{}, memoryReaderStub{}, disabledSkillReader{}, mcpRuntimeStub{}, factReaderStub{}, impressionReaderStub{})
	_, err := harness.ResolveSelection(t.Context(), "principal-one", "agent-one", conversation.RunSelection{MentionID: "skill:skill-one", Action: "use-skill-once"})
	if !errors.Is(err, skills.ErrDisabled) {
		t.Fatalf("error = %v", err)
	}
}

func TestRunResolvesContextBuildsToolsAndMapsEvents(t *testing.T) {
	items := make([]impression.Impression, 14)
	for index := range items {
		items[index] = impression.Impression{ID: string(rune('a' + index)), Kind: impression.KindCurrentTask, Summary: "unrelated", Confidence: .8, Salience: .8, Freshness: .8}
	}
	items[13].Summary = "current security task"
	stub := &runtimeStub{events: []runtime.Event{{Tool: &runtime.ToolEvent{Type: runtime.ToolStarted, ID: "call-one", Name: "get_current_time", Arguments: `{}`}}, {Delta: "done"}}}
	harness := New(stub, memoryReaderStub{}, skillReaderStub{}, mcpRuntimeStub{}, factReaderStub{}, impressionReaderStub{items: items})
	outputs, err := harness.Run(t.Context(), conversation.HarnessInput{
		PrincipalID: "owner", Agent: agent.Agent{ID: "agent", Name: "Chet", Description: "Personal Agent", SystemPrompt: "base"},
		Messages: []conversation.Message{{Role: "user", Content: "continue security work"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []conversation.HarnessOutput
	for output := range outputs {
		got = append(got, output)
	}
	if len(got) != 2 || got[0].Tool == nil || got[0].Tool.Type != conversation.HarnessToolStarted || got[1].Delta != "done" {
		t.Fatalf("outputs = %#v", got)
	}
	for _, expected := range []string{"Chet", "Personal Agent", "base", "prefers terse answers", "current security task", "go-review", "load_skill"} {
		if !strings.Contains(stub.input.Instruction, expected) {
			t.Fatalf("instruction missing %q: %s", expected, stub.input.Instruction)
		}
	}
	if strings.Contains(stub.input.Instruction, "full private instructions") {
		t.Fatalf("full Skill content leaked before progressive load: %s", stub.input.Instruction)
	}
	if strings.Count(stub.input.Instruction, "confidence=") != 12 {
		t.Fatalf("expected 12 ranked impressions: %s", stub.input.Instruction)
	}
	if len(stub.input.Tools) != 4 || stub.input.Tools[0].Name != "get_current_time" || stub.input.Tools[1].Name != "discover_capabilities" || stub.input.Tools[3].Name != "mcp__demo__read" {
		t.Fatalf("tools = %#v", stub.input.Tools)
	}
}

func TestHarnessToolsLoadSkillResourcesAndInvokeMCP(t *testing.T) {
	read := false
	mcpCalled := false
	context := agentContext{
		skills: []runtimeSkill{{name: "bundle-skill", description: "Uses a reference", content: "full private instructions", files: []runtimeSkillFile{
			{path: "SKILL.md", mediaType: "text/markdown", textReadable: true},
			{path: "references/guide.md", mediaType: "text/markdown", textReadable: true},
			{path: "assets/image.png", mediaType: "image/png", textReadable: false},
		}, readResource: func(_ context.Context, path string) (runtimeSkillResource, error) {
			read = true
			return runtimeSkillResource{path: path, mediaType: "text/markdown", content: "reference body"}, nil
		}}},
		tools: []runtime.Tool{{Name: "mcp__demo__read", InputSchema: json.RawMessage(`{"type":"object"}`), Invoke: func(_ context.Context, arguments string) (string, error) { mcpCalled = true; return arguments, nil }}},
	}
	tools := harnessTools(context)
	loaded, err := tools[2].Invoke(t.Context(), `{"name":"bundle-skill"}`)
	if err != nil || !strings.Contains(loaded, "references/guide.md") || !strings.Contains(loaded, "assets/image.png") {
		t.Fatalf("manifest=%q err=%v", loaded, err)
	}
	resource, err := tools[3].Invoke(t.Context(), `{"name":"bundle-skill","path":"references/guide.md"}`)
	if err != nil || !read || !strings.Contains(resource, "reference body") {
		t.Fatalf("resource=%q read=%v err=%v", resource, read, err)
	}
	if _, err := tools[3].Invoke(t.Context(), `{"name":"bundle-skill","path":"assets/image.png"}`); err == nil {
		t.Fatal("binary asset should not be exposed as text")
	}
	if _, err := tools[4].Invoke(t.Context(), `{}`); err != nil || !mcpCalled {
		t.Fatalf("MCP invocation called=%v err=%v", mcpCalled, err)
	}
}

func TestRunMapsForcedPolicyAndSkillValidation(t *testing.T) {
	stub := &runtimeStub{}
	harness := newTestHarness(stub)
	outputs, err := harness.Run(t.Context(), conversation.HarnessInput{
		PrincipalID: "owner", Agent: agent.Agent{ID: "agent", Name: "Aegis"}, Messages: []conversation.Message{{Role: "user", Content: "review"}},
		Policy: conversation.ExecutionPolicy{Mode: "use-skill-once", SkillName: "go-review", QualifiedToolName: "load_skill"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range outputs {
	}
	if stub.input.ToolChoice.Mode != runtime.ToolChoiceForceOnce || stub.input.ToolChoice.Name != "load_skill" {
		t.Fatalf("tool choice = %#v", stub.input.ToolChoice)
	}
	if err := stub.input.ToolChoice.ValidateArguments(`{"name":"other"}`); !errors.Is(err, conversation.ErrSelectedSkillMismatch) {
		t.Fatalf("validator error = %v", err)
	}
}

type runtimeStub struct {
	input  runtime.Input
	events []runtime.Event
}

func (stub *runtimeStub) Run(_ context.Context, input runtime.Input) <-chan runtime.Event {
	stub.input = input
	output := make(chan runtime.Event, len(stub.events))
	for _, event := range stub.events {
		output <- event
	}
	close(output)
	return output
}

func newTestHarness(agentRuntime runtime.Runtime) *Harness {
	return New(agentRuntime, memoryReaderStub{}, skillReaderStub{}, mcpRuntimeStub{}, factReaderStub{}, impressionReaderStub{})
}

type memoryReaderStub struct{}

func (memoryReaderStub) Context(context.Context, string, string) ([]memory.Memory, error) {
	return []memory.Memory{{ID: "memory-one", Kind: memory.Semantic, Content: "prefers terse answers"}}, nil
}

type skillReaderStub struct{}

func (skillReaderStub) List(context.Context, string, string) ([]skills.Skill, error) {
	return []skills.Skill{{ID: "skill-one", Name: "go-review", Description: "Review Go", Content: "full private instructions", Enabled: true}}, nil
}
func (skillReaderStub) Enabled(ctx context.Context, principalID, agentID string) ([]skills.Skill, error) {
	return skillReaderStub{}.List(ctx, principalID, agentID)
}
func (skillReaderStub) ReadFile(context.Context, string, string, string, string) (skills.File, error) {
	return skills.File{}, nil
}

type disabledSkillReader struct{ skillReaderStub }

func (disabledSkillReader) List(context.Context, string, string) ([]skills.Skill, error) {
	return []skills.Skill{{ID: "skill-one", Name: "go-review", Enabled: false}}, nil
}

type mcpRuntimeStub struct{}

func (mcpRuntimeStub) RuntimeTools(context.Context, string, string) ([]mcp.RuntimeTool, error) {
	return []mcp.RuntimeTool{{ToolID: "tool-one", ServerID: "server-one", Name: "read", QualifiedName: "mcp__demo__read", Description: "Read demo", InputSchema: json.RawMessage(`{"type":"object"}`)}}, nil
}
func (mcpRuntimeStub) ResolveMention(context.Context, string, string, string) (mcp.RuntimeTool, error) {
	return mcp.RuntimeTool{ToolID: "tool-one", Name: "read", QualifiedName: "mcp__demo__read"}, nil
}
func (mcpRuntimeStub) Invoke(context.Context, string, string, string, string, string) (string, error) {
	return "ok", nil
}

type factReaderStub struct{}

func (factReaderStub) RuntimeFacts(context.Context, string, string) ([]agent.ConfirmedFact, error) {
	return []agent.ConfirmedFact{{ID: "fact-one", Subject: agent.FactSubjectUser, Namespace: "preference", Key: "language", Value: map[string]any{"value": "Go"}}}, nil
}

type impressionReaderStub struct{ items []impression.Impression }

func (stub impressionReaderStub) List(context.Context, string, string, string) ([]impression.Impression, error) {
	return stub.items, nil
}
