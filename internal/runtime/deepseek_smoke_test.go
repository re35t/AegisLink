package runtime

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func TestDeepSeekSmoke(t *testing.T) {
	if os.Getenv("REAL_MODEL_SMOKE") != "1" {
		t.Skip("set REAL_MODEL_SMOKE=1 for an explicit real DeepSeek smoke test")
	}
	_ = godotenv.Load("../../.env")
	maxTokens, _ := strconv.Atoi(envOr("MODEL_MAX_TOKENS", "4096"))
	maxIterations, _ := strconv.Atoi(envOr("AGENT_MAX_ITERATIONS", "8"))
	agentRuntime, err := New(t.Context(), ModelConfig{
		ID: strings.TrimSpace(envOr("MODEL_ID", "deepseek-primary")), Driver: strings.TrimSpace(envOr("MODEL_DRIVER", "deepseek")),
		BaseURL: strings.TrimSpace(envOr("MODEL_BASE_URL", "https://api.deepseek.com")), APIKey: strings.TrimSpace(os.Getenv("MODEL_API_KEY")),
		Name: strings.TrimSpace(envOr("MODEL_NAME", "deepseek-chat")), Timeout: 2 * time.Minute, MaxTokens: maxTokens,
	}, Options{MaxIterations: maxIterations})
	if err != nil {
		t.Fatal(err)
	}
	currentTimeTool := Tool{Name: "get_current_time", Description: "Get current time.", InputSchema: []byte(`{"type":"object"}`), Invoke: func(context.Context, string) (string, error) { return time.Now().Format(time.RFC3339), nil }}
	var answer strings.Builder
	for output := range agentRuntime.Run(t.Context(), Input{
		Agent: Agent{Name: "Aegis smoke"}, Instruction: "You are a concise test agent.", Tools: []Tool{currentTimeTool},
		Messages: []Message{{Role: RoleUser, Content: "You must call get_current_time. Then reply with AEGISLINK_SMOKE_OK and the tool result."}},
	}) {
		if output.Err != nil {
			t.Fatal(output.Err)
		}
		answer.WriteString(output.Delta)
	}
	if !strings.Contains(answer.String(), "AEGISLINK_SMOKE_OK") {
		t.Fatalf("DeepSeek smoke response did not contain the expected marker")
	}
}

func envOr(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
