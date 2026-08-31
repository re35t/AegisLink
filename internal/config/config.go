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
	Server        Server
	Database      Database
	Auth          Auth
	Model         Model
	Curator       Model
	Runtime       AgentRuntime
	MCP           MCP
	Web           Web
	Security      Security
	AgentIndex    AgentIndex
	Encoder       Encoder
	Collaboration Collaboration
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
	ID           string
	Driver       string
	BaseURL      string
	APIKey       string
	Name         string
	Timeout      time.Duration
	MaxTokens    int
	JSONOutput   bool
	ThinkingMode string
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

type Security struct {
	AgentKeyEncryptionKey string
}

type AgentIndex struct {
	BaseURL           string
	RegistrationToken string
	QueryToken        string
	Timeout           time.Duration
}

type Encoder struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

type Collaboration struct {
	PublicBaseURL string
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
	curatorTimeout, err := optionalDurationEnv("CURATOR_MODEL_TIMEOUT", timeout)
	if err != nil {
		return Config{}, err
	}
	curatorMaxTokens, err := optionalIntEnv("CURATOR_MODEL_MAX_TOKENS", min(maxTokens, 2048))
	if err != nil {
		return Config{}, err
	}
	curatorJSONOutput, err := optionalBoolEnv("CURATOR_MODEL_JSON_OUTPUT", true)
	if err != nil {
		return Config{}, err
	}
	indexTimeout, err := durationEnv("AGENT_INDEX_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	encoderTimeout, err := durationEnv("DISCOVERY_ENCODER_TIMEOUT", 30*time.Second)
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
		Curator: Model{
			ID:           nonBlankEnv("CURATOR_MODEL_ID", strings.TrimSpace(env("MODEL_ID", "deepseek-primary"))+"-curator"),
			Driver:       nonBlankEnv("CURATOR_MODEL_DRIVER", driver),
			BaseURL:      nonBlankEnv("CURATOR_MODEL_BASE_URL", strings.TrimSpace(env("MODEL_BASE_URL", defaultBaseURL(driver)))),
			APIKey:       nonBlankEnv("CURATOR_MODEL_API_KEY", strings.TrimSpace(os.Getenv("MODEL_API_KEY"))),
			Name:         nonBlankEnv("CURATOR_MODEL_NAME", strings.TrimSpace(env("MODEL_NAME", defaultModelName(driver)))),
			Timeout:      curatorTimeout,
			MaxTokens:    curatorMaxTokens,
			JSONOutput:   curatorJSONOutput,
			ThinkingMode: strings.ToLower(strings.TrimSpace(env("CURATOR_MODEL_THINKING", "disabled"))),
		},
		Runtime: AgentRuntime{MaxIterations: maxIterations},
		MCP: MCP{
			Timeout: mcpTimeout, AllowPrivateNetworks: mcpAllowPrivateNetworks,
		},
		Web:      Web{Origin: strings.TrimSpace(env("WEB_ORIGIN", "http://127.0.0.1:5173"))},
		Security: Security{AgentKeyEncryptionKey: strings.TrimSpace(os.Getenv("AGENT_KEY_ENCRYPTION_KEY"))},
		AgentIndex: AgentIndex{
			BaseURL:           strings.TrimSpace(os.Getenv("AGENT_INDEX_BASE_URL")),
			RegistrationToken: strings.TrimSpace(os.Getenv("AGENT_INDEX_REGISTRATION_TOKEN")),
			QueryToken:        strings.TrimSpace(os.Getenv("AGENT_INDEX_QUERY_TOKEN")), Timeout: indexTimeout,
		},
		Encoder: Encoder{
			BaseURL: strings.TrimSpace(os.Getenv("DISCOVERY_ENCODER_BASE_URL")),
			APIKey:  strings.TrimSpace(os.Getenv("DISCOVERY_ENCODER_API_KEY")),
			Model:   strings.TrimSpace(os.Getenv("DISCOVERY_ENCODER_MODEL")), Timeout: encoderTimeout,
		},
		Collaboration: Collaboration{PublicBaseURL: strings.TrimRight(strings.TrimSpace(env("A2A_PUBLIC_BASE_URL", "http://127.0.0.1:4321")), "/")},
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) Validate() error {
	curator := cfg.EffectiveCurator()
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
	if curator.ID == "" || curator.Driver == "" || curator.APIKey == "" || curator.Name == "" || curator.MaxTokens < 1 || curator.Timeout <= 0 {
		problems = append(problems, errors.New("CURATOR_MODEL_* must resolve to a complete positive model configuration"))
	}
	if curator.ThinkingMode != "" && curator.ThinkingMode != "enabled" && curator.ThinkingMode != "disabled" {
		problems = append(problems, errors.New("CURATOR_MODEL_THINKING must be enabled, disabled, or empty"))
	}
	if cfg.Runtime.MaxIterations < 1 {
		problems = append(problems, errors.New("AGENT_MAX_ITERATIONS must be positive"))
	}
	if cfg.MCP.Timeout <= 0 {
		problems = append(problems, errors.New("MCP_TIMEOUT must be positive"))
	}
	indexConfigured := cfg.AgentIndex.BaseURL != "" || cfg.AgentIndex.RegistrationToken != "" || cfg.AgentIndex.QueryToken != "" || cfg.Encoder.BaseURL != "" || cfg.Encoder.APIKey != "" || cfg.Encoder.Model != ""
	if indexConfigured {
		if cfg.AgentIndex.BaseURL == "" || len(cfg.AgentIndex.RegistrationToken) < 32 || len(cfg.AgentIndex.QueryToken) < 32 {
			problems = append(problems, errors.New("AGENT_INDEX_* must provide a base URL and registration/query tokens of at least 32 characters"))
		}
		if cfg.AgentIndex.Timeout <= 0 {
			problems = append(problems, errors.New("AGENT_INDEX_TIMEOUT must be positive"))
		}
		if cfg.Encoder.BaseURL == "" || cfg.Encoder.Model == "" || cfg.Encoder.Timeout <= 0 {
			problems = append(problems, errors.New("DISCOVERY_ENCODER_* must provide a base URL, model, and positive timeout; API key is optional"))
		}
		if cfg.AgentIndex.BaseURL != "" {
			if err := validateURL("AGENT_INDEX_BASE_URL", cfg.AgentIndex.BaseURL); err != nil {
				problems = append(problems, err)
			}
		}
		if cfg.Encoder.BaseURL != "" {
			if err := validateURL("DISCOVERY_ENCODER_BASE_URL", cfg.Encoder.BaseURL); err != nil {
				problems = append(problems, err)
			}
		}
	}
	if err := validateURL("MODEL_BASE_URL", cfg.Model.BaseURL); err != nil {
		problems = append(problems, err)
	}
	if err := validateURL("CURATOR_MODEL_BASE_URL", curator.BaseURL); err != nil {
		problems = append(problems, err)
	}
	if err := validateURL("WEB_ORIGIN", cfg.Web.Origin); err != nil {
		problems = append(problems, err)
	}
	if cfg.Collaboration.PublicBaseURL != "" {
		if err := validateURL("A2A_PUBLIC_BASE_URL", cfg.Collaboration.PublicBaseURL); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

func (cfg Config) AgentIndexEnabled() bool {
	return cfg.AgentIndex.BaseURL != ""
}

func (cfg Config) EffectiveCurator() Model {
	if cfg.Curator.ID == "" && cfg.Curator.Driver == "" && cfg.Curator.BaseURL == "" && cfg.Curator.APIKey == "" && cfg.Curator.Name == "" && cfg.Curator.Timeout == 0 && cfg.Curator.MaxTokens == 0 && !cfg.Curator.JSONOutput && cfg.Curator.ThinkingMode == "" {
		return cfg.Model
	}
	return cfg.Curator
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func nonBlankEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(env(key, fallback.String()))
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	return parsed, nil
}

func optionalDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return fallback, nil
	}
	return durationEnv(key, fallback)
}

func intEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(env(key, strconv.Itoa(fallback)))
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func optionalIntEnv(key string, fallback int) (int, error) {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return fallback, nil
	}
	return intEnv(key, fallback)
}

func boolEnv(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(env(key, strconv.FormatBool(fallback)))
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return parsed, nil
}

func optionalBoolEnv(key string, fallback bool) (bool, error) {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return fallback, nil
	}
	return boolEnv(key, fallback)
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
