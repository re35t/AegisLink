package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   Server
	Database Database
	Auth     Auth
	Model    Model
	Runtime  AgentRuntime
	MCP      MCP
	Web      Web
}

type Server struct {
	Address         string
	ShutdownTimeout time.Duration
}

type Database struct {
	URL string
}

type Auth struct {
	SessionTTL   time.Duration
	CookieSecure bool
}

type Model struct {
	ID        string
	Driver    string
	BaseURL   string
	APIKey    string
	Name      string
	Timeout   time.Duration
	MaxTokens int
}

type AgentRuntime struct {
	MaxIterations int
}

type MCP struct {
	Timeout              time.Duration
	AllowPrivateNetworks bool
}

type Web struct {
	Origin string
}

func Load() (Config, error) {
	_ = godotenv.Load()
	timeout, err := durationEnv("MODEL_TIMEOUT", 2*time.Minute)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := durationEnv("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	maxTokens, err := intEnv("MODEL_MAX_TOKENS", 4096)
	if err != nil {
		return Config{}, err
	}
	maxIterations, err := intEnv("AGENT_MAX_ITERATIONS", 8)
	if err != nil {
		return Config{}, err
	}
	sessionTTL, err := durationEnv("AUTH_SESSION_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cookieSecure, err := boolEnv("AUTH_COOKIE_SECURE", false)
	if err != nil {
		return Config{}, err
	}
	mcpTimeout, err := durationEnv("MCP_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	mcpAllowPrivateNetworks, err := boolEnv("MCP_ALLOW_PRIVATE_NETWORKS", false)
	if err != nil {
		return Config{}, err
	}

	driver := strings.TrimSpace(env("MODEL_DRIVER", "deepseek"))
	cfg := Config{
		Server: Server{
			Address:         env("SERVER_ADDRESS", "127.0.0.1:4321"),
			ShutdownTimeout: shutdownTimeout,
		},
		Database: Database{URL: strings.TrimSpace(os.Getenv("DATABASE_URL"))},
		Auth:     Auth{SessionTTL: sessionTTL, CookieSecure: cookieSecure},
		Model: Model{
			ID:        strings.TrimSpace(env("MODEL_ID", "deepseek-primary")),
			Driver:    driver,
			BaseURL:   strings.TrimSpace(env("MODEL_BASE_URL", defaultBaseURL(driver))),
			APIKey:    strings.TrimSpace(os.Getenv("MODEL_API_KEY")),
			Name:      strings.TrimSpace(env("MODEL_NAME", defaultModelName(driver))),
			Timeout:   timeout,
			MaxTokens: maxTokens,
		},
		Runtime: AgentRuntime{MaxIterations: maxIterations},
		MCP: MCP{
			Timeout: mcpTimeout, AllowPrivateNetworks: mcpAllowPrivateNetworks,
		},
		Web: Web{Origin: strings.TrimSpace(env("WEB_ORIGIN", "http://127.0.0.1:5173"))},
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) Validate() error {
	var problems []error
	if strings.TrimSpace(cfg.Server.Address) == "" {
		problems = append(problems, errors.New("SERVER_ADDRESS is required"))
	}
	if cfg.Database.URL == "" {
		problems = append(problems, errors.New("DATABASE_URL is required"))
	}
	if cfg.Auth.SessionTTL <= 0 {
		problems = append(problems, errors.New("AUTH_SESSION_TTL must be positive"))
	}
	if cfg.Model.ID == "" {
		problems = append(problems, errors.New("MODEL_ID is required"))
	}
	if cfg.Model.Driver == "" {
		problems = append(problems, errors.New("MODEL_DRIVER is required"))
	}
	if cfg.Model.APIKey == "" {
		problems = append(problems, errors.New("MODEL_API_KEY is required"))
	}
	if cfg.Model.Name == "" {
		problems = append(problems, errors.New("MODEL_NAME is required"))
	}
	if cfg.Model.MaxTokens < 1 {
		problems = append(problems, errors.New("MODEL_MAX_TOKENS must be positive"))
	}
	if cfg.Model.Timeout <= 0 {
		problems = append(problems, errors.New("MODEL_TIMEOUT must be positive"))
	}
	if cfg.Runtime.MaxIterations < 1 {
		problems = append(problems, errors.New("AGENT_MAX_ITERATIONS must be positive"))
	}
	if cfg.MCP.Timeout <= 0 {
		problems = append(problems, errors.New("MCP_TIMEOUT must be positive"))
	}
	if err := validateURL("MODEL_BASE_URL", cfg.Model.BaseURL); err != nil {
		problems = append(problems, err)
	}
	if err := validateURL("WEB_ORIGIN", cfg.Web.Origin); err != nil {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(env(key, fallback.String()))
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	return parsed, nil
}

func intEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(env(key, strconv.Itoa(fallback)))
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func boolEnv(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(env(key, strconv.FormatBool(fallback)))
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return parsed, nil
}

func validateURL(name, value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", name)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", name)
	}
	return nil
}

func defaultBaseURL(driver string) string {
	if driver == "deepseek" {
		return "https://api.deepseek.com"
	}
	return ""
}

func defaultModelName(driver string) string {
	if driver == "deepseek" {
		return "deepseek-v4-flash"
	}
	return ""
}
