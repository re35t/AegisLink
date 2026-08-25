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
	if len(tools) != 3 {
		t.Fatalf("tool count = %d", len(tools))
	}
	discovery, ok := tools[0].(tool.InvokableTool)
	if !ok {
		t.Fatalf("discovery tool is not invokable: %T", tools[0])
	}
	discovered, err := discovery.InvokableRun(t.Context(), `{"query":"read demo data"}`)
	if err != nil || !strings.Contains(discovered, "test-skill") || !strings.Contains(discovered, "mcp__demo__read") {
		t.Fatalf("capability discovery output=%q err=%v", discovered, err)
	}
	loader, ok := tools[1].(tool.InvokableTool)
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
	dynamic, ok := tools[2].(tool.InvokableTool)
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

func TestRuntimeToolsReadSkillResourcesWithoutExecutingScripts(t *testing.T) {
	read := false
	tools, err := runtimeTools(t.Context(), nil, conversation.AgentContext{
		Skills: []conversation.RuntimeSkill{{
			Name: "bundle-skill", Description: "Uses a reference", Content: "Read references/guide.md",
			Files: []conversation.RuntimeSkillFile{
				{Path: "SKILL.md", MediaType: "text/markdown", SizeBytes: 32, TextReadable: true},
				{Path: "references/guide.md", MediaType: "text/markdown", SizeBytes: 12, TextReadable: true},
				{Path: "assets/image.png", MediaType: "image/png", SizeBytes: 20, TextReadable: false},
			},
			ReadResource: func(_ context.Context, filePath string) (conversation.RuntimeSkillResource, error) {
				read = true
				return conversation.RuntimeSkillResource{Path: filePath, MediaType: "text/markdown", Content: "reference body"}, nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 3 {
		t.Fatalf("tool count = %d", len(tools))
	}
	manifestLoader := tools[1].(tool.InvokableTool)
	manifest, err := manifestLoader.InvokableRun(t.Context(), `{"name":"bundle-skill"}`)
	if err != nil || !strings.Contains(manifest, "references/guide.md") || !strings.Contains(manifest, "assets/image.png") {
		t.Fatalf("bundle manifest metadata missing: output=%q err=%v", manifest, err)
	}
	resourceLoader := tools[2].(tool.InvokableTool)
	resource, err := resourceLoader.InvokableRun(t.Context(), `{"name":"bundle-skill","path":"references/guide.md"}`)
	if err != nil || !read || !strings.Contains(resource, "reference body") {
		t.Fatalf("resource was not loaded safely: read=%v output=%q err=%v", read, resource, err)
	}
	_, err = resourceLoader.InvokableRun(t.Context(), `{"name":"bundle-skill","path":"assets/image.png"}`)
	if err == nil {
		t.Fatal("binary asset should not be exposed as a text resource")
	}
}
