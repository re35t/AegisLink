package runtime

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/config"
	"github.com/re35t/AegisLink/internal/conversation"
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

	runtime, err := New(t.Context(), config.Model{
		ID:        "test-model",
		Driver:    "openai-compatible",
		BaseURL:   modelService.URL + "/v1",
		APIKey:    "test-key",
		Name:      "deepseek-mock",
		Timeout:   5 * time.Second,
		MaxTokens: 128,
	}, config.AgentRuntime{MaxIterations: 4})
	if err != nil {
		t.Fatal(err)
	}

	var answer strings.Builder
	for output := range runtime.Stream(context.Background(), conversation.RuntimeInput{
		Agent:    agent.Agent{Name: "Aegis", SystemPrompt: "Be concise."},
		Messages: []conversation.Message{{Role: "user", Content: "hello"}},
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
