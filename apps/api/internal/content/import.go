package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
)

// Report summarizes an import.
type Report struct {
	Courses         int
	NewVersions     int
	Unchanged       int
	Published       int
	PlannedLessons  int
	PublishedLabels []string
}

// ErrVersionReused means content changed but the challenge's version number didn't.
var ErrVersionReused = errors.New("content changed without a new version number")

// Import upserts courses and lessons and inserts any new challenge versions as drafts, all in
// one transaction. With publish=true each lesson's newest imported version is also published
// (operator path; the admin UI arrives in Part 18) and the previous current version retired.
func Import(ctx context.Context, pool *pgxpool.Pool, b Bundle, publish bool) (Report, error) {
	var r Report
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		r = Report{}
		q := dbgen.New(tx)
		for _, c := range b.Courses {
			courseID, err := q.UpsertCourse(ctx, dbgen.UpsertCourseParams{
				Slug: c.Slug, Title: c.Title, Summary: c.Summary, Description: c.Description,
				Difficulty: c.Difficulty, DisplayOrder: c.DisplayOrder, Prerequisites: nonNil(c.Prerequisites),
			})
			if err != nil {
				return fmt.Errorf("course %s: %w", c.Slug, err)
			}
			if err := q.SetCoursePublished(ctx, dbgen.SetCoursePublishedParams{ID: courseID, Published: true}); err != nil {
				return err
			}
			r.Courses++

			for i, slug := range c.Lessons {
				order := int32(i + 1)
				ch, written := b.Challenges[c.Slug+"/"+slug]
				if !written {
					// Planned lesson: keep its seeded shell in the right position if it exists.
					r.PlannedLessons++
					l, err := q.GetLessonBySlugs(ctx, dbgen.GetLessonBySlugsParams{CourseSlug: c.Slug, LessonSlug: slug})
					if errors.Is(err, pgx.ErrNoRows) {
						continue
					}
					if err != nil {
						return err
					}
					if err := q.SetLessonDisplayOrder(ctx, dbgen.SetLessonDisplayOrderParams{ID: l.ID, DisplayOrder: order}); err != nil {
						return err
					}
					continue
				}
				lessonID, err := q.UpsertLesson(ctx, dbgen.UpsertLessonParams{
					CourseID: courseID, Slug: slug, Title: ch.Title, Summary: ch.Summary,
					DisplayOrder: order, EstimatedMinutes: ch.EstimatedMinutes,
				})
				if err != nil {
					return fmt.Errorf("lesson %s: %w", ch.ID, err)
				}
				versionID, created, err := insertVersion(ctx, q, lessonID, ch)
				if err != nil {
					return err
				}
				if created {
					r.NewVersions++
				} else {
					r.Unchanged++
				}
				if publish {
					published, err := publishVersion(ctx, q, lessonID, versionID, ch)
					if err != nil {
						return err
					}
					if published {
						r.Published++
						r.PublishedLabels = append(r.PublishedLabels, fmt.Sprintf("%s v%d", ch.ID, ch.Version))
					}
				}
			}
		}
		return nil
	})
	return r, err
}

func insertVersion(ctx context.Context, q *dbgen.Queries, lessonID uuid.UUID, ch Challenge) (id uuid.UUID, created bool, err error) {
	existing, err := q.GetChallengeVersionBySHA(ctx, dbgen.GetChallengeVersionBySHAParams{LessonID: lessonID, ContentSha256: ch.SHA256})
	if err == nil {
		if existing.Version != ch.Version {
			return id, false, fmt.Errorf("%s: identical content is already imported as version %d", ch.ID, existing.Version)
		}
		return existing.ID, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return id, false, err
	}
	if _, err := q.GetChallengeVersionByNumber(ctx, dbgen.GetChallengeVersionByNumberParams{LessonID: lessonID, Version: ch.Version}); err == nil {
		return id, false, fmt.Errorf("%s version %d: %w; bump `version` in the challenge file", ch.ID, ch.Version, ErrVersionReused)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return id, false, err
	}
	newID, err := q.InsertChallengeVersion(ctx, dbgen.InsertChallengeVersionParams{
		LessonID: lessonID, Version: ch.Version, ContentSha256: ch.SHA256,
		Spec: ch.Spec, SimulatorVersion: ch.SimulatorVersion,
	})
	if err != nil {
		return id, false, fmt.Errorf("insert %s v%d: %w", ch.ID, ch.Version, err)
	}
	return newID, true, nil
}

func publishVersion(ctx context.Context, q *dbgen.Queries, lessonID, versionID uuid.UUID, ch Challenge) (bool, error) {
	lesson, err := q.GetLessonBySlugs(ctx, dbgen.GetLessonBySlugsParams{CourseSlug: ch.Course, LessonSlug: ch.Lesson})
	if err != nil {
		return false, err
	}
	if lesson.CurrentChallengeVersionID != nil && *lesson.CurrentChallengeVersionID == versionID {
		return false, nil
	}
	if _, err := q.PublishChallengeVersion(ctx, versionID); err != nil {
		return false, err
	}
	if err := q.SetLessonCurrentVersion(ctx, dbgen.SetLessonCurrentVersionParams{LessonID: lessonID, ChallengeVersionID: &versionID}); err != nil {
		return false, err
	}
	var oldRef *string
	if prev := lesson.CurrentChallengeVersionID; prev != nil {
		if err := q.RetireChallengeVersion(ctx, *prev); err != nil {
			return false, err
		}
		s := prev.String()
		oldRef = &s
	}
	newRef := versionID.String()
	details, _ := json.Marshal(map[string]any{"lesson": ch.ID, "version": ch.Version, "sha256": ch.SHA256})
	_, err = q.InsertAuditLog(ctx, dbgen.InsertAuditLogParams{
		ActorKind: "operator", Action: "content.published", EntityType: "lesson", EntityID: ch.ID,
		OldRef: oldRef, NewRef: &newRef, Details: details,
	})
	return true, err
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
