// Package courses serves the public catalog: published courses and lessons. Lesson content is
// mapped from the stored challenge spec into explicit public types, so hidden fields (goal
// targets, assertions, expected answers, hint text, reference solutions) can never be sent.
package courses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

// CourseSummary mirrors the OpenAPI CourseSummary.
type CourseSummary struct {
	Slug               string `json:"slug"`
	Title              string `json:"title"`
	Summary            string `json:"summary"`
	Difficulty         string `json:"difficulty"`
	LessonCount        int32  `json:"lessonCount"`
	PlannedLessonCount int32  `json:"plannedLessonCount"`
	EstimatedMinutes   int32  `json:"estimatedMinutes"`
	DisplayOrder       int32  `json:"displayOrder"`
}

// LessonSummary mirrors the OpenAPI LessonSummary.
type LessonSummary struct {
	Slug             string   `json:"slug"`
	CourseSlug       string   `json:"courseSlug"`
	Title            string   `json:"title"`
	Summary          string   `json:"summary"`
	EstimatedMinutes int32    `json:"estimatedMinutes"`
	DisplayOrder     int32    `json:"displayOrder"`
	GoalTypes        []string `json:"goalTypes"`
}

// CourseDetail mirrors the OpenAPI CourseDetail.
type CourseDetail struct {
	CourseSummary
	Description   string          `json:"description"`
	Prerequisites []string        `json:"prerequisites"`
	Lessons       []LessonSummary `json:"lessons"`
}

// PublicGoal is what learners may know about a goal.
type PublicGoal struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Input       string   `json:"input,omitempty"`
	Choices     []string `json:"choices,omitempty"`
}

// Reference is a documentation link.
type Reference struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Reward mirrors the OpenAPI LessonDetail.reward.
type Reward struct {
	BaseXP               int `json:"baseXp"`
	FirstTryBonusXP      int `json:"firstTryBonusXp"`
	HintPenaltyPerHintXP int `json:"hintPenaltyPerHintXp"`
}

// LessonRef points to a neighbouring lesson.
type LessonRef struct {
	CourseSlug string `json:"courseSlug"`
	Slug       string `json:"slug"`
	Title      string `json:"title"`
}

// Navigation links to the previous and next published lessons across the curriculum.
type Navigation struct {
	Previous *LessonRef `json:"previous"`
	Next     *LessonRef `json:"next"`
}

// LessonDetail mirrors the OpenAPI LessonDetail.
type LessonDetail struct {
	LessonSummary
	ChallengeVersionID uuid.UUID    `json:"challengeVersionId"`
	ContentSHA256      string       `json:"contentSha256"`
	Objectives         []string     `json:"objectives"`
	Prerequisites      []string     `json:"prerequisites"`
	PrereqLessons      []LessonRef  `json:"prerequisiteLessons"`
	Body               string       `json:"body"`
	References         []Reference  `json:"references"`
	Glossary           []string     `json:"glossary"`
	Goals              []PublicGoal `json:"goals"`
	HintCount          int          `json:"hintCount"`
	Reward             Reward       `json:"reward"`
	SimulatorVersion   string       `json:"simulatorVersion"`
	LastReviewedAt     string       `json:"lastReviewedAt"`
	Navigation         Navigation   `json:"navigation"`
}

// publicChallenge decodes only learner-visible fields of the stored spec. Anything not listed
// here (goal targets, assertions, expected answers, hint text, solution) is never decoded.
type publicChallenge struct {
	Challenge struct {
		Title            string            `json:"title"`
		Summary          string            `json:"summary"`
		EstimatedMinutes int32             `json:"estimatedMinutes"`
		LastReviewed     string            `json:"lastReviewed"`
		Objectives       []string          `json:"objectives"`
		Prerequisites    []string          `json:"prerequisites"`
		Body             string            `json:"body"`
		References       []Reference       `json:"references"`
		Glossary         []string          `json:"glossary"`
		SimulatorVersion string            `json:"simulatorVersion"`
		Goals            []PublicGoal      `json:"goals"`
		Hints            []json.RawMessage `json:"hints"`
		Reward           struct {
			BaseXP               int `json:"baseXP"`
			FirstTryBonusXP      int `json:"firstTryBonusXP"`
			HintPenaltyPerHintXP int `json:"hintPenaltyPerHintXP"`
		} `json:"reward"`
	} `json:"challenge"`
}

func decodePublic(spec []byte) (publicChallenge, error) {
	var p publicChallenge
	if err := json.Unmarshal(spec, &p); err != nil {
		return p, fmt.Errorf("decode challenge spec: %w", err)
	}
	for i, g := range p.Challenge.Goals {
		// Choices are only meaningful (and only public) for choice inputs.
		if g.Input != "choice" {
			p.Challenge.Goals[i].Choices = nil
		}
	}
	return p, nil
}

func goalTypes(goals []PublicGoal) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, g := range goals {
		if !seen[g.Type] {
			seen[g.Type] = true
			out = append(out, g.Type)
		}
	}
	return out
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// ErrNotFound covers missing and unpublished courses and lessons alike.
var ErrNotFound = errors.New("not found")

// Service reads the published catalog.
type Service struct{ pool *pgxpool.Pool }

// NewService returns a Service.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type lessonRow struct {
	summary LessonSummary
	row     dbgen.ListPublishedLessonsRow
	public  publicChallenge
}

