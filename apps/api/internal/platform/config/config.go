// Package config loads and validates process configuration from environment variables.
// Every variable is documented in the repository's .env.example.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Environment names. Production enables the strictest checks.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Config is the validated configuration shared by the api and worker binaries.
type Config struct {
	Env             string
	HTTPAddr        string
	DatabaseURL     string
	RedisURL        string
	LogLevel        string
	ShutdownTimeout time.Duration
}

// Load reads configuration using getenv (os.Getenv in production, a map in tests) and returns
// every validation problem at once so operators can fix them in one pass.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		Env:         get("APP_ENV", EnvDevelopment),
		HTTPAddr:    get("API_LISTEN_ADDR", ":8080"),
		DatabaseURL: get("DATABASE_URL", ""),
		RedisURL:    get("REDIS_URL", ""),
		LogLevel:    strings.ToLower(get("LOG_LEVEL", "info")),
	}

	switch cfg.Env {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be development, test or production, got %q", cfg.Env))
	}

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error, got %q", cfg.LogLevel))
	}

	if err := validateURL("DATABASE_URL", cfg.DatabaseURL, "postgres", "postgresql"); err != nil {
		errs = append(errs, err)
	}
	if err := validateURL("REDIS_URL", cfg.RedisURL, "redis", "rediss"); err != nil {
		errs = append(errs, err)
	}

	timeout, err := time.ParseDuration(get("SHUTDOWN_TIMEOUT", "15s"))
	if err != nil || timeout <= 0 || timeout > 5*time.Minute {
		errs = append(errs, errors.New("SHUTDOWN_TIMEOUT must be a duration between 0 and 5m, e.g. 15s"))
	}
	cfg.ShutdownTimeout = timeout

	if cfg.Env == EnvProduction && strings.HasPrefix(cfg.RedisURL, "redis://") {
		errs = append(errs, errors.New("REDIS_URL must use rediss:// (TLS) in production"))
	}

	return cfg, errors.Join(errs...)
}

// validateURL checks presence and scheme without ever echoing the value, which may hold credentials.
func validateURL(key, raw string, schemes ...string) error {
	if raw == "" {
		return fmt.Errorf("%s is required", key)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL", key)
	}
	for _, s := range schemes {
		if u.Scheme == s {
			return nil
		}
	}
	return fmt.Errorf("%s must use scheme %s", key, strings.Join(schemes, " or "))
}
