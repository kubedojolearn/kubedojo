# KubeDojo V1 — Complete Full-Stack Implementation Master Plan

> **Editorial note (added when committing):** This copy contains the executive specification and Parts 01–10 only. Parts 11–20 were not included in the source document provided on 22 October 2026 and must be added before work on Milestone M3 (Part 11) begins.

- **Project:** KubeDojo — Master Kubernetes, one command at a time.
- **Organization:** kubedojolearn (as selected in GitHub setup)
- **Repository:** kubedojo
- **Document date:** 22 October 2026
- **Document status:** Implementation blueprint / proposed engineering specification, not implemented code
- **Execution strategy:** Complete each of the 20 parts in order. Each part contains build steps, deliverables, testing criteria, and a copy-ready coding-agent prompt.

## Executive specification

### Vision

KubeDojo is an independent, open-source, browser-based platform that teaches Kubernetes through interactive lessons, a safe stateful kubectl simulator, a visual cluster explorer, and automatic challenges. Learners should see how desired state, controllers, Pods, Deployments, Services, and namespaces interact—not merely memorize commands.

### V1 definition of done

A new learner can: (1) visit the landing page, (2) sign in with GitHub, (3) complete onboarding, (4) browse 30 fully implemented challenges across six courses, (5) start/resume a persistent simulated cluster, (6) edit YAML, (7) execute the supported kubectl subset in an authentic-looking terminal, (8) watch simulated objects change in a cluster map, (9) get hints and deterministic validation, (10) complete challenges and earn XP, and (11) track progress, streaks, achievements, and optional leaderboard rankings. An admin can publish and revise lesson content safely. The service has CI, tests, logging, metrics, backups, deployment, and accessible responsive UI.

### Important promises and exclusions

**What V1 is:** a Kubernetes-inspired simulator implementing a documented subset of Kubernetes v1.37 behavior. Resource state, pseudo-controllers, synthetic events, and synthetic logs must be labeled as simulations. Content must explain differences from real clusters. The official Kubernetes release page listed v1.37.1 as of 22 October 2026; re-check the latest patch when you actually code.

**What V1 is not:** unrestricted shell, SSH, actual Pod/container execution, real networking, production-grade Kubernetes control plane, full kubectl CLI, arbitrary CRDs/Helm, production secrets vault, real-cluster sandbox hosting, paid plans, or CKA/CKAD/CKS certification guarantees. Real clusters may be used only in a private CI conformance test environment, not in user sessions. Advanced real-lab hosting, AI mentoring, and community-submitted challenges are V2 unless capacity permits after V1 quality gates pass.

### Quality and safety defaults

- **Server is authoritative:** clients never grant XP, complete lessons, or mutate resource state directly.
- **Session isolation:** every virtual resource belongs to exactly one lab session and user; all reads/writes enforce ownership.
- **No os/exec for user text:** command parser accepts an allowlisted kubectl grammar only; do not run any command typed into the terminal on a server.
- **Transactional mutations:** commands, reconciliation, event sequence, and snapshots should commit together or roll back together.
- **No fake success:** unsupported flags, behaviors, or Kubernetes kinds return informative UNSUPPORTED errors rather than pretending to work.
- **Accessibility:** WCAG 2.2 AA-targeted UI with keyboard and screen-reader equivalents to the topology view.
- **Security baseline:** OWASP ASVS 5.0 guidance and WebSocket security recommendations.
- **Version pinning:** use currently maintained Node LTS, Go stable, PostgreSQL stable, and verified lockfile versions at implementation; do not guess precise future patch versions.

### Priority labels

- **P0:** Essential for functioning V1 and launch.
- **P1:** Required for a polished public V1; may have smaller initial depth than P0.
- **P2:** Defer unless all gates are met.

### Recommended technology decisions (pin versions during Part 2)

| Concern | Choice | Why |
| --- | --- | --- |
| Web application | Next.js App Router, TypeScript | SEO + modern product UI + BFF |
| UI | Tailwind CSS, shadcn/ui, Lucide | Fast, accessible, consistent system |
| Data fetching | TanStack Query | Client queries, caching, mutation and refresh |
| Local layout state | Zustand | Editor/terminal/graph panel settings |
| Terminal | xterm.js | Browser terminal rendering; not server shell |
| YAML editor | Monaco Editor | Diagnostics and manifest editing |
| Resource graph | React Flow (@xyflow/react) | Node/edge visual learning |
| Backend | Go + Chi | Modular API and deterministic engine |
| Persistent DB | PostgreSQL | User, courses, sessions, commands, resource data |
| SQL access | pgx/v5 + sqlc, goose migrations | Explicit SQL, type-safe generated data layer |
| Coordination | Redis | One-use connection tickets, rate limits, pub/sub |
| Authentication | Auth.js + GitHub OAuth | User-facing sessions held on web side |
| Internal auth | Short-lived server-signed JWT (Ed25519) | Trust boundary Next.js BFF → Go API |
| Streaming | WebSockets + REST snapshot recovery | Interactive output and resource updates |
| Metrics/tracing | OpenTelemetry and structured logs | Diagnosability |
| Test tools | Go test, Vitest, Playwright, axe | Core behavior + browser + accessibility |
| Local infra | Docker Compose, Makefile | One-command startup |
| CI/CD | GitHub Actions | Quality gates and tagged releases |

### Architecture overview

