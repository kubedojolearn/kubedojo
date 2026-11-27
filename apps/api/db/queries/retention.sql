-- Data retention queries executed by the background worker (docs/operations/database.md).

-- name: ListEndedLabsForPurge :many
-- Ended labs older than cutoff that still hold simulated state or history.
SELECT ls.id
FROM lab_sessions ls
WHERE ls.ended_at IS NOT NULL
  AND ls.ended_at < sqlc.arg(before)::timestamptz
  AND (
    EXISTS (SELECT 1 FROM lab_resources r WHERE r.session_id = ls.id)
    OR EXISTS (SELECT 1 FROM lab_files f WHERE f.session_id = ls.id)
    OR EXISTS (SELECT 1 FROM lab_events e WHERE e.session_id = ls.id)
    OR EXISTS (SELECT 1 FROM command_executions c WHERE c.session_id = ls.id)
    OR EXISTS (SELECT 1 FROM lab_answers a WHERE a.session_id = ls.id)
  )
ORDER BY ls.ended_at
LIMIT sqlc.arg(batch_limit);

-- name: PurgeLabResources :execrows
DELETE FROM lab_resources WHERE session_id = ANY(sqlc.arg(session_ids)::uuid[]);

-- name: PurgeLabFiles :execrows
DELETE FROM lab_files WHERE session_id = ANY(sqlc.arg(session_ids)::uuid[]);

-- name: PurgeLabEvents :execrows
DELETE FROM lab_events WHERE session_id = ANY(sqlc.arg(session_ids)::uuid[]);

-- name: PurgeLabCommands :execrows
DELETE FROM command_executions WHERE session_id = ANY(sqlc.arg(session_ids)::uuid[]);

-- name: PurgeLabAnswers :execrows
DELETE FROM lab_answers WHERE session_id = ANY(sqlc.arg(session_ids)::uuid[]);

-- name: PruneCommandExecutions :execrows
-- Caps command history across all labs (90 days).
DELETE FROM command_executions
WHERE ctid IN (
  SELECT ctid FROM command_executions
  WHERE executed_at < sqlc.arg(before)::timestamptz
  LIMIT sqlc.arg(batch_limit)
);

-- name: PrunePublishedOutbox :execrows
-- Removes published outbox notifications after 7 days.
DELETE FROM outbox_events
WHERE id IN (
  SELECT id FROM outbox_events
  WHERE published_at IS NOT NULL AND published_at < sqlc.arg(before)::timestamptz
  ORDER BY published_at
  LIMIT sqlc.arg(batch_limit)
);

-- name: PruneActivityEvents :execrows
-- Removes learner activity history after 180 days (migration 00007).
DELETE FROM activity_events
WHERE id IN (
  SELECT id FROM activity_events
  WHERE occurred_at < sqlc.arg(before)::timestamptz
  ORDER BY occurred_at
  LIMIT sqlc.arg(batch_limit)
);

-- name: EnableAuditRetention :exec
-- Opts the current transaction into the 1-year audit retention window checked by audit_logs_append_only.
SELECT set_config('kubedojo.audit_retention', 'on', true);

-- name: PruneAuditLogs :execrows
-- Removes audit entries older than 1 year (trigger enforces age >= 1 year).
DELETE FROM audit_logs
WHERE id IN (
  SELECT id FROM audit_logs
  WHERE created_at < sqlc.arg(before)::timestamptz
  ORDER BY created_at
  LIMIT sqlc.arg(batch_limit)
);
