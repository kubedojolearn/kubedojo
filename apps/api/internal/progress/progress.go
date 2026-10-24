// Package progress serves the learner's own progress, activity, achievements and public
// profiles. It never marks lessons complete or awards XP; only the verified challenge
// evaluator (Part 16) does that.
package progress

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

// CourseProgress mirrors Progress.courses[].
type CourseProgress struct {
	CourseSlug       string `json:"courseSlug"`
	CompletedLessons int32  `json:"completedLessons"`
	TotalLessons     int32  `json:"totalLessons"`
}

// LessonProgress mirrors the OpenAPI LessonProgress.
type LessonProgress struct {
	LessonID      string     `json:"lessonId"`
	CourseSlug    string     `json:"courseSlug"`
	LessonSlug    string     `json:"lessonSlug"`
	Status        string     `json:"status"`
	BestScore     *int16     `json:"bestScore"`
	CompletedAt   *time.Time `json:"completedAt"`
	LastStartedAt *time.Time `json:"lastStartedAt"`
}

// Totals mirrors Progress.totals.
type Totals struct {
	TotalXP            int32   `json:"totalXp"`
	CompletedLessons   int32   `json:"completedLessons"`
	CurrentStreak      int32   `json:"currentStreak"`
	LongestStreak      int32   `json:"longestStreak"`
	LastCompletionDate *string `json:"lastCompletionDate"`
}

// LessonPointer identifies a lesson to continue or start next.
type LessonPointer struct {
	CourseSlug string     `json:"courseSlug"`
	Slug       string     `json:"slug"`
	Title      string     `json:"title"`
	LastAt     *time.Time `json:"lastActivityAt,omitempty"`
}

// Progress mirrors the OpenAPI Progress.
type Progress struct {
	Courses           []CourseProgress `json:"courses"`
	Lessons           []LessonProgress `json:"lessons"`
	Totals            Totals           `json:"totals"`
	ContinueLesson    *LessonPointer   `json:"continueLesson"`
	RecommendedLesson *LessonPointer   `json:"recommendedLesson"`
}

