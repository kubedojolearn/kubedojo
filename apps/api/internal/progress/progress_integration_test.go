//go:build integration

package progress_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/content"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/progress"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/seed"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/users"
)

var ctx = context.Background()

func setup(t *testing.T) (*progress.Service, *users.Store, func(subject, login string) users.ProvisionResult) {
	t.Helper()
	pool := testdb.New(t)
	if _, err := seed.Run(ctx, pool); err != nil {
		t.Fatal(err)
	}
	b, err := content.Load("../../../../content")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := content.Import(ctx, pool, b, true); err != nil {
		t.Fatal(err)
	}
	us := users.NewStore(pool)
	mk := func(subject, login string) users.ProvisionResult {
		r, err := us.Provision(ctx, users.ProvisionInput{Provider: "github", ProviderSubject: subject, Login: login, Name: login})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	return progress.NewService(pool), us, mk
}

func TestNewLearnerProgress(t *testing.T) {
	svc, _, mk := setup(t)
	u := mk("1", "newbie")
	p, next, err := svc.Progress(ctx, u.UserID, nil, nil, 20)
	if err != nil {
		t.Fatal(err)
	}
	if next != nil || len(p.Lessons) != 3 {
		t.Fatalf("lessons %d next %v", len(p.Lessons), next)
	}
	for _, l := range p.Lessons {
		if l.Status != "not-started" || l.CompletedAt != nil {
			t.Fatalf("new learner has progress: %+v", l)
		}
	}
	// Only published lessons count toward totals.
	if len(p.Courses) != 2 || p.Courses[1].CourseSlug != "workloads" || p.Courses[1].TotalLessons != 2 || p.Courses[1].CompletedLessons != 0 {
		t.Fatalf("courses %+v", p.Courses)
	}
	if p.Totals.TotalXP != 0 || p.Totals.CurrentStreak != 0 || p.ContinueLesson != nil {
		t.Fatalf("totals %+v continue %+v", p.Totals, p.ContinueLesson)
	}
	if p.RecommendedLesson == nil || p.RecommendedLesson.Slug != "cluster-architecture" {
		t.Fatalf("recommended %+v", p.RecommendedLesson)
	}
}

func TestOpeningLessonsDrivesContinueLearning(t *testing.T) {
	svc, us, mk := setup(t)
	u := mk("2", "reader")
	if err := svc.MarkOpened(ctx, u.UserID, "workloads", "inspect-a-pod", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkOpened(ctx, u.UserID, "workloads", "scaling-deployments", false); err != nil {
		t.Fatal(err)
	}
	p, _, _ := svc.Progress(ctx, u.UserID, nil, nil, 20)
	if p.ContinueLesson == nil || p.ContinueLesson.Slug != "scaling-deployments" || p.ContinueLesson.Title != "Scale a Deployment" {
		t.Fatalf("continue %+v", p.ContinueLesson)
	}
	// Opening is not completing.
	if p.Courses[1].CompletedLessons != 0 {
		t.Fatal("opening a lesson must not complete it")
	}
	items, _, _ := svc.Activity(ctx, u.UserID, nil, 10)
	if len(items) != 2 || items[0].Type != "lesson-opened" || *items[0].LessonID != "workloads/scaling-deployments" {
		t.Fatalf("activity %+v", items)
	}

	// Unpublished and unknown lessons are rejected.
	for _, l := range []string{"launch-a-pod", "nope"} {
		if err := svc.MarkOpened(ctx, u.UserID, "workloads", l, false); !errors.Is(err, progress.ErrNotFound) {
			t.Fatalf("%s: %v", l, err)
		}
	}

	// Analytics opt-out: continue-learning still works, no activity event is recorded.
	other := mk("3", "private")
	optOut := true
	if err := us.UpdateMe(ctx, other.UserID, users.UpdateMeRequest{Settings: &users.SettingsPatch{AnalyticsOptOut: &optOut}}); err != nil {
		t.Fatal(err)
	}
	flag, _ := us.AnalyticsOptOut(ctx, other.UserID)
	if err := svc.MarkOpened(ctx, other.UserID, "workloads", "inspect-a-pod", flag); err != nil {
		t.Fatal(err)
	}
	items, _, _ = svc.Activity(ctx, other.UserID, nil, 10)
	op, _, _ := svc.Progress(ctx, other.UserID, nil, nil, 20)
	if len(items) != 0 || op.ContinueLesson == nil {
		t.Fatalf("opt-out: activity %d continue %+v", len(items), op.ContinueLesson)
	}

	// Learners never see each other's progress or activity.
	op2, _, _ := svc.Progress(ctx, mk("4", "stranger").UserID, nil, nil, 20)
	if op2.ContinueLesson != nil {
		t.Fatal("progress leaked across users")
	}
}

func TestProgressPagination(t *testing.T) {
	svc, _, mk := setup(t)
	u := mk("5", "pager")
	var seen []string
	var after *progress.LessonCursor
	for i := 0; i < 5; i++ {
		p, next, err := svc.Progress(ctx, u.UserID, nil, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range p.Lessons {
			seen = append(seen, l.LessonID)
		}
		if next == nil {
			break
		}
		after = next
	}
	want := []string{"fundamentals/cluster-architecture", "workloads/inspect-a-pod", "workloads/scaling-deployments"}
	if len(seen) != 3 || seen[0] != want[0] || seen[1] != want[1] || seen[2] != want[2] {
		t.Fatalf("paged %v", seen)
	}
	course := "workloads"
	p, _, _ := svc.Progress(ctx, u.UserID, &course, nil, 20)
	if len(p.Lessons) != 2 {
		t.Fatalf("course filter: %d", len(p.Lessons))
	}
}

func TestAchievementsAndProfilePrivacy(t *testing.T) {
	svc, us, mk := setup(t)
	u := mk("6", "Profiler")
	a, err := svc.Achievements(ctx, u.UserID)
	if err != nil || len(a) != 9 || a[0].Key != "first-lesson" || a[0].AwardedAt != nil {
		t.Fatalf("achievements %+v %v", a, err)
	}

	if _, err := svc.PublicProfile(ctx, "profiler"); !errors.Is(err, progress.ErrNotFound) {
		t.Fatalf("private profile visible: %v", err)
	}
	public := true
	if err := us.UpdateMe(ctx, u.UserID, users.UpdateMeRequest{Settings: &users.SettingsPatch{ProfilePublic: &public}}); err != nil {
		t.Fatal(err)
	}
	p, err := svc.PublicProfile(ctx, "PROFILER")
	if err != nil || p.Handle != "Profiler" || p.TotalXP != 0 || len(p.Achievements) != 0 {
		t.Fatalf("public profile %+v %v", p, err)
	}
}
