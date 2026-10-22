# Architecture overview

- **Status:** Accepted (Part 01)
- Decisions behind this design: [ADR index](../adr/README.md)

## 1. Components

```mermaid
flowchart LR
    subgraph Browser
        UI["Next.js pages<br/>xterm.js · Monaco · React Flow"]
    end

    subgraph Edge["Edge reverse proxy (Caddy / ingress) — single public origin"]
        R{{"route by path"}}
    end

    subgraph Web["apps/web (Next.js, Node runtime)"]
        Pages["Pages & RSC"]
        BFF["BFF /api/bff/*<br/>Auth.js session check<br/>mints Ed25519 assertion"]
        AuthJS["Auth.js<br/>GitHub OAuth"]
    end

    subgraph API["apps/api (Go, Chi) — private network"]
        REST["REST /api/v1/*<br/>assertion required"]
        WS["Realtime /api/v1/ws/*<br/>Origin + one-use ticket"]
        Domains["auth · users · courses · progress<br/>labs · commands · validators<br/>gamification · admin"]
        Sim["simulator (pure)"]
    end

    Worker["cmd/worker<br/>outbox · expiry · retention"]
    PG[("PostgreSQL<br/>system of record")]
    Redis[("Redis<br/>tickets · rate limits · pub/sub")]
    GitHub["GitHub OAuth"]

    UI -- "HTTPS" --> R
    UI -. "WSS" .-> R
    R -- "/api/v1/ws/*" --> WS
    R -- "everything else" --> Pages
    R --> BFF
    AuthJS <--> GitHub
    BFF -- "bearer assertion" --> REST
    REST --> Domains --> Sim
    WS --> Domains
    Domains --> PG
    Domains --> Redis
    Worker --> PG
    Worker -- "publish lab:{id}" --> Redis
    Redis -- "subscribe" --> WS
```

| Component | Owns | Trusts |
| --- | --- | --- |
| Browser | Nothing authoritative; UI state only | Nothing |
| Edge proxy | TLS termination, routing | — |
| Next.js / BFF | Browser sessions (Auth.js cookie), the signing key for internal assertions | GitHub (OAuth), Go responses |
| Go API | All business rules and authorization | Assertions signed by a configured `kid`; its own database |
| Worker | Background jobs | Its own database |
| PostgreSQL | All durable state | — |
| Redis | Ephemeral coordination | — |

## 2. Request flows

### 2.1 Sign-in and provisioning

```mermaid
sequenceDiagram
    autonumber
    participant B as Browser
    participant W as Next.js (Auth.js + BFF)
    participant G as GitHub
    participant A as Go API
    participant P as PostgreSQL

    B->>W: GET /login → "Sign in with GitHub"
    W->>G: OAuth authorize (state, PKCE)
    G-->>B: redirect to /api/auth/callback/github?code=…
    B->>W: callback
    W->>G: exchange code → access token + profile (id, login, name, avatar)
    W->>W: mint provisioning assertion (aud=kubedojo-api-provision, sub=github:<id>, profile claims, exp ≤ 60 s)
    W->>A: POST /api/v1/auth/provision
    A->>P: upsert oauth_identities + users (one transaction)
    A-->>W: 200 {user}
    W-->>B: Set-Cookie: Auth.js session (HttpOnly, Secure, SameSite=Lax) → redirect to original page
    Note over W,A: If provisioning fails, no session is created and /login shows an error.
```

### 2.2 Authenticated REST call

```mermaid
sequenceDiagram
    autonumber
    participant B as Browser
    participant W as Next.js BFF
    participant A as Go API
    participant P as PostgreSQL

    B->>W: POST /api/bff/labs {challengeVersionId} (session cookie, Origin)
    W->>W: verify Auth.js session, then check Origin (CSRF)
    W->>W: mint assertion (aud=kubedojo-api, sub=github:<id>, exp ≤ 60 s)
    W->>A: POST /api/v1/labs  Authorization: Bearer <assertion>
    A->>A: verify EdDSA signature, kid, iss, aud, exp, iat
    A->>P: resolve sub → user_id, role (oauth_identities, users)
    A->>P: business transaction (ownership checked in SQL)
    A-->>W: 201 {data, meta.requestId}
    W-->>B: 201 (sanitized, then no internal headers)
```

### 2.3 Lab connection, command and reconnect

```mermaid
sequenceDiagram
    autonumber
    participant B as Browser
    participant W as Next.js BFF
    participant A as Go API (REST)
    participant S as Go API (WS, any replica)
    participant R as Redis
    participant P as PostgreSQL
    participant K as Worker

    B->>W: POST /api/bff/labs/{id}/ws-ticket
    W->>A: POST /api/v1/labs/{id}/ws-ticket (assertion)
    A->>P: check lab owned by user and active
    A->>R: SET wsticket:<sha256> {userId, labId} EX 30
    A-->>B: {ticket} (via BFF)
    B->>S: WSS /api/v1/ws/labs/{id}
    S->>S: check Origin ∈ WS_ALLOWED_ORIGINS
    B->>S: {"type":"auth","protocolVersion":1,"ticket":"…"} (within 3 s)
    S->>R: GETDEL wsticket:<sha256>
    S->>P: load lab version
    S-->>B: {"type":"auth.ok","sessionId","snapshotVersion":42}
    B->>W: GET /api/bff/labs/{id}/snapshot
    W-->>B: snapshot v42

    B->>S: {"type":"command.run","requestId":"r1","input":"kubectl scale deploy nginx --replicas=3"}
    S->>P: BEGIN, then SELECT … FOR UPDATE lab_sessions
    S->>P: dedup on (session_id, request_id)
    S->>S: parse → simulate → reconcile (pure)
    S->>P: write resources, events, command result, version=43, outbox row, then COMMIT
    S-->>B: {"type":"command.result","requestId":"r1","sequence":7,"exitCode":0,"snapshotVersion":43,…}
    K->>P: read unpublished outbox rows
    K->>R: PUBLISH lab:<id> {"snapshotVersion":43}
    R-->>S: message (every replica with a socket on this lab)
    S-->>B: {"type":"state.changed","snapshotVersion":43}
    B->>W: GET /api/bff/labs/{id}/snapshot (only if local version < 43)

    Note over B,S: Connection drops
    B->>W: new ws-ticket, reconnect, auth → auth.ok {snapshotVersion: 43}
    B->>W: GET snapshot (always after reconnect)
    Note over B: A command whose result never arrived is shown as "result unknown".<br/>Re-sending the same requestId returns the stored result without re-executing.
```

