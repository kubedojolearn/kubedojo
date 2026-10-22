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
