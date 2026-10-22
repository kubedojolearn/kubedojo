-- Progress, grading, XP ledger, streak totals and achievements.

-- +goose Up
CREATE TABLE lesson_progress (
  user_id           uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  lesson_id         uuid NOT NULL REFERENCES lessons (id) ON DELETE RESTRICT,
  best_score        smallint CHECK (best_score BETWEEN 0 AND 100),
  attempts          integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  first_started_at  timestamptz,
  last_started_at   timestamptz,
  last_opened_at    timestamptz,
  -- Set only by the verified challenge evaluator (Part 16).
  completed_at      timestamptz,
  updated_at        timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, lesson_id),
  CHECK (completed_at IS NULL OR best_score IS NOT NULL)
);
CREATE INDEX lesson_progress_user_completed_idx ON lesson_progress (user_id, completed_at DESC);
CREATE INDEX lesson_progress_lesson_idx ON lesson_progress (lesson_id);
CREATE TRIGGER lesson_progress_set_updated_at BEFORE UPDATE ON lesson_progress FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE challenge_attempts (
  id                    uuid PRIMARY KEY DEFAULT uuidv7(),
  session_id            uuid NOT NULL REFERENCES lab_sessions (id) ON DELETE CASCADE,
  user_id               uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  challenge_version_id  uuid NOT NULL REFERENCES challenge_versions (id) ON DELETE RESTRICT,
  idempotency_key       varchar(128) NOT NULL CHECK (idempotency_key ~ '^[A-Za-z0-9_-]{8,128}$'),
  passed                boolean NOT NULL,
  score                 smallint NOT NULL CHECK (score BETWEEN 0 AND 100),
  hints_used            smallint NOT NULL CHECK (hints_used BETWEEN 0 AND 10),
  goal_results          jsonb NOT NULL CHECK (jsonb_typeof(goal_results) = 'array'),
  evaluated_at          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (session_id, idempotency_key)
);
CREATE INDEX challenge_attempts_user_idx ON challenge_attempts (user_id, evaluated_at DESC);
CREATE INDEX challenge_attempts_version_idx ON challenge_attempts (challenge_version_id);

CREATE TABLE hint_usage (
  session_id   uuid NOT NULL REFERENCES lab_sessions (id) ON DELETE CASCADE,
  hint_index   smallint NOT NULL CHECK (hint_index BETWEEN 0 AND 9),
  revealed_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, hint_index)
);

-- Append-only XP ledger. Totals in user_totals are derived from it in the same transaction.
CREATE TABLE xp_events (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  -- Deterministic per award, e.g. lesson-complete:<lesson id>; makes awards idempotent.
  event_key   varchar(200) NOT NULL CHECK (event_key ~ '^[a-z0-9-]+:[A-Za-z0-9:._-]+$'),
  delta       integer NOT NULL CHECK (delta <> 0 AND delta BETWEEN -10000 AND 10000),
  reason      text NOT NULL CHECK (reason IN ('lesson_completed', 'achievement', 'adjustment')),
  attempt_id  uuid REFERENCES challenge_attempts (id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, event_key)
);
CREATE INDEX xp_events_attempt_idx ON xp_events (attempt_id) WHERE attempt_id IS NOT NULL;

-- Rows are never edited; deletes happen only during account erasure, which must opt in with
-- SET LOCAL kubedojo.erasing_user = '<user id>'. ON DELETE SET NULL of attempt_id is allowed.
-- +goose StatementBegin
CREATE FUNCTION xp_events_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF current_setting('kubedojo.erasing_user', true) = OLD.user_id::text THEN
      RETURN OLD;
    END IF;
    RAISE EXCEPTION 'xp_events is append-only' USING ERRCODE = 'integrity_constraint_violation';
  END IF;
  IF NEW.id = OLD.id AND NEW.user_id = OLD.user_id AND NEW.event_key = OLD.event_key AND NEW.delta = OLD.delta
     AND NEW.reason = OLD.reason AND NEW.created_at = OLD.created_at AND NEW.attempt_id IS NULL THEN
    RETURN NEW; -- attempt_id set to NULL by ON DELETE SET NULL
  END IF;
  RAISE EXCEPTION 'xp_events is append-only' USING ERRCODE = 'integrity_constraint_violation';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER xp_events_append_only BEFORE UPDATE OR DELETE ON xp_events
  FOR EACH ROW EXECUTE FUNCTION xp_events_append_only();

CREATE TABLE user_totals (
  user_id               uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  total_xp              integer NOT NULL DEFAULT 0 CHECK (total_xp >= 0),
  completed_lessons     integer NOT NULL DEFAULT 0 CHECK (completed_lessons >= 0),
  current_streak        integer NOT NULL DEFAULT 0 CHECK (current_streak >= 0),
  longest_streak        integer NOT NULL DEFAULT 0 CHECK (longest_streak >= current_streak),
  -- Calendar date in the user's time zone of the last completion.
  last_completion_date  date,
  updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_totals_leaderboard_idx ON user_totals (total_xp DESC, user_id);
CREATE TRIGGER user_totals_set_updated_at BEFORE UPDATE ON user_totals FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE achievement_definitions (
  key            varchar(64) PRIMARY KEY CHECK (key ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  title          varchar(80) NOT NULL,
  description    varchar(240) NOT NULL,
  icon           varchar(64) NOT NULL,
  -- Declarative condition evaluated by the gamification service, e.g. {"type":"lessons_completed","count":10}.
  condition      jsonb NOT NULL CHECK (jsonb_typeof(condition) = 'object' AND condition ? 'type'),
  display_order  integer NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_achievements (
  user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  achievement_key  varchar(64) NOT NULL REFERENCES achievement_definitions (key) ON DELETE RESTRICT,
  awarded_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, achievement_key)
);

-- +goose Down
DROP TABLE user_achievements;
DROP TABLE achievement_definitions;
DROP TABLE user_totals;
DROP TABLE xp_events;
DROP FUNCTION xp_events_append_only();
DROP TABLE hint_usage;
DROP TABLE challenge_attempts;
DROP TABLE lesson_progress;