```text
Browser
  ├── Next.js public pages / authenticated app
  ├── xterm.js + Monaco + React Flow
  └── Auth.js session cookie (Secure, HttpOnly, SameSite)
        │ HTTPS (same origin)
        ▼
Next.js BFF /api/bff/*
  ├── Checks Auth.js server-side session
  ├── Creates short-lived Ed25519 signed internal user assertion
  └── Proxies typed REST requests to private Go API
        │ private network, bearer assertion
        ▼
Go API (modular monolith)
  ├── auth/identity mapping and DB roles
  ├── course/content service
  ├── learner/progress service
  ├── lab/session service + command adapter
  ├── pure simulator engine + reconciliation
  ├── challenge evaluator
  └── gamification/admin/telemetry
        │
        ├── PostgreSQL (system of record)
        └── Redis (tickets, throttling, fan-out)

Browser -- WSS --> Go /api/v1/ws/labs/{id}
  └── restricted Origin + one-use first-frame ticket (short timeout)
        └── verified user & session; command, output, state version, heartbeat
```

**Authentication detail:** Auth.js establishes the GitHub login and retains the provider's stable account ID in its server-validated session token. The BFF issues an Ed25519-signed, short-lived JWT with `iss=kubedojo-web`, `aud=kubedojo-api`, `sub=github:<immutable-provider-id>`, `iat`, `exp` (≤ 60 seconds), `jti`. Go validates it and maps the immutable provider ID to a local user_id via oauth_identities (insert on first trusted request). Go looks up roles in PostgreSQL; it never trusts an admin field from browser input. The browser never receives this internal JWT. GitHub is the V1 provider; adding Google later requires explicit identity linking, not matching email strings.

**WebSocket detail:** the BFF requests a one-use, 30-second ticket for a lab via an authenticated API call. Browser opens a WSS connection to the Go realtime endpoint, then sends `{type:"auth",ticket:"..."}` as its first frame within three seconds. Go validates Origin, rate-limits the upgrade, redeems the Redis ticket atomically, binds it to user/lab, and authorizes all later frames. Keep untrusted connections resource-limited, do not accept unauthenticated commands, and close on expiry/revocation. Use wss:// in production. REST snapshots make reconnect robust. Configure allowed web origins and deployment routing explicitly.

### Monorepo structure (final target)

```text
kubedojo/
├── apps/
│   ├── web/
│   │   ├── app/{(marketing),(auth),(app),api}/
│   │   ├── components/{layout,learning,terminal,graph,editor,ui}/
│   │   ├── features/{auth,courses,labs,progress,admin}/
│   │   ├── lib/{api,schemas,auth,utils}/
│   │   └── tests/
│   └── api/
│       ├── cmd/{api,worker}/
│       ├── internal/{auth,users,courses,progress,labs,simulator,commands,validators,gamification,admin,realtime}/
│       ├── internal/platform/{database,redis,config,logging,observability}/
│       ├── db/{migrations,queries,generated}/
│       └── tests/{integration,fixtures,conformance}/
├── packages/
│   ├── api-client/                  # Generated types/client from OpenAPI
│   ├── content-schema/              # JSON schemas + validator fixtures
│   └── eslint-config/
├── content/
│   ├── courses/                     # Human-reviewed versioned Markdown/YAML
│   ├── challenges/                  # Machine-validated challenge specs
│   └── fixtures/                    # Simulated initial cluster state
├── infra/{compose,deploy,monitoring}/
├── docs/{architecture,adr,api,security,content,operations}/
├── .github/{workflows,ISSUE_TEMPLATE}/
├── scripts/
├── .env.example
├── Makefile
├── pnpm-workspace.yaml
├── README.md
├── CONTRIBUTING.md
├── SECURITY.md
├── CODE_OF_CONDUCT.md
└── LICENSE
```

### Core domain contracts (freeze before features)

- **Course** → many **Lesson**; each Lesson points to immutable, versioned **ChallengeDefinition**.
- **ChallengeDefinition** contains introductory learning content, starter fixtures, accepted commands, initial virtual files, hints, and machine-checkable goals.
- **LabSession** = one user + one challenge version + initial snapshot + a versioned virtual Kubernetes state + lifecycle/expiry timestamps.
- **SimResource** = typed Kubernetes-like object with apiVersion, kind, metadata (name, namespace, uid, labels, ownerReferences, resourceVersion, generation), spec, status.
- **CommandExecution** = ordered sequence, request text, parsed operation, stdout, stderr, exit code, duration, state version; all outputs are simulation-generated.
- **ChallengeAttempt** records user, session, challenge version, score, result, hint usage, idempotency key, timestamps.
- **XPEvent** is an append-only ledger. XP balances are derived/updated transactionally from that ledger, not trusted from the client.
- **ClusterSnapshot** = serializable state with monotonically increasing version used by terminal, validator and visualizer.

### Database schema (logical tables; finalized in Part 5)

| Table | Essential columns / notes |
| --- | --- |
| users | id UUID PK, handle UNIQUE, display_name, avatar_url, role, created_at, last_seen_at, deleted_at |
| oauth_identities | (provider, provider_subject) UNIQUE, user_id FK, timestamps |
| user_settings | user_id PK, theme, reduced motion, leaderboard opt-in, privacy |
| courses | slug, title, difficulty, display_order, published flag |
| lessons | course FK, slug, title, lesson_type, order, estimated_minutes |
| challenge_versions | lesson FK, version, content SHA256, spec JSONB, published_at; immutable after publish |
| lesson_progress | (user_id, lesson_id) UNIQUE, best score, completed_at, last_started_at |
| lab_sessions | user/challenge FKs, state, version BIGINT, command_seq BIGINT, started/expires/last_active timestamps |
| lab_resources | (session_id,kind,namespace,name) UNIQUE, metadata/spec/status JSONB, resource_version |
| lab_files | (session_id,path) UNIQUE, YAML text with byte cap, updated_at |
| lab_events | session, version, seq, reason, type, resource key, message, occurred_at |
| command_executions | (session_id,sequence) UNIQUE, command, results, exit code, timing; bounded retention |
| challenge_attempts | session/challenge/user, idempotency key, outcome, score, hint count, evaluated_at |
| hint_usage | session + hint index + timestamp, unique |
| xp_events | user, event_key UNIQUE, delta, reason, attempt FK, created_at |
| user_totals | user PK, total_xp, completed_lessons, last_completion_date, streak |
| achievement_definitions | key, title, description, condition, icon |
| user_achievements | (user_id,achievement_key) UNIQUE, awarded_at |
| audit_logs | actor, action, entity, old/new IDs, redacted details, timestamp |
| outbox_events | type, aggregate ID, payload, created/published timestamps, retry count |

