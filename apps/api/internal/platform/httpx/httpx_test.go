package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %q", rec.Body.String())
	}
	return m
}

func withID(h http.Handler) http.Handler { return WithRequestID(slog.New(slog.DiscardHandler))(h) }

func TestEnvelopes(t *testing.T) {
	rec := httptest.NewRecorder()
	withID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, r, http.StatusCreated, map[string]int{"n": 1})
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := decodeBody(t, rec)
	if rec.Code != 201 || body["data"].(map[string]any)["n"] != float64(1) || body["meta"].(map[string]any)["requestId"] == "" {
		t.Fatalf("success envelope: %d %v", rec.Code, body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("API responses must not be cached")
	}

	rec = httptest.NewRecorder()
	cursor := "next"
	withID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WritePage(w, r, []int{1}, &cursor)
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if decodeBody(t, rec)["meta"].(map[string]any)["nextCursor"] != "next" {
		t.Fatal("page meta")
	}
	rec = httptest.NewRecorder()
	withID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { WritePage(w, r, []int{}, nil) })).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if m := decodeBody(t, rec)["meta"].(map[string]any); m["nextCursor"] != nil || !strings.Contains(rec.Body.String(), `"nextCursor":null`) {
		t.Fatal("last page must have nextCursor: null")
	}
}

func TestWriteErrorHidesInternalCauses(t *testing.T) {
	rec := httptest.NewRecorder()
	withID(Handler(func(http.ResponseWriter, *http.Request) error {
		return errors.New("pq: password authentication failed for user kubedojo")
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := decodeBody(t, rec)
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("leak: %s", rec.Body.String())
	}
	e := body["error"].(map[string]any)
	if e["code"] != "INTERNAL" || e["details"] == nil {
		t.Fatalf("error envelope %v", body)
	}
}

func TestRateLimitedSetsRetryAfter(t *testing.T) {
	rec := httptest.NewRecorder()
	withID(Handler(func(http.ResponseWriter, *http.Request) error { return RateLimited(7) })).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 429 || rec.Header().Get("Retry-After") != "7" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestDecodeJSON(t *testing.T) {
	type req struct {
		DisplayName string `json:"displayName"`
	}
	cases := []struct {
		name, ctype, body string
		status            int
		field             string
	}{
		{"valid", "application/json", `{"displayName":"Ada"}`, 0, ""},
		{"charset param ok", "application/json; charset=utf-8", `{"displayName":"Ada"}`, 0, ""},
		{"role smuggling", "application/json", `{"displayName":"Ada","role":"admin"}`, 400, "role"},
		{"xp smuggling", "application/json", `{"xp":9999}`, 400, "xp"},
		{"wrong type", "application/json", `{"displayName":5}`, 400, "displayName"},
		{"form encoded", "application/x-www-form-urlencoded", `displayName=Ada`, 415, ""},
		{"missing content type", "", `{}`, 415, ""},
		{"invalid json", "application/json", `{"displayName":`, 400, ""},
		{"trailing data", "application/json", `{"displayName":"a"}{"x":1}`, 400, ""},
		{"empty body", "application/json", ``, 400, ""},
		{"too large", "application/json", `{"displayName":"` + strings.Repeat("a", 2000) + `"}`, 413, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h := withID(BodyLimit(1024)(Handler(func(w http.ResponseWriter, r *http.Request) error {
				var v req
				if err := DecodeJSON(r, &v); err != nil {
					return err
				}
				WriteJSON(w, r, 200, v)
				return nil
			})))
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			r.ContentLength = -1 // force the streaming limit path, not just the header check
			if tc.ctype != "" {
				r.Header.Set("Content-Type", tc.ctype)
			}
			h.ServeHTTP(rec, r)
			want := tc.status
			if want == 0 {
				want = 200
			}
			if rec.Code != want {
				t.Fatalf("status %d, want %d: %s", rec.Code, want, rec.Body.String())
			}
			if tc.field != "" {
				d := decodeBody(t, rec)["error"].(map[string]any)["details"].([]any)
				if len(d) != 1 || d[0].(map[string]any)["field"] != tc.field {
					t.Fatalf("details %v", d)
				}
			}
		})
	}
}

func TestBodyLimitRejectsDeclaredLength(t *testing.T) {
	rec := httptest.NewRecorder()
	called := false
	withID(BodyLimit(10)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))).
		ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(make([]byte, 11))))
	if rec.Code != 413 || called {
		t.Fatalf("status %d called %v", rec.Code, called)
	}
}

