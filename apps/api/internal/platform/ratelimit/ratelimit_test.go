package ratelimit

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

type memLimiter struct {
	counts map[string]int
	err    error
}

func (m *memLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (Decision, error) {
	if m.err != nil {
		return Decision{}, m.err
	}
	m.counts[key]++
	if m.counts[key] > limit {
		return Decision{Allowed: false, RetryAfter: window}, nil
	}
	return Decision{Allowed: true, Remaining: limit - m.counts[key]}, nil
}

func ok() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
}

func run(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	httpx.WithRequestID(slog.New(slog.DiscardHandler))(h).ServeHTTP(rec, r)
	return rec
}

func TestPerUserLimitsEachUserSeparately(t *testing.T) {
	l := &memLimiter{counts: map[string]int{}}
	h := PerUser(l, Rule{Name: "t", Limit: 2, Window: time.Minute})(ok())
	reqFor := func(id uuid.UUID) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		return r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{UserID: id}))
	}
	a, b := uuid.New(), uuid.New()
	for i := 0; i < 2; i++ {
		if c := run(h, reqFor(a)).Code; c != 204 {
			t.Fatalf("request %d: %d", i, c)
		}
	}
	rec := run(h, reqFor(a))
	if rec.Code != 429 || rec.Header().Get("Retry-After") != "60" {
		t.Fatalf("third request: %d retry %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if c := run(h, reqFor(b)).Code; c != 204 {
		t.Fatalf("other user throttled: %d", c)
	}
	if c := run(h, httptest.NewRequest(http.MethodGet, "/", nil)).Code; c != 401 {
		t.Fatalf("no principal: %d", c)
	}
}

func TestLimiterOutage(t *testing.T) {
	down := &memLimiter{err: errors.New("redis: connection refused")}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if c := run(PerIP(down, Rule{Name: "open", Limit: 1, Window: time.Minute}, &ClientIPResolver{})(ok()), r).Code; c != 204 {
		t.Fatalf("fail-open rule: %d", c)
	}
	if c := run(PerIP(down, Rule{Name: "closed", Limit: 1, Window: time.Minute, FailClosed: true}, &ClientIPResolver{})(ok()), r).Code; c != 503 {
		t.Fatalf("fail-closed rule: %d", c)
	}
}

func TestClientIP(t *testing.T) {
	ips, err := NewClientIPResolver("10.0.0.0/8, 172.16.0.0/12")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, remote, xff, want string
	}{
		{"direct client, header ignored", "203.0.113.9:5000", "1.2.3.4", "203.0.113.9"},
		{"via trusted edge", "10.1.2.3:443", "198.51.100.7", "198.51.100.7"},
		{"spoofed leftmost hop ignored", "10.1.2.3:443", "1.1.1.1, 198.51.100.7", "198.51.100.7"},
		{"chain of trusted proxies", "10.1.2.3:443", "198.51.100.7, 172.20.0.5", "198.51.100.7"},
		{"trusted edge without header", "10.1.2.3:443", "", "10.1.2.3"},
		{"garbage header", "10.1.2.3:443", "not-an-ip", "10.1.2.3"},
		{"ipv6 client", "[2001:db8::1]:443", "", "2001:db8::1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remote
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := ips.ClientIP(r); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
	if _, err := NewClientIPResolver("not-a-cidr"); err == nil {
		t.Fatal("invalid CIDR accepted")
	}
}
