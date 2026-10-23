-- Learning activity: the learner's own history (shown on their dashboard) and the source of
-- first-party learning analytics. Never contains command text, YAML or answers (requirements PR3).

-- +goose Up
CREATE TABLE activity_events (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  type         text NOT NULL CHECK (type IN ('lesson-opened', 'lab-started', 'lesson-completed', 'achievement-earned', 'hint-revealed')),
  lesson_id    uuid REFERENCES lessons (id) ON DELETE SET NULL,
  achievement_key varchar(64) REFERENCES achievement_definitions (key) ON DELETE SET NULL,
  xp_delta     integer,
  occurred_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX activity_events_user_recent_idx ON activity_events (user_id, occurred_at DESC, id DESC);
-- 180-day retention job (Part 19).
CREATE INDEX activity_events_occurred_idx ON activity_events (occurred_at);

-- +goose Down
DROP TABLE activity_events;
