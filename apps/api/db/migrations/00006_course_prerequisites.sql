-- Course-level prerequisites (advisory, shown in the catalog).

-- +goose Up
ALTER TABLE courses
  ADD COLUMN prerequisites text[] NOT NULL DEFAULT '{}'
  CHECK (cardinality(prerequisites) <= 10);

-- +goose Down
ALTER TABLE courses DROP COLUMN prerequisites;
