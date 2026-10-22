package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
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

func apiEnv(t *testing.T) map[string]string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(pub)
	e := validEnv()
	e["API_JWT_PUBLIC_KEYS"] = "dev-1:" + base64.StdEncoding.EncodeToString(der)
	return e
}

func TestLoadAPI(t *testing.T) {
	cfg, err := LoadAPI(env(apiEnv(t)))
	if err != nil {
		t.Fatalf("LoadAPI: %v", err)
	}
	if len(cfg.JWTPublicKeys) != 1 || cfg.RequestTimeout != 5*time.Second || cfg.MaxBodyBytes != 128<<10 {
		t.Fatalf("defaults: %+v", cfg)
	}
}

func TestLoadAPIReportsProblems(t *testing.T) {
	e := apiEnv(t)
	delete(e, "API_JWT_PUBLIC_KEYS")
	e["TRUSTED_PROXY_CIDRS"] = "10.0.0.0/8,banana"
	e["REQUEST_TIMEOUT"] = "500ms"
	e["MAX_BODY_BYTES"] = "10"
	_, err := LoadAPI(env(e))
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"API_JWT_PUBLIC_KEYS", "TRUSTED_PROXY_CIDRS", "REQUEST_TIMEOUT", "MAX_BODY_BYTES"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q not reported in %v", want, err)
		}
	}
}
