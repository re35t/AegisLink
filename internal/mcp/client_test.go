package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOfficialClientEndpointPolicy(t *testing.T) {
	publicClient := NewOfficialClient(time.Second, false)
	if err := publicClient.ValidateEndpoint("https://example.com/mcp"); err != nil {
		t.Fatalf("public HTTPS endpoint rejected: %v", err)
	}
	for _, endpoint := range []string{
		"http://example.com/mcp",
		"https://user:password@example.com/mcp",
		"https://example.com/mcp?token=secret",
		"https://example.com/mcp#fragment",
	} {
		if err := publicClient.ValidateEndpoint(endpoint); err == nil {
			t.Fatalf("unsafe endpoint accepted: %s", endpoint)
		}
	}
	localClient := NewOfficialClient(time.Second, true)
	if err := localClient.ValidateEndpoint("http://127.0.0.1:9000/mcp"); err != nil {
		t.Fatalf("explicit local endpoint rejected: %v", err)
	}
}

func TestOfficialClientDiscoversDisabledToolsAndInvokes(t *testing.T) {
	type input struct {
		Name string `json:"name" jsonschema:"required"`
	}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "test", Version: "1.0.0"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "greet", Description: "Greet a person",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, arguments input) (*mcpsdk.CallToolResult, any, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "hello " + arguments.Name}}}, nil, nil
	})
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	client := NewOfficialClient(3*time.Second, true)
	tools, protocol, err := client.Discover(t.Context(), httpServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	if protocol == "" || len(tools) != 1 || tools[0].Name != "greet" {
		t.Fatalf("unexpected discovery: protocol=%q tools=%#v", protocol, tools)
	}
	if tools[0].Enabled || tools[0].RiskLevel != ReadOnly {
		t.Fatalf("tool hints must not auto-authorize execution: %#v", tools[0])
	}
	output, err := client.Invoke(t.Context(), httpServer.URL, "greet", `{"name":"Aegis"}`)
	if err != nil {
		t.Fatal(err)
	}
	if output != "hello Aegis" {
		t.Fatalf("output = %q", output)
	}
}
