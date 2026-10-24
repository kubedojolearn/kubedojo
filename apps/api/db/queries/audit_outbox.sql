-- name: InsertAuditLog :one
INSERT INTO audit_logs (actor_user_id, actor_kind, action, entity_type, entity_id, old_ref, new_ref, details)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
WHERE (sqlc.narg(before_created_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(before_created_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: InsertOutboxEvent :one
INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ClaimOutboxBatch :many
-- Concurrent workers skip each other's rows.
SELECT * FROM outbox_events
WHERE published_at IS NULL
ORDER BY id
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxPublished :exec
UPDATE outbox_events SET published_at = now() WHERE id = ANY(sqlc.arg(ids)::bigint[]);

-- name: RecordOutboxFailure :exec
UPDATE outbox_events SET attempts = attempts + 1, last_error = left(sqlc.arg(last_error)::text, 512) WHERE id = sqlc.arg(id);
