package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth/authtest"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

type fakeResolver map[string]Principal

func (f fakeResolver) Resolve(_ context.Context, provider, subject string) (Principal, error) {
	if subject == "500" {
		return Principal{}, errors.New("database down")
	}
	p, ok := f[provider+":"+subject]
	if !ok {
		return Principal{}, ErrIdentityNotLinked
	}
	return p, nil
}

var (
	learnerID = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	adminID   = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	resolver  = fakeResolver{
		"github:100": {UserID: learnerID, Role: RoleLearner, Provider: "github", ProviderSubject: "100"},
		"github:200": {UserID: adminID, Role: RoleAdmin, Provider: "github", ProviderSubject: "200"},
	}
)

func serve(t *testing.T, s authtest.Signer, h http.Handler, header http.Header) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	chain := httpx.WithRequestID(slog.New(slog.DiscardHandler))(Authenticate(NewVerifier(s.Keys(), fixedNow), resolver)(h))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func echoPrincipal() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := PrincipalFrom(r.Context())
		_, _ = io.WriteString(w, p.UserID.String()+" "+string(p.Role))
	})
}

func TestAuthenticate(t *testing.T) {
	s := authtest.NewSigner(t, "dev-1")
	bearer := func(tok string) http.Header { return http.Header{"Authorization": {"Bearer " + tok}} }

	t.Run("valid assertion sets the principal from the resolver", func(t *testing.T) {
		rec, _ := serve(t, s, echoPrincipal(), bearer(s.Token(t, "100", now)))
		if rec.Code != 200 || rec.Body.String() != learnerID.String()+" learner" {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("identity and role headers are ignored", func(t *testing.T) {
		h := bearer(s.Token(t, "100", now))
		h.Set("X-User-Id", adminID.String())
		h.Set("X-Role", "admin")
		h.Set("X-Forwarded-User", "github:200")
		rec, _ := serve(t, s, echoPrincipal(), h)
		if rec.Body.String() != learnerID.String()+" learner" {
			t.Fatalf("headers influenced identity: %s", rec.Body.String())
		}
	})

	unauth := map[string]http.Header{
		"no header":             nil,
		"basic scheme":          {"Authorization": {"Basic dXNlcjpwYXNz"}},
		"empty bearer":          {"Authorization": {"Bearer "}},
		"invalid token":         bearer("abc.def.ghi"),
		"raw GitHub token":      bearer("gho_16C7e42F292c6912E7710c838347Ae178B4a"),
		"unlinked identity":     bearer(s.Token(t, "999", now)),
		"identity headers only": {"X-User-Id": {learnerID.String()}, "X-Role": {"admin"}},
	}
	for name, h := range unauth {
		t.Run(name+" is 401", func(t *testing.T) {
			rec, body := serve(t, s, echoPrincipal(), h)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status %d", rec.Code)
			}
			errObj, _ := body["error"].(map[string]any)
			if errObj["code"] != "UNAUTHENTICATED" || errObj["message"] != "Authentication required." {
				t.Fatalf("body %v", body)
			}
			if meta, _ := body["meta"].(map[string]any); meta["requestId"] == "" {
				t.Fatal("missing requestId")
			}
		})
	}

	t.Run("resolver failure is 500 without details", func(t *testing.T) {
		rec, body := serve(t, s, echoPrincipal(), bearer(s.Token(t, "500", now)))
		if rec.Code != 500 || rec.Body.String() == "" {
			t.Fatalf("status %d", rec.Code)
		}
		if msg := body["error"].(map[string]any)["message"]; msg != "Something went wrong." {
			t.Fatalf("leaked message %v", msg)
		}
	})
}

func TestRequireRole(t *testing.T) {
	s := authtest.NewSigner(t, "dev-1")
	adminOnly := RequireRole(RoleAdmin)(echoPrincipal())
	cases := []struct {
		sub  string
		want int
	}{{"100", http.StatusForbidden}, {"200", http.StatusOK}}
	for _, tc := range cases {
		rec, body := serve(t, s, adminOnly, http.Header{"Authorization": {"Bearer " + s.Token(t, tc.sub, now)}})
		if rec.Code != tc.want {
			t.Fatalf("sub %s: status %d", tc.sub, rec.Code)
		}
		if tc.want == http.StatusForbidden && body["error"].(map[string]any)["code"] != "FORBIDDEN" {
			t.Fatalf("body %v", body)
		}
	}
	// Without Authenticate in front, RequireRole must not let anything through.
	rec := httptest.NewRecorder()
	RequireRole(RoleAdmin)(echoPrincipal()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}
