package testdb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
)

// SimulatorVersion used by fixtures.
const SimulatorVersion = "kubernetes-1.37-subset-1"

// CreateUser inserts an active user with a linked GitHub identity, settings and totals.
func CreateUser(t testing.TB, pool *pgxpool.Pool, handle string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	q := dbgen.New(pool)
	id, err := q.InsertUser(ctx, dbgen.InsertUserParams{Handle: handle, DisplayName: handle})
	if err != nil {
		t.Fatalf("insert user %s: %v", handle, err)
	}
	subject := strings.TrimLeft(strings.ReplaceAll(uuid.NewString(), "-", "")[:12], "0")
	subject = "1" + strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return '7'
	}, subject)
	if err := q.InsertIdentity(ctx, dbgen.InsertIdentityParams{Provider: "github", ProviderSubject: subject, UserID: id, ProviderLogin: handle}); err != nil {
		t.Fatal(err)
	}
	if err := q.InsertUserSettings(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := q.InsertUserTotals(ctx, id); err != nil {
		t.Fatal(err)
	}
	return id
}

// Lesson is a course + lesson + published challenge version.
type Lesson struct {
	CourseID, LessonID, VersionID uuid.UUID
}

// CreatePublishedLesson inserts a course and lesson with one published challenge version.
func CreatePublishedLesson(t testing.TB, pool *pgxpool.Pool, slug string) Lesson {
	t.Helper()
	ctx := context.Background()
	q := dbgen.New(pool)
	courseID, err := q.UpsertCourse(ctx, dbgen.UpsertCourseParams{
		Slug: "course-" + slug, Title: "Course " + slug, Summary: "A test course.", Difficulty: "beginner", DisplayOrder: 1, Prerequisites: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	lessonID, err := q.UpsertLesson(ctx, dbgen.UpsertLessonParams{
		CourseID: courseID, Slug: slug, Title: "Lesson " + slug, DisplayOrder: 1, EstimatedMinutes: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	versionID := InsertVersion(t, pool, lessonID, 1, `{"id":"`+slug+`"}`)
	if n, err := q.PublishChallengeVersion(ctx, versionID); err != nil || n != 1 {
		t.Fatalf("publish: %v (%d rows)", err, n)
	}
	if err := q.SetLessonCurrentVersion(ctx, dbgen.SetLessonCurrentVersionParams{LessonID: lessonID, ChallengeVersionID: &versionID}); err != nil {
		t.Fatal(err)
	}
	return Lesson{CourseID: courseID, LessonID: lessonID, VersionID: versionID}
}

// InsertVersion inserts a draft challenge version with the given JSON spec.
func InsertVersion(t testing.TB, pool *pgxpool.Pool, lessonID uuid.UUID, version int32, spec string) uuid.UUID {
	t.Helper()
	sum := sha256.Sum256([]byte(spec))
	id, err := dbgen.New(pool).InsertChallengeVersion(context.Background(), dbgen.InsertChallengeVersionParams{
		LessonID: lessonID, Version: version, ContentSha256: hex.EncodeToString(sum[:]),
		Spec: json.RawMessage(spec), SimulatorVersion: SimulatorVersion,
	})
	if err != nil {
		t.Fatalf("insert challenge version: %v", err)
	}
	return id
}

// CreateLab inserts an active lab session for user on the lesson's published version.
func CreateLab(t testing.TB, pool *pgxpool.Pool, userID uuid.UUID, l Lesson) dbgen.LabSession {
	t.Helper()
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s, err := dbgen.New(pool).CreateLabSession(context.Background(), dbgen.CreateLabSessionParams{
		UserID: userID, LessonID: l.LessonID, ChallengeVersionID: l.VersionID, SimulatorVersion: SimulatorVersion,
		Seed: 42, SimTime: epoch, ExpiresAt: time.Now().Add(14 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create lab: %v", err)
	}
	return s
}

// CreateLaunchableLesson inserts a published course and lesson whose current published
// challenge version embeds fixtureJSON, so labs can be started from it.
func CreateLaunchableLesson(t testing.TB, pool *pgxpool.Pool, slug, fixtureJSON string) Lesson {
	t.Helper()
	ctx := context.Background()
	q := dbgen.New(pool)
	l := CreatePublishedLesson(t, pool, slug)
	if err := q.SetCoursePublished(ctx, dbgen.SetCoursePublishedParams{ID: l.CourseID, Published: true}); err != nil {
		t.Fatal(err)
	}
	l.VersionID = PublishVersion(t, pool, l.LessonID, 2, fixtureJSON)
	return l
}

// PublishVersion inserts a challenge version with the fixture, publishes it and makes it the
// lesson's current version.
func PublishVersion(t testing.TB, pool *pgxpool.Pool, lessonID uuid.UUID, version int32, fixtureJSON string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	q := dbgen.New(pool)
	spec := `{"challenge":{"version":` + strconv.Itoa(int(version)) + `},"fixture":` + fixtureJSON + `}`
	id := InsertVersion(t, pool, lessonID, version, spec)
	if n, err := q.PublishChallengeVersion(ctx, id); err != nil || n != 1 {
		t.Fatalf("publish: %v (%d rows)", err, n)
	}
	if err := q.SetLessonCurrentVersion(ctx, dbgen.SetLessonCurrentVersionParams{LessonID: lessonID, ChallengeVersionID: &id}); err != nil {
		t.Fatal(err)
	}
	return id
}
