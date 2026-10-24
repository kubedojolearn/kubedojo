package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// Role is stored in PostgreSQL (users.role). It is never read from a request.
type Role string

// Roles.
const (
	RoleLearner Role = "learner"
	RoleAdmin   Role = "admin"
)

// Principal is the authenticated caller of a request.
type Principal struct {
	UserID uuid.UUID
	Role   Role
	// Provider identity the assertion was issued for, e.g. github / 12345.
	Provider        string
	ProviderSubject string
}

// ErrIdentityNotLinked means the provider identity has no local user (not provisioned, or deleted).
var ErrIdentityNotLinked = errors.New("identity not linked to a user")

// IdentityResolver maps a verified provider identity to the local user and its current role.
type IdentityResolver interface {
	Resolve(ctx context.Context, provider, providerSubject string) (Principal, error)
}

type principalKey struct{}

// WithPrincipal stores p in ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the authenticated principal, if any.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
