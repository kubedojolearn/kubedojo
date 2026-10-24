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
		headers map[string]string
	}{
		{"lab command", readyRouter, http.MethodPost, "/api/v1/labs/66666666-6666-4666-8666-666666666666/commands", token, `{"requestId":"abababab-abab-4bab-8bab-abababababab","input":"kubectl get pods"}`, 200, nil},
		{"lab command failing", readyRouter, http.MethodPost, "/api/v1/labs/66666666-6666-4666-8666-666666666666/commands", token, `{"requestId":"abababab-abab-4bab-8bab-abababababab","input":"kubectl get pod nope"}`, 200, nil},
		{"lab command bad request id", readyRouter, http.MethodPost, "/api/v1/labs/66666666-6666-4666-8666-666666666666/commands", token, `{"requestId":"1","input":"kubectl get pods"}`, 400, nil},
		{"lab command empty", readyRouter, http.MethodPost, "/api/v1/labs/66666666-6666-4666-8666-666666666666/commands", token, `{"requestId":"abababab-abab-4bab-8bab-abababababab","input":""}`, 400, nil},
		{"lab command nul", readyRouter, http.MethodPost, "/api/v1/labs/66666666-6666-4666-8666-666666666666/commands", token, `{"requestId":"abababab-abab-4bab-8bab-abababababab","input":"kubectl\u0000"}`, 400, nil},
		{"lab command ended lab", readyRouter, http.MethodPost, "/api/v1/labs/77777777-7777-4777-8777-777777777777/commands", token, `{"requestId":"abababab-abab-4bab-8bab-abababababab","input":"kubectl get pods"}`, 409, nil},
		{"lab command expired lab", readyRouter, http.MethodPost, "/api/v1/labs/88888888-8888-4888-8888-888888888888/commands", token, `{"requestId":"abababab-abab-4bab-8bab-abababababab","input":"kubectl get pods"}`, 410, nil},
		{"lab command other user", readyRouter, http.MethodPost, "/api/v1/labs/12121212-1212-4121-8121-121212121212/commands", token, `{"requestId":"abababab-abab-4bab-8bab-abababababab","input":"kubectl get pods"}`, 404, nil},
		{"lab commands list", readyRouter, "", "/api/v1/labs/66666666-6666-4666-8666-666666666666/commands?limit=1", token, "", 200, nil},
		{"lab commands bad cursor", readyRouter, "", "/api/v1/labs/66666666-6666-4666-8666-666666666666/commands?cursor=forged.sig", token, "", 400, nil},
		{"labs list", readyRouter, "", "/api/v1/labs?state=active&limit=1", token, "", 200, nil},
		{"labs list bad state", readyRouter, "", "/api/v1/labs?state=running", token, "", 400, nil},
		{"labs list unauthenticated", readyRouter, "", "/api/v1/labs", "", "", 401, nil},
		{"lab create", readyRouter, http.MethodPost, "/api/v1/labs", token, `{"challengeVersionId":"55555555-5555-4555-8555-555555555555"}`, 201, nil},
		{"lab create bad id", readyRouter, http.MethodPost, "/api/v1/labs", token, `{"challengeVersionId":"x"}`, 400, nil},
		{"lab create unknown field", readyRouter, http.MethodPost, "/api/v1/labs", token, `{"challengeVersionId":"55555555-5555-4555-8555-555555555555","seed":1}`, 400, nil},
		{"lab create unknown version", readyRouter, http.MethodPost, "/api/v1/labs", token, `{"challengeVersionId":"12121212-1212-4121-8121-121212121212"}`, 404, nil},
		{"lab create stale version", readyRouter, http.MethodPost, "/api/v1/labs", token, `{"challengeVersionId":"99999999-9999-4999-8999-999999999999"}`, 409, nil},
		{"lab get", readyRouter, "", "/api/v1/labs/66666666-6666-4666-8666-666666666666", token, "", 200, nil},
		{"lab get other user", readyRouter, "", "/api/v1/labs/12121212-1212-4121-8121-121212121212", token, "", 404, nil},
		{"lab get malformed id", readyRouter, "", "/api/v1/labs/not-a-uuid", token, "", 404, nil},
		{"lab resume", readyRouter, http.MethodPost, "/api/v1/labs/66666666-6666-4666-8666-666666666666/resume", token, "", 200, nil},
		{"lab resume ended", readyRouter, http.MethodPost, "/api/v1/labs/77777777-7777-4777-8777-777777777777/resume", token, "", 409, nil},
		{"lab resume expired", readyRouter, http.MethodPost, "/api/v1/labs/88888888-8888-4888-8888-888888888888/resume", token, "", 410, nil},
		{"lab reset", readyRouter, http.MethodPost, "/api/v1/labs/66666666-6666-4666-8666-666666666666/reset", token, "", 200, map[string]string{"Idempotency-Key": "4b1f0c8e-reset-1"}},
		{"lab reset without key", readyRouter, http.MethodPost, "/api/v1/labs/66666666-6666-4666-8666-666666666666/reset", token, "", 400, nil},
		{"lab reset reused key", readyRouter, http.MethodPost, "/api/v1/labs/66666666-6666-4666-8666-666666666666/reset", token, "", 409, map[string]string{"Idempotency-Key": "reused-key"}},
		{"lab reset expired", readyRouter, http.MethodPost, "/api/v1/labs/88888888-8888-4888-8888-888888888888/reset", token, "", 410, map[string]string{"Idempotency-Key": "4b1f0c8e-reset-1"}},
		{"lab finish", readyRouter, http.MethodPost, "/api/v1/labs/66666666-6666-4666-8666-666666666666/finish", token, "", 200, map[string]string{"Idempotency-Key": "4b1f0c8e-reset-1"}},
		{"lab finish other user", readyRouter, http.MethodPost, "/api/v1/labs/12121212-1212-4121-8121-121212121212/finish", token, "", 404, map[string]string{"Idempotency-Key": "4b1f0c8e-reset-1"}},
		{"lab snapshot", readyRouter, "", "/api/v1/labs/66666666-6666-4666-8666-666666666666/snapshot", token, "", 200, nil},
		{"lab snapshot other user", readyRouter, "", "/api/v1/labs/12121212-1212-4121-8121-121212121212/snapshot", token, "", 404, nil},
		{"lab events", readyRouter, "", "/api/v1/labs/66666666-6666-4666-8666-666666666666/events?limit=1", token, "", 200, nil},
		{"lab events bad cursor", readyRouter, "", "/api/v1/labs/66666666-6666-4666-8666-666666666666/events?cursor=forged.sig", token, "", 400, nil},
		{"lab create rate limited", readyRouter, http.MethodPost, "/api/v1/labs", s.Token(t, "429", time.Now()), `{"challengeVersionId":"55555555-5555-4555-8555-555555555555"}`, 429, nil},
		{"catalog courses", readyRouter, "", "/api/v1/catalog/courses", "", "", 200, nil},
		{"catalog course", readyRouter, "", "/api/v1/catalog/courses/workloads", "", "", 200, nil},
		{"catalog course missing", readyRouter, "", "/api/v1/catalog/courses/nope", "", "", 404, nil},
		{"catalog lesson", readyRouter, "", "/api/v1/catalog/courses/workloads/lessons/scaling-deployments", "", "", 200, nil},
		{"catalog lesson missing", readyRouter, "", "/api/v1/catalog/courses/workloads/lessons/launch-a-pod", "", "", 404, nil},
		{"progress", readyRouter, "", "/api/v1/users/me/progress?limit=1", token, "", 200, nil},
		{"progress bad cursor", readyRouter, "", "/api/v1/users/me/progress?cursor=forged.sig", token, "", 400, nil},
		{"progress unauthenticated", readyRouter, "", "/api/v1/users/me/progress", "", "", 401, nil},
		{"lesson opened", readyRouter, http.MethodPost, "/api/v1/users/me/progress/opened", token, `{"courseSlug":"workloads","lessonSlug":"scaling-deployments"}`, 204, nil},
		{"lesson opened unpublished", readyRouter, http.MethodPost, "/api/v1/users/me/progress/opened", token, `{"courseSlug":"workloads","lessonSlug":"launch-a-pod"}`, 404, nil},
		{"achievements", readyRouter, "", "/api/v1/users/me/achievements", token, "", 200, nil},
		{"activity", readyRouter, "", "/api/v1/users/me/activity", token, "", 200, nil},
		{"public profile", readyRouter, "", "/api/v1/profiles/public-learner", "", "", 200, nil},
		{"admin content", readyRouter, "", "/api/v1/admin/content", s.Token(t, "900", time.Now()), "", 200, nil},
		{"admin content as learner", readyRouter, "", "/api/v1/admin/content", token, "", 403, nil},
		{"admin content unauthenticated", readyRouter, "", "/api/v1/admin/content", "", "", 401, nil},
		{"admin audit", readyRouter, "", "/api/v1/admin/audit?limit=5", s.Token(t, "900", time.Now()), "", 200, nil},
		{"admin audit as learner", readyRouter, "", "/api/v1/admin/audit", token, "", 403, nil},
		{"admin audit bad cursor", readyRouter, "", "/api/v1/admin/audit?cursor=forged.sig", s.Token(t, "900", time.Now()), "", 400, nil},
		{"leaderboard", readyRouter, "", "/api/v1/leaderboard?limit=1", "", "", 200, nil},
		{"leaderboard bad cursor", readyRouter, "", "/api/v1/leaderboard?cursor=forged.sig", "", "", 400, nil},
		{"leaderboard bad limit", readyRouter, "", "/api/v1/leaderboard?limit=0", "", "", 400, nil},
		{"export", readyRouter, "", "/api/v1/users/me/export", token, "", 200, nil},
		{"export unauthenticated", readyRouter, "", "/api/v1/users/me/export", "", "", 401, nil},
		{"deletion", readyRouter, http.MethodPost, "/api/v1/users/me/deletion", token, `{"confirmHandle":"kube-learner"}`, 202, map[string]string{"Idempotency-Key": "delete-me-1"}},
		{"deletion wrong handle", readyRouter, http.MethodPost, "/api/v1/users/me/deletion", token, `{"confirmHandle":"someone-else"}`, 400, map[string]string{"Idempotency-Key": "delete-me-1"}},
		{"deletion without key", readyRouter, http.MethodPost, "/api/v1/users/me/deletion", token, `{"confirmHandle":"kube-learner"}`, 400, nil},
		{"deletion unknown field", readyRouter, http.MethodPost, "/api/v1/users/me/deletion", token, `{"confirmHandle":"kube-learner","now":true}`, 400, map[string]string{"Idempotency-Key": "delete-me-1"}},
		{"deletion unauthenticated", readyRouter, http.MethodPost, "/api/v1/users/me/deletion", "", `{"confirmHandle":"kube-learner"}`, 401, map[string]string{"Idempotency-Key": "delete-me-1"}},
		{"private profile", readyRouter, "", "/api/v1/profiles/private-learner", "", "", 404, nil},
		{"provision", readyRouter, http.MethodPost, "/api/v1/auth/provision", provisionToken(t, s, "583231"), "", 200, nil},
		{"provision unauthenticated", readyRouter, http.MethodPost, "/api/v1/auth/provision", "", "", 401, nil},
		{"update me", readyRouter, http.MethodPatch, "/api/v1/users/me", token, `{"displayName":"Ada","settings":{"theme":"dark"}}`, 200, nil},
		{"update me invalid", readyRouter, http.MethodPatch, "/api/v1/users/me", token, `{"handle":"-x-"}`, 400, nil},
		{"update me conflict", readyRouter, http.MethodPatch, "/api/v1/users/me", token, `{"handle":"taken"}`, 409, nil},
		{"healthz", readyRouter, "", "/api/v1/healthz", "", "", 200, nil},
		{"readyz ready", readyRouter, "", "/api/v1/readyz", "", "", 200, nil},
		{"readyz unavailable", downRouter, "", "/api/v1/readyz", "", "", 503, nil},
		{"users/me", readyRouter, "", "/api/v1/users/me", token, "", 200, nil},
		{"users/me unauthenticated", readyRouter, "", "/api/v1/users/me", "", "", 401, nil},
		{"users/me rate limited", readyRouter, "", "/api/v1/users/me", s.Token(t, "429", time.Now()), "", 429, nil},
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
			for k, v := range tc.headers {
				req.Header.Set(k, v)
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
