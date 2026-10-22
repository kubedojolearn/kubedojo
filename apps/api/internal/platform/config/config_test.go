package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://kubedojo:pw@localhost:5432/kubedojo?sslmode=disable",
		"REDIS_URL":    "redis://localhost:6379/0",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(validEnv()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Env != EnvDevelopment || cfg.HTTPAddr != ":8080" || cfg.LogLevel != "info" || cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadReportsAllProblemsWithoutLeakingValues(t *testing.T) {
	_, err := Load(env(map[string]string{
		"APP_ENV":          "staging",
		"LOG_LEVEL":        "verbose",
		"DATABASE_URL":     "mysql://root:supersecret@db/x",
		"SHUTDOWN_TIMEOUT": "forever",
	}))
	if err == nil {
		t.Fatal("expected errors")
	}
	msg := err.Error()
	for _, want := range []string{"APP_ENV", "LOG_LEVEL", "DATABASE_URL must use scheme", "REDIS_URL is required", "SHUTDOWN_TIMEOUT"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not mention %q", msg, want)
		}
	}
	if strings.Contains(msg, "supersecret") {
		t.Fatalf("error message leaks a credential: %q", msg)
	}
}

func TestProductionRequiresRedisTLS(t *testing.T) {
	e := validEnv()
	e["APP_ENV"] = EnvProduction
	if _, err := Load(env(e)); err == nil || !strings.Contains(err.Error(), "rediss://") {
		t.Fatalf("expected TLS requirement, got %v", err)
	}
	e["REDIS_URL"] = "rediss://cache.internal:6380/0"
	if _, err := Load(env(e)); err != nil {
		t.Fatalf("Load: %v", err)
	}
}
