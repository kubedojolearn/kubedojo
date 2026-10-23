-- Simulator state that must survive between commands, and idempotency records for lab actions.

-- +goose Up
-- Position of the lab's deterministic ID generator (UIDs, Pod name suffixes). Together with
-- lab_sessions.seed it makes every generated name reproducible (ADR-0006).
ALTER TABLE lab_sessions ADD COLUMN generator_counter bigint NOT NULL DEFAULT 0 CHECK (generator_counter >= 0);
-- Last resourceVersion handed out. Stored rather than derived from surviving objects, so a
-- resourceVersion is never reused after the newest object is deleted.
ALTER TABLE lab_sessions ADD COLUMN resource_revision bigint NOT NULL DEFAULT 0 CHECK (resource_revision >= 0);

-- Results of idempotent operations (reset, finish, later validate): a retry with the same key
-- returns the stored response instead of acting twice.
CREATE TABLE idempotency_records (
  user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  operation    varchar(64) NOT NULL,
  key          varchar(128) NOT NULL CHECK (key ~ '^[A-Za-z0-9_-]{8,128}$'),
  target_id    uuid NOT NULL,
  status_code  smallint NOT NULL,
  response     jsonb NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, operation, key)
);
CREATE INDEX idempotency_records_created_idx ON idempotency_records (created_at);

-- +goose Down
DROP TABLE idempotency_records;
ALTER TABLE lab_sessions DROP COLUMN resource_revision;
ALTER TABLE lab_sessions DROP COLUMN generator_counter;
