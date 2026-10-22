# Threat model

- **Status:** Accepted (Part 01); revisit at the end of each milestone.
- **Method:** asset-driven STRIDE over the data flows in [overview.md](../architecture/overview.md), with controls mapped to OWASP ASVS 5.0 chapters.
- Every control lists the part that implements it and the test that proves it. "Planned" means not yet implemented.

## Assets

| Asset | Why it matters |
| --- | --- |
| A1 User accounts and roles | Admin role can publish content to every learner |
| A2 Learner progress, XP, achievements | Integrity of the leaderboard and the learner's record |
| A3 Lab state | Learner's work; must stay private to them |
| A4 BFF signing key (Ed25519 private key) | Anyone holding it can act as any user against Go |
| A5 Auth.js secret, GitHub OAuth client secret | Session forgery; OAuth app impersonation |
| A6 Lesson content | Rendered to every learner; integrity and XSS surface |
| A7 Service availability | One noisy user must not degrade others |

## Trust boundaries

1. Internet ↔ edge proxy (everything from the browser is untrusted).
2. Edge ↔ Next.js (session cookies, BFF).
3. Next.js ↔ Go REST (private network; signed assertions).
4. Edge ↔ Go WebSocket (internet-facing; Origin + ticket).
5. Go ↔ PostgreSQL / Redis (private network; credentials).
6. Git ↔ content import (PR review + CI).

## Threats and controls

