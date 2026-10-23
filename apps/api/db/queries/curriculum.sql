-- name: UpsertCourse :one
INSERT INTO courses (slug, title, summary, description, difficulty, display_order, prerequisites)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.arg(prerequisites)::text[])
ON CONFLICT (slug) DO UPDATE
SET title = EXCLUDED.title, summary = EXCLUDED.summary, description = EXCLUDED.description,
    difficulty = EXCLUDED.difficulty, display_order = EXCLUDED.display_order, prerequisites = EXCLUDED.prerequisites
RETURNING id;

-- name: UpsertLesson :one
INSERT INTO lessons (course_id, slug, title, summary, display_order, estimated_minutes)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (course_id, slug) DO UPDATE
SET title = EXCLUDED.title, summary = EXCLUDED.summary, display_order = EXCLUDED.display_order,
    estimated_minutes = EXCLUDED.estimated_minutes
RETURNING id;

-- name: InsertChallengeVersion :one
-- Idempotent import: the same content for the same lesson is never stored twice.
INSERT INTO challenge_versions (lesson_id, version, content_sha256, spec, simulator_version)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (lesson_id, content_sha256) DO NOTHING
RETURNING id;

-- name: PublishChallengeVersion :execrows
UPDATE challenge_versions SET status = 'published', published_at = now()
WHERE id = $1 AND status = 'draft';

-- name: SetLessonCurrentVersion :exec
UPDATE lessons SET current_challenge_version_id = sqlc.narg(challenge_version_id) WHERE id = sqlc.arg(lesson_id);

-- name: UpsertAchievementDefinition :exec
INSERT INTO achievement_definitions (key, title, description, icon, condition, display_order)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (key) DO UPDATE
SET title = EXCLUDED.title, description = EXCLUDED.description, icon = EXCLUDED.icon,
    condition = EXCLUDED.condition, display_order = EXCLUDED.display_order;

-- name: SetCoursePublished :exec
UPDATE courses SET published = $2 WHERE id = $1;

-- name: GetLessonBySlugs :one
SELECT l.* FROM lessons l JOIN courses c ON c.id = l.course_id
WHERE c.slug = sqlc.arg(course_slug) AND l.slug = sqlc.arg(lesson_slug);

-- name: GetChallengeVersionByNumber :one
SELECT id, content_sha256, status FROM challenge_versions WHERE lesson_id = $1 AND version = $2;

-- name: GetChallengeVersionBySHA :one
SELECT id, version, status FROM challenge_versions WHERE lesson_id = $1 AND content_sha256 = $2;

-- name: RetireChallengeVersion :exec
UPDATE challenge_versions SET status = 'retired', retired_at = now() WHERE id = $1 AND status = 'published';

-- name: SetLessonDisplayOrder :exec
UPDATE lessons SET display_order = $2 WHERE id = $1;

-- name: ListPublishedCourses :many
-- Courses with at least one published lesson, in display order.
SELECT c.id, c.slug, c.title, c.summary, c.description, c.difficulty, c.display_order, c.prerequisites,
       count(l.id) FILTER (WHERE l.current_challenge_version_id IS NOT NULL)::int AS published_lessons,
       count(l.id)::int AS planned_lessons,
       COALESCE(sum(l.estimated_minutes) FILTER (WHERE l.current_challenge_version_id IS NOT NULL), 0)::int AS estimated_minutes
FROM courses c
LEFT JOIN lessons l ON l.course_id = c.id
WHERE c.published
GROUP BY c.id
HAVING count(l.id) FILTER (WHERE l.current_challenge_version_id IS NOT NULL) > 0
ORDER BY c.display_order;

-- name: ListPublishedLessons :many
-- Published lessons with their current version's spec (mapped to public fields in Go).
SELECT l.id, l.slug, l.display_order, l.estimated_minutes, c.slug AS course_slug,
       cv.id AS version_id, cv.content_sha256, cv.spec
FROM lessons l
JOIN courses c ON c.id = l.course_id
JOIN challenge_versions cv ON cv.id = l.current_challenge_version_id
WHERE c.published AND (sqlc.narg(course_id)::uuid IS NULL OR l.course_id = sqlc.narg(course_id)::uuid)
ORDER BY c.display_order, l.display_order;
