package runtime

import (
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"

	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/config"
	"github.com/re35t/AegisLink/internal/conversation"
)

func TestDeepSeekSmoke(t *testing.T) {
	if os.Getenv("REAL_MODEL_SMOKE") != "1" {
		t.Skip("set REAL_MODEL_SMOKE=1 for an explicit real DeepSeek smoke test")
	}
	_ = godotenv.Load("../../.env")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model.Driver != "deepseek" {
		t.Skip("active model is not the DeepSeek provider")
	}
	runtime, err := New(t.Context(), cfg.Model, cfg.Runtime)
	if err != nil {
		t.Fatal(err)
	}

	var answer strings.Builder
	for output := range runtime.Stream(t.Context(), conversation.RuntimeInput{
		Agent: agent.Agent{Name: "Aegis smoke", SystemPrompt: "You are a concise test agent."},
		Messages: []conversation.Message{{
			Role:    "user",
			Content: "You must call get_current_time with timezone Asia/Shanghai. Then reply with AEGISLINK_SMOKE_OK and the tool result.",
		}},
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