// Achievement mirrors the OpenAPI Achievement.
type Achievement struct {
	Key         string     `json:"key"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Icon        string     `json:"icon"`
	AwardedAt   *time.Time `json:"awardedAt"`
}

// ActivityItem mirrors the OpenAPI ActivityItem.
type ActivityItem struct {
	Type           string    `json:"type"`
	LessonID       *string   `json:"lessonId,omitempty"`
	AchievementKey *string   `json:"achievementKey,omitempty"`
	XPDelta        *int32    `json:"xpDelta,omitempty"`
	OccurredAt     time.Time `json:"occurredAt"`
}

// PublicProfile mirrors the OpenAPI PublicProfile.
type PublicProfile struct {
	Handle           string        `json:"handle"`
	DisplayName      string        `json:"displayName"`
	AvatarURL        *string       `json:"avatarUrl"`
	TotalXP          int32         `json:"totalXp"`
	CompletedLessons int32         `json:"completedLessons"`
	Achievements     []Achievement `json:"achievements"`
	JoinedAt         time.Time     `json:"joinedAt"`
}

// CurrentStreak returns the stored streak if it is still alive on `now` in the learner's time
// zone: the last completion was today or yesterday. Otherwise the streak has lapsed.
func CurrentStreak(stored int32, lastCompletion *time.Time, tz string, now time.Time) int32 {
	if lastCompletion == nil || stored == 0 {
		return 0
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	y, m, d := now.In(loc).Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	ly, lm, ld := lastCompletion.Date()
	last := time.Date(ly, lm, ld, 0, 0, 0, 0, time.UTC)
	if today.Sub(last) <= 24*time.Hour {
		return stored
	}
	return 0
}

// Service reads progress data.
type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// NewService returns a Service.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, now: time.Now} }

// LessonCursor is the keyset position in the lesson progress list.
type LessonCursor struct {
	CourseOrder int32 `json:"c"`
	LessonOrder int32 `json:"l"`
}

// ErrNotFound covers missing, unpublished or private items.
var ErrNotFound = errors.New("not found")

// Progress returns course totals, a page of lesson progress and the dashboard pointers.
func (s *Service) Progress(ctx context.Context, userID uuid.UUID, course *string, after *LessonCursor, limit int32) (Progress, *LessonCursor, error) {
	q := dbgen.New(s.pool)
	p := Progress{Courses: []CourseProgress{}, Lessons: []LessonProgress{}}

	courses, err := q.CourseProgress(ctx, userID)
	if err != nil {
		return p, nil, err
	}
	for _, c := range courses {
		p.Courses = append(p.Courses, CourseProgress{CourseSlug: c.Slug, CompletedLessons: c.CompletedLessons, TotalLessons: c.TotalLessons})
	}

	params := dbgen.ListLessonProgressParams{UserID: userID, CourseSlug: course, PageLimit: limit + 1}
	if after != nil {
		params.AfterCourseOrder, params.AfterLessonOrder = &after.CourseOrder, &after.LessonOrder
	}
	rows, err := q.ListLessonProgress(ctx, params)
	if err != nil {
		return p, nil, err
	}
	var next *LessonCursor
	if len(rows) > int(limit) {
		last := rows[limit-1]
		next = &LessonCursor{CourseOrder: last.CourseOrder, LessonOrder: last.LessonOrder}
		rows = rows[:limit]
	}
	for _, r := range rows {
		status := "not-started"
		switch {
		case r.CompletedAt != nil:
			status = "completed"
		case r.LastStartedAt != nil:
			status = "in-progress"
		}
		p.Lessons = append(p.Lessons, LessonProgress{
			LessonID: r.CourseSlug + "/" + r.LessonSlug, CourseSlug: r.CourseSlug, LessonSlug: r.LessonSlug,
			Status: status, BestScore: r.BestScore, CompletedAt: r.CompletedAt, LastStartedAt: r.LastStartedAt,
		})
	}

	t, err := q.GetTotalsWithTimeZone(ctx, userID)
	if err != nil {
		return p, nil, fmt.Errorf("totals: %w", err)
	}
	p.Totals = Totals{
		TotalXP: t.TotalXp, CompletedLessons: t.CompletedLessons, LongestStreak: t.LongestStreak,
		CurrentStreak: CurrentStreak(t.CurrentStreak, t.LastCompletionDate, t.TimeZone, s.now()),
	}
	if t.LastCompletionDate != nil {
		d := t.LastCompletionDate.Format("2006-01-02")
		p.Totals.LastCompletionDate = &d
	}

	if c, err := q.ContinueLesson(ctx, userID); err == nil {
		at := c.LastActivityAt
		p.ContinueLesson = &LessonPointer{CourseSlug: c.CourseSlug, Slug: c.LessonSlug, Title: c.Title, LastAt: &at}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return p, nil, err
	}
	if r, err := q.RecommendedLesson(ctx, userID); err == nil {
		p.RecommendedLesson = &LessonPointer{CourseSlug: r.CourseSlug, Slug: r.LessonSlug, Title: r.Title}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return p, nil, err
	}
	return p, next, nil
}

// MarkOpened records that the learner opened a published lesson (for "continue learning"),
// and, unless they opted out of analytics, a lesson-opened activity event.
func (s *Service) MarkOpened(ctx context.Context, userID uuid.UUID, courseSlug, lessonSlug string, analyticsOptOut bool) error {
	q := dbgen.New(s.pool)
	lessonID, err := q.MarkLessonOpened(ctx, dbgen.MarkLessonOpenedParams{UserID: userID, CourseSlug: courseSlug, LessonSlug: lessonSlug})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if analyticsOptOut {
		return nil
	}
	return q.InsertActivity(ctx, dbgen.InsertActivityParams{UserID: userID, Type: "lesson-opened", LessonID: &lessonID})
}

// Achievements lists every achievement with the learner's award time (nil if not earned).
func (s *Service) Achievements(ctx context.Context, userID uuid.UUID) ([]Achievement, error) {
	rows, err := dbgen.New(s.pool).ListAchievements(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Achievement, 0, len(rows))
	for _, r := range rows {
		out = append(out, Achievement{Key: r.Key, Title: r.Title, Description: r.Description, Icon: r.Icon, AwardedAt: r.AwardedAt})
	}
	return out, nil
}

// ActivityCursor is the keyset position in the activity list.
type ActivityCursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
}

// Activity returns the learner's activity, newest first.
func (s *Service) Activity(ctx context.Context, userID uuid.UUID, before *ActivityCursor, limit int32) ([]ActivityItem, *ActivityCursor, error) {
	params := dbgen.ListActivityParams{UserID: userID, PageLimit: limit + 1}
	if before != nil {
		params.BeforeAt, params.BeforeID = &before.At, &before.ID
	}
	rows, err := dbgen.New(s.pool).ListActivity(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	var next *ActivityCursor
	if len(rows) > int(limit) {
		next = &ActivityCursor{At: rows[limit-1].OccurredAt, ID: rows[limit-1].ID}
		rows = rows[:limit]
	}
	out := make([]ActivityItem, 0, len(rows))
	for _, r := range rows {
		item := ActivityItem{Type: r.Type, AchievementKey: r.AchievementKey, XPDelta: r.XpDelta, OccurredAt: r.OccurredAt}
		if r.CourseSlug != nil && r.LessonSlug != nil {
			id := *r.CourseSlug + "/" + *r.LessonSlug
			item.LessonID = &id
		}
		out = append(out, item)
	}
	return out, next, nil
}

// PublicProfile returns a profile only if its owner made it public.
func (s *Service) PublicProfile(ctx context.Context, handle string) (PublicProfile, error) {
	q := dbgen.New(s.pool)
	u, err := q.GetPublicProfile(ctx, handle)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicProfile{}, ErrNotFound
	}
	if err != nil {
		return PublicProfile{}, err
	}
	awarded, err := q.ListAwardedAchievements(ctx, u.ID)
	if err != nil {
		return PublicProfile{}, err
	}
	p := PublicProfile{
		Handle: u.Handle, DisplayName: u.DisplayName, AvatarURL: u.AvatarUrl, TotalXP: u.TotalXp,
		CompletedLessons: u.CompletedLessons, JoinedAt: u.CreatedAt, Achievements: []Achievement{},
	}
	for _, a := range awarded {
		at := a.AwardedAt
		p.Achievements = append(p.Achievements, Achievement{Key: a.Key, Title: a.Title, Description: a.Description, Icon: a.Icon, AwardedAt: &at})
	}
	return p, nil
}

// Reader is what the handlers need (fakeable in router tests).
type Reader interface {
	Progress(ctx context.Context, userID uuid.UUID, course *string, after *LessonCursor, limit int32) (Progress, *LessonCursor, error)
	MarkOpened(ctx context.Context, userID uuid.UUID, courseSlug, lessonSlug string, analyticsOptOut bool) error
	Achievements(ctx context.Context, userID uuid.UUID) ([]Achievement, error)
	Activity(ctx context.Context, userID uuid.UUID, before *ActivityCursor, limit int32) ([]ActivityItem, *ActivityCursor, error)
	PublicProfile(ctx context.Context, handle string) (PublicProfile, error)
}

// SettingsReader tells MarkOpened whether the learner opted out of analytics.
type SettingsReader interface {
	AnalyticsOptOut(ctx context.Context, userID uuid.UUID) (bool, error)
}

// Handler serves progress routes.
type Handler struct {
	Progress Reader
	Settings SettingsReader
	Cursors  *httpx.Cursors
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// GetProgress handles GET /users/me/progress.
func (h Handler) GetProgress(w http.ResponseWriter, r *http.Request) error {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return httpx.Unauthenticated()
	}
	page, err := httpx.ParsePage(r)
	if err != nil {
		return err
	}
	var course *string
	if c := r.URL.Query().Get("course"); c != "" {
		if !slugPattern.MatchString(c) {
			return httpx.Validation("Invalid course.", httpx.FieldError{Field: "course", Message: "must be a course slug"})
		}
		course = &c
	}
	var after *LessonCursor
	if page.Cursor != "" {
		after = &LessonCursor{}
		if err := h.Cursors.Decode(page.Cursor, after); err != nil {
			return err
		}
	}
	prog, next, err := h.Progress.Progress(r.Context(), p.UserID, course, after, int32(page.Limit)) //nolint:gosec // limit is 1-100
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WritePage(w, r, prog, h.encode(next))
	return nil
}

// OpenedRequest is the body of POST /users/me/progress/opened.
type OpenedRequest struct {
	CourseSlug string `json:"courseSlug"`
	LessonSlug string `json:"lessonSlug"`
}

// MarkOpened handles POST /users/me/progress/opened.
func (h Handler) MarkOpened(w http.ResponseWriter, r *http.Request) error {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return httpx.Unauthenticated()
	}
	var req OpenedRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if !slugPattern.MatchString(req.CourseSlug) || !slugPattern.MatchString(req.LessonSlug) {
		return httpx.Validation("Invalid lesson.")
	}
	optOut, err := h.Settings.AnalyticsOptOut(r.Context(), p.UserID)
	if err != nil {
		return httpx.Internal(err)
	}
	if err := h.Progress.MarkOpened(r.Context(), p.UserID, req.CourseSlug, req.LessonSlug, optOut); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.NotFound("Lesson")
		}
		return httpx.Internal(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetAchievements handles GET /users/me/achievements.
func (h Handler) GetAchievements(w http.ResponseWriter, r *http.Request) error {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return httpx.Unauthenticated()
	}
	a, err := h.Progress.Achievements(r.Context(), p.UserID)
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, a)
	return nil
}

// GetActivity handles GET /users/me/activity.
func (h Handler) GetActivity(w http.ResponseWriter, r *http.Request) error {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return httpx.Unauthenticated()
	}
	page, err := httpx.ParsePage(r)
	if err != nil {
		return err
	}
	var before *ActivityCursor
	if page.Cursor != "" {
		before = &ActivityCursor{}
		if err := h.Cursors.Decode(page.Cursor, before); err != nil {
			return err
		}
	}
	items, next, err := h.Progress.Activity(r.Context(), p.UserID, before, int32(page.Limit)) //nolint:gosec // limit is 1-100
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WritePage(w, r, items, h.encode(next))
	return nil
}

// GetPublicProfile handles GET /profiles/{handle} (public).
func (h Handler) GetPublicProfile(w http.ResponseWriter, r *http.Request) error {
	prof, err := h.Progress.PublicProfile(r.Context(), chi.URLParam(r, "handle"))
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("Profile")
	}
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, prof)
	return nil
}

func (h Handler) encode(pos any) *string {
	switch v := pos.(type) {
	case *LessonCursor:
		if v == nil {
			return nil
		}
	case *ActivityCursor:
		if v == nil {
			return nil
		}
	}
	c, err := h.Cursors.Encode(pos)
	if err != nil {
		return nil
	}
	return &c
}
