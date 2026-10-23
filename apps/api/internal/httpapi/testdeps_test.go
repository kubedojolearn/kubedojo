package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth/authtest"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/courses"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/health"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/ratelimit"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/progress"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/users"
)

var (
	testUserID = uuid.MustParse("33333333-3333-4333-8333-333333333333")
	throttled  = uuid.MustParse("44444444-4444-4444-8444-444444444444")
)

type identities struct{}

func (identities) Resolve(_ context.Context, _, subject string) (auth.Principal, error) {
	switch subject {
	case "100":
		return auth.Principal{UserID: testUserID, Role: auth.RoleLearner}, nil
	case "429":
		return auth.Principal{UserID: throttled, Role: auth.RoleLearner}, nil
	}
	return auth.Principal{}, auth.ErrIdentityNotLinked
}

type userStore struct{}

func (userStore) GetMe(_ context.Context, id uuid.UUID) (users.Me, error) {
	avatar := "https://avatars.githubusercontent.com/u/100"
	return users.Me{
		ID: id, Handle: "kube-learner", DisplayName: "Kube Learner", AvatarURL: &avatar, Role: auth.RoleLearner,
		CreatedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
		Settings:  users.Settings{Theme: "system", TimeZone: "UTC"},
	}, nil
}

func (userStore) UpdateMe(_ context.Context, _ uuid.UUID, r users.UpdateMeRequest) error {
	if r.Handle != nil && *r.Handle == "taken" {
		return users.ErrHandleTaken
	}
	return nil
}

func (s userStore) Provision(_ context.Context, in users.ProvisionInput) (users.ProvisionResult, error) {
	if in.ProviderSubject == "100" {
		return users.ProvisionResult{UserID: testUserID}, nil
	}
	return users.ProvisionResult{UserID: testUserID, Created: true}, nil
}

type catalog struct{}

func (catalog) ListCourses(context.Context) ([]courses.CourseSummary, error) {
	return []courses.CourseSummary{{Slug: "workloads", Title: "Pods and workloads", Summary: "Run apps.", Difficulty: "beginner", LessonCount: 1, PlannedLessonCount: 6, EstimatedMinutes: 8, DisplayOrder: 2}}, nil
}

func (catalog) GetCourse(_ context.Context, slug string) (courses.CourseDetail, error) {
	if slug != "workloads" {
		return courses.CourseDetail{}, courses.ErrNotFound
	}
	list, _ := catalog{}.ListCourses(context.Background())
	return courses.CourseDetail{CourseSummary: list[0], Description: "x", Prerequisites: []string{"fundamentals"}, Lessons: []courses.LessonSummary{
		{Slug: "scaling-deployments", CourseSlug: "workloads", Title: "Scale", Summary: "s", EstimatedMinutes: 8, DisplayOrder: 5, GoalTypes: []string{"state"}},
	}}, nil
}

func (catalog) GetLesson(_ context.Context, c, l string) (courses.LessonDetail, error) {
	if c != "workloads" || l != "scaling-deployments" {
		return courses.LessonDetail{}, courses.ErrNotFound
	}
	return courses.LessonDetail{
		LessonSummary:      courses.LessonSummary{Slug: l, CourseSlug: c, Title: "Scale", Summary: "s", EstimatedMinutes: 8, DisplayOrder: 5, GoalTypes: []string{"answer"}},
		ChallengeVersionID: uuid.MustParse("55555555-5555-4555-8555-555555555555"),
		ContentSHA256:      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Objectives:         []string{"o"}, Prerequisites: []string{"workloads/create-a-deployment"}, PrereqLessons: []courses.LessonRef{}, Body: "# hi",
		References: []courses.Reference{{Title: "docs", URL: "https://kubernetes.io/"}}, Glossary: []string{"pod"},
		Goals:     []courses.PublicGoal{{ID: "q", Type: "answer", Description: "Pick.", Input: "choice", Choices: []string{"a", "b"}}},
		HintCount: 3, Reward: courses.Reward{BaseXP: 100, FirstTryBonusXP: 25, HintPenaltyPerHintXP: 10},
		SimulatorVersion: "kubernetes-1.37-subset-1", LastReviewedAt: "2026-10-23",
		Navigation: courses.Navigation{Previous: &courses.LessonRef{CourseSlug: c, Slug: "inspect-a-pod", Title: "Inspect"}},
	}, nil
}

