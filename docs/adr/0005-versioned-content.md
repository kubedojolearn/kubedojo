# ADR-0005: Lesson content lives in Git and is published as immutable versions

- **Status:** Accepted · **Date:** 2026-10-22
- Resolves plan-review "two sources of content".

## Context

The plan says content lives in reviewed files under `content/` *and* that admins can "publish and revise" lesson content. Two editable sources would drift, and an admin text editor is an injection surface (admin-authored Markdown rendered to every learner).

## Decision

- **Git is the only source of lesson content.** `content/courses/*.yaml`, `content/challenges/**.yaml` and `content/fixtures/**.yaml` are edited only through pull requests.
- **CI validates every change:** JSON Schema ([packages/content-schema](../../packages/content-schema/README.md)), cross-references (every lesson has a challenge, every fixture exists, every kind is supported), and—from Part 17—a replay of each challenge's hidden reference solution through the simulator, which must pass all goals.
- **Import, then publish.** A release includes a content bundle. On deploy, `kubedojo-api content import` inserts any challenge version whose SHA-256 is new as a **draft**. Importing is idempotent.
- **Admins publish or unpublish versions; they cannot edit text.** Publishing sets `lessons.current_challenge_version_id`. Every publish/unpublish is written to `audit_logs`.
- **Published versions are immutable.** A database trigger rejects updates to `spec`/`content_sha256` once `published_at` is set. A changed lesson is a new version number.
- **In-progress labs keep their version.** `lab_sessions.challenge_version_id` is fixed at creation; new labs use the lesson's current version.
- Lesson Markdown is rendered with a fixed, safe Markdown pipeline (no MDX evaluation, raw HTML sanitized to an allowlist). See Part 07.

## Consequences

- Every content change has review, history and CI proof that it is solvable.
- Fixing a typo needs a PR and a deploy. Acceptable for V1; a later "content-only release" pipeline can shorten this.
- The admin UI is small: list versions, show diff and hash, publish/unpublish, read audit log.

## Alternatives considered

- **CMS / admin editor as source of truth:** faster edits, but no code review, weaker validation, and a stored-XSS surface.
- **Content compiled into the API binary:** simple, but removes the publish step and the ability to stage a version before learners see it.