| # | Threat | STRIDE | Scenario | Controls | Part | Verification |
| --- | --- | --- | --- | --- | --- | --- |
| T1 | **Untrusted terminal input executes on the host** | E | Learner types `kubectl get pods; rm -rf /`, `$(curl …)`, or a long nested string to exhaust the parser | Allowlist tokenizer rejects `; && \|\| \| > < $( `` ` `` &` and newlines before parsing; only `kubectl`/`k`/`clear`/`help`; no `os/exec` anywhere in `apps/api` (CI grep + `forbidigo` lint); 16 KiB / 64-token caps; parser is non-recursive | 10 | Golden injection tests; lint rule fails the build on `os/exec` import |
| T2 | **Forged identity at the Go API** | S | Attacker reaches Go REST directly and sends a crafted JWT, an `alg: none` token, an HS256 token signed with the public key, a GitHub access token, or `X-User-Id` headers | Go verifies `EdDSA` only, `kid` must be in configured set, `iss`/`aud`/`exp`/`iat` required, lifetime ≤ 60 s, ±10 s skew; ignores all identity headers/body fields; REST not routed from the internet | 04 | Unit tests: tampered signature, wrong alg, `none`, wrong kid, wrong aud, expired, future `iat`, lifetime > 60 s, missing sub, non-`github:` sub |
| T3 | **Provisioning assertion misuse** | S/E | Attacker replays a provisioning assertion against other endpoints, or a request assertion against provisioning | Distinct audience `kubedojo-api-provision` accepted only by `POST /auth/provision`, which accepts nothing else | 04, 06 | Cross-audience tests both directions |
| T4 | **Signing key theft** | S | Key leaked from env, logs or a client bundle | Key only in Next.js server env (never `NEXT_PUBLIC_*`); build fails if the key name appears in client chunks; separate dev/prod keys; rotation procedure in ADR-0003; never logged | 02, 06 | Bundle scan test; gitleaks in CI |
| T5 | **IDOR on labs, files, attempts** | I/T | User A requests `/labs/{B's id}/snapshot` or sends commands to B's lab | Every lab query filters by `user_id` in SQL; non-owned labs return 404; IDs are UUIDv4 (not enumerable) but never relied on | 09+ | Integration tests: every `/labs/{id}/*` route as a second user returns 404 and changes nothing |
| T6 | **Cross-site WebSocket hijacking** | S | Malicious site opens a socket to our WS endpoint using the victim's cookies | WS ignores cookies entirely; requires `Origin ∈ WS_ALLOWED_ORIGINS` and a one-use ticket (30 s TTL, `GETDEL`) obtained via a same-origin BFF call; ticket bound to user and lab; unauthenticated sockets closed after 3 s; message size cap 32 KiB; ≤ 3 sockets per user per lab | 14 | Tests: bad Origin, missing/expired/reused ticket, ticket for other lab, first frame not `auth`, slow auth |
| T7 | **CSRF on BFF mutations** | T | Attacker's page submits a form to `/api/bff/labs/{id}/reset` | `SameSite=Lax` cookie; BFF rejects mutating requests whose `Origin` isn't `WEB_ORIGIN` and requires `Content-Type: application/json` | 06 | Playwright + unit tests with foreign Origin |
| T8 | **Double XP / replayed completion** | T | Learner double-clicks validate, retries after a timeout, or scripts parallel requests | Unique `xp_events.event_key` per user+lesson; XP insert and totals update in one transaction under the lab row lock; idempotency keys on validate/finish/reset | 05, 16, 18 | Concurrency test: 20 parallel validates → exactly one XP event |
| T9 | **Client-granted progress** | T | Learner calls an API with `{"completed": true, "xp": 9999}` | No endpoint accepts completion, score, XP or role; request schemas reject unknown fields | 04+ | Contract tests with extra fields → 400 |
| T10 | **Answer brute force** | T | Script tries every value for an answer goal | Max 10 submissions per goal per session; choice goals max `choices - 1`; rate limits | 16 | Tests at the limit |
| T11 | **Stored XSS via lesson content** | T/E | Malicious Markdown/HTML in a content PR, or an admin-compromised account publishing content | Content only via reviewed PRs (ADR-0005); admins cannot edit text; Markdown rendered without raw HTML except a sanitized allowlist; no MDX evaluation; strict CSP with nonces | 07, 18 | Renderer tests with XSS payload corpus; CSP header test |
| T12 | **XSS via simulator output** | T | Learner names a Pod `<img onerror=…>` (invalid DNS name) or puts HTML in labels/annotations/ConfigMap data, which is then rendered in the graph or lesson UI | Kubernetes name validation rejects most; all simulator output is rendered as text (xterm.js writes text; React escapes); no `dangerouslySetInnerHTML` on simulator data (lint rule) | 10, 14, 15 | Tests with HTML payloads in annotations rendered in every view |
| T13 | **Multi-tenant state leak via shared caches/pub-sub** | I | Notification or cached snapshot for lab X delivered to user Y | Pub/sub messages carry only `{labId, version}`; the socket for lab X was authorized for its owner; snapshots are never cached across users | 09, 14 | Test: two users, two labs, assert no cross-delivery |
| T14 | **Resource exhaustion** | D | Learner creates 10,000 Pods, a 50 MB manifest, floods commands or opens 1,000 sockets | Per-lab limits (100 resources, 50 Pods per command, 64 KiB files, 20 files); body size caps; rate limits (requirements S6); reconcile pass cap (32); per-request deadline (5 s) | 04, 09, 10, 14 | Limit tests; load test (Part 19) |
| T15 | **Secrets in logs** | I | Request bodies, YAML or Secret values logged; tokens in URLs | Structured logger with field allowlist; redaction of `authorization`, `cookie`, `ticket`; command text logged only as parsed verb+kind; tickets never in URLs | 02, 04 | Log-capture tests asserting absence |
| T16 | **Learners paste real credentials into simulated Secrets** | I | Learner uses a real API key as Secret data | UI warning on Secret creation and in lessons; Secret values excluded from logs, analytics and transcript exports; lab data purged per retention policy | 10, 13, 17 | Export and log tests |
| T17 | **Admin privilege escalation** | E | Learner sets `role: admin` via `PATCH /users/me`, or a compromised admin grants admin to others | `PATCH /users/me` accepts an explicit field allowlist; role changes only via operator CLI with audit log; no API grants roles | 05, 06 | Tests: role in body ignored/rejected |
| T18 | **Account takeover via email matching** | S | Future Google login with the same email merges into a GitHub account | Identities keyed by `(provider, provider_subject)`; no email stored or matched; linking requires both sessions | 06 | Test: two identities never merge |
| T19 | **Dependency / supply chain compromise** | T | Malicious npm/Go package update | Lockfiles committed; Dependabot with review; CI uses `--frozen-lockfile`; GitHub Actions pinned to commit SHAs | 02 | CI config review |
| T20 | **Open redirect after login** | S | `callbackUrl=https://evil.example` | Auth.js `redirect` callback allows only same-origin relative paths | 06 | Unit test |
| T21 | **Deleted-account token reuse** | S | Assertion minted just before deletion used afterwards | Identity link deleted at deletion; any assertion for that `sub` → 401 | 06, 19 | Test |

## Residual risks (accepted for V1)

- **No `jti` replay store** for request assertions (ADR-0003). Replay requires network access between Next.js and Go plus a token < 60 s old; impact is limited to what that user could already do.
- **Single signing key in Next.js** is a high-value secret. Mitigated by environment-only storage and rotation; a KMS-backed signer is a V2 option.
- **Simulator logic bugs** could let a lesson pass incorrectly. Not a security issue but an integrity one; mitigated by reference-solution CI and conformance tests.

## Review checklist for security-sensitive PRs

- [ ] Does any new code path accept identity, role, XP, score or completion from the client?
- [ ] Does every new lab query filter by the owner?
- [ ] Is any user text passed to a shell, template engine, regex compiler or `dangerouslySetInnerHTML`?
- [ ] Are new request bodies size-limited and schema-validated with unknown fields rejected?
- [ ] Could new log lines include secrets, YAML or Secret values?
- [ ] Is a new rate limit needed?
- [ ] Are CODEOWNERS for `apps/api/internal/auth`, `apps/web/lib/auth`, `apps/api/internal/commands` and `docs/security` on the review?
