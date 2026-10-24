// Package httpapi assembles the HTTP router and its middleware chain.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/account"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/admin"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/courses"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/health"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/ratelimit"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/progress"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/users"
)

// Deps are the router's collaborators.
type Deps struct {
	Log            *slog.Logger
	Health         health.Handler
	Verifier       *auth.Verifier
	Identities     auth.IdentityResolver
	Users          users.Service
	Catalog        courses.Catalog
	Progress       progress.Reader
	Labs           labs.Lifecycle
	Accounts       account.Accounts
	Admin          admin.Reader
	Settings       progress.SettingsReader
	Cursors        *httpx.Cursors
	Limiter        ratelimit.Limiter
	RequestTimeout time.Duration
	MaxBodyBytes   int64
}

// Rate limits for authenticated API traffic (requirements S6). Route-specific limits for
// commands, labs and validation are added with those routes.
var (
	userGeneralLimit = ratelimit.Rule{Name: "api", Limit: 300, Window: time.Minute}
	// Sign-in provisioning, per GitHub identity; fails closed (requirements S6).
	provisionLimit = ratelimit.Rule{Name: "provision", Limit: 10, Window: time.Minute, FailClosed: true}
	// Starting and resetting labs loads a whole fixture; both are rare in normal use.
	labLifecycleLimit = ratelimit.Rule{Name: "lab-lifecycle", Limit: 30, Window: time.Hour}
	// Terminal commands (simulator-compatibility.md §5).
	commandLimit = ratelimit.Rule{Name: "commands", Limit: 100, Window: time.Minute}
	// A full export reads every row the learner has; deletion is once per account.
	exportLimit   = ratelimit.Rule{Name: "export", Limit: 10, Window: time.Hour}
	deletionLimit = ratelimit.Rule{Name: "deletion", Limit: 5, Window: time.Hour, FailClosed: true}
)

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

		usersHandler := users.Handler{Users: d.Users}
		catalog := courses.Handler{Catalog: d.Catalog}
		progressHandler := progress.Handler{Progress: d.Progress, Settings: d.Settings, Cursors: d.Cursors}
		accountHandler := account.Handler{Accounts: d.Accounts, Cursors: d.Cursors}
		r.Get("/profiles/{handle}", httpx.Handler(progressHandler.GetPublicProfile).ServeHTTP)
		r.Get("/leaderboard", httpx.Handler(accountHandler.GetLeaderboard).ServeHTTP)

		// Public catalog (published content only; hidden challenge fields are never mapped).
		r.Get("/catalog/courses", httpx.Handler(catalog.ListCourses).ServeHTTP)
		r.Get("/catalog/courses/{courseSlug}", httpx.Handler(catalog.GetCourse).ServeHTTP)
		r.Get("/catalog/courses/{courseSlug}/lessons/{lessonSlug}", httpx.Handler(catalog.GetLesson).ServeHTTP)

		// Provisioning: only the BFF's provisioning assertion is accepted here.
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthenticateProvision(d.Verifier))
			r.Use(ratelimit.PerKey(d.Limiter, provisionLimit, func(r *http.Request) (string, bool) {
				c, ok := auth.ProvisionClaimsFrom(r.Context())
				return c.Provider + ":" + c.ProviderSubject, ok
			}))
			r.Method(http.MethodPost, "/auth/provision", httpx.Handler(usersHandler.Provision))
		})

		// Authenticated with a request assertion.
		r.Group(func(r chi.Router) {
			r.Use(auth.Authenticate(d.Verifier, d.Identities))
			r.Use(ratelimit.PerUser(d.Limiter, userGeneralLimit))

			r.Method(http.MethodGet, "/users/me", httpx.Handler(usersHandler.GetMe))
			r.Method(http.MethodPatch, "/users/me", httpx.Handler(usersHandler.UpdateMe))
			r.Method(http.MethodGet, "/users/me/progress", httpx.Handler(progressHandler.GetProgress))
			r.Method(http.MethodPost, "/users/me/progress/opened", httpx.Handler(progressHandler.MarkOpened))
			r.Method(http.MethodGet, "/users/me/achievements", httpx.Handler(progressHandler.GetAchievements))
			r.Method(http.MethodGet, "/users/me/activity", httpx.Handler(progressHandler.GetActivity))
			r.With(ratelimit.PerUser(d.Limiter, exportLimit)).Method(http.MethodGet, "/users/me/export", httpx.Handler(accountHandler.ExportMe))
			r.With(ratelimit.PerUser(d.Limiter, deletionLimit)).Method(http.MethodPost, "/users/me/deletion", httpx.Handler(accountHandler.RequestDeletion))

			// Labs: ownership is enforced in every query; another user's lab is a 404.
			labsHandler := labs.Handler{Labs: d.Labs, Cursors: d.Cursors}
			lifecycle := ratelimit.PerUser(d.Limiter, labLifecycleLimit)
			r.Method(http.MethodGet, "/labs", httpx.Handler(labsHandler.List))
			r.With(lifecycle).Method(http.MethodPost, "/labs", httpx.Handler(labsHandler.Create))
			r.Method(http.MethodGet, "/labs/{labId}", httpx.Handler(labsHandler.Get))
			r.Method(http.MethodPost, "/labs/{labId}/resume", httpx.Handler(labsHandler.Resume))
			r.With(lifecycle).Method(http.MethodPost, "/labs/{labId}/reset", httpx.Handler(labsHandler.Reset))
			r.Method(http.MethodPost, "/labs/{labId}/finish", httpx.Handler(labsHandler.Finish))
			r.Method(http.MethodGet, "/labs/{labId}/snapshot", httpx.Handler(labsHandler.GetSnapshot))
			r.Method(http.MethodGet, "/labs/{labId}/events", httpx.Handler(labsHandler.ListEvents))
			r.Method(http.MethodGet, "/labs/{labId}/commands", httpx.Handler(labsHandler.ListCommands))
			r.With(ratelimit.PerUser(d.Limiter, commandLimit)).Method(http.MethodPost, "/labs/{labId}/commands", httpx.Handler(labsHandler.RunCommand))

			// Admin: the role is read from the database for every request (never from the assertion).
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRole(auth.RoleAdmin))
				adminHandler := admin.Handler{Admin: d.Admin, Cursors: d.Cursors}
				r.Method(http.MethodGet, "/admin/content", httpx.Handler(adminHandler.ListContent))
				r.Method(http.MethodGet, "/admin/audit", httpx.Handler(adminHandler.ListAudit))
			})
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
