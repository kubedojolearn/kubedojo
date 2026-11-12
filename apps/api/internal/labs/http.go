package labs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

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
	Files(ctx context.Context, userID, labID uuid.UUID) ([]LabFile, error)
	SaveFile(ctx context.Context, userID, labID uuid.UUID, path, content string, base *time.Time) (LabFile, error)
	DeleteFile(ctx context.Context, userID, labID uuid.UUID, path string) error
	SubmitAnswer(ctx context.Context, userID, labID uuid.UUID, goalID, answer string) (AnswerStatus, error)
	RevealHint(ctx context.Context, userID, labID uuid.UUID, index int) (Hint, error)
	Validate(ctx context.Context, userID, labID uuid.UUID, key string) (ValidationResult, error)
	Goals(ctx context.Context, userID, labID uuid.UUID) (LabGoals, error)
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
	case errors.Is(err, ErrInvalidFile):
		return httpx.Validation("Invalid file.",
			httpx.FieldError{Field: "path", Message: "must look like manifests/<name>.yaml (lowercase letters, digits, '.', '_' or '-')"},
			httpx.FieldError{Field: "content", Message: "must be UTF-8 text of at most 65536 bytes without NUL characters"})
	case errors.Is(err, ErrFileLimit):
		return httpx.Validation(fmt.Sprintf("A lab can hold at most %d files.", MaxFiles), httpx.FieldError{Field: "path", Message: "lab file limit reached"})
	case errors.Is(err, ErrGoalNotFound):
		return httpx.NotFound("Goal")
	case errors.Is(err, ErrInvalidAnswer):
		return httpx.Validation("Invalid answer.", httpx.FieldError{Field: "answer", Message: "must be 1-256 characters; a number for number questions, or one of the choices"})
	case errors.Is(err, ErrNoSubmissionsLeft):
		return httpx.Conflict("You've used every answer for this question in this lab. Reset the lab to try again.")
	case errors.Is(err, ErrHintNotFound):
		return httpx.NotFound("Hint")
	case errors.Is(err, ErrHintOrder):
		return httpx.Conflict("Reveal the earlier hints first.")
	case errors.Is(err, ErrFileNotFound):
		return httpx.NotFound("File")
	case errors.Is(err, ErrStaleFile):
		return &httpx.Error{Status: http.StatusConflict, Code: httpx.CodeStaleVersion, Message: "This file changed since you opened it (perhaps in another tab). Reload it before saving."}
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

// ListFiles handles GET /labs/{labId}/files.
func (h Handler) ListFiles(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	files, err := h.Labs.Files(r.Context(), p.UserID, id)
	if err != nil {
		return MapError(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, files)
	return nil
}

// SaveFileRequest is the PUT /labs/{labId}/files body.
type SaveFileRequest struct {
	Path          string     `json:"path"`
	Content       *string    `json:"content"`
	BaseUpdatedAt *time.Time `json:"baseUpdatedAt"`
}

// SaveFile handles PUT /labs/{labId}/files.
func (h Handler) SaveFile(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	var req SaveFileRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.Content == nil {
		return httpx.Validation("Invalid file.", httpx.FieldError{Field: "content", Message: "is required"})
	}
	if !validFile(req.Path, *req.Content) {
		return MapError(ErrInvalidFile)
	}
	file, err := h.Labs.SaveFile(r.Context(), p.UserID, id, req.Path, *req.Content, req.BaseUpdatedAt)
	if err != nil {
		return MapError(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, file)
	return nil
}

// DeleteFile handles DELETE /labs/{labId}/files?path=manifests/<name>.yaml.
func (h Handler) DeleteFile(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	path := r.URL.Query().Get("path")
	if !FilePath.MatchString(path) {
		return MapError(ErrInvalidFile)
	}
	if err := h.Labs.DeleteFile(r.Context(), p.UserID, id, path); err != nil {
		return MapError(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// AnswerRequest is the POST /labs/{labId}/answers body.
type AnswerRequest struct {
	GoalID string `json:"goalId"`
	Answer string `json:"answer"`
}

var goalID = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// SubmitAnswer handles POST /labs/{labId}/answers.
func (h Handler) SubmitAnswer(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	var req AnswerRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if len(req.GoalID) > 64 || !goalID.MatchString(req.GoalID) {
		return httpx.Validation("Invalid goal.", httpx.FieldError{Field: "goalId", Message: "must be a goal ID"})
	}
	if l := len(req.Answer); l == 0 || l > 256 {
		return MapError(ErrInvalidAnswer)
	}
	st, err := h.Labs.SubmitAnswer(r.Context(), p.UserID, id, req.GoalID, req.Answer)
	if err != nil {
		return MapError(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, st)
	return nil
}

// Validate handles POST /labs/{labId}/validate.
func (h Handler) Validate(w http.ResponseWriter, r *http.Request) error {
	key, err := readIdempotencyKey(r)
	if err != nil {
		return err
	}
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	res, err := h.Labs.Validate(r.Context(), p.UserID, id, key)
	if err != nil {
		return MapError(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, res)
	return nil
}

// RevealHint handles POST /labs/{labId}/hints/{index}.
func (h Handler) RevealHint(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	index, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil || index < 0 || index > 9 {
		return httpx.NotFound("Hint")
	}
	hint, err := h.Labs.RevealHint(r.Context(), p.UserID, id, index)
	if err != nil {
		return MapError(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, hint)
	return nil
}

// GetGoals handles GET /labs/{labId}/goals.
func (h Handler) GetGoals(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := LabID(r)
	if err != nil {
		return err
	}
	goals, err := h.Labs.Goals(r.Context(), p.UserID, id)
	if err != nil {
		return MapError(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, goals)
	return nil
}
