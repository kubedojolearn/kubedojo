-- Audit log and transactional outbox.

-- +goose Up
CREATE TABLE audit_logs (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  -- Kept as an ID when the actor is deleted; personal fields are never stored here.
  actor_user_id  uuid REFERENCES users (id) ON DELETE SET NULL,
  -- 'operator' for CLI actions without a user.
  actor_kind     text NOT NULL DEFAULT 'user' CHECK (actor_kind IN ('user', 'operator', 'system')),
  action         varchar(64) NOT NULL CHECK (action ~ '^[a-z]+(\.[a-z_]+)+$'),
  entity_type    varchar(64) NOT NULL,
  entity_id      varchar(128) NOT NULL,
  old_ref        varchar(128),
  new_ref        varchar(128),
  -- Redacted structured details. Never command text, YAML or Secret values (threat T15).
  details        jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(details) = 'object' AND octet_length(details::text) <= 8192),
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC, id DESC);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id);

-- Audit entries can't be edited; only the actor link may be nulled when a user is erased, and
-- rows may be deleted by the 1-year retention job (which sets kubedojo.audit_retention).
-- +goose StatementBegin
CREATE FUNCTION audit_logs_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF current_setting('kubedojo.audit_retention', true) = 'on' AND OLD.created_at < now() - interval '1 year' THEN
      RETURN OLD;
    END IF;
    RAISE EXCEPTION 'audit_logs is append-only' USING ERRCODE = 'integrity_constraint_violation';
  END IF;
  IF NEW.actor_user_id IS NULL AND OLD.actor_user_id IS NOT NULL
     AND (to_jsonb(NEW) - 'actor_user_id') = (to_jsonb(OLD) - 'actor_user_id') THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'audit_logs is append-only' USING ERRCODE = 'integrity_constraint_violation';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER audit_logs_append_only BEFORE UPDATE OR DELETE ON audit_logs
  FOR EACH ROW EXECUTE FUNCTION audit_logs_append_only();

-- Written in the same transaction as the state change it announces; published to Redis by the
-- worker (ADR-0004). Payloads are small notifications, never state.
CREATE TABLE outbox_events (
  id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  event_type      varchar(64) NOT NULL CHECK (event_type ~ '^[a-z]+(\.[a-z_]+)+$'),
  aggregate_type  varchar(32) NOT NULL,
  aggregate_id    uuid NOT NULL,
  payload         jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object' AND octet_length(payload::text) <= 4096),
  created_at      timestamptz NOT NULL DEFAULT now(),
  published_at    timestamptz,
  attempts        integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  last_error      varchar(512)
);
CREATE INDEX outbox_events_unpublished_idx ON outbox_events (id) WHERE published_at IS NULL;
CREATE INDEX outbox_events_published_idx ON outbox_events (published_at) WHERE published_at IS NOT NULL;

-- +goose Down
DROP TABLE outbox_events;
DROP TABLE audit_logs;
DROP FUNCTION audit_logs_append_only();
