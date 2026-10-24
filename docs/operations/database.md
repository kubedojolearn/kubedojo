# Database operations

## Migrations

Migrations are SQL files in `apps/api/db/migrations`, embedded in the API binary and applied with [goose](https://github.com/pressly/goose).

| Task | Command |
| --- | --- |
| Apply pending migrations | `make migrate-up` (or `api migrate up` in a container) |
| Show status | `make migrate-status` |
| Roll back the latest (development only) | `make migrate-down-dev` |
| Migrate and seed in one step | `api bootstrap` (used by the Compose `migrate` service and `make dev`) |
| Regenerate the query layer after changing SQL | `make sqlc` |

Rules:

- **Forward-only in production.** `api migrate down|reset` refuses to run when `APP_ENV=production`. Fix mistakes with a new migration.
- Every migration has a working `Down` section; CI applies all migrations from zero, rolls them all back, and applies them again (`TestMigrationsUpDownUp`).
- Never edit a migration that has been merged; add a new one.
- Destructive changes (dropping columns, tightening constraints) ship in two releases: stop using the column, then remove it.
- **Rollback procedure:** deploy the previous application version (it must tolerate the new schema, which is why changes are additive first). If data must be restored, use point-in-time recovery to a new instance and repoint `DATABASE_URL`.

## Seeds

`api seed` upserts reference data by natural key, so it is safe to run on every deploy:

- 6 course shells and 30 lesson shells from the [course outline](../product/course-outline.md), **unpublished**. The content import (Part 07) fills in text and versions.
- 9 achievement definitions.

## Initial admin

Admins are never created through the API. After the person has signed in once (so their account exists):

```sh
api admin set-role -handle their-handle -role admin -reason "initial maintainer"
```

The change and its reason are recorded in `audit_logs`. Use `-role learner` to revoke.

## Tests

Integration tests (`make test-integration`) never touch the development database. `internal/platform/testdb` migrates a template database once per schema version (named by a hash of the migrations) and gives every test its own copy, dropped afterwards. They fail rather than skip if `TEST_DATABASE_URL` is missing.

## Data retention and deletion

Retention windows are defined in [requirements.md](../architecture/requirements.md#data-retention-and-deletion). Jobs that enforce them run in the worker (`cmd/worker`, the `worker` Compose service). Lab expiry and idempotency cleanup run since Part 09, account erasure since the account release; the rest arrive in Part 19, and the schema already supports them:

| Job | What it does | Schema support |
| --- | --- | --- |
| Lab expiry (**running**, every minute) | Active labs idle for 14 days → `expired` (`IDLE_TIMEOUT`), in batches of 500 with `SKIP LOCKED`. A write that finds a lapsed lab first also expires it, and labs on an unsupported simulator version expire as `SIMULATOR_UPGRADED`. | `lab_sessions_expiry_idx` |
| Idempotency cleanup (**running**, hourly) | Delete `idempotency_records` older than 7 days | `idempotency_records_created_idx` |
| Account erasure (**running**, hourly) | Hard-delete accounts deleted more than 30 days ago, 100 per run, one transaction each (`account.EraseDue`) | `users.deleted_at`, cascades, `kubedojo.erasing_user` |
| Lab content purge | 30 days after a lab ends, delete its resources, files, events, commands and answers (session row and attempts stay) | `lab_sessions_ended_idx`, cascades from `lab_sessions` children |
| Command history cap | Delete `command_executions` older than 90 days | `command_executions_executed_at_idx` |
| Outbox cleanup | Delete published outbox rows after 7 days | `outbox_events_published_idx` |
| Audit retention | Delete audit entries older than 1 year | Trigger allows it only with `SET LOCAL kubedojo.audit_retention = 'on'` |

### Account deletion

1. **Immediately** (`POST /api/v1/users/me/deletion` with the handle typed as confirmation; one transaction): set `users.deleted_at`, clear `display_name`, `avatar_url`, experience level and goal, delete `oauth_identities`, turn off the public profile and leaderboard listing, end active labs (`ACCOUNT_DELETED`) and write a `user.deletion_requested` audit entry. The user can no longer sign in to this account; signing in again creates a new, empty account (tested).
2. **After 30 days** (worker, account erasure job): in a transaction that sets `SET LOCAL kubedojo.erasing_user = '<user id>'`, `DELETE FROM users WHERE id = …`. Cascades remove settings, totals, labs and their contents, progress, attempts, hints, XP events and achievements. Audit entries keep their row with the actor link set to `NULL`; they never contained personal fields. A `user.erased` entry records the erasure.
3. The handle stays reserved (step 1 keeps the row) until step 2 frees it.

### Export

`GET /api/v1/users/me/export` (Settings → Your data; 10 per hour) returns one JSON document built by a single query (`ExportUserData`): profile, linked identities, settings, totals, lesson progress, XP events, achievements, activity, and labs with their command history. Command output is exported as stored, so Secret values stay redacted. Lab files, answers, attempts and hints join the export when Parts 11–16 start writing them.
