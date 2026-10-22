// Package config loads and validates process configuration from environment variables.
// Every variable is documented in the repository's .env.example.
package config

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/ratelimit"
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

// API is the configuration of the HTTP API binary: the shared Config plus API-only settings.
type API struct {
	Config
	JWTPublicKeys     map[string]ed25519.PublicKey
	TrustedProxyCIDRs string
	RequestTimeout    time.Duration
	MaxBodyBytes      int64
}

// LoadAPI loads and validates the API's configuration, reporting every problem at once.
func LoadAPI(getenv func(string) string) (API, error) {
	base, baseErr := Load(getenv)
	errs := []error{baseErr}
	cfg := API{Config: base}

	keys, err := auth.ParsePublicKeys(getenv("API_JWT_PUBLIC_KEYS"))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.JWTPublicKeys = keys

	cfg.TrustedProxyCIDRs = strings.TrimSpace(getenv("TRUSTED_PROXY_CIDRS"))
	if _, err := ratelimit.NewClientIPResolver(cfg.TrustedProxyCIDRs); err != nil {
		errs = append(errs, err)
	}

	cfg.RequestTimeout = 5 * time.Second
	if v := strings.TrimSpace(getenv("REQUEST_TIMEOUT")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Second || d > time.Minute {
			errs = append(errs, errors.New("REQUEST_TIMEOUT must be a duration between 1s and 1m"))
		}
		cfg.RequestTimeout = d
	}

	cfg.MaxBodyBytes = 128 << 10
	if v := strings.TrimSpace(getenv("MAX_BODY_BYTES")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1024 || n > 8<<20 {
			errs = append(errs, errors.New("MAX_BODY_BYTES must be an integer between 1024 and 8388608"))
		}
		cfg.MaxBodyBytes = n
	}
	return cfg, errors.Join(errs...)
}
