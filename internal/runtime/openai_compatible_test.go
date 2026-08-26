package runtime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAICompatibleModelServiceStreamsThroughEino(t *testing.T) {
	t.Parallel()
	modelService := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			http.NotFound(response, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(response, "missing model authorization", http.StatusUnauthorized)
			return
		}
		response.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := response.(http.Flusher)
		if !ok {
			http.Error(response, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(response, "data: {\"id\":\"mock\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"deepseek-mock\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello \"},\"finish_reason\":null}]}\n\n")
		flusher.Flush()
		_, _ = io.WriteString(response, "data: {\"id\":\"mock\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"deepseek-mock\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"from service\"},\"finish_reason\":null}]}\n\n")
		_, _ = io.WriteString(response, "data: {\"id\":\"mock\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"deepseek-mock\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
		flusher.Flush()
	}))
	t.Cleanup(modelService.Close)

	runtime, err := New(t.Context(), ModelConfig{
		ID:        "test-model",
		Driver:    "openai-compatible",
		BaseURL:   modelService.URL + "/v1",
		APIKey:    "test-key",
		Name:      "deepseek-mock",
		Timeout:   5 * time.Second,
		MaxTokens: 128,
	}, Options{MaxIterations: 4})
	if err != nil {
		t.Fatal(err)
	}

	var answer strings.Builder
	for output := range runtime.Run(context.Background(), Input{
		Agent: Agent{Name: "Aegis"}, Instruction: "Be concise.",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	}) {
		if output.Err != nil {
			t.Fatalf("unexpected runtime error: %v", output.Err)
		}
		answer.WriteString(output.Delta)
	}
	if answer.String() != "hello from service" {
		t.Fatalf("unexpected streamed answer %q", answer.String())
	}
}

func TestOpenAICompatibleModelServiceRunsSingleTurn(t *testing.T) {
	t.Parallel()
	modelService := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			http.NotFound(response, request)
			return
		}
		var body struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		if body.Stream || len(body.Messages) != 2 || body.Messages[0].Role != "system" {
			http.Error(response, "single-turn request must be a non-streaming system plus user completion", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"id":"mock","object":"chat.completion","created":1,"model":"deepseek-mock","choices":[{"index":0,"message":{"role":"assistant","content":"{\"impressions\":[],\"facts\":[]}"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(modelService.Close)

	agentRuntime, err := New(t.Context(), ModelConfig{
		ID: "test-model", Driver: "openai-compatible", BaseURL: modelService.URL + "/v1", APIKey: "test-key",
		Name: "deepseek-mock", Timeout: 5 * time.Second, MaxTokens: 128,
	}, Options{MaxIterations: 1})
	if err != nil {
		t.Fatal(err)
	}

	var answer strings.Builder
	for output := range agentRuntime.Run(context.Background(), Input{
		ExecutionMode: ExecutionModeSingleTurn,
		Instruction:   "Return JSON only.",
		Messages:      []Message{{Role: RoleUser, Content: "{}"}},
	}) {
		if output.Err != nil {
			t.Fatalf("unexpected runtime error: %v", output.Err)
		}
		answer.WriteString(output.Delta)
	}
	if answer.String() != `{"impressions":[],"facts":[]}` {
		t.Fatalf("unexpected single-turn answer %q", answer.String())
	}
}
