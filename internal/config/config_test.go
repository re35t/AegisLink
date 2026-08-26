package config

import (
	"testing"
	"time"
)

func TestValidateAcceptsNonEmptyModelDriversForRuntimeRegistry(t *testing.T) {
	t.Parallel()
	for _, driver := range []string{"deepseek", "openai-compatible", "future-provider"} {
		cfg := validConfig()
		cfg.Model.Driver = driver
		if err := cfg.Validate(); err != nil {
			t.Fatalf("driver %s should be valid: %v", driver, err)
		}
	}
}

func TestValidateRejectsMissingSecretsAndInvalidURLs(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Model.ID = ""
	cfg.Model.APIKey = ""
	cfg.Model.BaseURL = "not-a-url"
	cfg.Runtime.MaxIterations = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid configuration")
	}
}

func TestDefaultDeepSeekModelUsesCurrentModelName(t *testing.T) {
	t.Parallel()
	if got := defaultModelName("deepseek"); got != "deepseek-v4-flash" {
		t.Fatalf("default DeepSeek model = %q", got)
	}
}

func TestBlankCuratorOverridesFallBack(t *testing.T) {
	t.Setenv("CURATOR_MODEL_NAME", "   ")
	if got := nonBlankEnv("CURATOR_MODEL_NAME", "primary-model"); got != "primary-model" {
		t.Fatalf("blank override = %q", got)
	}
}

func validConfig() Config {
	return Config{
		Server:   Server{Address: "127.0.0.1:4321", ShutdownTimeout: time.Second},
		Database: Database{URL: "postgres://localhost/aegislink"},
		Auth:     Auth{SessionTTL: 24 * time.Hour},
		Model: Model{
			ID: "deepseek-primary", Driver: "deepseek", BaseURL: "https://api.deepseek.com", APIKey: "secret", Name: "deepseek-v4-flash", Timeout: time.Minute, MaxTokens: 1024,
		},
		Runtime: AgentRuntime{MaxIterations: 8},
		MCP:     MCP{Timeout: 15 * time.Second},
		Web:     Web{Origin: "http://127.0.0.1:5173"},
	}
}
