-- name: InsertXPEvent :one
-- Returns no row when this award was already granted (idempotent by event_key).
INSERT INTO xp_events (user_id, event_key, delta, reason, attempt_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, event_key) DO NOTHING
RETURNING id, delta;

-- name: AddXP :one
UPDATE user_totals SET total_xp = total_xp + sqlc.arg(delta)
WHERE user_id = sqlc.arg(user_id)
RETURNING total_xp;

-- name: GetUserTotals :one
SELECT * FROM user_totals WHERE user_id = $1;

-- name: SumXPEvents :one
SELECT COALESCE(SUM(delta), 0)::bigint FROM xp_events WHERE user_id = $1;

-- name: RecordBestScore :exec
UPDATE lesson_progress SET best_score = GREATEST(COALESCE(best_score, 0), sqlc.arg(score)::smallint)
WHERE user_id = sqlc.arg(user_id) AND lesson_id = sqlc.arg(lesson_id);

-- name: CompleteLessonProgress :execrows
-- Marks the lesson completed once; returns 0 rows if it already was.
UPDATE lesson_progress
SET completed_at = now(), best_score = GREATEST(COALESCE(best_score, 0), sqlc.arg(score)::smallint)
WHERE user_id = sqlc.arg(user_id) AND lesson_id = sqlc.arg(lesson_id) AND completed_at IS NULL;

-- name: LockTotalsWithTimeZone :one
SELECT t.completed_lessons, t.current_streak, t.longest_streak, t.last_completion_date, s.time_zone
FROM user_totals t JOIN user_settings s ON s.user_id = t.user_id
WHERE t.user_id = $1
FOR UPDATE OF t;

-- name: RecordCompletionTotals :one
UPDATE user_totals SET
  completed_lessons = completed_lessons + 1,
  current_streak = sqlc.arg(streak),
  longest_streak = GREATEST(longest_streak, sqlc.arg(streak)),
  last_completion_date = sqlc.arg(completion_date)
WHERE user_id = sqlc.arg(user_id)
RETURNING completed_lessons, current_streak;

-- name: ListAchievementConditions :many
SELECT key, title, condition FROM achievement_definitions ORDER BY display_order;

-- name: XPForAttempt :one
SELECT COALESCE(SUM(delta), 0)::integer FROM xp_events WHERE attempt_id = $1;

-- name: AwardAchievement :execrows
INSERT INTO user_achievements (user_id, achievement_key) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: CompletedCourseSlugs :many
-- Published courses in which the learner has completed every published lesson.
SELECT c.slug FROM courses c
JOIN lessons l ON l.course_id = c.id AND l.current_challenge_version_id IS NOT NULL
LEFT JOIN lesson_progress p ON p.lesson_id = l.id AND p.user_id = $1 AND p.completed_at IS NOT NULL
WHERE c.published
GROUP BY c.id, c.slug
HAVING count(*) = count(p.lesson_id);

-- name: CountHintFreePasses :one
SELECT count(DISTINCT s.lesson_id) FROM challenge_attempts a JOIN lab_sessions s ON s.id = a.session_id
WHERE a.user_id = $1 AND a.passed AND a.hints_used = 0;

-- name: CountFirstTryPasses :one
-- Lessons whose first check by this learner passed.
SELECT count(*) FROM (
  SELECT DISTINCT ON (s.lesson_id) a.passed
  FROM challenge_attempts a JOIN lab_sessions s ON s.id = a.session_id
  WHERE a.user_id = $1
  ORDER BY s.lesson_id, a.evaluated_at, a.id
) first WHERE first.passed;
