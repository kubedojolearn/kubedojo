# ADR-0004: PostgreSQL holds all state; Redis only coordinates

- **Status:** Accepted · **Date:** 2026-10-22

## Context

A command can change several resources, emit events, bump the session version, record the command result and (for grading) award XP. Partial writes would corrupt a lab or double-award XP. Several API replicas may serve one learner (REST on one, WebSocket on another).

## Decision

- **PostgreSQL is the only system of record.** Lab state is stored as one row per resource in `lab_resources`. These rows are authoritative. A "snapshot" is a read model: the session's rows plus `lab_sessions.version`, assembled on request. There is no separate stored snapshot blob to drift from the rows.
- The **starting state** of every lab is the fixture embedded in its immutable `challenge_versions` row, so reset reloads from there; nothing else is needed to restore it.
- **Every mutation is one transaction:** `SELECT … FOR UPDATE` on the `lab_sessions` row → load resources → run the pure simulator → write changed/deleted resources, events and the command result → increment `version` and `command_seq` → insert an `outbox_events` row → commit. Any failure rolls back everything.
- **Command dedup:** `command_executions` has `UNIQUE (session_id, request_id)`. A retried `requestId` returns the stored result without re-executing.
- **One active session per lesson:** partial unique index on `lab_sessions (user_id, lesson_id) WHERE state = 'active'`.
- **Redis holds nothing durable:** WebSocket tickets (30 s), rate-limit counters, and pub/sub notifications. Flushing Redis drops connections and resets limits but loses no learner data.
- **Notifications:** the worker publishes committed `outbox_events` to Redis pub/sub channel `lab:<id>`; every API replica holding a socket for that lab sends `state.changed`. Clients then fetch `GET /snapshot`. Notifications carry only a version number, never state.

## Consequences

- Correctness doesn't depend on Redis or on message ordering; a lost notification only delays a refresh (clients also refresh on reconnect and on any version gap).
- The per-session row lock serializes writes to one lab. That matches reality (one learner, one terminal) and keeps the code simple.
- Outbox + worker adds a moving part. For a single-replica deployment it could be replaced by an in-process notify after commit; the outbox keeps multi-replica correct from day one and doubles as an audit trail of state changes.

## Alternatives considered

- **Event-sourced lab state (store only commands, replay to rebuild):** elegant with a deterministic simulator, but makes every read cost a replay and complicates simulator upgrades.
- **Redis as primary lab store:** fast but forces a second durability story and cross-store transactions with XP.
- **PostgreSQL `LISTEN/NOTIFY` instead of Redis pub/sub:** viable; Redis is already needed for tickets and rate limits, and its pub/sub does not hold a DB connection per listener.
