package mcp

import "testing"

func TestQualifiedToolNameIsStableBoundedAndUnique(t *testing.T) {
	first := qualifiedToolName(RuntimeTool{
		ServerID: "server-one", ServerName: "same server", Name: "a very long tool name with spaces and punctuation !!!!!!!!!!!!!!!!!!!!!!!!!!!!!",
	})
	second := qualifiedToolName(RuntimeTool{
		ServerID: "server-two", ServerName: "same-server", Name: "a very long tool name with spaces and punctuation !!!!!!!!!!!!!!!!!!!!!!!!!!!!!",
	})
	if len(first) > 64 || len(second) > 64 {
		t.Fatalf("qualified names exceed model tool limit: %q %q", first, second)
	}
	if first == second {
		t.Fatalf("different MCP targets collided: %q", first)
	}
	if repeated := qualifiedToolName(RuntimeTool{
		ServerID: "server-one", ServerName: "same server", Name: "a very long tool name with spaces and punctuation !!!!!!!!!!!!!!!!!!!!!!!!!!!!!",
	}); repeated != first {
		t.Fatalf("qualified name is unstable: %q != %q", repeated, first)
	}
}