Indexes: all user/session foreign keys, lessons (course_id, display_order), progress (user_id, completed_at), lab resources (session_id, kind, namespace), command (session_id,sequence DESC), leaderboard (total_xp DESC, user_id), outbox unpublished rows. Keep schema changes in forward-only migration files with tested rollback procedures. Use ON DELETE rules deliberately; never cascade-delete shared curriculum when deleting a user.

### HTTP contract (initial OpenAPI outline)

All `/api/v1/*` endpoints return JSON except health, and require an internal assertion unless marked public. Browser calls `/api/bff/*`; Go is not intended as a public browser-auth endpoint.

```text
GET    /api/v1/healthz                                    [public]
GET    /api/v1/readyz                                     [internal]
GET    /api/v1/catalog/courses                            [public]
GET    /api/v1/catalog/courses/{slug}                     [public]
GET    /api/v1/catalog/lessons/{slug}                     [public preview; locked solutions omitted]
GET    /api/v1/users/me
PATCH  /api/v1/users/me
GET    /api/v1/users/me/progress
GET    /api/v1/users/me/achievements
GET    /api/v1/users/me/activity
GET    /api/v1/leaderboard                              [opt-in only]
POST   /api/v1/labs                                     [challenge_version_id]
GET    /api/v1/labs/{labID}
POST   /api/v1/labs/{labID}/resume
POST   /api/v1/labs/{labID}/reset
POST   /api/v1/labs/{labID}/finish
GET    /api/v1/labs/{labID}/snapshot
GET    /api/v1/labs/{labID}/events
GET    /api/v1/labs/{labID}/commands
POST   /api/v1/labs/{labID}/commands                    [REST fallback]
GET    /api/v1/labs/{labID}/files
PUT    /api/v1/labs/{labID}/files                       [virtual path + content]
POST   /api/v1/labs/{labID}/validate
POST   /api/v1/labs/{labID}/hints/{index}
POST   /api/v1/labs/{labID}/ws-ticket
WS     /api/v1/ws/labs/{labID}
GET    /api/v1/admin/content
POST   /api/v1/admin/content/validate
POST   /api/v1/admin/content/publish
GET    /api/v1/admin/audit
```

Response envelope: success `{ "data": ..., "meta": {"requestId":"..."}}`; error `{ "error": {"code":"...", "message":"...", "details":[]}, "meta":{"requestId":"..."}}`. Pagination: cursor + limit (max 100). Dates UTC RFC3339. Stable error codes: `UNAUTHENTICATED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `RATE_LIMITED`, `SESSION_EXPIRED`, `UNSUPPORTED_COMMAND`, `VALIDATION_FAILED`, `STALE_VERSION`. Use idempotency keys for reset, finish, and validation-completion operations; Go verifies ownership on every `/labs/{id}` call.

### WebSocket message contract (V1)

```json
{"type":"auth","ticket":"one-use-token"}
{"type":"auth.ok","sessionId":"uuid","snapshotVersion":42}
{"type":"command.run","requestId":"uuid","input":"kubectl get pods"}
{"type":"command.result","requestId":"uuid","sequence":7,"stdout":"...","stderr":"","exitCode":0,"snapshotVersion":43}
{"type":"state.changed","snapshotVersion":43,"eventCount":3}
{"type":"heartbeat","timestamp":"2026-10-22T10:00:00Z"}
{"type":"error","code":"UNSUPPORTED_COMMAND","message":"Pipes are not supported in simulator mode"}
```

All user-visible state comes from `GET /snapshot` after initial connect or a `state.changed` notification; this avoids fragile cross-worker incremental patch ordering. A command executes at most once per requestId in a lab; deduplicate command submission in PostgreSQL or a transactionally consistent execution table. `state.changed` is a notification, not an authoritative mutation. Client detects version gaps and refreshes snapshot. A reconnect never silently replays a command.

### Challenge schema (sample; use JSON Schema to validate authoring)

```yaml
id: intro-scale-deployment
version: 1
course: workloads
lesson: scaling-deployments
title: Scale a Deployment
simulatorVersion: kubernetes-1.37-subset-1
estimatedMinutes: 8
initialFixture: fixtures/nginx-one-replica.yaml
terminal:
  allowedFamilies: [get, describe, scale, rollout]
goals:
  - id: deployment-replicas
    resource:
      apiVersion: apps/v1
      kind: Deployment
      namespace: default
      name: nginx
    assertions:
      - path: spec.replicas
        operator: equals
        value: 3
      - path: status.readyReplicas
        operator: equals
        value: 3
hints:
  - "Try kubectl get deployments."
  - "kubectl scale can change the desired replica count."
  - "Try: kubectl scale deployment nginx --replicas=3"
reward:
  baseXP: 100
  firstTryBonusXP: 25
  hintPenaltyPerHintXP: 10
