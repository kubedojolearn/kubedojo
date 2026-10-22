// Package seed loads idempotent reference data: unpublished course and lesson shells from the
// V1 outline (docs/product/course-outline.md) and achievement definitions. Lesson content and
// publishing come from the content import (Part 07), which updates these rows by slug.
package seed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
)

type lesson struct{ slug, title string }

type course struct {
	slug, title, summary, difficulty string
	lessons                          []lesson
}

// Shell lessons get a placeholder estimate; the content import replaces it.
const shellEstimatedMinutes = 10

var courses = []course{
	{"fundamentals", "Kubernetes fundamentals", "How a cluster is put together, how to find your way around it, and how to describe what you want.", "beginner", []lesson{
		{"cluster-architecture", "How a cluster is put together"},
		{"namespace-navigation", "Finding your way with namespaces"},
		{"listing-resources", "Listing and reading resources"},
		{"labels-and-selectors", "Labels and selectors"},
		{"declarative-vs-imperative", "Imperative commands vs. declarative manifests"},
	}},
	{"workloads", "Pods and workloads", "Run, inspect, scale and roll out applications with Pods and Deployments.", "beginner", []lesson{
		{"launch-a-pod", "Launch your first Pod"},
		{"inspect-a-pod", "Inspect a running Pod"},
		{"troubleshoot-pending-pod", "Why is my Pod Pending?"},
		{"create-a-deployment", "Create a Deployment"},
		{"scaling-deployments", "Scale a Deployment"},
		{"rollout-and-rollback", "Roll out and roll back"},
	}},
	{"networking", "Networking", "Give Pods a stable address with Services, and work out why traffic isn't arriving.", "beginner", []lesson{
		{"service-basics", "What a Service is"},
		{"expose-a-deployment", "Expose a Deployment"},
		{"selectors-and-endpoints", "How Services find Pods"},
		{"clusterip-vs-nodeport", "ClusterIP vs. NodePort"},
		{"debug-missing-endpoints", "Debug a Service with no endpoints"},
	}},
	{"configuration", "Configuration", "Keep configuration out of images with ConfigMaps, edit manifests safely, and understand Secrets.", "intermediate", []lesson{
		{"configmaps", "Store configuration in a ConfigMap"},
		{"editing-manifests", "Edit and re-apply a manifest"},
		{"environment-from-configmaps", "Feed environment variables from a ConfigMap"},
		{"secrets-awareness", "Secrets are not encryption"},
	}},
	{"cluster-operations", "Cluster operations", "Inspect nodes, scopes, labels and scheduling, and read the events that explain what happened.", "intermediate", []lesson{
		{"inspect-nodes", "Inspect Nodes"},
		{"namespaces-and-scope", "Namespaced vs. cluster-scoped"},
		{"labels-and-annotations", "Labels vs. annotations"},
		{"requests-limits-scheduling", "Requests, limits and scheduling"},
		{"events-and-diagnostics", "Read events like a detective"},
	}},
	{"troubleshooting", "Troubleshooting", "Fix the failures you'll meet in real clusters, from image pull errors to a full outage.", "intermediate", []lesson{
		{"image-pull-failure", "Fix an image pull failure"},
		{"crashloopbackoff", "Fix a CrashLoopBackOff"},
		{"wrong-labels", "A Deployment whose Pods nobody can reach"},
		{"replica-mismatch", "Desired 4, ready 1"},
		{"capstone-recovery", "Capstone: bring the shop back online"},
	}},
}

type achievement struct {
	key, title, description, icon string
	condition                     map[string]any
}

var achievements = []achievement{
	{"first-lesson", "First steps", "Complete your first lesson.", "Footprints", map[string]any{"type": "lessons_completed", "count": 1}},
	{"five-lessons", "Getting the hang of it", "Complete five lessons.", "Layers", map[string]any{"type": "lessons_completed", "count": 5}},
	{"first-course", "Course complete", "Complete every lesson in a course.", "GraduationCap", map[string]any{"type": "courses_completed", "count": 1}},
	{"troubleshooter", "Troubleshooter", "Complete the Troubleshooting course.", "Wrench", map[string]any{"type": "course_completed", "course": "troubleshooting"}},
	{"all-lessons", "Black belt", "Complete all 30 lessons.", "Award", map[string]any{"type": "lessons_completed", "count": 30}},
	{"no-hints", "No help needed", "Complete a lesson without revealing a hint.", "Lightbulb", map[string]any{"type": "lessons_without_hints", "count": 1}},
	{"first-try", "First try", "Pass a lesson on your first check.", "Target", map[string]any{"type": "first_try_passes", "count": 1}},
	{"streak-3", "On a roll", "Complete lessons three days in a row.", "Flame", map[string]any{"type": "streak", "days": 3}},
	{"streak-7", "Week of practice", "Complete lessons seven days in a row.", "CalendarCheck", map[string]any{"type": "streak", "days": 7}},
}

// Counts reports what Run wrote, for logs.
type Counts struct{ Courses, Lessons, Achievements int }

// Run upserts all reference data in one transaction. Running it again changes nothing new.
func Run(ctx context.Context, pool *pgxpool.Pool) (Counts, error) {
	var c Counts
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		c = Counts{}
		q := dbgen.New(tx)
		for i, co := range courses {
			courseID, err := q.UpsertCourse(ctx, dbgen.UpsertCourseParams{
				Slug: co.slug, Title: co.title, Summary: co.summary, Description: "",
				Difficulty: co.difficulty, DisplayOrder: int32(i + 1),
			})
			if err != nil {
				return fmt.Errorf("course %s: %w", co.slug, err)
			}
			c.Courses++
			for j, l := range co.lessons {
				if _, err := q.UpsertLesson(ctx, dbgen.UpsertLessonParams{
					CourseID: courseID, Slug: l.slug, Title: l.title, Summary: "",
					DisplayOrder: int32(j + 1), EstimatedMinutes: shellEstimatedMinutes,
				}); err != nil {
					return fmt.Errorf("lesson %s/%s: %w", co.slug, l.slug, err)
				}
				c.Lessons++
			}
		}
		for i, a := range achievements {
			cond, err := json.Marshal(a.condition)
			if err != nil {
				return err
			}
			if err := q.UpsertAchievementDefinition(ctx, dbgen.UpsertAchievementDefinitionParams{
				Key: a.key, Title: a.title, Description: a.description, Icon: a.icon, Condition: cond, DisplayOrder: int32(i + 1),
			}); err != nil {
				return fmt.Errorf("achievement %s: %w", a.key, err)
			}
			c.Achievements++
		}
		return nil
	})
	return c, err
}
