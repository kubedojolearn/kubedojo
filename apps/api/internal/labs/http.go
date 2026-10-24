package labs

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

// Lifecycle is what the HTTP handlers need from Service.
type Lifecycle interface {
	Create(ctx context.Context, userID, challengeVersionID uuid.UUID) (Lab, bool, error)
	List(ctx context.Context, userID uuid.UUID, state *string, after *ListCursor, limit int32) ([]Lab, *ListCursor, error)
	Get(ctx context.Context, userID, labID uuid.UUID) (Lab, error)
	Resume(ctx context.Context, userID, labID uuid.UUID) (Lab, error)
	Reset(ctx context.Context, userID, labID uuid.UUID, key string) (Lab, error)
	Finish(ctx context.Context, userID, labID uuid.UUID, key string) (Lab, error)
	Snapshot(ctx context.Context, userID, labID uuid.UUID) (Snapshot, error)
	Events(ctx context.Context, userID, labID uuid.UUID, afterSeq int64, limit int32) ([]SimEvent, *int64, error)
	RunCommand(ctx context.Context, userID, labID, requestID uuid.UUID, input string) (CommandResult, error)
	Commands(ctx context.Context, userID, labID uuid.UUID, before *CommandCursor, limit int32) ([]CommandResult, *CommandCursor, error)
}

// Handler serves lab routes. Every route is scoped to the authenticated user.
type Handler struct {
	Labs    Lifecycle
	Cursors *httpx.Cursors
}

// MapError turns lab errors into API errors.
func MapError(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return httpx.NotFound("Lab")
	case errors.Is(err, ErrChallengeNotFound):
		return httpx.NotFound("Challenge version")
	case errors.Is(err, ErrStaleChallenge):
		return httpx.Conflict("This lesson has been updated. Reload the page to start the current version.")
	case errors.Is(err, ErrNotActive):
		return httpx.Conflict("This lab has ended. Start the lesson again for a new lab.")
	case errors.Is(err, ErrExpired):
		return &httpx.Error{Status: http.StatusGone, Code: httpx.CodeSessionExpired, Message: "This lab expired. Start the lesson again for a fresh lab."}
	case errors.Is(err, ErrInvalidInput):
		return httpx.Validation("Invalid command.", httpx.FieldError{Field: "input", Message: "must be 1-16384 bytes of UTF-8 text without NUL characters"})
	case errors.Is(err, ErrIdempotencyConflict):
		return httpx.Conflict("This Idempotency-Key was already used for a different request.")
	}
	return httpx.Internal(err)
}

func principal(r *http.Request) (auth.Principal, error) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return p, httpx.Unauthenticated()
	}
	return p, nil
}

// LabID parses the {labId} path parameter. Malformed IDs are reported as not found, like
// labs that don't exist or belong to someone else.
func LabID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "labId"))
	if err != nil {
		return id, httpx.NotFound("Lab")
	}
	return id, nil
}

var idempotencyKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

func readIdempotencyKey(r *http.Request) (string, error) {
	k := r.Header.Get("Idempotency-Key")
	if !idempotencyKey.MatchString(k) {
		return "", httpx.Validation("Idempotency-Key header is required.", httpx.FieldError{Field: "Idempotency-Key", Message: "8-128 characters: letters, digits, '-' or '_'"})
	}
	return k, nil
}

// CreateRequest is the POST /labs body.
type CreateRequest struct {
	ChallengeVersionID string `json:"challengeVersionId"`
}

// Create handles POST /labs.
func (h Handler) Create(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	var req CreateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	cv, err := uuid.Parse(req.ChallengeVersionID)
	if err != nil {
		return httpx.Validation("Invalid challenge version.", httpx.FieldError{Field: "challengeVersionId", Message: "must be a UUID"})
	}
	lab, created, err := h.Labs.Create(r.Context(), p.UserID, cv)
	if err != nil {
		return MapError(err)
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, r, status, lab)
	return nil
}

var labStates = map[string]bool{"active": true, "completed": true, "abandoned": true, "expired": true}

// List handles GET /labs.
func (h Handler) List(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	page, err := httpx.ParsePage(r)
	if err != nil {
		return err
	}
	var state *string
	if s := r.URL.Query().Get("state"); s != "" {
		if !labStates[s] {
			return httpx.Validation("Invalid state.", httpx.FieldError{Field: "state", Message: "must be active, completed, abandoned or expired"})
		}
		state = &s
	}
	var after *ListCursor
	if page.Cursor != "" {
		after = &ListCursor{}
		if err := h.Cursors.Decode(page.Cursor, after); err != nil {
			return err
		}
	}
	labs, next, err := h.Labs.List(r.Context(), p.UserID, state, after, int32(page.Limit)) //nolint:gosec // limit is 1-100
	if err != nil {
		return httpx.Internal(err)
	}
	if next != nil {
		return h.writePage(w, r, labs, *next)
	}
	return h.writePage(w, r, labs, nil)
}

