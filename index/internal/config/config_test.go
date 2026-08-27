package config

import (
	"strings"
	"testing"
	"time"
)

const testRegistrationToken = "0123456789abcdef0123456789abcdef"

func TestLoadUsesIndependentSettings(t *testing.T) {
	t.Setenv("INDEX_SERVER_ADDRESS", "127.0.0.1:4331")
	t.Setenv("INDEX_SERVER_SHUTDOWN_TIMEOUT", "10s")
	t.Setenv("INDEX_DATABASE_URL", "postgres://aegislink:aegislink@127.0.0.1:55434/aegislink_index?sslmode=disable")
	t.Setenv("INDEX_REGISTRATION_TOKEN", testRegistrationToken)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Address != "127.0.0.1:4331" || cfg.Server.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected server config: %#v", cfg.Server)
	}
	if !strings.Contains(cfg.Database.URL, "aegislink_index") || cfg.Security.RegistrationToken != testRegistrationToken {
		t.Fatalf("unexpected dependency config: %#v", cfg)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	validURL := "postgres://aegislink:aegislink@127.0.0.1:55434/aegislink_index?sslmode=disable"
	for _, test := range []struct {
		name     string
		address  string
		timeout  string
		database string
		token    string
		want     string
	}{
		{name: "blank address", address: " ", timeout: "10s", database: validURL, token: testRegistrationToken, want: "INDEX_SERVER_ADDRESS"},
		{name: "invalid timeout", address: "127.0.0.1:4331", timeout: "later", database: validURL, token: testRegistrationToken, want: "INDEX_SERVER_SHUTDOWN_TIMEOUT"},
		{name: "non-positive timeout", address: "127.0.0.1:4331", timeout: "0s", database: validURL, token: testRegistrationToken, want: "must be positive"},
		{name: "missing database", address: "127.0.0.1:4331", timeout: "10s", token: testRegistrationToken, want: "INDEX_DATABASE_URL"},
		{name: "invalid database", address: "127.0.0.1:4331", timeout: "10s", database: "http://localhost/index", token: testRegistrationToken, want: "PostgreSQL URL"},
		{name: "short token", address: "127.0.0.1:4331", timeout: "10s", database: validURL, token: "short", want: "at least 32"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("INDEX_SERVER_ADDRESS", test.address)
			t.Setenv("INDEX_SERVER_SHUTDOWN_TIMEOUT", test.timeout)
			t.Setenv("INDEX_DATABASE_URL", test.database)
			t.Setenv("INDEX_REGISTRATION_TOKEN", test.token)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, expected %q", err, test.want)
			}
		})
	}
}
