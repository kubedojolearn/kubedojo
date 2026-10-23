-- name: ResolveIdentity :one
-- Maps a verified provider identity to an active (not deleted) user and its current role.
SELECT u.id, u.role
FROM oauth_identities i
JOIN users u ON u.id = i.user_id
WHERE i.provider = $1 AND i.provider_subject = $2 AND u.deleted_at IS NULL;

-- name: GetIdentityForUpdate :one
SELECT i.user_id, u.deleted_at
FROM oauth_identities i
JOIN users u ON u.id = i.user_id
WHERE i.provider = $1 AND i.provider_subject = $2
FOR UPDATE OF i;

-- name: HandleTaken :one
SELECT EXISTS (SELECT 1 FROM users WHERE lower(handle) = lower(sqlc.arg(handle)::text));

-- name: InsertUser :one
INSERT INTO users (handle, display_name, avatar_url)
VALUES ($1, $2, $3)
RETURNING id;

-- name: InsertIdentity :exec
INSERT INTO oauth_identities (provider, provider_subject, user_id, provider_login)
VALUES ($1, $2, $3, $4);

-- name: InsertUserSettings :exec
INSERT INTO user_settings (user_id) VALUES ($1);

-- name: InsertUserTotals :exec
INSERT INTO user_totals (user_id) VALUES ($1);

-- name: TouchIdentity :exec
UPDATE oauth_identities
SET last_login_at = now(), provider_login = sqlc.arg(provider_login)
WHERE provider = sqlc.arg(provider) AND provider_subject = sqlc.arg(provider_subject);

-- name: RefreshProviderProfile :exec
-- Updates fields GitHub owns, without overwriting a display name the user chose themselves.
UPDATE users
SET display_name = CASE WHEN display_name_customized THEN display_name ELSE sqlc.arg(display_name) END,
    avatar_url   = sqlc.narg(avatar_url),
    last_seen_at = now()
WHERE id = sqlc.arg(id);

-- name: GetMe :one
SELECT u.id, u.handle, u.display_name, u.avatar_url, u.role, u.created_at, u.onboarding_completed_at,
       s.theme, s.reduced_motion, s.leaderboard_opt_in, s.profile_public, s.analytics_opt_out, s.time_zone
FROM users u
JOIN user_settings s ON s.user_id = u.id
WHERE u.id = $1 AND u.deleted_at IS NULL;

-- name: SetRoleByHandle :one
-- Operator-only (CLI). Returns the previous role for the audit log.
WITH target AS (
  SELECT id, role AS old_role FROM users
  WHERE lower(handle) = lower(sqlc.arg(handle)::text) AND deleted_at IS NULL
  FOR UPDATE
)
UPDATE users u SET role = sqlc.arg(role)
FROM target
WHERE u.id = target.id
RETURNING u.id, target.old_role, u.role;

-- name: UpdateProfile :exec
-- Partial update; NULL arguments leave the column unchanged. Setting a display name marks it
-- as chosen by the user so provider sign-ins stop overwriting it.
UPDATE users
SET display_name            = COALESCE(sqlc.narg(display_name), display_name),
    display_name_customized = display_name_customized OR sqlc.narg(display_name)::text IS NOT NULL,
    handle                  = COALESCE(sqlc.narg(handle), handle),
    experience_level        = COALESCE(sqlc.narg(experience_level), experience_level),
    learning_goal           = COALESCE(sqlc.narg(learning_goal), learning_goal),
    onboarding_completed_at = CASE WHEN sqlc.arg(complete_onboarding)::boolean
                                   THEN COALESCE(onboarding_completed_at, now())
                                   ELSE onboarding_completed_at END
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: UpdateSettings :exec
UPDATE user_settings
SET theme              = COALESCE(sqlc.narg(theme), theme),
    reduced_motion     = COALESCE(sqlc.narg(reduced_motion), reduced_motion),
    leaderboard_opt_in = COALESCE(sqlc.narg(leaderboard_opt_in), leaderboard_opt_in),
    profile_public     = COALESCE(sqlc.narg(profile_public), profile_public),
    analytics_opt_out  = COALESCE(sqlc.narg(analytics_opt_out), analytics_opt_out),
    time_zone          = COALESCE(sqlc.narg(time_zone), time_zone)
WHERE user_id = sqlc.arg(user_id);
