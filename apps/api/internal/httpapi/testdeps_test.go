package httpapi

import (
	"context"
	"encoding/json"
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
	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
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

var (
	testLabID    = uuid.MustParse("66666666-6666-4666-8666-666666666666")
	endedLabID   = uuid.MustParse("77777777-7777-4777-8777-777777777777")
	expiredLabID = uuid.MustParse("88888888-8888-4888-8888-888888888888")
	testVersion  = uuid.MustParse("55555555-5555-4555-8555-555555555555")
	staleVersion = uuid.MustParse("99999999-9999-4999-8999-999999999999")
)

type labsFake struct{}

func fakeLab(state string) labs.Lab {
	t := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	l := labs.Lab{
		ID: testLabID, LessonID: "workloads/scaling-deployments", CourseSlug: "workloads", LessonSlug: "scaling-deployments",
		Title: "Scale", ChallengeVersionID: testVersion, State: state, Version: 1, SimulatorVersion: "kubernetes-1.37-subset-1",
		StartedAt: t, LastActiveAt: t, ExpiresAt: t.Add(labs.IdleTTL),
	}
	if state != "active" {
		l.EndedAt = &t
	}
	return l
}

func labErr(id uuid.UUID) error {
	switch id {
	case testLabID:
		return nil
	case endedLabID:
		return labs.ErrNotActive
	case expiredLabID:
		return labs.ErrExpired
	}
	return labs.ErrNotFound
}

func (labsFake) Create(_ context.Context, _, cv uuid.UUID) (labs.Lab, bool, error) {
	switch cv {
	case testVersion:
		return fakeLab("active"), true, nil
	case staleVersion:
		return labs.Lab{}, false, labs.ErrStaleChallenge
	}
	return labs.Lab{}, false, labs.ErrChallengeNotFound
}

func (labsFake) List(context.Context, uuid.UUID, *string, *labs.ListCursor, int32) ([]labs.Lab, *labs.ListCursor, error) {
	l := fakeLab("active")
	return []labs.Lab{l}, &labs.ListCursor{LastActiveAt: l.LastActiveAt, ID: l.ID}, nil
}

func (labsFake) Get(_ context.Context, _, id uuid.UUID) (labs.Lab, error) {
	return fakeLab("active"), labErr(id)
}

func (labsFake) Resume(_ context.Context, _, id uuid.UUID) (labs.Lab, error) {
	return fakeLab("active"), labErr(id)
}

func (labsFake) Reset(_ context.Context, _, id uuid.UUID, key string) (labs.Lab, error) {
	if key == "reused-key" {
		return labs.Lab{}, labs.ErrIdempotencyConflict
	}
	l := fakeLab("active")
	l.Version = 2
	return l, labErr(id)
}

func (labsFake) Finish(_ context.Context, _, id uuid.UUID, _ string) (labs.Lab, error) {
	return fakeLab("abandoned"), labErr(id)
}

func (labsFake) Snapshot(_ context.Context, _, id uuid.UUID) (labs.Snapshot, error) {
	return labs.Snapshot{
		LabID: testLabID, Version: 1, SimulatorVersion: "kubernetes-1.37-subset-1", SimTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrentNamespace: "default",
		Resources:        []json.RawMessage{json.RawMessage(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"default","uid":"0b6f2c1e-4c49-4d8a-9a51-1f1d2f3e4a5b","resourceVersion":"1","creationTimestamp":"2025-12-02T00:00:00Z","annotations":{"kubedojo.dev/simulated":"true"}},"status":{"phase":"Active"}}`)},
	}, labErr(id)
}

func (labsFake) Events(_ context.Context, _, id uuid.UUID, _ int64, _ int32) ([]labs.SimEvent, *int64, error) {
	next := int64(1)
	return []labs.SimEvent{{
		Seq: 1, Version: 1, Type: "Normal", Reason: "Scheduled", Message: "Successfully assigned default/web-1 to node-2",
		InvolvedObject: labs.InvolvedObject{Kind: "Pod", Namespace: "default", Name: "web-1"}, OccurredAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}}, &next, labErr(id)
}

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
		Labs:           labsFake{},
		Settings:       userStore{},
		Cursors:        httpx.NewCursors([]byte("0123456789abcdef0123456789abcdef")),
		Limiter:        limiter{},
		RequestTimeout: 5 * time.Second,
		MaxBodyBytes:   128 << 10,
	}), s
}
