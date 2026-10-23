-- name: MarkLessonOpened :one
-- Records that the learner opened a published lesson. Never touches completion or scores.
INSERT INTO lesson_progress (user_id, lesson_id, last_opened_at)
SELECT sqlc.arg(user_id), l.id, now()
FROM lessons l JOIN courses c ON c.id = l.course_id
WHERE c.slug = sqlc.arg(course_slug) AND l.slug = sqlc.arg(lesson_slug)
  AND c.published AND l.current_challenge_version_id IS NOT NULL
ON CONFLICT (user_id, lesson_id) DO UPDATE SET last_opened_at = now()
RETURNING lesson_id;

-- name: InsertActivity :exec
INSERT INTO activity_events (user_id, type, lesson_id, achievement_key, xp_delta)
VALUES ($1, $2, $3, $4, $5);

-- name: CourseProgress :many
-- Per-course completion over published lessons only.
SELECT c.slug,
       count(l.id)::int AS total_lessons,
       count(lp.completed_at)::int AS completed_lessons
FROM courses c
JOIN lessons l ON l.course_id = c.id AND l.current_challenge_version_id IS NOT NULL
LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = sqlc.arg(user_id)
WHERE c.published
GROUP BY c.slug, c.display_order
ORDER BY c.display_order;

-- name: ListLessonProgress :many
-- Published lessons with the learner's progress, in curriculum order (keyset paginated).
SELECT c.slug AS course_slug, c.display_order AS course_order, l.slug AS lesson_slug, l.display_order AS lesson_order,
       lp.best_score, lp.completed_at, lp.last_started_at, lp.last_opened_at
FROM lessons l
JOIN courses c ON c.id = l.course_id
LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = sqlc.arg(user_id)
WHERE c.published AND l.current_challenge_version_id IS NOT NULL
  AND (sqlc.narg(course_slug)::text IS NULL OR c.slug = sqlc.narg(course_slug)::text)
  AND (sqlc.narg(after_course_order)::int IS NULL
       OR (c.display_order, l.display_order) > (sqlc.narg(after_course_order)::int, sqlc.narg(after_lesson_order)::int))
ORDER BY c.display_order, l.display_order
LIMIT sqlc.arg(page_limit);

-- name: ContinueLesson :one
-- The most recently opened or started lesson that isn't completed yet.
SELECT c.slug AS course_slug, l.slug AS lesson_slug, l.title,
       GREATEST(lp.last_opened_at, lp.last_started_at)::timestamptz AS last_activity_at
FROM lesson_progress lp
JOIN lessons l ON l.id = lp.lesson_id
JOIN courses c ON c.id = l.course_id
WHERE lp.user_id = $1 AND lp.completed_at IS NULL
  AND c.published AND l.current_challenge_version_id IS NOT NULL
  AND COALESCE(lp.last_opened_at, lp.last_started_at) IS NOT NULL
ORDER BY GREATEST(lp.last_opened_at, lp.last_started_at) DESC NULLS LAST
LIMIT 1;

-- name: RecommendedLesson :one
-- The first published lesson in curriculum order the learner hasn't completed.
SELECT c.slug AS course_slug, l.slug AS lesson_slug, l.title
FROM lessons l
JOIN courses c ON c.id = l.course_id
LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = $1
WHERE c.published AND l.current_challenge_version_id IS NOT NULL AND lp.completed_at IS NULL
ORDER BY c.display_order, l.display_order
LIMIT 1;

-- name: GetTotalsWithTimeZone :one
SELECT t.total_xp, t.completed_lessons, t.current_streak, t.longest_streak, t.last_completion_date, s.time_zone
FROM user_totals t JOIN user_settings s ON s.user_id = t.user_id
WHERE t.user_id = $1;

-- name: ListActivity :many
SELECT a.id, a.type, a.xp_delta, a.occurred_at, a.achievement_key, c.slug AS course_slug, l.slug AS lesson_slug
FROM activity_events a
LEFT JOIN lessons l ON l.id = a.lesson_id
LEFT JOIN courses c ON c.id = l.course_id
WHERE a.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(before_at)::timestamptz IS NULL
       OR (a.occurred_at, a.id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY a.occurred_at DESC, a.id DESC
LIMIT sqlc.arg(page_limit);

-- name: ListAchievements :many
SELECT d.key, d.title, d.description, d.icon, ua.awarded_at
FROM achievement_definitions d
LEFT JOIN user_achievements ua ON ua.achievement_key = d.key AND ua.user_id = $1
ORDER BY d.display_order;

-- name: GetPublicProfile :one
-- Only users who made their profile public; everyone else is indistinguishable from missing.
SELECT u.id, u.handle, u.display_name, u.avatar_url, u.created_at, t.total_xp, t.completed_lessons
FROM users u
JOIN user_settings s ON s.user_id = u.id
JOIN user_totals t ON t.user_id = u.id
WHERE lower(u.handle) = lower(sqlc.arg(handle)::text) AND u.deleted_at IS NULL AND s.profile_public;

-- name: ListAwardedAchievements :many
SELECT d.key, d.title, d.description, d.icon, ua.awarded_at
FROM user_achievements ua JOIN achievement_definitions d ON d.key = ua.achievement_key
WHERE ua.user_id = $1
ORDER BY ua.awarded_at DESC;
