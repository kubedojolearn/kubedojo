// Package admin serves the read-only admin views: imported content versions and the audit log.
// Publishing from the browser (validate, publish, unpublish) arrives with Part 18; until then
// content is published with `api content import -publish`.
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

// ChallengeVersion mirrors the OpenAPI AdminChallengeVersion.
type ChallengeVersion struct {
	ID            uuid.UUID  `json:"id"`
	Version       int32      `json:"version"`
	ContentSHA256 string     `json:"contentSha256"`
	Status        string     `json:"status"`
	ImportedAt    time.Time  `json:"importedAt"`
	PublishedAt   *time.Time `json:"publishedAt"`
}

// LessonContent mirrors the OpenAPI AdminLessonContent.
type LessonContent struct {
	LessonID         string             `json:"lessonId"`
	Title            string             `json:"title"`
	CurrentVersionID *uuid.UUID         `json:"currentVersionId"`
	Versions         []ChallengeVersion `json:"versions"`
}

// AuditEntry mirrors the OpenAPI AuditEntry.
type AuditEntry struct {
	ID          uuid.UUID       `json:"id"`
	ActorUserID *uuid.UUID      `json:"actorUserId"`
	Action      string          `json:"action"`
	EntityType  string          `json:"entityType"`
	EntityID    string          `json:"entityId"`
	Details     json.RawMessage `json:"details"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// AuditCursor is the keyset position after the last entry of a page.
type AuditCursor struct {
	CreatedAt time.Time `json:"t"`
	ID        uuid.UUID `json:"i"`
}

// Service reads admin views.
type Service struct {
	pool *pgxpool.Pool
}

// NewService returns a Service backed by pool.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Content lists every lesson with all of its imported versions, newest version first.
func (s *Service) Content(ctx context.Context) ([]LessonContent, error) {
	rows, err := dbgen.New(s.pool).ListAdminContent(ctx)
	if err != nil {
		return nil, fmt.Errorf("admin content: %w", err)
	}
	out := []LessonContent{}
	for _, r := range rows {
		id := r.CourseSlug + "/" + r.LessonSlug
		if len(out) == 0 || out[len(out)-1].LessonID != id {
			out = append(out, LessonContent{LessonID: id, Title: r.Title, CurrentVersionID: r.CurrentChallengeVersionID, Versions: []ChallengeVersion{}})
		}
		if r.VersionID == nil {
			continue
		}
		l := &out[len(out)-1]
		l.Versions = append(l.Versions, ChallengeVersion{
			ID: *r.VersionID, Version: *r.Version, ContentSHA256: strings.TrimSpace(*r.ContentSha256),
			Status: *r.Status, ImportedAt: *r.ImportedAt, PublishedAt: r.PublishedAt,
		})
	}
	return out, nil
}

// Audit returns one page of the audit log, newest first.
func (s *Service) Audit(ctx context.Context, before *AuditCursor, limit int32) ([]AuditEntry, *AuditCursor, error) {
	params := dbgen.ListAuditLogsParams{PageLimit: limit + 1}
	if before != nil {
		params.BeforeCreatedAt, params.BeforeID = &before.CreatedAt, &before.ID
	}
	rows, err := dbgen.New(s.pool).ListAuditLogs(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("audit log: %w", err)
	}
	var next *AuditCursor
	if len(rows) > int(limit) {
		last := rows[limit-1]
		next = &AuditCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		rows = rows[:limit]
	}
	out := make([]AuditEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, AuditEntry{
			ID: r.ID, ActorUserID: r.ActorUserID, Action: r.Action, EntityType: r.EntityType,
			EntityID: r.EntityID, Details: r.Details, CreatedAt: r.CreatedAt,
		})
	}
	return out, next, nil
}

// Reader is what the handlers need (fakeable in router tests).
type Reader interface {
	Content(ctx context.Context) ([]LessonContent, error)
	Audit(ctx context.Context, before *AuditCursor, limit int32) ([]AuditEntry, *AuditCursor, error)
}

// Handler serves GET /admin/content and GET /admin/audit. The router allows admins only.
type Handler struct {
	Admin   Reader
	Cursors *httpx.Cursors
}

// ListContent handles GET /admin/content.
func (h Handler) ListContent(w http.ResponseWriter, r *http.Request) error {
	c, err := h.Admin.Content(r.Context())
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, c)
	return nil
}

// ListAudit handles GET /admin/audit.
func (h Handler) ListAudit(w http.ResponseWriter, r *http.Request) error {
	page, err := httpx.ParsePage(r)
	if err != nil {
		return err
	}
	var before *AuditCursor
	if page.Cursor != "" {
		before = &AuditCursor{}
		if err := h.Cursors.Decode(page.Cursor, before); err != nil {
			return err
		}
	}
	entries, next, err := h.Admin.Audit(r.Context(), before, int32(page.Limit)) //nolint:gosec // limit is 1-100
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
