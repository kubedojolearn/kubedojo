package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth/authtest"
)

func provisionToken(t *testing.T, s authtest.Signer, subject string) string {
	c := authtest.Claims(subject, time.Now())
	c["aud"] = "kubedojo-api-provision"
	c["login"] = "octocat"
	c["name"] = "The Octocat"
	c["avatar_url"] = "https://avatars.githubusercontent.com/u/1"
	return s.Sign(t, c)
}

func do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) (string, []map[string]any) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string           `json:"code"`
			Details []map[string]any `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Error.Code, body.Error.Details
}

func TestProvisionAudienceSeparation(t *testing.T) {
	h, s := testRouter(t, true)
	if rec := do(h, http.MethodPost, "/api/v1/auth/provision", provisionToken(t, s, "583231"), ""); rec.Code != 200 {
		t.Fatalf("provisioning assertion: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodPost, "/api/v1/auth/provision", s.Token(t, "100", time.Now()), ""); rec.Code != 401 {
		t.Fatalf("request assertion accepted for provisioning: %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/v1/users/me", provisionToken(t, s, "100"), ""); rec.Code != 401 {
		t.Fatalf("provisioning assertion accepted for API: %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/v1/auth/provision", "", ""); rec.Code != 401 {
		t.Fatalf("no token: %d", rec.Code)
	}
}

func TestUpdateMe(t *testing.T) {
	h, s := testRouter(t, true)
	tok := s.Token(t, "100", time.Now())
	cases := []struct {
		name, body string
		status     int
		field      string
	}{
		{"display name", `{"displayName":"Ada"}`, 200, ""},
		{"settings", `{"settings":{"theme":"dark","timeZone":"Europe/London","profilePublic":true}}`, 200, ""},
		{"onboarding", `{"onboarding":{"experienceLevel":"new","learningGoal":"fundamentals"},"completeOnboarding":true}`, 200, ""},
		{"empty", `{}`, 400, ""},
		{"role smuggling", `{"displayName":"x","role":"admin"}`, 400, "role"},
		{"xp smuggling", `{"settings":{"totalXp":5}}`, 400, "totalXp"},
		{"bad handle", `{"handle":"-nope-"}`, 400, "handle"},
		{"reserved handle", `{"handle":"Admin"}`, 400, "handle"},
		{"taken handle", `{"handle":"taken"}`, 409, "handle"},
		{"bad time zone", `{"settings":{"timeZone":"Mars/Olympus"}}`, 400, "settings.timeZone"},
		{"bad theme", `{"settings":{"theme":"neon"}}`, 400, "settings.theme"},
		{"blank name", "{\"displayName\":\"  \u200b \"}", 400, "displayName"},
		{"onboarding false", `{"completeOnboarding":false}`, 400, "completeOnboarding"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(h, http.MethodPatch, "/api/v1/users/me", tok, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.field != "" {
				_, details := errorCode(t, rec)
				if len(details) == 0 || details[0]["field"] != tc.field {
					t.Fatalf("details %v", details)
				}
			}
		})
	}
	if rec := do(h, http.MethodPatch, "/api/v1/users/me", "", `{"displayName":"x"}`); rec.Code != 401 {
		t.Fatalf("unauthenticated PATCH: %d", rec.Code)
	}
}