func TestRequestID(t *testing.T) {
	var seen string
	h := withID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = RequestID(r.Context()) }))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if len(seen) != 36 || rec.Header().Get(RequestIDHeader) != seen {
		t.Fatalf("generated id %q / header %q", seen, rec.Header().Get(RequestIDHeader))
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(RequestIDHeader, "bff-req-0123456789")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if seen != "bff-req-0123456789" {
		t.Fatalf("incoming id not accepted: %q", seen)
	}

	for _, bad := range []string{"short", "has spaces in it ok", strings.Repeat("a", 65), "inject\nheader-xx"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set(RequestIDHeader, bad)
		h.ServeHTTP(httptest.NewRecorder(), r)
		if seen == bad {
			t.Fatalf("malformed id %q accepted", bad)
		}
	}
}

func TestRecover(t *testing.T) {
	var logs bytes.Buffer
	h := WithRequestID(slog.New(slog.NewJSONHandler(&logs, nil)))(Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret-internal-state")
	})))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "secret-internal-state") || strings.Contains(rec.Body.String(), "goroutine") {
		t.Fatalf("panic leaked: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "secret-internal-state") || !strings.Contains(logs.String(), "request_id") {
		t.Fatal("panic must be logged with the request id")
	}
}

func TestDeadline(t *testing.T) {
	var deadline time.Time
	var ok bool
	Deadline(time.Second)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		deadline, ok = r.Context().Deadline()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !ok || time.Until(deadline) > time.Second {
		t.Fatal("deadline not applied")
	}
}

func TestAccessLogOmitsSensitiveRequestData(t *testing.T) {
	var logs bytes.Buffer
	h := WithRequestID(slog.New(slog.NewJSONHandler(&logs, nil)))(AccessLog(func(*http.Request) string { return "/api/v1/users/me" })(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users/me?token=abc123", strings.NewReader(`{"password":"hunter2"}`))
	r.Header.Set("Authorization", "Bearer eyJsecret")
	r.Header.Set("Cookie", "session=xyz")
	h.ServeHTTP(httptest.NewRecorder(), r)
	out := logs.String()
	for _, leaked := range []string{"abc123", "hunter2", "eyJsecret", "session=xyz"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("access log leaked %q: %s", leaked, out)
		}
	}
	if !strings.Contains(out, `"status":204`) || !strings.Contains(out, `"route":"/api/v1/users/me"`) {
		t.Fatalf("access log: %s", out)
	}
}

func TestCursors(t *testing.T) {
	c := NewCursors([]byte("0123456789abcdef0123456789abcdef"))
	type pos struct {
		Seq int    `json:"seq"`
		ID  string `json:"id"`
	}
	cur, err := c.Encode(pos{Seq: 42, ID: "x"})
	if err != nil {
		t.Fatal(err)
	}
	var got pos
	if err := c.Decode(cur, &got); err != nil || got.Seq != 42 {
		t.Fatalf("round trip: %v %+v", err, got)
	}
	payload, sig, _ := strings.Cut(cur, ".")
	forged, _ := c.Encode(pos{Seq: 1})
	forgedPayload, _, _ := strings.Cut(forged, ".")
	for name, bad := range map[string]string{
		"tampered payload": forgedPayload + "." + sig,
		"tampered sig":     payload + ".AAAAAAAAAAAAAAAAAAAAAA",
		"other key":        mustEncode(t, NewCursors([]byte("another-key-another-key-another-k")), pos{Seq: 42}),
		"no dot":           payload,
		"empty":            "",
		"too long":         strings.Repeat("a", 600),
	} {
		if err := c.Decode(bad, &got); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func mustEncode(t *testing.T, c *Cursors, v any) string {
	t.Helper()
	s, err := c.Encode(v)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestParsePage(t *testing.T) {
	for q, want := range map[string]int{"": 20, "limit=1": 1, "limit=100": 100} {
		p, err := ParsePage(httptest.NewRequest(http.MethodGet, "/?"+q, nil))
		if err != nil || p.Limit != want {
			t.Fatalf("%q: %v %d", q, err, p.Limit)
		}
	}
	for _, q := range []string{"limit=0", "limit=101", "limit=abc", "limit=-1"} {
		if _, err := ParsePage(httptest.NewRequest(http.MethodGet, "/?"+q, nil)); err == nil {
			t.Fatalf("%q accepted", q)
		}
	}
}

func TestLoggerFallsBack(t *testing.T) {
	if Logger(context.Background()) == nil {
		t.Fatal("nil logger")
	}
}