type progressFake struct{}

func (progressFake) Progress(context.Context, uuid.UUID, *string, *progress.LessonCursor, int32) (progress.Progress, *progress.LessonCursor, error) {
	return progress.Progress{
		Courses:           []progress.CourseProgress{{CourseSlug: "workloads", CompletedLessons: 0, TotalLessons: 2}},
		Lessons:           []progress.LessonProgress{{LessonID: "workloads/scaling-deployments", CourseSlug: "workloads", LessonSlug: "scaling-deployments", Status: "not-started"}},
		Totals:            progress.Totals{},
		RecommendedLesson: &progress.LessonPointer{CourseSlug: "workloads", Slug: "scaling-deployments", Title: "Scale"},
	}, &progress.LessonCursor{CourseOrder: 2, LessonOrder: 5}, nil
}

func (progressFake) MarkOpened(_ context.Context, _ uuid.UUID, c, l string, _ bool) error {
	if c != "workloads" || l != "scaling-deployments" {
		return progress.ErrNotFound
	}
	return nil
}

func (progressFake) Achievements(context.Context, uuid.UUID) ([]progress.Achievement, error) {
	return []progress.Achievement{{Key: "first-lesson", Title: "First steps", Description: "d", Icon: "Footprints"}}, nil
}

func (progressFake) Activity(context.Context, uuid.UUID, *progress.ActivityCursor, int32) ([]progress.ActivityItem, *progress.ActivityCursor, error) {
	id := "workloads/scaling-deployments"
	return []progress.ActivityItem{{Type: "lesson-opened", LessonID: &id, OccurredAt: time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)}}, nil, nil
}

func (progressFake) PublicProfile(_ context.Context, handle string) (progress.PublicProfile, error) {
	if handle != "public-learner" {
		return progress.PublicProfile{}, progress.ErrNotFound
	}
	return progress.PublicProfile{Handle: handle, DisplayName: "P", TotalXP: 10, CompletedLessons: 1, Achievements: []progress.Achievement{}, JoinedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}, nil
}

func (userStore) AnalyticsOptOut(context.Context, uuid.UUID) (bool, error) { return false, nil }

type limiter struct{}

func (limiter) Allow(ctx context.Context, key string, _ int, _ time.Duration) (ratelimit.Decision, error) {
	if key == "api:user:"+throttled.String() {
		return ratelimit.Decision{Allowed: false, RetryAfter: 30 * time.Second}, nil
	}
	return ratelimit.Decision{Allowed: true}, nil
}

func pinger(err error) health.Pinger {
	return health.PingFunc(func(context.Context) error { return err })
}

// testRouter returns a router wired with in-memory fakes and a signer whose tokens it accepts.
func testRouter(t *testing.T, ready bool) (http.Handler, authtest.Signer) {
	t.Helper()
	s := authtest.NewSigner(t, "test-1")
	var pgErr error
	if !ready {
		pgErr = errors.New("down")
	}
	return NewRouter(Deps{
		Log:            slog.New(slog.DiscardHandler),
		Health:         health.Handler{Postgres: pinger(pgErr), Redis: pinger(nil)},
		Verifier:       auth.NewVerifier(s.Keys(), nil),
		Identities:     identities{},
		Users:          userStore{},
		Catalog:        catalog{},
		Progress:       progressFake{},
		Settings:       userStore{},
		Cursors:        httpx.NewCursors([]byte("0123456789abcdef0123456789abcdef")),
		Limiter:        limiter{},
		RequestTimeout: 5 * time.Second,
		MaxBodyBytes:   128 << 10,
	}), s
}
