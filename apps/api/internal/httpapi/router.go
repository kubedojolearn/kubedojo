// Package httpapi assembles the HTTP router for the API binary.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/health"
)

// Deps are the collaborators the router needs.
type Deps struct {
	Health health.Handler
}

// NewRouter returns the API's HTTP handler.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", d.Health.Live)
		r.Get("/readyz", d.Health.Ready)
	})
	return r
}