```

The challenge evaluator reads state, never an exact required command string. For example, YAML apply and scale both earn credit if the desired and observed state meets assertions. Never allow arbitrary user-provided JavaScript, JSONPath evaluation scripts, shell commands, or executable content in authoring specs. Provide a curated assertion DSL only.

### Six courses / 30 challenges (full catalog built in Part 17)

1. **Kubernetes fundamentals (5):** architecture overview; namespace navigation; listing resources; labels/selectors; declarative vs imperative.
2. **Pods & workloads (6):** launch a Pod; inspect Pod; troubleshoot Pending Pod; create Deployment; scale replicas; rollout and rollback.
3. **Networking (5):** Service basics; expose a Deployment; selectors/endpoints; ClusterIP vs NodePort concepts; debug missing endpoints.
4. **Configuration (4):** ConfigMaps; manifest edits; environment references (simulated); safe Secrets awareness (dummy values only).
5. **Cluster operations (5):** inspect Nodes; namespaces and scopes; labels/annotations; resource limits and scheduling concepts (documented simulation); events and diagnostics.
6. **Troubleshooting (5):** image pull failure (synthetic); CrashLoopBackOff (synthetic); wrong labels; mismatch in desired/current replica counts; capstone recovery.

### Milestones and dependency graph

```text
M1 Foundation:      Parts 01–06 (repo, API, schema, auth)
M2 Product:         Parts 07–08 (courses, dashboard)
M3 Core simulator:  Parts 09–13 (state, commands, controllers, YAML)
M4 Lab UX:          Parts 14–16 (terminal, graph, grading)
M5 Content/retain:  Parts 17–18 (30 labs, XP/admin)
M6 Public release:  Parts 19–20 (hardening, deploy, docs, release)
```

### Mandatory coding-agent instructions for every phase

Copy the prompt for the desired part, one part per coding session / PR. Every prompt inherits these rules:

> **Global agent contract:** You are implementing KubeDojo V1 in kubedojolearn/kubedojo, an independent public open-source monorepo. First inspect the existing repository, follow the architecture and contracts in docs/architecture/, and preserve work completed in previous phases. Implement only the requested phase plus unavoidable dependencies. Use Go + Chi + pgx/sqlc and Next.js TypeScript. Do not run arbitrary user terminal text on the host, invent unimplemented Kubernetes semantics, trust client-supplied role/user/XP, or store real secrets in simulated resources. Add migrations, schemas, error handling, tests, accessibility checks and docs for every new behavior. Prefer simple production-safe code over excessive abstractions. No placeholder TODOs or fake success paths for in-scope functionality. Run relevant lint/typecheck/test commands, fix failures, and finish with a concise list of files changed, commands run, tests passed/failed, risks, and manual QA instructions. Do not claim tests ran unless they actually ran. Wait for review before starting the next part.

---

## The 20 implementation parts

### Part 01/20 — Product specification, engineering RFCs, threat model and contracts

**Priority:** P0 · **Dependencies:** none · **Outcome:** eliminate guesswork before writing code.

#### Implement

1. Write docs/product/PRD.md: audience (complete beginners, developers, DevOps learners), goals, user journeys, success metrics, feature acceptance criteria, and realistic exclusions.
2. Write docs/architecture/overview.md with a Mermaid component diagram, REST + WS sequence diagrams, state-machine for a lab, and data ownership boundaries.
3. Add architecture decision records: 0001-modular-monolith, 0002-simulator-not-real-cluster, 0003-auth-bff, 0004-pg-state-redis-events, 0005-versioned-content, 0006-deterministic-reconciliation.
4. Write docs/security/threat-model.md covering untrusted terminal input, API/BFF identity forgery, broken object-level auth (IDOR), WebSocket CSRF/Origin, rate limiting, multi-tenant state leaks, admin content injection, replay/double XP, XSS, logs and simulated secret fields.
5. Define exact supported command matrix and resource matrix; mark supported, partial, concept-only, not supported, and explain simulation vs actual Kubernetes.
6. Draft OpenAPI 3.1 contract, JSON Schema for challenge YAML, WS event version (protocolVersion:1), and simulator state version (simulatorVersion).
7. Establish accessibility, performance, security, testing and privacy requirements; state user deletion behavior, retention windows and launch acceptance criteria.
8. Draft 30 lesson IDs/slugs and map them to supported simulator operations before content writing.

#### Outputs

docs/product/PRD.md, six ADRs, threat model, versioned API and challenge schemas, command support matrix, acceptance checklist, course outline.

#### Acceptance tests / gate

- A new developer can describe where truth is stored, who authenticates each request, how a lab reconnects, and how completion is prevented from awarding duplicate XP.
- Every V1 lesson maps to supported commands/resources and one or more state assertions.
- Security review identifies risks and planned controls, not just generic best practices.

#### Copy-ready implementation prompt

> PART 01 — SPECIFICATION AND ARCHITECTURE
>
> Read the KubeDojo V1 master plan and inspect the repository. Create a precise technical and product specification, six ADRs, risk register, OWASP-style threat model, simulator compatibility matrix, initial OpenAPI 3.1 contract, WS event schema, challenge JSON Schema, and an end-to-end user journey. Model GitHub OAuth -> Auth.js BFF -> short-lived signed Go API assertion; one-use WSS ticket; PostgreSQL resource snapshots; Redis fan-out; deterministic command execution; idempotent completion and XP. Define 30 challenge slugs mapped to supported behaviors. Explicitly separate simulated and real Kubernetes behavior. Deliver files under docs/ and packages/content-schema/; include Mermaid architecture, sequence and lifecycle diagrams, unresolved design tradeoffs and review checklist. No feature implementation yet. Validate all YAML/JSON specs. Report exact artifacts and any unresolved blockers.

### Part 02/20 — Monorepo, toolchains, local environment and CI skeleton

**Priority:** P0 · **Dependencies:** Part 01 · **Outcome:** clone → setup → run all services predictably.

#### Implement

1. Set up pnpm workspace for apps/web and packages/*; Go workspace/module for apps/api; commit pnpm-lock.yaml and pinned toolchain recommendations in .tool-versions or mise.toml.
2. Scaffold Next.js App Router + TS strict mode, Tailwind, UI primitives; Go Chi API with cmd/api/main.go, cmd/worker/main.go, config loading, graceful shutdown and structured JSON logging.
3. Configure Docker Compose for PostgreSQL, Redis, API and web. Add database health checks; web/API wait only for healthy prerequisites; do not bake credentials into images.
4. Add Makefile: make setup, dev, up, down, migrate-up, migrate-down-dev, seed, lint, typecheck, test, test-e2e, build, check.
5. Configure .env.example with explicit variable names and explanations, without real tokens; gitignore all actual .env* except examples.
6. Add ESLint, Prettier, Go fmt/vet, Go race-test hooks, dependabot or Renovate, secret scanning, license check, and CI workflow matrix.
7. Provide dev/test database isolation and test fixtures. Add status endpoints returning build version (no secrets).
8. Define branching and PR conventions (feat/part-03-design-system, Conventional Commits optional) and CODEOWNERS for sensitive paths.

#### Suggested environment keys

`DATABASE_URL`, `REDIS_URL`, `API_INTERNAL_URL`, `WEB_ORIGIN`, `AUTH_SECRET`, `AUTH_GITHUB_ID`, `AUTH_GITHUB_SECRET`, `BFF_SIGNING_PRIVATE_KEY`, `BFF_SIGNING_KID`, `API_JWT_PUBLIC_KEYS`, `WS_ALLOWED_ORIGINS`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `LOG_LEVEL`. Document how to generate and rotate keys. Use different signing keys for development and production.

#### Acceptance tests / gate

- Fresh clone follows README and launches web + Go + PG + Redis; /healthz and /readyz work as documented.
- make check is green in local and CI; build is reproducible from lockfiles.
- No secrets or .env.local in Git; Docker processes run with constrained permissions when practical.

#### Copy-ready implementation prompt

> PART 02 — MONOREPO AND DEVELOPMENT INFRASTRUCTURE
>
> Implement the repo layout from the agreed architecture, including apps/web Next.js + TS, apps/api Go + Chi, pnpm workspaces, Makefile, Docker Compose for PostgreSQL and Redis, health/readiness endpoints, strict lint/typecheck, unit-test scaffolding, OpenAPI validation, pre-commit quality gates and GitHub Actions. Pin vetted currently supported tool versions in lockfiles. Create a comprehensive .env.example, README quickstart and contributor setup. Support Mac/Linux development with optional Windows WSL notes. Do not add a second ORM or framework. Ensure `make setup`, `make dev`, `make check` are actually executable; test from a clean environment if possible. Write minimal tests and summarize commands run. No business features yet.

### Part 03/20 — Brand system, UX flows, responsive frontend shell

**Priority:** P0/P1 · **Dependencies:** Part 02 · **Outcome:** polished design with reusable components.

#### Implement

1. Establish KubeDojo palette: dark navy #09111F, surface #111C2E, primary Kubernetes blue #326CE5, accent cyan #22D3EE, success #22C55E, warning amber, error red; accessible light theme alternatives.
2. Import approved project logo as SVG/optimized PNG (do not assume the generated logo is a usable vector). Create app icon, favicon, OpenGraph and social image assets.
3. Define tokens for typography (Inter/Geist + JetBrains Mono), spacing, border, focus states, shadows, breakpoint behavior, terminal colors and graph node styles.
4. Build responsive marketing pages: /, /learn, /about, /faq, /privacy, /terms, /changelog; use real copy, semantic metadata, social cards.
5. Build authenticated app layout shell: sidebar, top nav, user menu, notifications, command palette/search placeholder only if marked disabled until functional, mobile drawer.
6. Build reusable skeletons, empty/error states, toast system, confirmation dialogs, keyboard shortcuts and nonblocking loading states.
7. Define Figma-equivalent wireframes in docs/design/screens.md: home, course catalog/detail, learner dashboard, lesson page, lab (terminal/editor/graph), profile, leaderboard, admin.
8. Add Storybook or equivalent component stories and visual snapshots for foundational components.

#### Route map

```text
/                  /learn              /learn/[course]
/learn/[course]/[lesson]               /login
/dashboard         /paths              /profile/[handle]
/settings          /leaderboard        /lab/[sessionId]
/admin             /admin/content      /admin/audit
```

#### Acceptance tests / gate

- No horizontal overflow on narrow phone, tablet or desktop; keyboard navigation and visible focus.
- Lighthouse / axe baseline recorded; no critical axe violations.
- The UI looks purposeful and consistent rather than a random set of shadcn components.

#### Copy-ready implementation prompt

> PART 03 — DESIGN SYSTEM AND FRONTEND FOUNDATION
>
> Implement a premium KubeDojo design system in apps/web using Next.js App Router, Tailwind and shadcn/ui. Use navy/blue/cyan Kubernetes-inspired branding, clean typography, dark and light themes, strong contrast, accessible keyboard states and responsive layouts. Build fully responsive marketing pages, signed-in application shell, navigation, empty/loading/error/success components, toast and modal primitives, metadata/SEO and polished realistic copy. Create design tokens, component stories, and docs/design/screens.md detailing every intended V1 screen. Routing can use documented sample data only where no API exists; visually mark incomplete interactive functionality and do not imply it works. Add rendering, navigation, accessibility and responsive Playwright tests. Keep components reusable and type-safe.

### Part 04/20 — Go API skeleton, internal auth boundary and OpenAPI clients

**Priority:** P0 · **Dependencies:** Parts 01–02 · **Outcome:** secure, typed request lifecycle.

#### Implement

1. Use Chi with middleware order: request ID, panic recovery, deadline, body-size cap, structured logs (redact secrets), secure headers, route-specific rate limit, auth, authorization, handler.
2. Organize internal/{handlers,services,repository} per domain; use explicit dependency injection and interfaces only for true boundaries (time, persistence, event bus).
3. Implement versioned GET /api/v1/healthz and internal readiness; environment config strict validation; graceful server shutdown.
4. Implement Ed25519 JWT verification with configured allowed key IDs; reject wrong issuer/audience/algorithm, bad time, expired tokens, missing/invalid immutable provider subject. Prevent accepting arbitrary external OAuth access tokens directly.
5. Build request-scoped Principal containing resolved DB user ID; roles come from the database, not user-supplied claims.
6. Define common JSON envelope, field validation, structured error codes, pagination, UTC clocks and request tracing.
7. Add OpenAPI-first contract tests and generated TS client in packages/api-client (choose a maintained generator); never hand-maintain duplicate API types.
8. Add a protected GET /users/me stub that will be completed with persistence in Parts 05/06. For public handlers, explicitly bypass internal assertion; make backend-to-backend network restriction documented.

#### Acceptance tests / gate

- Invalid JWT signatures, wrong audience, stale tokens and unexpected algorithms return 401.
- User role claims in request headers/body are ignored; protected endpoint denies absent identity.
- Generated client builds and OpenAPI lint passes.

#### Copy-ready implementation prompt

> PART 04 — BACKEND HTTP FOUNDATION AND API TRUST
>
> Build a production-quality modular Go Chi API under apps/api with config validation, graceful shutdown, correlation IDs, structured logs, recoverer, secure error envelopes, pagination, OpenAPI 3.1 and typed generated TypeScript API client. Implement Ed25519 BFF assertion validation (`iss`, `aud`, `sub`, `iat`, `exp`, `kid`) with strict algorithm allowlisting and clock skew policy. Never accept user identity/role via client headers, body, or raw OAuth token; the verified GitHub provider subject becomes a principal mapped to the DB in later phases. Build a bounded, testable middleware chain and 401/403/404 error semantics. Add unit and integration tests for signature tampering, replay windows, wrong keys/audience and privilege escalation attempts. Keep persistence behind interfaces without introducing mock production behavior.

### Part 05/20 — PostgreSQL schema, migrations, SQL repository layer and seed data

**Priority:** P0 · **Dependencies:** Part 04 · **Outcome:** robust relational source of truth.

#### Implement

1. Write goose SQL migrations for the logical schema in the executive specification. Use UUID IDs, TIMESTAMPTZ, bounded VARCHAR/TEXT, JSONB only for content specs and simulator snapshots, CHECK constraints for status/score/XP.
2. Use users + oauth_identities for immutable provider identity; reserve handle with case-insensitive uniqueness and suffix collisions. users.role defaults to learner; never from public signup.
3. challenge_versions are immutable after publishing; use (lesson_id,version) and sha256 uniqueness. Published lessons point to an explicit current version rather than rewriting in-progress sessions.
4. lab_sessions includes lifecycle enum (active, completed, expired, abandoned) and optimistic version; session rows are locking roots for all writes.
5. lab_resources unique session/kind/ns/name, with metadata/spec/status JSONB, object resource version, timestamps and indexes; adopt a consistent namespace sentinel for cluster-scoped objects (e.g. '').
6. Write db/queries/*.sql, configure sqlc pgx/v5 codegen and transaction helpers. No inline SQL in HTTP handlers.
7. Seed course shells, badge definitions, initial admin handle via controlled DB migration/operator path not public API; idempotent seeds.
8. Add integration tests with an ephemeral PostgreSQL test DB (Testcontainers or Docker Compose) for constraints, transactions, cascade rules, pagination and migration up/down in test only.
9. Include user data export/deletion plan with tombstone and retention; audit logs must not contain arbitrary command/YAML content.

#### Acceptance tests / gate

- CI applies migrations from zero; repeated seed is idempotent.
- Duplicate OAuth identity, identical user handle and duplicate XP event key cannot be persisted.
- Simultaneous lab writes cannot produce mismatched versions; SQL queries compile with sqlc.

#### Copy-ready implementation prompt

> PART 05 — DATABASE AND DATA ACCESS
>
> Implement complete PostgreSQL schema for users, oauth identities, settings, courses, lessons, immutable challenge versions, lesson progress, lab sessions/resources/files/events/command execution, challenge attempts, hint usage, append-only XP ledger, cached user totals, achievements, audit and outbox events. Use goose SQL migrations, proper foreign keys, unique constraints, timestamps, selective JSONB, indexes and row-level ownership at service boundaries. Generate a type-safe Go data-access package via sqlc+pgx/v5. Add transaction helpers for locking a lab session, incrementing a state version, inserting outbox events and idempotent XP grants. Write useful seeds and migration/integration tests. Avoid ORM drift and never log secret-like values. Deliver a concise ERD Mermaid diagram and data retention document.

### Part 06/20 — GitHub OAuth, user provisioning, BFF and authorization

**Priority:** P0 · **Dependencies:** Parts 04–05 · **Outcome:** secure sign-in and owned API calls.

#### Implement

1. Configure Auth.js GitHub OAuth, callbacks and correct callback URLs; AUTH_SECRET from secret management. Build /login and sign-out flow with meaningful auth error states.
2. Add immutable github:<providerAccountId> to the Auth.js server-validated session token during provider callback. Never build internal subject from editable GitHub username or public email.
3. On each authenticated BFF request, validate Auth.js session server-side, issue short-lived Ed25519 API JWT, call API_INTERNAL_URL, and return only sanitized API responses.
4. On first trusted request, Go atomically upserts the identity and user; assign unique handle; sync safe profile fields with strict length limits; prevent overwriting user-chosen display preferences on subsequent OAuth logins.
5. Protect dashboard, profile settings, lab and admin routes; use API authorization as final enforcement, not only frontend redirection.
6. Implement GET /users/me, PATCH /users/me (allowed fields only), avatar URL sanitation, settings and DELETE/EXPORT initiation UX (actual data deletion workflow covered by hardening).
7. Enforce CSRF protections appropriate to Auth.js and same-origin BFF mutations; strict cookie attributes; no API assertions in browser JS; keep BFF on a trusted Node runtime.
8. Add GitHub OAuth local dev instructions and test auth substitutes used only in tests, with hard production guards.

#### Acceptance tests / gate

- New OAuth identity gets one user; repeat login reuses it; distinct identities cannot merge by matching email.
- Browser cannot modify role, xp, user_id, or provider ID; all cross-user requests 403/404.
- Auth failure, expired login, cancellation, logout and deleted account paths are tested.

#### Copy-ready implementation prompt

> PART 06 — COMPLETE AUTHENTICATION AND AUTHORIZATION
>
> Implement GitHub OAuth through Auth.js, session-backed route protection, user provisioning and profile settings. Build the Next.js BFF for `/api/bff/*`: authenticate session server-side, derive immutable GitHub provider ID, mint ≤60-second Ed25519 JWT only on the server, forward to internal Go, and sanitize returned errors. Implement Go upsert of provider identity -> local user and enforce DB-controlled learner/admin RBAC on every protected resource. Add login/logout UI, handles, avatar, settings and mobile behavior. Test OAuth callback cancellation, invalid/expired sessions, JWT tampering, BFF CSRF, duplicate profiles, unauthorized admin access and lab IDOR. No OAuth secrets or internal tokens may enter client bundles or logs. Document production callback origin and signing-key rotation.

### Part 07/20 — Versioned curriculum and lesson content rendering

**Priority:** P0 · **Dependencies:** Parts 05–06 · **Outcome:** actual educational product, not empty course cards.

#### Implement

1. Create content/courses/*.yaml, content/challenges/*.yaml, and content/fixtures/*.yaml conventions, backed by JSON Schema validation at build/publish time.
2. Separate learner-facing description, objectives, prerequisites, concept reading, examples, references, starter manifest files, hidden validation goals and hints. Avoid shipping hidden solutions inside public API response/JS bundles.
3. Add backend catalog endpoints: course list/details, lessons, public preview and authenticated lesson content; publish state and versioned content hash.
4. Build course catalog with skill difficulty, expected time, modules, completion counts, filters (beginner/intermediate), searchable title and prerequisites.
5. Build lesson reader with MDX/Markdown rendered from trusted reviewed content only; sanitize any HTML and disallow arbitrary executable MDX contributed via admin.
6. Add Previous/Next, breadcrumbs, learning objectives, documentation links, glossary, callouts, keyboard navigation and embedded code blocks with copy action.
7. Track content quality metadata: last reviewed version/date, simulator compatibility, time estimate, expected initial state, validation fixtures.
8. Publish an initial three canonical lesson templates before building the remaining 30 in Part 17.

#### Acceptance tests / gate

- Publishing invalid schema is rejected with actionable errors.
- Unpublished and solution-only content is inaccessible to learners, even by URL guessing.
- All catalog/lesson pages support keyboard and mobile, link checks and SSR where appropriate.

#### Copy-ready implementation prompt

> PART 07 — CURRICULUM PLATFORM AND LESSON READER
>
> Implement versioned course/lesson/challenge content as reviewed files in content/, JSON Schema validation, publishing state, immutable challenge versions and typed catalog APIs. Add responsive `/learn`, `/learn/[course]`, `/learn/[course]/[lesson]` pages with filters, progress badges, prerequisites, markdown content, code examples, glossary, error/empty/loading UX and SEO. Do not expose hidden validators/solutions in public API responses or browser bundles. Provide a safe Markdown rendering approach, no arbitrary user MDX evaluation. Seed three real complete sample lessons with fixture references, make schema-validation CLI and write catalog API/UI tests. Update docs/content/authoring.md with examples, content review workflow, and accessibility requirements.

### Part 08/20 — Learner onboarding, dashboard, progress persistence and profile

**Priority:** P0 · **Dependencies:** Parts 06–07 · **Outcome:** a user has meaningful progress and a home base.

#### Implement

1. On first login show minimal onboarding: experience level, learning goal, optional display handle; defer unnecessary data collection.
2. Build /dashboard with continue-last-lesson, recommended next challenge, course progress, recent achievements, current streak, current XP, and learning activity.
3. Implement lesson_progress read/write service with completed_at, best score, last opened; do not mark completion here without verified challenge evaluator (Part 16).
4. Implement /profile/[handle], /settings, optional public profile toggle, avatar/name, theme, reduced-motion and leaderboards opt-in. Default profile privacy to conservative settings.
5. Build GET /users/me/progress and course progress aggregation in SQL with pagination/filtering. Memoize only safe public catalog data; invalidate progress after confirmed completion.
6. Add skeleton loading, clear offline/401/500 states, empty new-user onboarding UX and mobile navigation.
7. Add basic activity event capture for learning analytics, opt-out/retention and no raw terminal command telemetry.
8. Include user settings validation and user progress response contract in OpenAPI.

#### Acceptance tests / gate

- New user sees meaningful empty state; returning user sees last lesson and accurate completion count.
- Changing profile settings persists; private users do not leak profile activity.
- Progress cannot be edited via browser requests and excludes unpublished challenges.

#### Copy-ready implementation prompt

> PART 08 — LEARNER DASHBOARD AND PROFILES
>
> Implement onboarding, learner dashboard, persistent profile/settings and course progress aggregation using the existing Go API, sqlc repositories, BFF and Next.js UI. Include continue learning, recommended next challenge, course completion percentages, activity history, profile privacy, theme, reduced-motion, leaderboard opt-in and responsive account settings. Design backend-owned progress updates; completion/XP must remain locked behind the future verified challenge service. Add empty/loading/error states, accessible forms, optimistic updates only for reversible preferences and robust permission checks. Test multiple users, private profiles, page refresh, stale cookies, pagination and profile handle collisions. No hardcoded fake learner statistics in production routes.

### Part 09/20 — Stateful virtual Kubernetes resource engine

**Priority:** P0 · **Dependencies:** Part 05 · **Outcome:** the core learning simulator holds real logical state.

#### Implement

1. Create pure Go SimulatorEngine with State, SimResource, Command, Event, Clock, IDGenerator, ReconcileResult interfaces. Its methods accept an input state and produce a new state plus effects; no direct SQL calls inside core logic.
2. Implement supported V1 kinds: Namespace, Node (read-only seeded), Pod, Deployment, ReplicaSet, Service, ConfigMap, and Secret (dummy/training values; no real credentials). Event is stored as diagnostic output, not a user-created resource.
3. Validate apiVersion, kind, namespaces, names, required fields, immutable fields, labels, selectors, owner references, spec shape and size limits using a documented compatibility subset.
4. Create deterministic clock + seeded UID/IP allocators; stable ordering in list responses; ensure identical commands and fixture state produce deterministic results.
5. Implement versioned state persistence: take row lock on lab_sessions, load resource state, execute pure engine, reconcile, persist changed/deleted objects/events/command result, increment versions and append outbox event in one SQL transaction.
6. Enforce a bounded resource budget per lab: proposed 100 resources, max 100 commands/minute/user, max 16 KiB command, and max 64 KiB per virtual manifest (tunable config, not immutable product claims).
7. Add POST /labs, GET /labs/{id}, GET /snapshot, ownership checks, immutable challenge fixture reference, reset and TTL cleanup design.
8. State serialization must carry schemaVersion + simulatorVersion; migration/version rejection plan for old sessions.

#### Acceptance tests / gate

- Identical fixture+input yields identical resource state and outputs.
- User A never reads/mutates user B's session; concurrent writes serialize correctly; no half-committed state.
- Restarting API process does not erase live lab state; session version moves monotonically.

#### Copy-ready implementation prompt

> PART 09 — VIRTUAL KUBERNETES STATE ENGINE
>
> Implement a deterministic pure-Go Kubernetes resource state engine for Namespace, seeded Node, Pod, Deployment, ReplicaSet, Service, ConfigMap and safe dummy Secret. Define typed metadata/spec/status models, stable resource keys, namespace scoping, label selectors, resource version, generation, owner references, seeded clock/UID/IP generators and strict input validation. Persist state in PostgreSQL lab_sessions/lab_resources with serialized per-session transactional writes and an outbox event. Implement lab create/get/snapshot/reset and protected access. Load fixtures from immutable challenge versions, enforce TTL/resource quotas, and make all logic testable without a real cluster. Add golden tests for state, collision handling, optimistic concurrency, rollback and cross-user IDOR. No `os/exec`, Docker, real Pods, or real network calls from the simulation.

### Part 10/20 — Safe kubectl parser, read-only commands and terminal output engine

**Priority:** P0 · **Dependencies:** Part 09 · **Outcome:** users can inspect the simulated cluster.

#### Implement

1. Implement a tokenizer/parser supporting quoted values and allowlisted command grammar. Reject `;`, `&&`, pipes, shell substitution, redirects, unknown flags, sh, bash, curl, helm, direct OS commands and excessively nested input; do not pass commands to a shell.
2. V1 read operations: kubectl get pods|deployments|rs|services|namespaces|nodes|configmaps, singular/plural aliases, -n, -A, -l, -o wide|json|yaml; kubectl describe TYPE NAME; kubectl explain on documented subset; kubectl logs POD returns clearly simulated fixtures; kubectl get events returns simulated events.
3. Correct default namespace behavior and namespace existence checks; reject unsupported namespaces and cross-scope flags gracefully.
4. Format tables close to kubectl conventions: NAME, READY, STATUS, RESTARTS, AGE; use simulated clock for consistent AGE. JSON/YAML should include plausible but explicitly simulated metadata fields.
5. Implement human-friendly error messages (NotFound, bad flag, missing type, ambiguous name), usage, kubectl help, and supported-command discovery UX.
6. Add POST /labs/{id}/commands through BFF for initial synchronous terminal tests before WebSocket implementation. Store result sequence and dedup by client requestId.
7. Include terminal transcript exports in a sanitized bounded format; no real secret-like values in logs.

#### Acceptance tests / gate

- A read-only command never mutates state or grants XP.
- Unknown command returns correct nonzero code and clear error; malicious shell syntax is never executed.
- Golden output tests cover aliases, namespace, selector, formatting and unsupported flags.

#### Copy-ready implementation prompt

> PART 10 — KUBECTL PARSER AND READ COMMANDS
>
> Build a safe, explicitly allowlisted kubectl parser and read-only command adapter over the virtual Go simulator. Support help, get (Pod/Deployment/ReplicaSet/Service/Namespace/Node/ConfigMap), describe, explain for supported fields, synthetic logs/events, namespace flag, selectors, get all where documented, and table/wide/json/yaml output. Match representative kubectl formatting with stable test fixtures but label synthetic semantics. Tokenize carefully; disallow shell chaining, pipes, redirections, command substitution and all unsupported execution rather than trying to sanitize a shell. Expose protected command POST endpoint with requestId dedup and persisted sequence. Write golden tests for valid commands, wrong kind/name, namespace errors, flags, UTF-8, long payloads and injection attempts.

---

## Parts 11–20

Not included in the source document. Per the milestone graph above, they cover: mutation commands and controllers (Parts 11–13, including YAML editing/apply), terminal, cluster graph and grading UX (Parts 14–16), the full 30-lab catalog and XP/admin (Parts 17–18), and hardening, deployment, docs and release (Parts 19–20).
