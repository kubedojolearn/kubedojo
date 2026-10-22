-- Curriculum: courses, lessons and immutable challenge versions (ADR-0005).

-- +goose Up
CREATE TABLE courses (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  slug           varchar(64) NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  title          varchar(80) NOT NULL,
  summary        varchar(200) NOT NULL,
  description    text NOT NULL DEFAULT '' CHECK (octet_length(description) <= 50000),
  difficulty     text NOT NULL CHECK (difficulty IN ('beginner', 'intermediate', 'advanced')),
  display_order  integer NOT NULL CHECK (display_order BETWEEN 1 AND 1000),
  published      boolean NOT NULL DEFAULT false,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX courses_published_order_idx ON courses (published, display_order);
CREATE TRIGGER courses_set_updated_at BEFORE UPDATE ON courses FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE lessons (
  id                            uuid PRIMARY KEY DEFAULT uuidv7(),
  -- Curriculum is shared: it is never deleted when users are, and courses with lessons can't be deleted.
  course_id                     uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
  slug                          varchar(64) NOT NULL CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  title                         varchar(80) NOT NULL,
  summary                       varchar(200) NOT NULL DEFAULT '',
  lesson_type                   text NOT NULL DEFAULT 'lab' CHECK (lesson_type IN ('lab', 'reading')),
  display_order                 integer NOT NULL CHECK (display_order BETWEEN 1 AND 1000),
  estimated_minutes             integer NOT NULL CHECK (estimated_minutes BETWEEN 1 AND 120),
  -- The published version new labs use; NULL means the lesson is not published.
  current_challenge_version_id  uuid,
  created_at                    timestamptz NOT NULL DEFAULT now(),
  updated_at                    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (course_id, slug),
  -- Deferrable so lessons can be reordered in one transaction.
  CONSTRAINT lessons_course_order_key UNIQUE (course_id, display_order) DEFERRABLE INITIALLY DEFERRED
);
CREATE TRIGGER lessons_set_updated_at BEFORE UPDATE ON lessons FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE challenge_versions (
  id                 uuid PRIMARY KEY DEFAULT uuidv7(),
  lesson_id          uuid NOT NULL REFERENCES lessons (id) ON DELETE RESTRICT,
  version            integer NOT NULL CHECK (version >= 1),
  content_sha256     char(64) NOT NULL CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
  -- The validated challenge document, including hidden goals and solution (never sent to browsers).
  spec               jsonb NOT NULL CHECK (jsonb_typeof(spec) = 'object' AND octet_length(spec::text) <= 1048576),
  simulator_version  varchar(64) NOT NULL,
  status             text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'retired')),
  imported_at        timestamptz NOT NULL DEFAULT now(),
  published_at       timestamptz,
  retired_at         timestamptz,
  UNIQUE (lesson_id, version),
  UNIQUE (lesson_id, content_sha256),
  CHECK ((status = 'draft') = (published_at IS NULL)),
  CHECK ((status = 'retired') = (retired_at IS NOT NULL))
);

ALTER TABLE lessons
  ADD CONSTRAINT lessons_current_version_fkey
  FOREIGN KEY (current_challenge_version_id) REFERENCES challenge_versions (id) ON DELETE RESTRICT;

-- Published content is immutable: only the status/retired_at bookkeeping may change, and a
-- published version can never go back to draft or be deleted.
-- +goose StatementBegin
CREATE FUNCTION challenge_versions_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF OLD.published_at IS NOT NULL THEN
      RAISE EXCEPTION 'challenge version % was published and cannot be deleted', OLD.id
        USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN OLD;
  END IF;
  IF OLD.published_at IS NOT NULL AND (
       NEW.spec IS DISTINCT FROM OLD.spec
    OR NEW.content_sha256 IS DISTINCT FROM OLD.content_sha256
    OR NEW.version IS DISTINCT FROM OLD.version
    OR NEW.lesson_id IS DISTINCT FROM OLD.lesson_id
    OR NEW.simulator_version IS DISTINCT FROM OLD.simulator_version
    OR NEW.published_at IS DISTINCT FROM OLD.published_at
  ) THEN
    RAISE EXCEPTION 'challenge version % is published and immutable', OLD.id
      USING ERRCODE = 'integrity_constraint_violation';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER challenge_versions_immutable BEFORE UPDATE OR DELETE ON challenge_versions
  FOR EACH ROW EXECUTE FUNCTION challenge_versions_guard();

-- A lesson's current version must be a published version of that same lesson.
-- +goose StatementBegin
CREATE FUNCTION lessons_current_version_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.current_challenge_version_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM challenge_versions cv
    WHERE cv.id = NEW.current_challenge_version_id AND cv.lesson_id = NEW.id AND cv.status = 'published'
  ) THEN
    RAISE EXCEPTION 'current version of lesson % must be a published version of the same lesson', NEW.id
      USING ERRCODE = 'integrity_constraint_violation';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER lessons_current_version_valid BEFORE INSERT OR UPDATE OF current_challenge_version_id ON lessons
  FOR EACH ROW EXECUTE FUNCTION lessons_current_version_guard();

-- +goose Down
ALTER TABLE lessons DROP CONSTRAINT lessons_current_version_fkey;
DROP TABLE challenge_versions;
DROP FUNCTION challenge_versions_guard();
DROP TABLE lessons;
DROP FUNCTION lessons_current_version_guard();
DROP TABLE courses;