// writePage writes a page; next is the cursor position of the following page, or nil.
func (h Handler) writePage(w http.ResponseWriter, r *http.Request, data any, next any) error {
	var cursor *string
	if next != nil {
		c, err := h.Cursors.Encode(next)
		if err != nil {
			return httpx.Internal(err)
		}
		cursor = &c
	}
	httpx.WritePage(w, r, data, cursor)
	return nil
}

func (h Handler) one(w http.ResponseWriter, r *http.Request, fn func(userID, labID uuid.UUID) (Lab, error)) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	lab, err := fn(p.UserID, id)
	if err != nil {
		return MapError(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, lab)
	return nil
}

// Get handles GET /labs/{labId}.
func (h Handler) Get(w http.ResponseWriter, r *http.Request) error {
	return h.one(w, r, func(u, l uuid.UUID) (Lab, error) { return h.Labs.Get(r.Context(), u, l) })
}

// Resume handles POST /labs/{labId}/resume.
func (h Handler) Resume(w http.ResponseWriter, r *http.Request) error {
	return h.one(w, r, func(u, l uuid.UUID) (Lab, error) { return h.Labs.Resume(r.Context(), u, l) })
}

// Reset handles POST /labs/{labId}/reset.
func (h Handler) Reset(w http.ResponseWriter, r *http.Request) error {
	key, err := readIdempotencyKey(r)
	if err != nil {
		return err
	}
	return h.one(w, r, func(u, l uuid.UUID) (Lab, error) { return h.Labs.Reset(r.Context(), u, l, key) })
}

// Finish handles POST /labs/{labId}/finish.
func (h Handler) Finish(w http.ResponseWriter, r *http.Request) error {
	key, err := readIdempotencyKey(r)
	if err != nil {
		return err
	}
	return h.one(w, r, func(u, l uuid.UUID) (Lab, error) { return h.Labs.Finish(r.Context(), u, l, key) })
}

// GetSnapshot handles GET /labs/{labId}/snapshot.
func (h Handler) GetSnapshot(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	snap, err := h.Labs.Snapshot(r.Context(), p.UserID, id)
	if err != nil {
		return MapError(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, snap)
	return nil
}

type eventCursor struct {
	Seq int64 `json:"s"`
}

// ListEvents handles GET /labs/{labId}/events.
func (h Handler) ListEvents(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	page, err := httpx.ParsePage(r)
	if err != nil {
		return err
	}
	var after eventCursor
	if page.Cursor != "" {
		if err := h.Cursors.Decode(page.Cursor, &after); err != nil {
			return err
		}
	}
	events, next, err := h.Labs.Events(r.Context(), p.UserID, id, after.Seq, int32(page.Limit)) //nolint:gosec // limit is 1-100
	if err != nil {
		return MapError(err)
	}
	if next != nil {
		return h.writePage(w, r, events, eventCursor{Seq: *next})
	}
	return h.writePage(w, r, events, nil)
}

// RunCommandRequest is the POST /labs/{labId}/commands body.
type RunCommandRequest struct {
	RequestID string `json:"requestId"`
	Input     string `json:"input"`
}

// RunCommand handles POST /labs/{labId}/commands. Failed commands are terminal output (200 with
// a non-zero exit code), not API errors.
func (h Handler) RunCommand(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	var req RunCommandRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	requestID, err := uuid.Parse(req.RequestID)
	if err != nil {
		return httpx.Validation("Invalid request ID.", httpx.FieldError{Field: "requestId", Message: "must be a UUID"})
	}
	if !ValidInput(req.Input) {
		return MapError(ErrInvalidInput)
	}
	res, err := h.Labs.RunCommand(r.Context(), p.UserID, id, requestID, req.Input)
	if err != nil {
		return MapError(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, res)
	return nil
}

// ListCommands handles GET /labs/{labId}/commands (newest first).
func (h Handler) ListCommands(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	page, err := httpx.ParsePage(r)
	if err != nil {
		return err
	}
	var before *CommandCursor
	if page.Cursor != "" {
		before = &CommandCursor{}
		if err := h.Cursors.Decode(page.Cursor, before); err != nil {
			return err
		}
	}
	cmds, next, err := h.Labs.Commands(r.Context(), p.UserID, id, before, int32(page.Limit)) //nolint:gosec // limit is 1-100
	if err != nil {
		return MapError(err)
	}
	if next != nil {
		return h.writePage(w, r, cmds, *next)
	}
	return h.writePage(w, r, cmds, nil)
}
