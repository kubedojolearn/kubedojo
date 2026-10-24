// Package users serves the current user's profile and settings.
package users

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

// Settings mirrors the OpenAPI Settings schema.
type Settings struct {
	Theme            string `json:"theme"`
	ReducedMotion    bool   `json:"reducedMotion"`
	LeaderboardOptIn bool   `json:"leaderboardOptIn"`
	ProfilePublic    bool   `json:"profilePublic"`
	AnalyticsOptOut  bool   `json:"analyticsOptOut"`
	TimeZone         string `json:"timeZone"`
}

// Me mirrors the OpenAPI Me schema.
type Me struct {
	ID                    uuid.UUID  `json:"id"`
	Handle                string     `json:"handle"`
	DisplayName           string     `json:"displayName"`
	AvatarURL             *string    `json:"avatarUrl"`
	Role                  auth.Role  `json:"role"`
	CreatedAt             time.Time  `json:"createdAt"`
	Settings              Settings   `json:"settings"`
	OnboardingCompletedAt *time.Time `json:"onboardingCompletedAt"`
}

// ErrNotFound means the user does not exist (or was deleted after the principal was resolved).
var ErrNotFound = errors.New("user not found")

// Reader loads users by ID.
type Reader interface {
	GetMe(ctx context.Context, userID uuid.UUID) (Me, error)
}

// Service is everything the HTTP handlers need from the user store.
type Service interface {
	Reader
	UpdateMe(ctx context.Context, userID uuid.UUID, r UpdateMeRequest) error
	Provision(ctx context.Context, in ProvisionInput) (ProvisionResult, error)
}

// Handler serves /users/me and /auth/provision.
type Handler struct {
	Users Service
}

// GetMe returns the authenticated user. The user ID comes only from the verified principal.
func (h Handler) GetMe(w http.ResponseWriter, r *http.Request) error {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return httpx.Unauthenticated()
	}
	me, err := h.Users.GetMe(r.Context(), p.UserID)
	if errors.Is(err, ErrNotFound) {
		return httpx.Unauthenticated()
	}
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, me)
	return nil
}

// UpdateMe applies a partial profile/settings update for the authenticated user and returns
// the updated user.
func (h Handler) UpdateMe(w http.ResponseWriter, r *http.Request) error {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return httpx.Unauthenticated()
	}
	var req UpdateMeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}
	if err := h.Users.UpdateMe(r.Context(), p.UserID, req); err != nil {
		if errors.Is(err, ErrHandleTaken) {
			return &httpx.Error{Status: http.StatusConflict, Code: httpx.CodeConflict, Message: "That handle is taken.",
				Details: []httpx.FieldError{{Field: "handle", Message: "is already taken"}}}
		}
		return httpx.Internal(err)
	}
	return h.GetMe(w, r)
}

// Provision creates or refreshes the user for a verified provisioning assertion (ADR-0003).
func (h Handler) Provision(w http.ResponseWriter, r *http.Request) error {
	c, ok := auth.ProvisionClaimsFrom(r.Context())
	if !ok {
		return httpx.Unauthenticated()
	}
	res, err := h.Users.Provision(r.Context(), ProvisionInput{
		Provider: c.Provider, ProviderSubject: c.ProviderSubject, Login: c.Login, Name: c.Name, AvatarURL: c.AvatarURL,
	})
	if err != nil {
		return httpx.Internal(err)
	}
	me, err := h.Users.GetMe(r.Context(), res.UserID)
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.Logger(r.Context()).Info("user provisioned", "user_id", res.UserID, "created", res.Created)
	httpx.WriteJSON(w, r, http.StatusOK, me)
	return nil
}
