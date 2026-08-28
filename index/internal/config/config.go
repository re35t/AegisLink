package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   Server
	Database Database
	Security Security
}

type Server struct {
	Address         string
	ShutdownTimeout time.Duration
}

type Database struct {
	URL string
}

type Security struct {
	RegistrationToken string
	QueryToken        string
}

func Load() (Config, error) {
	_ = godotenv.Load()
	shutdownTimeout, err := durationEnv("INDEX_SERVER_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Server: Server{
			Address:         strings.TrimSpace(env("INDEX_SERVER_ADDRESS", "127.0.0.1:4331")),
			ShutdownTimeout: shutdownTimeout,
		},
		Database: Database{URL: strings.TrimSpace(os.Getenv("INDEX_DATABASE_URL"))},
		Security: Security{
			RegistrationToken: strings.TrimSpace(os.Getenv("INDEX_REGISTRATION_TOKEN")),
			QueryToken:        strings.TrimSpace(os.Getenv("INDEX_QUERY_TOKEN")),
		},
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) Validate() error {
	var problems []error
	if strings.TrimSpace(cfg.Server.Address) == "" {
		problems = append(problems, errors.New("INDEX_SERVER_ADDRESS is required"))
	}
	if cfg.Server.ShutdownTimeout <= 0 {
		problems = append(problems, errors.New("INDEX_SERVER_SHUTDOWN_TIMEOUT must be positive"))
	}
	if err := validateDatabaseURL(cfg.Database.URL); err != nil {
		problems = append(problems, err)
	}
	if utf8.RuneCountInString(cfg.Security.RegistrationToken) < 32 {
		problems = append(problems, errors.New("INDEX_REGISTRATION_TOKEN must contain at least 32 characters"))
	}
	if utf8.RuneCountInString(cfg.Security.QueryToken) < 32 {
		problems = append(problems, errors.New("INDEX_QUERY_TOKEN must contain at least 32 characters"))
	}
	return errors.Join(problems...)
}

func validateDatabaseURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("INDEX_DATABASE_URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" || strings.Trim(parsed.Path, "/") == "" {
		return errors.New("INDEX_DATABASE_URL must be a PostgreSQL URL with a database name")
	}
	return nil
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
