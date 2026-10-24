package account

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

// Accounts is what the handlers need (fakeable in router tests).
type Accounts interface {
	Leaderboard(ctx context.Context, after *LeaderboardCursor, limit int32) ([]LeaderboardEntry, *LeaderboardCursor, error)
	Export(ctx context.Context, userID uuid.UUID) (json.RawMessage, error)
	RequestDeletion(ctx context.Context, userID uuid.UUID, confirmHandle string) error
}

// Handler serves /leaderboard, /users/me/export and /users/me/deletion.
type Handler struct {
	Accounts Accounts
	Cursors  *httpx.Cursors
}

var (
	handlePattern         = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)
)

// GetLeaderboard handles GET /leaderboard (public; only learners who opted in are listed).
func (h Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) error {
	page, err := httpx.ParsePage(r)
	if err != nil {
		return err
	}
	var after *LeaderboardCursor
	if page.Cursor != "" {
		after = &LeaderboardCursor{}
		if err := h.Cursors.Decode(page.Cursor, after); err != nil {
			return err
		}
	}
	entries, next, err := h.Accounts.Leaderboard(r.Context(), after, int32(page.Limit)) //nolint:gosec // limit is 1-100
	if err != nil {
		return httpx.Internal(err)
	}
	if next == nil {
		httpx.WritePage(w, r, entries, nil)
		return nil
	}
	c, err := h.Cursors.Encode(next)
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WritePage(w, r, entries, &c)
	return nil
}

// ExportMe handles GET /users/me/export.
func (h Handler) ExportMe(w http.ResponseWriter, r *http.Request) error {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return httpx.Unauthenticated()
	}
	doc, err := h.Accounts.Export(r.Context(), p.UserID)
	if err != nil {
		return httpx.Internal(err)
	}
	w.Header().Set("Content-Disposition", `attachment; filename="kubedojo-export.json"`)
	httpx.WriteJSON(w, r, http.StatusOK, doc)
	return nil
}

// DeletionRequest is the body of POST /users/me/deletion.
type DeletionRequest struct {
	ConfirmHandle string `json:"confirmHandle"`
}

// RequestDeletion handles POST /users/me/deletion. A retry after success finds no signed-in
// user (the identity is unlinked) and gets 401, which clients treat as already deleted.
func (h Handler) RequestDeletion(w http.ResponseWriter, r *http.Request) error {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return httpx.Unauthenticated()
	}
	if !idempotencyKeyPattern.MatchString(r.Header.Get("Idempotency-Key")) {
		return httpx.Validation("Idempotency-Key header is required.", httpx.FieldError{Field: "Idempotency-Key", Message: "8-128 characters: letters, digits, '-' or '_'"})
	}
	var req DeletionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if !handlePattern.MatchString(req.ConfirmHandle) {
		return httpx.Validation("Type your handle to confirm.", httpx.FieldError{Field: "confirmHandle", Message: "must be your handle"})
	}
	err := h.Accounts.RequestDeletion(r.Context(), p.UserID, req.ConfirmHandle)
	if errors.Is(err, ErrHandleMismatch) {
		return httpx.Validation("That isn't your handle.", httpx.FieldError{Field: "confirmHandle", Message: "must match your handle exactly"})
	}
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.Logger(r.Context()).Info("account deletion requested", "user_id", p.UserID)
	w.WriteHeader(http.StatusAccepted)
	return nil
}
