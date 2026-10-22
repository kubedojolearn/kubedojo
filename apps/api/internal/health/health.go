// Package health serves the liveness and readiness probes.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/buildinfo"
)

// Pinger is satisfied by *pgxpool.Pool (via PingFunc) and the Redis client adapter.
type Pinger interface {
	Ping(ctx context.Context) error
}

// PingFunc adapts a function to Pinger.
type PingFunc func(ctx context.Context) error

// Ping calls f.
func (f PingFunc) Ping(ctx context.Context) error { return f(ctx) }

// Handler serves /healthz and /readyz.
type Handler struct {
	Postgres Pinger
	Redis    Pinger
	Timeout  time.Duration
}

type healthBody struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

type readinessBody struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Live reports that the process is serving. It checks no dependencies.
func (h Handler) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthBody{Status: "ok", Version: buildinfo.Version, Commit: buildinfo.Commit})
}

// Ready checks PostgreSQL and Redis concurrently and returns 503 if either fails.
// Failure details are logged by the caller's middleware, never returned, to avoid leaking topology.
func (h Handler) Ready(w http.ResponseWriter, r *http.Request) {
	timeout := h.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	checks := map[string]Pinger{"postgres": h.Postgres, "redis": h.Redis}
	results := make(map[string]string, len(checks))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, p := range checks {
		wg.Go(func() {
			status := "ok"
			if p == nil || p.Ping(ctx) != nil {
				status = "error"
			}
			mu.Lock()
			results[name] = status
			mu.Unlock()
		})
	}
	wg.Wait()

	body := readinessBody{Status: "ready", Checks: results}
	code := http.StatusOK
	for _, s := range results {
		if s != "ok" {
			body.Status = "unavailable"
			code = http.StatusServiceUnavailable
		}
	}
	writeJSON(w, code, body)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
