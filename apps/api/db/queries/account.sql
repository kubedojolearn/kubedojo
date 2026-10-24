-- Leaderboard, data export and account deletion.

-- name: ListLeaderboard :many
-- Opted-in, active learners by XP. Ties share a rank (standard competition ranking); pages are
-- keyed on (total_xp, user_id) so a page never repeats or skips a learner.
WITH ranked AS (
  SELECT u.id AS user_id, u.handle, u.display_name, u.avatar_url, t.total_xp, t.completed_lessons,
         rank() OVER (ORDER BY t.total_xp DESC) AS rank
  FROM user_totals t
  JOIN users u ON u.id = t.user_id
  JOIN user_settings s ON s.user_id = t.user_id
  WHERE s.leaderboard_opt_in AND u.deleted_at IS NULL
)
SELECT user_id, handle, display_name, avatar_url, total_xp, completed_lessons, rank::bigint AS rank
FROM ranked
WHERE sqlc.narg(after_xp)::integer IS NULL
   OR (total_xp, user_id) < (sqlc.narg(after_xp)::integer, sqlc.narg(after_user_id)::uuid)
ORDER BY total_xp DESC, user_id DESC
LIMIT sqlc.arg(page_limit);

-- name: ExportUserData :one
-- Everything stored about one user, as a single JSON document (requirements PR4). Command output
-- is exported as stored, which already has Secret values redacted.
SELECT jsonb_build_object(
  'exportedAt', now(),
  'profile', (
    SELECT jsonb_build_object(
      'handle', u.handle, 'displayName', u.display_name, 'avatarUrl', u.avatar_url, 'role', u.role,
      'experienceLevel', u.experience_level, 'learningGoal', u.learning_goal,
      'onboardingCompletedAt', u.onboarding_completed_at, 'createdAt', u.created_at, 'lastSeenAt', u.last_seen_at)
    FROM users u WHERE u.id = sqlc.arg(user_id)),
  'identities', COALESCE((
    SELECT jsonb_agg(jsonb_build_object(
      'provider', i.provider, 'providerSubject', i.provider_subject, 'providerLogin', i.provider_login,
      'linkedAt', i.created_at, 'lastLoginAt', i.last_login_at) ORDER BY i.created_at)
    FROM oauth_identities i WHERE i.user_id = sqlc.arg(user_id)), '[]'::jsonb),
  'settings', (
    SELECT jsonb_build_object(
      'theme', s.theme, 'reducedMotion', s.reduced_motion, 'leaderboardOptIn', s.leaderboard_opt_in,
      'profilePublic', s.profile_public, 'analyticsOptOut', s.analytics_opt_out, 'timeZone', s.time_zone)
    FROM user_settings s WHERE s.user_id = sqlc.arg(user_id)),
  'totals', (
    SELECT jsonb_build_object(
      'totalXp', t.total_xp, 'completedLessons', t.completed_lessons, 'currentStreak', t.current_streak,
      'longestStreak', t.longest_streak, 'lastCompletionDate', t.last_completion_date)
    FROM user_totals t WHERE t.user_id = sqlc.arg(user_id)),
  'lessonProgress', COALESCE((
    SELECT jsonb_agg(to_jsonb(p) - 'user_id' ORDER BY p.lesson_id) FROM lesson_progress p WHERE p.user_id = sqlc.arg(user_id)), '[]'::jsonb),
  'xpEvents', COALESCE((
    SELECT jsonb_agg(to_jsonb(x) - 'user_id' ORDER BY x.created_at) FROM xp_events x WHERE x.user_id = sqlc.arg(user_id)), '[]'::jsonb),
  'achievements', COALESCE((
    SELECT jsonb_agg(to_jsonb(a) - 'user_id' ORDER BY a.awarded_at) FROM user_achievements a WHERE a.user_id = sqlc.arg(user_id)), '[]'::jsonb),
  'activity', COALESCE((
    SELECT jsonb_agg(to_jsonb(e) - 'user_id' ORDER BY e.occurred_at) FROM activity_events e WHERE e.user_id = sqlc.arg(user_id)), '[]'::jsonb),
  'labs', COALESCE((
    SELECT jsonb_agg(jsonb_build_object(
      'id', l.id,
      'lesson', (SELECT c.slug || '/' || le.slug FROM lessons le JOIN courses c ON c.id = le.course_id WHERE le.id = l.lesson_id),
      'state', l.state, 'startedAt', l.started_at, 'lastActiveAt', l.last_active_at,
      'endedAt', l.ended_at,
      'commands', COALESCE((
        SELECT jsonb_agg(jsonb_build_object(
          'sequence', c.sequence, 'input', c.input, 'stdout', c.stdout, 'stderr', c.stderr, 'exitCode', c.exit_code,
          'executedAt', c.executed_at) ORDER BY c.sequence)
        FROM command_executions c WHERE c.session_id = l.id), '[]'::jsonb)
    ) ORDER BY l.started_at)
    FROM lab_sessions l WHERE l.user_id = sqlc.arg(user_id)), '[]'::jsonb)
)::jsonb AS document;

-- name: TombstoneUser :one
-- Starts account deletion: hides the account everywhere and clears profile fields at once. The
-- handle must match (a typed confirmation). Remaining rows are erased by EraseUser after 30 days.
UPDATE users
SET deleted_at = now(), display_name = '', display_name_customized = false, avatar_url = NULL,
    experience_level = NULL, learning_goal = NULL
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND lower(handle) = lower(sqlc.arg(confirm_handle)::text)
RETURNING id;

-- name: UnlinkIdentities :exec
-- After this the GitHub account can sign up again as a new user (the old handle stays reserved).
DELETE FROM oauth_identities WHERE user_id = $1;

-- name: HideUser :exec
UPDATE user_settings SET profile_public = false, leaderboard_opt_in = false WHERE user_id = $1;

-- name: EndUserLabs :exec
UPDATE lab_sessions SET state = 'abandoned', state_reason = 'ACCOUNT_DELETED', ended_at = now()
WHERE user_id = $1 AND state = 'active';

-- name: ListUsersDueForErasure :many
SELECT id FROM users
WHERE deleted_at IS NOT NULL AND deleted_at < sqlc.arg(cutoff)::timestamptz
ORDER BY deleted_at
LIMIT sqlc.arg(batch);

-- name: AllowUserErasure :exec
-- Lets the append-only XP ledger delete this user's rows, for this transaction only.
SELECT set_config('kubedojo.erasing_user', sqlc.arg(user_id)::text, true);

-- name: EraseUser :execrows
-- Hard-deletes a tombstoned user; every personal row cascades. Audit entries keep only a NULL actor.
DELETE FROM users WHERE id = $1 AND deleted_at IS NOT NULL;
