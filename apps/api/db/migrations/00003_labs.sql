-- Lab sessions and their simulated state (ADR-0004, ADR-0006). The lab_sessions row is the
-- locking root: every write to a lab takes SELECT ... FOR UPDATE on it first.

-- +goose Up
CREATE TABLE lab_sessions (
  -- Random v4: lab IDs appear in URLs and must not be guessable from time.
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id               uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  lesson_id             uuid NOT NULL REFERENCES lessons (id) ON DELETE RESTRICT,
  challenge_version_id  uuid NOT NULL REFERENCES challenge_versions (id) ON DELETE RESTRICT,
  state                 text NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'completed', 'abandoned', 'expired')),
  state_reason          varchar(64),
  -- Monotonic state version (never decreases, including on reset).
  version               bigint NOT NULL DEFAULT 1 CHECK (version >= 1),
  command_seq           bigint NOT NULL DEFAULT 0 CHECK (command_seq >= 0),
  event_seq             bigint NOT NULL DEFAULT 0 CHECK (event_seq >= 0),
  simulator_version     varchar(64) NOT NULL,
  state_schema_version  integer NOT NULL DEFAULT 1 CHECK (state_schema_version >= 1),
  -- Seed for deterministic UIDs, names and IPs; logical clock; simulated kubeconfig namespace.
  seed                  bigint NOT NULL,
  sim_time              timestamptz NOT NULL,
  current_namespace     varchar(63) NOT NULL DEFAULT 'default',
  hints_used            smallint NOT NULL DEFAULT 0 CHECK (hints_used BETWEEN 0 AND 10),
  started_at            timestamptz NOT NULL DEFAULT now(),
  last_active_at        timestamptz NOT NULL DEFAULT now(),
  expires_at            timestamptz NOT NULL,
  ended_at              timestamptz,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  CHECK ((state = 'active') = (ended_at IS NULL)),
  CHECK (expires_at > started_at)
);
-- One active lab per user per lesson (ADR-0004).
CREATE UNIQUE INDEX lab_sessions_one_active_per_lesson ON lab_sessions (user_id, lesson_id) WHERE state = 'active';
CREATE INDEX lab_sessions_user_recent_idx ON lab_sessions (user_id, last_active_at DESC, id);
CREATE INDEX lab_sessions_expiry_idx ON lab_sessions (expires_at) WHERE state = 'active';
CREATE INDEX lab_sessions_ended_idx ON lab_sessions (ended_at) WHERE ended_at IS NOT NULL;
CREATE INDEX lab_sessions_challenge_version_idx ON lab_sessions (challenge_version_id);
CREATE INDEX lab_sessions_lesson_idx ON lab_sessions (lesson_id);
CREATE TRIGGER lab_sessions_set_updated_at BEFORE UPDATE ON lab_sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Authoritative simulated resources: one row per object (ADR-0004). Cluster-scoped kinds use
-- namespace '' as the sentinel. Events are not resources (see lab_events).
CREATE TABLE lab_resources (
  session_id        uuid NOT NULL REFERENCES lab_sessions (id) ON DELETE CASCADE,
  kind              varchar(32) NOT NULL CHECK (kind ~ '^[A-Z][A-Za-z]{1,31}$'),
  namespace         varchar(63) NOT NULL DEFAULT '' CHECK (namespace = '' OR namespace ~ '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'),
  name              varchar(253) NOT NULL CHECK (name ~ '^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$'),
  api_version       varchar(64) NOT NULL,
  uid               uuid NOT NULL,
  resource_version  bigint NOT NULL CHECK (resource_version >= 1),
  generation        bigint NOT NULL DEFAULT 1 CHECK (generation >= 1),
  metadata          jsonb NOT NULL CHECK (jsonb_typeof(metadata) = 'object'),
  spec              jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(spec) = 'object'),
  status            jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(status) = 'object'),
  -- Kind-specific top-level fields (ConfigMap/Secret data, Secret type, EndpointSlice endpoints...).
  extra             jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(extra) = 'object'),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, kind, namespace, name),
  UNIQUE (session_id, uid),
  -- Backstop for the simulator's own limits (64 KiB manifests).
  CHECK (octet_length(metadata::text) + octet_length(spec::text) + octet_length(status::text) + octet_length(extra::text) <= 262144)
);
CREATE TRIGGER lab_resources_set_updated_at BEFORE UPDATE ON lab_resources FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE lab_files (
  session_id  uuid NOT NULL REFERENCES lab_sessions (id) ON DELETE CASCADE,
  path        varchar(80) NOT NULL CHECK (path ~ '^manifests/[a-z0-9][a-z0-9._-]{0,62}\.ya?ml$'),
  content     text NOT NULL CHECK (octet_length(content) <= 65536),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, path)
);

CREATE TABLE lab_events (
  session_id          uuid NOT NULL REFERENCES lab_sessions (id) ON DELETE CASCADE,
  seq                 bigint NOT NULL CHECK (seq >= 1),
  version             bigint NOT NULL CHECK (version >= 1),
  type                text NOT NULL CHECK (type IN ('Normal', 'Warning')),
  reason              varchar(64) NOT NULL,
  involved_kind       varchar(32) NOT NULL,
  involved_namespace  varchar(63) NOT NULL DEFAULT '',
  involved_name       varchar(253) NOT NULL,
  message             varchar(1024) NOT NULL,
  -- Simulated time (ADR-0006), not wall time.
  occurred_at         timestamptz NOT NULL,
  PRIMARY KEY (session_id, seq)
);

CREATE TABLE command_executions (
  session_id        uuid NOT NULL REFERENCES lab_sessions (id) ON DELETE CASCADE,
  sequence          bigint NOT NULL CHECK (sequence >= 1),
  -- Client-generated; executing at most once per request ID (WebSocket protocol v1).
  request_id        uuid NOT NULL,
  input             text NOT NULL CHECK (octet_length(input) BETWEEN 1 AND 16384),
  -- Structured parse (verb, kind, name, namespace, output...) used by observation goals.
  parsed            jsonb,
  stdout            text NOT NULL DEFAULT '' CHECK (octet_length(stdout) <= 262144),
  stderr            text NOT NULL DEFAULT '' CHECK (octet_length(stderr) <= 65536),
  exit_code         smallint NOT NULL CHECK (exit_code BETWEEN 0 AND 255),
  error_code        text CHECK (error_code IN ('UNSUPPORTED_COMMAND', 'VALIDATION_FAILED', 'NOT_FOUND', 'CONFLICT')),
  mutated           boolean NOT NULL,
  snapshot_version  bigint NOT NULL CHECK (snapshot_version >= 1),
  duration_ms       integer NOT NULL CHECK (duration_ms >= 0),
  executed_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, sequence),
  UNIQUE (session_id, request_id)
);
CREATE INDEX command_executions_executed_at_idx ON command_executions (executed_at);

-- Answers to answer-type goals (ADR-0007). At most 10 submissions per goal per session.
CREATE TABLE lab_answers (
  session_id     uuid NOT NULL REFERENCES lab_sessions (id) ON DELETE CASCADE,
  goal_id        varchar(64) NOT NULL CHECK (goal_id ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  submission_no  smallint NOT NULL CHECK (submission_no BETWEEN 1 AND 10),
  answer         varchar(256) NOT NULL,
  submitted_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, goal_id, submission_no)
);

-- +goose Down
DROP TABLE lab_answers;
DROP TABLE command_executions;
DROP TABLE lab_events;
DROP TABLE lab_files;
DROP TABLE lab_resources;
DROP TABLE lab_sessions;
