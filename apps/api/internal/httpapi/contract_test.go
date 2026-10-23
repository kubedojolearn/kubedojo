package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
)

// The OpenAPI document is the contract (docs/api/openapi.yaml). These tests fail when an
// implemented route is missing from it or answers with a shape it doesn't describe.

const specPath = "../../../../docs/api/openapi.yaml"

func loadValidator(t *testing.T) (validator.Validator, []byte) {
	t.Helper()
	spec, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	v, errs := validator.NewValidator(doc)
	if len(errs) > 0 {
		t.Fatalf("build validator: %v", errs)
	}
	if ok, verrs := v.ValidateDocument(); !ok {
		for _, e := range verrs {
			t.Errorf("spec invalid: %s: %s", e.Message, e.Reason)
		}
		t.FailNow()
	}
	return v, spec
}

func TestEveryRouteIsInTheContract(t *testing.T) {
	_, spec := loadValidator(t)
	h, _ := testRouter(t, true)
	routes, ok := h.(chi.Routes)
	if !ok {
		t.Fatal("router is not chi.Routes")
	}
	param := regexp.MustCompile(`\{[^}]+\}`)
	specText := string(spec)
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// Match path keys loosely on parameter names; the method must be listed under the path.
		pattern := regexp.QuoteMeta(param.ReplaceAllString(route, "{X}"))
		pattern = strings.ReplaceAll(pattern, `\{X\}`, `\{[^}]+\}`)
		re := regexp.MustCompile(`(?m)^  ` + pattern + `:\n((?:    .*\n|\n)*)`)
		m := re.FindStringSubmatch(specText)
		if m == nil {
			t.Errorf("%s %s is not in openapi.yaml", method, route)
			return nil
		}
		if !strings.Contains(m[1], "\n    "+strings.ToLower(method)+":") && !strings.HasPrefix(m[1], "    "+strings.ToLower(method)+":") {
			t.Errorf("%s %s: method not documented", method, route)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestResponsesMatchTheContract(t *testing.T) {
	v, _ := loadValidator(t)
	readyRouter, s := testRouter(t, true)
	downRouter, _ := testRouter(t, false)
	token := s.Token(t, "100", time.Now())

	cases := []struct {
		name    string
		handler http.Handler
		method  string
		path    string
		auth    string
		body    string
		status  int
	}{
		{"catalog courses", readyRouter, "", "/api/v1/catalog/courses", "", "", 200},
		{"catalog course", readyRouter, "", "/api/v1/catalog/courses/workloads", "", "", 200},
		{"catalog course missing", readyRouter, "", "/api/v1/catalog/courses/nope", "", "", 404},
		{"catalog lesson", readyRouter, "", "/api/v1/catalog/courses/workloads/lessons/scaling-deployments", "", "", 200},
		{"catalog lesson missing", readyRouter, "", "/api/v1/catalog/courses/workloads/lessons/launch-a-pod", "", "", 404},
		{"progress", readyRouter, "", "/api/v1/users/me/progress?limit=1", token, "", 200},
		{"progress bad cursor", readyRouter, "", "/api/v1/users/me/progress?cursor=forged.sig", token, "", 400},
		{"progress unauthenticated", readyRouter, "", "/api/v1/users/me/progress", "", "", 401},
		{"lesson opened", readyRouter, http.MethodPost, "/api/v1/users/me/progress/opened", token, `{"courseSlug":"workloads","lessonSlug":"scaling-deployments"}`, 204},
		{"lesson opened unpublished", readyRouter, http.MethodPost, "/api/v1/users/me/progress/opened", token, `{"courseSlug":"workloads","lessonSlug":"launch-a-pod"}`, 404},
		{"achievements", readyRouter, "", "/api/v1/users/me/achievements", token, "", 200},
		{"activity", readyRouter, "", "/api/v1/users/me/activity", token, "", 200},
		{"public profile", readyRouter, "", "/api/v1/profiles/public-learner", "", "", 200},
		{"private profile", readyRouter, "", "/api/v1/profiles/private-learner", "", "", 404},
		{"provision", readyRouter, http.MethodPost, "/api/v1/auth/provision", provisionToken(t, s, "583231"), "", 200},
		{"provision unauthenticated", readyRouter, http.MethodPost, "/api/v1/auth/provision", "", "", 401},
		{"update me", readyRouter, http.MethodPatch, "/api/v1/users/me", token, `{"displayName":"Ada","settings":{"theme":"dark"}}`, 200},
		{"update me invalid", readyRouter, http.MethodPatch, "/api/v1/users/me", token, `{"handle":"-x-"}`, 400},
		{"update me conflict", readyRouter, http.MethodPatch, "/api/v1/users/me", token, `{"handle":"taken"}`, 409},
		{"healthz", readyRouter, "", "/api/v1/healthz", "", "", 200},
		{"readyz ready", readyRouter, "", "/api/v1/readyz", "", "", 200},
		{"readyz unavailable", downRouter, "", "/api/v1/readyz", "", "", 503},
		{"users/me", readyRouter, "", "/api/v1/users/me", token, "", 200},
		{"users/me unauthenticated", readyRouter, "", "/api/v1/users/me", "", "", 401},
		{"users/me rate limited", readyRouter, "", "/api/v1/users/me", s.Token(t, "429", time.Now()), "", 429},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			method := tc.method
			if method == "" {
				method = http.MethodGet
			}
			req, _ := http.NewRequest(method, srv.URL+tc.path, strings.NewReader(tc.body))
			if tc.auth != "" {
				req.Header.Set("Authorization", "Bearer "+tc.auth)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tc.status {
				t.Fatalf("status %d, want %d", resp.StatusCode, tc.status)
			}
			// Validate against the spec using the same path the server saw.
			specReq, _ := http.NewRequest(method, "http://localhost:8080"+tc.path, strings.NewReader(tc.body))
			specReq.Header = req.Header.Clone()
			if tc.body != "" && tc.status < 300 {
				if ok, errs := v.ValidateHttpRequest(specReq); !ok {
					for _, e := range errs {
						t.Errorf("request: %s: %s", e.Message, e.Reason)
					}
				}
				specReq, _ = http.NewRequest(method, "http://localhost:8080"+tc.path, strings.NewReader(tc.body))
				specReq.Header = req.Header.Clone()
			}
			if ok, errs := v.ValidateHttpResponse(specReq, resp); !ok {
				for _, e := range errs {
					t.Errorf("%s: %s (%s)", e.Message, e.Reason, e.HowToFix)
				}
			}
		})
	}
}
