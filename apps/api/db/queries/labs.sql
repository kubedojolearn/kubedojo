-- name: CreateLabSession :one
INSERT INTO lab_sessions (user_id, lesson_id, challenge_version_id, simulator_version, seed, sim_time, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetLabSession :one
-- Ownership is part of the key: another user's lab is indistinguishable from a missing one.
SELECT * FROM lab_sessions WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: LockLabSession :one
-- Locking root for every write to a lab (ADR-0004).
SELECT * FROM lab_sessions WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) FOR UPDATE;

-- name: BumpLabVersion :one
UPDATE lab_sessions SET version = version + 1, last_active_at = now()
WHERE id = $1
RETURNING version;

-- name: ListUserLabs :many
-- Keyset pagination, most recently active first.
SELECT * FROM lab_sessions
WHERE user_id = sqlc.arg(user_id)
  AND (sqlc.narg(state)::text IS NULL OR state = sqlc.narg(state)::text)
  AND (sqlc.narg(after_active_at)::timestamptz IS NULL
       OR (last_active_at, id) < (sqlc.narg(after_active_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY last_active_at DESC, id DESC
LIMIT sqlc.arg(page_limit);
