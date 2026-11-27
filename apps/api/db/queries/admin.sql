-- Admin read and publishing views.

-- name: ListAdminContent :many
-- Every lesson with every imported version, in curriculum order (lessons without versions too).
SELECT c.slug AS course_slug, l.slug AS lesson_slug, l.title, l.current_challenge_version_id,
       v.id AS version_id, v.version, v.content_sha256, v.status, v.imported_at, v.published_at
FROM lessons l
JOIN courses c ON c.id = l.course_id
LEFT JOIN challenge_versions v ON v.lesson_id = l.id
ORDER BY c.display_order, l.display_order, v.version DESC;

-- name: ListAdminLessonContent :many
-- One lesson with every imported version, newest version first.
SELECT c.slug AS course_slug, l.slug AS lesson_slug, l.title, l.current_challenge_version_id,
       v.id AS version_id, v.version, v.content_sha256, v.status, v.imported_at, v.published_at
FROM lessons l
JOIN courses c ON c.id = l.course_id
LEFT JOIN challenge_versions v ON v.lesson_id = l.id
WHERE l.id = $1
ORDER BY v.version DESC;

-- name: GetAdminChallengeVersion :one
-- Loads a challenge version together with its lesson and course metadata for validation/publishing.
SELECT v.id, v.lesson_id, v.version, v.content_sha256, v.spec, v.simulator_version, v.status,
       v.imported_at, v.published_at, v.retired_at,
       l.course_id, l.slug AS lesson_slug, l.title AS lesson_title, l.current_challenge_version_id,
       c.slug AS course_slug
FROM challenge_versions v
JOIN lessons l ON l.id = v.lesson_id
JOIN courses c ON c.id = l.course_id
WHERE v.id = $1;

-- name: LockLessonForPublish :one
-- Locks the lesson row before switching its published version.
SELECT id, course_id, current_challenge_version_id
FROM lessons
WHERE id = $1
FOR UPDATE;

-- name: ActivateChallengeVersion :exec
-- Marks a draft or retired version as published without mutating an existing published_at.
UPDATE challenge_versions
SET status = 'published', published_at = COALESCE(published_at, now()), retired_at = NULL
WHERE id = $1;
