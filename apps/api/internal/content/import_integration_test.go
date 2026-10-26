//go:build integration

package content_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/content"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/courses"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/seed"
)

var ctx = context.Background()

const contentDir = "../../../../content"

func loadRepoContent(t *testing.T) content.Bundle {
	t.Helper()
	b, err := content.Load(contentDir)
	if err != nil {
		t.Fatalf("load repository content: %v", err)
	}
	return b
}

// partialContent is the repository content with only three lessons written, so tests can check
// how courses with planned (unwritten) lessons are served.
func partialContent(t *testing.T) content.Bundle {
	t.Helper()
	b := loadRepoContent(t)
	keep := map[string]bool{
		"fundamentals/cluster-architecture": true, "workloads/inspect-a-pod": true, "workloads/scaling-deployments": true,
	}
	for id := range b.Challenges {
		if !keep[id] {
			delete(b.Challenges, id)
		}
	}
	return b
}

func TestFullCurriculumImportsAndEveryCourseIsComplete(t *testing.T) {
	pool := testdb.New(t)
	b := loadRepoContent(t)
	if _, err := content.Import(ctx, pool, b, true); err != nil {
		t.Fatalf("import: %v", err)
	}
	list, err := courses.NewService(pool).ListCourses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 6 {
		t.Fatalf("courses %+v", list)
	}
	total := 0
	for _, c := range list {
		if c.LessonCount != c.PlannedLessonCount {
			t.Errorf("%s: %d of %d lessons published", c.Slug, c.LessonCount, c.PlannedLessonCount)
		}
		total += int(c.LessonCount)
	}
	if total != 30 || len(b.Challenges) != 30 {
		t.Fatalf("published %d lessons, loaded %d", total, len(b.Challenges))
	}
}

func TestImportPublishesAndCatalogServesOnlyPublicFields(t *testing.T) {
	pool := testdb.New(t)
	if _, err := seed.Run(ctx, pool); err != nil {
		t.Fatal(err)
	}
	b := partialContent(t)
	rep, err := content.Import(ctx, pool, b, true)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if rep.Courses != 6 || rep.NewVersions != len(b.Challenges) || rep.Published != len(b.Challenges) {
		t.Fatalf("report %+v", rep)
	}

	svc := courses.NewService(pool)
	list, err := svc.ListCourses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Only courses with at least one published lesson are listed.
	if len(list) != 2 || list[0].Slug != "fundamentals" || list[1].Slug != "workloads" {
		t.Fatalf("courses %+v", list)
	}
	if list[1].LessonCount != 2 || list[1].PlannedLessonCount != 6 {
		t.Fatalf("workloads counts %+v", list[1])
	}

	detail, err := svc.GetCourse(ctx, "workloads")
	if err != nil || len(detail.Lessons) != 2 || detail.Lessons[0].Slug != "inspect-a-pod" {
		t.Fatalf("course detail %+v %v", detail, err)
	}

	lesson, err := svc.GetLesson(ctx, "workloads", "scaling-deployments")
	if err != nil {
		t.Fatal(err)
	}
	if lesson.HintCount != 3 || len(lesson.Goals) != 1 || lesson.Goals[0].Type != "state" ||
		lesson.Navigation.Previous == nil || lesson.Navigation.Previous.Slug != "inspect-a-pod" || lesson.Navigation.Next != nil {
		t.Fatalf("lesson %+v", lesson)
	}

	// Hidden fields must not appear anywhere in public responses.
	for _, id := range []string{"fundamentals/cluster-architecture", "workloads/inspect-a-pod", "workloads/scaling-deployments"} {
		c, l, _ := strings.Cut(id, "/")
		d, err := svc.GetLesson(ctx, c, l)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(d)
		for _, secret := range []string{
			"assertions", "expected", "solution", "failureMessage", "target", "readyReplicas",
			"Try: kubectl scale deployment nginx --replicas=3", // hint text
			"kubectl scale deployment nginx --replicas=3\"",    // solution command
			"initialFixture", "imageCatalog", "listening on 0.0.0.0:8080",
		} {
			if strings.Contains(string(out), secret) {
				t.Errorf("%s leaks %q", id, secret)
			}
		}
	}

	// Planned and unknown lessons are not found.
	for _, l := range []string{"launch-a-pod", "does-not-exist"} {
		if _, err := svc.GetLesson(ctx, "workloads", l); !errors.Is(err, courses.ErrNotFound) {
			t.Fatalf("%s: %v", l, err)
		}
	}
	if _, err := svc.GetCourse(ctx, "networking"); !errors.Is(err, courses.ErrNotFound) {
		t.Fatalf("course without published lessons: %v", err)
	}

	var audits int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM audit_logs WHERE action = 'content.published'").Scan(&audits)
	if audits != len(b.Challenges) {
		t.Fatalf("audit entries %d", audits)
	}
}

func TestReimportIsIdempotentAndVersionsAreEnforced(t *testing.T) {
	pool := testdb.New(t)
	b := loadRepoContent(t)
	if _, err := content.Import(ctx, pool, b, true); err != nil {
		t.Fatal(err)
	}
	rep, err := content.Import(ctx, pool, b, true)
	if err != nil || rep.NewVersions != 0 || rep.Published != 0 || rep.Unchanged != len(b.Challenges) {
		t.Fatalf("re-import %+v %v", rep, err)
	}

	// Edited content with the same version number is refused.
	edited := copyContent(t)
	path := filepath.Join(edited, "challenges", "workloads", "scaling-deployments.yaml")
	text, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(text), "Scale a Deployment", "Scale a Deployment, revised", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	eb, err := content.Load(edited)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := content.Import(ctx, pool, eb, true); !errors.Is(err, content.ErrVersionReused) {
		t.Fatalf("expected ErrVersionReused, got %v", err)
	}

	// With a new version it becomes current; the old one is retired, not deleted.
	text, _ = os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(text), "\nversion: 1\n", "\nversion: 2\n", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	eb, _ = content.Load(edited)
	rep, err = content.Import(ctx, pool, eb, true)
	if err != nil || rep.NewVersions != 1 || rep.Published != 1 {
		t.Fatalf("new version %+v %v", rep, err)
	}
	var published, retired int
	_ = pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status = 'published'), count(*) FILTER (WHERE status = 'retired')
		FROM challenge_versions cv JOIN lessons l ON l.id = cv.lesson_id WHERE l.slug = 'scaling-deployments'`).Scan(&published, &retired)
	if published != 1 || retired != 1 {
		t.Fatalf("published %d retired %d", published, retired)
	}
	d, _ := courses.NewService(pool).GetLesson(ctx, "workloads", "scaling-deployments")
	if d.Title != "Scale a Deployment, revised" {
		t.Fatalf("current version not served: %q", d.Title)
	}
}

func TestImportWithoutPublishKeepsDrafts(t *testing.T) {
	pool := testdb.New(t)
	if _, err := content.Import(ctx, pool, loadRepoContent(t), false); err != nil {
		t.Fatal(err)
	}
	list, _ := courses.NewService(pool).ListCourses(ctx)
	if len(list) != 0 {
		t.Fatalf("drafts must not be listed: %+v", list)
	}
}

func copyContent(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(contentDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(contentDir, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o750)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}
