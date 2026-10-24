package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func get(h http.Handler, path string, header http.Header) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range header {
		r.Header[k] = v
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestRouterSemantics(t *testing.T) {
	h, s := testRouter(t, true)
	bearer := http.Header{"Authorization": {"Bearer " + s.Token(t, "100", time.Now())}}

	cases := []struct {
		name   string
		method string
		path   string
		header http.Header
		status int
		code   string
	}{
		{"unknown route is a JSON 404", http.MethodGet, "/api/v1/nope", nil, 404, "NOT_FOUND"},
		{"unknown route outside /api", http.MethodGet, "/", nil, 404, "NOT_FOUND"},
		{"wrong method is a JSON 405", http.MethodDelete, "/api/v1/users/me", bearer, 405, "VALIDATION_FAILED"},
		{"protected route without identity", http.MethodGet, "/api/v1/users/me", nil, 401, "UNAUTHENTICATED"},
		{"protected route with valid identity", http.MethodGet, "/api/v1/users/me", bearer, 200, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, nil)
			for k, v := range tc.header {
				r.Header[k] = v
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			// Every response carries the request ID and security headers, errors included.
			if rec.Header().Get("X-Request-ID") == "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("missing standard headers: %v", rec.Header())
			}
			if tc.code != "" {
				var body struct {
					Error struct{ Code string } `json:"error"`
					Meta  struct{ RequestID string }
				}
				_ = json.Unmarshal(rec.Body.Bytes(), &body)
				if body.Error.Code != tc.code || body.Meta.RequestID != rec.Header().Get("X-Request-ID") {
					t.Fatalf("body %s", rec.Body.String())
				}
			}
		})
	}
}

func TestUsersMeUsesOnlyThePrincipal(t *testing.T) {
	h, s := testRouter(t, true)
	header := http.Header{
		"Authorization": {"Bearer " + s.Token(t, "100", time.Now())},
		"X-User-Id":     {"44444444-4444-4444-8444-444444444444"},
	}
	rec := get(h, "/api/v1/users/me?userId=44444444-4444-4444-8444-444444444444", header)
	var body struct {
		Data struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Data.ID != testUserID.String() || body.Data.Role != "learner" {
		t.Fatalf("identity taken from request: %s", rec.Body.String())
	}
}

func TestPerUserRateLimitAppliesAfterAuth(t *testing.T) {
	h, s := testRouter(t, true)
	rec := get(h, "/api/v1/users/me", http.Header{"Authorization": {"Bearer " + s.Token(t, "429", time.Now())}})
	if rec.Code != 429 || rec.Header().Get("Retry-After") != "30" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
}