func (s *Service) publishedLessons(ctx context.Context, courseID *uuid.UUID) ([]lessonRow, error) {
	rows, err := dbgen.New(s.pool).ListPublishedLessons(ctx, courseID)
	if err != nil {
		return nil, err
	}
	out := make([]lessonRow, 0, len(rows))
	for _, r := range rows {
		p, err := decodePublic(r.Spec)
		if err != nil {
			return nil, err
		}
		out = append(out, lessonRow{row: r, public: p, summary: LessonSummary{
			Slug: r.Slug, CourseSlug: r.CourseSlug, Title: p.Challenge.Title, Summary: p.Challenge.Summary,
			EstimatedMinutes: r.EstimatedMinutes, DisplayOrder: r.DisplayOrder, GoalTypes: goalTypes(p.Challenge.Goals),
		}})
	}
	return out, nil
}

// ListCourses returns courses with at least one published lesson.
func (s *Service) ListCourses(ctx context.Context) ([]CourseSummary, error) {
	rows, err := dbgen.New(s.pool).ListPublishedCourses(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CourseSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, CourseSummary{
			Slug: r.Slug, Title: r.Title, Summary: r.Summary, Difficulty: r.Difficulty,
			LessonCount: r.PublishedLessons, PlannedLessonCount: r.PlannedLessons,
			EstimatedMinutes: r.EstimatedMinutes, DisplayOrder: r.DisplayOrder,
		})
	}
	return out, nil
}

// GetCourse returns a published course with its published lessons.
func (s *Service) GetCourse(ctx context.Context, slug string) (CourseDetail, error) {
	rows, err := dbgen.New(s.pool).ListPublishedCourses(ctx)
	if err != nil {
		return CourseDetail{}, err
	}
	for _, r := range rows {
		if r.Slug != slug {
			continue
		}
		lessons, err := s.publishedLessons(ctx, &r.ID)
		if err != nil {
			return CourseDetail{}, err
		}
		d := CourseDetail{
			CourseSummary: CourseSummary{
				Slug: r.Slug, Title: r.Title, Summary: r.Summary, Difficulty: r.Difficulty,
				LessonCount: r.PublishedLessons, PlannedLessonCount: r.PlannedLessons,
				EstimatedMinutes: r.EstimatedMinutes, DisplayOrder: r.DisplayOrder,
			},
			Description: r.Description, Prerequisites: orEmpty(r.Prerequisites), Lessons: []LessonSummary{},
		}
		for _, l := range lessons {
			d.Lessons = append(d.Lessons, l.summary)
		}
		return d, nil
	}
	return CourseDetail{}, ErrNotFound
}

// GetLesson returns a published lesson's public content and its curriculum neighbours.
func (s *Service) GetLesson(ctx context.Context, courseSlug, lessonSlug string) (LessonDetail, error) {
	all, err := s.publishedLessons(ctx, nil)
	if err != nil {
		return LessonDetail{}, err
	}
	for i, l := range all {
		if l.row.CourseSlug != courseSlug || l.row.Slug != lessonSlug {
			continue
		}
		c := l.public.Challenge
		d := LessonDetail{
			LessonSummary: l.summary, ChallengeVersionID: l.row.VersionID, ContentSHA256: l.row.ContentSha256,
			Objectives: orEmpty(c.Objectives), Prerequisites: orEmpty(c.Prerequisites), Body: c.Body,
			References: orEmpty(c.References), Glossary: orEmpty(c.Glossary), Goals: orEmpty(c.Goals),
			HintCount: len(c.Hints), SimulatorVersion: c.SimulatorVersion, LastReviewedAt: c.LastReviewed,
			Reward: Reward{BaseXP: c.Reward.BaseXP, FirstTryBonusXP: c.Reward.FirstTryBonusXP, HintPenaltyPerHintXP: c.Reward.HintPenaltyPerHintXP},
		}
		ref := func(x lessonRow) *LessonRef {
			return &LessonRef{CourseSlug: x.row.CourseSlug, Slug: x.row.Slug, Title: x.summary.Title}
		}
		d.PrereqLessons = []LessonRef{}
		for _, p := range d.Prerequisites {
			for _, x := range all {
				if x.row.CourseSlug+"/"+x.row.Slug == p {
					d.PrereqLessons = append(d.PrereqLessons, *ref(x))
				}
			}
		}
		if i > 0 {
			d.Navigation.Previous = ref(all[i-1])
		}
		if i+1 < len(all) {
			d.Navigation.Next = ref(all[i+1])
		}
		return d, nil
	}
	return LessonDetail{}, ErrNotFound
}

// Catalog is the read interface the handlers need (fakeable in router tests).
type Catalog interface {
	ListCourses(ctx context.Context) ([]CourseSummary, error)
	GetCourse(ctx context.Context, slug string) (CourseDetail, error)
	GetLesson(ctx context.Context, courseSlug, lessonSlug string) (LessonDetail, error)
}

// Handler serves the public catalog routes.
type Handler struct{ Catalog Catalog }

// ListCourses handles GET /catalog/courses.
func (h Handler) ListCourses(w http.ResponseWriter, r *http.Request) error {
	cs, err := h.Catalog.ListCourses(r.Context())
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, cs)
	return nil
}

// GetCourse handles GET /catalog/courses/{courseSlug}.
func (h Handler) GetCourse(w http.ResponseWriter, r *http.Request) error {
	c, err := h.Catalog.GetCourse(r.Context(), chi.URLParam(r, "courseSlug"))
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("Course")
	}
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, c)
	return nil
}

// GetLesson handles GET /catalog/courses/{courseSlug}/lessons/{lessonSlug}.
func (h Handler) GetLesson(w http.ResponseWriter, r *http.Request) error {
	l, err := h.Catalog.GetLesson(r.Context(), chi.URLParam(r, "courseSlug"), chi.URLParam(r, "lessonSlug"))
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("Lesson")
	}
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WriteJSON(w, r, http.StatusOK, l)
	return nil
}
