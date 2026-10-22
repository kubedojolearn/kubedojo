// Package httpapi assembles the HTTP router and its middleware chain.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/health"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/ratelimit"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/users"
)

// Deps are the router's collaborators.
type Deps struct {
	Log            *slog.Logger
	Health         health.Handler
	Verifier       *auth.Verifier
	Identities     auth.IdentityResolver
	Users          users.Reader
	Limiter        ratelimit.Limiter
	RequestTimeout time.Duration
	MaxBodyBytes   int64
}

// Rate limits for authenticated API traffic (requirements S6). Route-specific limits for
// commands, labs and validation are added with those routes.
var userGeneralLimit = ratelimit.Rule{Name: "api", Limit: 300, Window: time.Minute}

// NewRouter builds the handler. Middleware order (outermost first): request ID, panic recovery,
// deadline, body cap, access log, security headers; then per route group: authentication,
// per-user rate limit, authorization, handler.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.NotFound(httpx.NotFoundHandler)
	r.MethodNotAllowed(httpx.MethodNotAllowedHandler)

	r.Use(httpx.WithRequestID(d.Log))
	r.Use(httpx.Recover)
	r.Use(httpx.Deadline(d.RequestTimeout))
	r.Use(httpx.BodyLimit(d.MaxBodyBytes))
	r.Use(httpx.AccessLog(routePattern))
	r.Use(httpx.SecureHeaders)

	r.Route("/api/v1", func(r chi.Router) {
		// Public: probes (no envelope, see OpenAPI).
		r.Get("/healthz", d.Health.Live)
		r.Get("/readyz", d.Health.Ready)

		// Authenticated with a request assertion.
		r.Group(func(r chi.Router) {
			r.Use(auth.Authenticate(d.Verifier, d.Identities))
			r.Use(ratelimit.PerUser(d.Limiter, userGeneralLimit))

			r.Method(http.MethodGet, "/users/me", httpx.Handler(users.Handler{Users: d.Users}.GetMe))
		})
	})
	return r
}

func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}
