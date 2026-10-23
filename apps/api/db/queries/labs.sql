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
-- Every state change counts as activity and pushes the idle expiry out.
UPDATE lab_sessions SET version = version + 1, last_active_at = now(), expires_at = sqlc.arg(expires_at)
WHERE id = sqlc.arg(id)
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

-- name: GetLaunchableChallengeVersion :one
-- A lab can only be started from a lesson's current published version in a published course.
SELECT cv.id, cv.lesson_id, cv.spec, cv.simulator_version,
       (l.current_challenge_version_id = cv.id)::boolean AS is_current
FROM challenge_versions cv
JOIN lessons l ON l.id = cv.lesson_id
JOIN courses c ON c.id = l.course_id
WHERE cv.id = sqlc.arg(id) AND cv.status = 'published' AND c.published
  AND l.current_challenge_version_id IS NOT NULL;

-- name: GetChallengeVersionSpec :one
-- Challenge versions are immutable once published, so callers may cache the result by id.
SELECT spec FROM challenge_versions WHERE id = $1;

-- name: GetActiveLabForLesson :one
SELECT * FROM lab_sessions
WHERE user_id = sqlc.arg(user_id) AND lesson_id = sqlc.arg(lesson_id) AND state = 'active';

-- name: GetLabView :one
-- Ownership is part of the key (threat T5).
SELECT s.*, c.slug AS course_slug, l.slug AS lesson_slug, l.title AS lesson_title
FROM lab_sessions s
JOIN lessons l ON l.id = s.lesson_id
JOIN courses c ON c.id = l.course_id
WHERE s.id = sqlc.arg(id) AND s.user_id = sqlc.arg(user_id);

-- name: ListUserLabViews :many
-- Keyset pagination, most recently active first.
SELECT s.*, c.slug AS course_slug, l.slug AS lesson_slug, l.title AS lesson_title
FROM lab_sessions s
JOIN lessons l ON l.id = s.lesson_id
JOIN courses c ON c.id = l.course_id
WHERE s.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(state)::text IS NULL OR s.state = sqlc.narg(state)::text)
  AND (sqlc.narg(after_active_at)::timestamptz IS NULL
       OR (s.last_active_at, s.id) < (sqlc.narg(after_active_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY s.last_active_at DESC, s.id DESC
LIMIT sqlc.arg(page_limit);

-- name: SaveLabEngineState :exec
-- Persists the simulator's clock, generator position and event sequence after a mutation.
UPDATE lab_sessions
SET sim_time = sqlc.arg(sim_time), generator_counter = sqlc.arg(generator_counter),
    resource_revision = sqlc.arg(resource_revision), event_seq = sqlc.arg(event_seq),
    current_namespace = sqlc.arg(current_namespace)
WHERE id = sqlc.arg(id);

-- name: TouchLab :exec
-- Marks the lab used now and pushes its idle expiry out.
UPDATE lab_sessions SET last_active_at = now(), expires_at = sqlc.arg(expires_at)
WHERE id = sqlc.arg(id) AND state = 'active';

-- name: EndLab :exec
UPDATE lab_sessions SET state = sqlc.arg(state), state_reason = sqlc.arg(state_reason), ended_at = now()
WHERE id = sqlc.arg(id) AND state = 'active';

-- name: ExpireLabs :execrows
-- TTL sweep: ends active labs past their expiry, a bounded batch at a time.
UPDATE lab_sessions SET state = 'expired', state_reason = 'IDLE_TIMEOUT', ended_at = now()
WHERE id IN (
  SELECT id FROM lab_sessions
  WHERE state = 'active' AND expires_at <= now()
  ORDER BY expires_at
  LIMIT sqlc.arg(batch_size)
  FOR UPDATE SKIP LOCKED
);

-- name: ListLabResources :many
SELECT kind, namespace, name, api_version, uid, resource_version, generation, metadata, spec, status, extra
FROM lab_resources WHERE session_id = $1
ORDER BY kind, namespace, name;

-- name: UpsertLabResource :exec
INSERT INTO lab_resources (session_id, kind, namespace, name, api_version, uid, resource_version, generation, metadata, spec, status, extra)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (session_id, kind, namespace, name) DO UPDATE SET
  api_version = EXCLUDED.api_version, uid = EXCLUDED.uid, resource_version = EXCLUDED.resource_version,
  generation = EXCLUDED.generation, metadata = EXCLUDED.metadata, spec = EXCLUDED.spec,
  status = EXCLUDED.status, extra = EXCLUDED.extra;

-- name: DeleteLabResource :exec
DELETE FROM lab_resources
WHERE session_id = sqlc.arg(session_id) AND kind = sqlc.arg(kind) AND namespace = sqlc.arg(namespace) AND name = sqlc.arg(name);

-- name: DeleteAllLabResources :exec
DELETE FROM lab_resources WHERE session_id = $1;

-- name: InsertLabEvent :exec
INSERT INTO lab_events (session_id, seq, version, type, reason, involved_kind, involved_namespace, involved_name, message, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: DeleteLabEvents :exec
DELETE FROM lab_events WHERE session_id = $1;

-- name: ListLabEvents :many
-- Oldest first, after a sequence number.
SELECT * FROM lab_events
WHERE session_id = sqlc.arg(session_id) AND seq > sqlc.arg(after_seq)
ORDER BY seq
LIMIT sqlc.arg(page_limit);

-- name: GetIdempotencyRecord :one
SELECT * FROM idempotency_records
WHERE user_id = sqlc.arg(user_id) AND operation = sqlc.arg(operation) AND key = sqlc.arg(key);

-- name: InsertIdempotencyRecord :exec
INSERT INTO idempotency_records (user_id, operation, key, target_id, status_code, response)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: DeleteOldIdempotencyRecords :execrows
DELETE FROM idempotency_records WHERE created_at < sqlc.arg(before);

-- name: RecordLabStarted :exec
-- Counts an attempt on the lesson; never touches completion or scores.
INSERT INTO lesson_progress (user_id, lesson_id, attempts, first_started_at, last_started_at)
VALUES (sqlc.arg(user_id), sqlc.arg(lesson_id), 1, now(), now())
ON CONFLICT (user_id, lesson_id) DO UPDATE SET
  attempts = lesson_progress.attempts + 1,
  first_started_at = COALESCE(lesson_progress.first_started_at, now()),
  last_started_at = now();

-- name: PruneLabEvents :exec
-- Keeps the newest events only, like the API server's event TTL.
DELETE FROM lab_events WHERE session_id = sqlc.arg(session_id) AND seq <= sqlc.arg(through_seq);
