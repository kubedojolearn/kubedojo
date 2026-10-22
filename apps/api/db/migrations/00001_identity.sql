-- Identity: users, linked OAuth identities and per-user settings.
-- Conventions for all migrations: UUID keys (uuidv7 for time-ordered rows, gen_random_uuid for
-- IDs that appear in URLs), TIMESTAMPTZ in UTC, bounded text, CHECK constraints for enums.

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TABLE users (
  id                       uuid PRIMARY KEY DEFAULT uuidv7(),
  -- GitHub-style handle; unique case-insensitively, including tombstoned users (reserved 30 days).
  handle                   varchar(39) NOT NULL
                           CHECK (handle ~ '^[A-Za-z0-9]([A-Za-z0-9-]{0,37}[A-Za-z0-9])?$'),
  display_name             varchar(80) NOT NULL DEFAULT '',
  -- Set when the user edits their name, so later sign-ins don't overwrite it with GitHub's.
  display_name_customized  boolean NOT NULL DEFAULT false,
  avatar_url               varchar(512) CHECK (avatar_url ~ '^https://'),
  -- Never set from public sign-up; changed only by the operator CLI (with an audit entry).
  role                     text NOT NULL DEFAULT 'learner' CHECK (role IN ('learner', 'admin')),
  experience_level         text CHECK (experience_level IN ('new', 'some', 'comfortable')),
  learning_goal            text CHECK (learning_goal IN ('fundamentals', 'troubleshooting', 'certification-prep', 'work-project', 'curiosity')),
  onboarding_completed_at  timestamptz,
  created_at               timestamptz NOT NULL DEFAULT now(),
  updated_at               timestamptz NOT NULL DEFAULT now(),
  last_seen_at             timestamptz,
  deleted_at               timestamptz
);
CREATE UNIQUE INDEX users_handle_lower_key ON users (lower(handle));
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Immutable provider identity -> local user. Never matched by email (ADR-0003).
CREATE TABLE oauth_identities (
  provider          text NOT NULL CHECK (provider IN ('github')),
  provider_subject  varchar(32) NOT NULL CHECK (provider_subject ~ '^[1-9][0-9]{0,19}$'),
  user_id           uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  -- Last seen provider username; informational only, never used for identity.
  provider_login    varchar(39) NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  last_login_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (provider, provider_subject),
  UNIQUE (user_id, provider)
);

CREATE TABLE user_settings (
  user_id             uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  theme               text NOT NULL DEFAULT 'system' CHECK (theme IN ('system', 'dark', 'light')),
  reduced_motion      boolean NOT NULL DEFAULT false,
  -- Conservative privacy defaults (requirements PR2).
  leaderboard_opt_in  boolean NOT NULL DEFAULT false,
  profile_public      boolean NOT NULL DEFAULT false,
  analytics_opt_out   boolean NOT NULL DEFAULT false,
  -- IANA zone name (validated by the API); used for streak calendar days.
  time_zone           varchar(64) NOT NULL DEFAULT 'UTC' CHECK (time_zone ~ '^[A-Za-z0-9_+/-]{1,64}$'),
  updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER user_settings_set_updated_at BEFORE UPDATE ON user_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE user_settings;
DROP TABLE oauth_identities;
DROP TABLE users;
DROP FUNCTION set_updated_at();
