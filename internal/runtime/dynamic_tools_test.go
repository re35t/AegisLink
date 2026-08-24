package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/re35t/AegisLink/internal/conversation"
)

func TestRuntimeToolsUseProgressiveSkillLoadingAndDynamicMCP(t *testing.T) {
	called := false
	tools, err := runtimeTools(t.Context(), nil, conversation.AgentContext{
		Skills: []conversation.RuntimeSkill{{
			Name: "test-skill", Description: "Use for tests", Content: "full private instructions",
		}},
		Tools: []conversation.RuntimeTool{{
			Name: "mcp__demo__read", Description: "Read demo data",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}}}`),
			Invoke: func(_ context.Context, arguments string) (string, error) {
				called = true
				return arguments, nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 {
		t.Fatalf("tool count = %d", len(tools))
	}
	loader, ok := tools[0].(tool.InvokableTool)
	if !ok {
		t.Fatalf("skill loader is not invokable: %T", tools[0])
	}
	loaded, err := loader.InvokableRun(t.Context(), `{"name":"test-skill"}`)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == "" || loaded == `{"name":"test-skill"}` {
		t.Fatalf("skill content was not loaded: %q", loaded)
	}
	dynamic, ok := tools[1].(tool.InvokableTool)
	if !ok {
		t.Fatalf("MCP tool is not invokable: %T", tools[1])
	}
	output, err := dynamic.InvokableRun(t.Context(), `{"id":"one"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !called || output != `{"id":"one"}` {
		t.Fatalf("dynamic invocation called=%v output=%q", called, output)
	}
}

func TestAgentInstructionInjectsMemoryButOnlySkillMetadata(t *testing.T) {
	instruction := agentInstruction("base", conversation.AgentContext{
		Memories: []conversation.RuntimeMemory{{ID: "memory-one", Kind: "semantic", Content: "prefers terse answers"}},
		Skills: []conversation.RuntimeSkill{{
			Name: "test-skill", Description: "Use for tests", Content: "full private instructions",
		}},
	})
	for _, expected := range []string{"base", "prefers terse answers", "test-skill", "Use for tests", "load_skill"} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("instruction missing %q: %s", expected, instruction)
		}
	}
	if strings.Contains(instruction, "full private instructions") {
		t.Fatalf("full skill content leaked before progressive load: %s", instruction)
	}
}
