package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func ok() Pinger   { return PingFunc(func(context.Context) error { return nil }) }
func fail() Pinger { return PingFunc(func(context.Context) error { return errors.New("down") }) }

func TestLive(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler{}.Live(rec, httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] == "" || body["commit"] == "" || len(body) != 3 {
		t.Fatalf("unexpected body %v", body)
	}
}

func TestReady(t *testing.T) {
	cases := []struct {
		name     string
		h        Handler
		code     int
		postgres string
		redis    string
	}{
		{"all ok", Handler{Postgres: ok(), Redis: ok()}, 200, "ok", "ok"},
		{"postgres down", Handler{Postgres: fail(), Redis: ok()}, 503, "error", "ok"},
		{"redis down", Handler{Postgres: ok(), Redis: fail()}, 503, "ok", "error"},
		{"not configured", Handler{}, 503, "error", "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.h.Ready(rec, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
			if rec.Code != tc.code {
				t.Fatalf("status %d, want %d", rec.Code, tc.code)
			}
			var body readinessBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Checks["postgres"] != tc.postgres || body.Checks["redis"] != tc.redis {
				t.Fatalf("checks %v", body.Checks)
			}
		})
	}
}

func TestReadyRespectsTimeout(t *testing.T) {
	slow := PingFunc(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	h := Handler{Postgres: slow, Redis: ok(), Timeout: 50 * time.Millisecond}
	start := time.Now()
	rec := httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable || time.Since(start) > time.Second {
		t.Fatalf("status %d after %v", rec.Code, time.Since(start))
	}
}
