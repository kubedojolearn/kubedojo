package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

// Authenticate requires a valid request assertion (aud=kubedojo-api) and resolves it to a
// Principal. It ignores every other source of identity (cookies, X-User-Id, bodies).
func Authenticate(v *Verifier, resolver IdentityResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				httpx.WriteError(w, r, httpx.Unauthenticated())
				return
			}
			claims, err := v.Verify(token, AudienceAPI)
			if err != nil {
				httpx.Logger(r.Context()).Info("assertion rejected", "reason", err.Error())
				httpx.WriteError(w, r, httpx.Unauthenticated())
				return
			}
			p, err := resolver.Resolve(r.Context(), claims.Provider, claims.ProviderSubject)
			switch {
			case errors.Is(err, ErrIdentityNotLinked):
				httpx.WriteError(w, r, httpx.Unauthenticated())
				return
			case err != nil:
				httpx.WriteError(w, r, httpx.Internal(err))
				return
			}
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
		})
	}
}

// RequireRole allows only principals with one of roles (read from the database by the resolver).
func RequireRole(roles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, httpx.Unauthenticated())
				return
			}
			for _, role := range roles {
				if p.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.WriteError(w, r, httpx.Forbidden())
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	scheme, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