### 2.4 Grading and XP

```mermaid
sequenceDiagram
    autonumber
    participant B as Browser
    participant W as Next.js BFF
    participant A as Go API
    participant P as PostgreSQL

    B->>W: POST /api/bff/labs/{id}/validate  Idempotency-Key: k1
    W->>A: POST /api/v1/labs/{id}/validate (assertion, Idempotency-Key)
    A->>P: BEGIN, then lock lab_sessions row
    A->>P: idempotency: existing attempt with key k1? → return it
    A->>A: evaluate goals against current state, answers and command log (pure)
    A->>P: insert challenge_attempts
    alt all goals pass
        A->>P: lab_sessions.state = completed
        A->>P: upsert lesson_progress (completed_at, best_score)
        A->>P: INSERT xp_events (event_key = lesson-complete:<user>:<lesson>) ON CONFLICT DO NOTHING
        A->>P: if inserted: update user_totals (total_xp, streak in user's time zone)
    end
    A->>P: COMMIT
    A-->>B: per-goal results, score, xpAwarded (0 if already awarded)
```

Duplicate XP is impossible by construction: the XP event key is unique per user and lesson, the insert and the total update share a transaction, and retries with the same idempotency key return the stored attempt.

## 3. Lab session lifecycle

```mermaid
stateDiagram-v2
    [*] --> active: POST /labs (fixture loaded, version 1)
    active --> active: command / file save / answer (version++ on mutation)
    active --> active: reset (state reloaded from fixture, version++)
    active --> completed: validate passes (XP awarded once per lesson)
    active --> abandoned: finish (learner ends lab)
    active --> expired: no activity for 14 days (worker)
    completed --> [*]
    abandoned --> [*]
    expired --> [*]
```

- **One active session per user per lesson** (partial unique index). `POST /labs` for a lesson with an active session returns that session (`200`, not `201`).
- **Completed, abandoned and expired sessions are read-only.** Starting the lesson again creates a new session; completing it again gives no XP.
- **Reset** never decreases `version`; clients treat it like any other state change.
- **Resource purge:** 30 days after a session leaves `active`, its resources, files, events and commands are deleted; the session row and attempt history remain for progress.
- **Simulator upgrades:** a session stores `simulator_version`. If the running API no longer supports it, the session is marked `expired` with reason `SIMULATOR_UPGRADED` and the learner is offered a fresh start; old state is never reinterpreted by a newer simulator.

## 4. Data ownership

| Data | Owner (writes) | Readers | Storage |
| --- | --- | --- | --- |
| Identity, roles | `auth`, `users` | all domains (via principal) | `users`, `oauth_identities` |
| Profile, settings | `users` | `progress`, `gamification` (leaderboard opt-in) | `users`, `user_settings` |
| Courses, lessons, challenge versions | `courses` (import), `admin` (publish) | `labs`, `validators`, `progress` | `courses`, `lessons`, `challenge_versions` |
| Lab sessions, resources, files, events, commands, answers | `labs` (via `simulator` + `commands`) | `validators`, `realtime` | `lab_*`, `command_executions` |
| Attempts, hints | `validators` | `progress`, `gamification` | `challenge_attempts`, `hint_usage` |
| Lesson progress | `progress` (only on verified completion or lesson open) | dashboard | `lesson_progress` |
| XP, streaks, achievements | `gamification` | dashboard, leaderboard, profile | `xp_events`, `user_totals`, `user_achievements` |
| Audit log | `admin`, `auth` | `admin` | `audit_logs` |
| Notifications | any writer, via outbox | `realtime` | `outbox_events` → Redis |

Rules:

- A domain writes only its own tables; other domains go through its Go interface.
- Every query on lab data includes `user_id` (directly or via a join to `lab_sessions`) so that ownership is enforced in SQL, not just in handlers. A lab that exists but belongs to someone else returns `404 NOT_FOUND`, not `403`, so lab IDs can't be probed.
- No domain trusts XP, scores, completion, role or user ID from a request body.

## 5. Questions a new developer should be able to answer

| Question | Answer |
| --- | --- |
| Where is truth stored? | PostgreSQL. Redis is disposable; the browser is a view. |
| Who authenticates each request? | Browser → Next.js: Auth.js session cookie. Next.js → Go: Ed25519 assertion (≤ 60 s). Browser → Go WebSocket: Origin check + one-use ticket issued through the BFF. |
| How does a lab reconnect? | New ticket → new socket → `auth.ok` with the current version → `GET /snapshot`. Commands are never replayed automatically; re-sending a `requestId` returns the stored result. |
| How is duplicate XP prevented? | Unique `xp_events.event_key` per user and lesson, inserted in the same transaction that updates totals; validation calls carry idempotency keys. |
