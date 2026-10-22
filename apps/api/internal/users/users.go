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

// Handler serves /users/me.
type Handler struct {
	Users Reader
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
