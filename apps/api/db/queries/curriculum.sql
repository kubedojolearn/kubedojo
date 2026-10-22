-- name: UpsertCourse :one
INSERT INTO courses (slug, title, summary, description, difficulty, display_order)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (slug) DO UPDATE
SET title = EXCLUDED.title, summary = EXCLUDED.summary, description = EXCLUDED.description,
    difficulty = EXCLUDED.difficulty, display_order = EXCLUDED.display_order
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
