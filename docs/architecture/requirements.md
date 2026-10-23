# Non-functional requirements and launch criteria

- **Status:** Accepted (Part 01). Each requirement names the part that implements it and how it is checked.

## Accessibility

Target: WCAG 2.2 level AA for every page, including the lab.

| ID | Requirement | Check |
| --- | --- | --- |
| A1 | Every interactive element is reachable and operable by keyboard, with a visible focus indicator (≥ 3:1 contrast, not color alone). | Playwright keyboard tests (Part 03+) |
| A2 | Text contrast ≥ 4.5:1 (≥ 3:1 for large text and UI components) in both themes, including terminal colors and graph node styles. | Token contrast unit test (Part 03); axe |
| A3 | No critical or serious axe violations on any route. | `@axe-core/playwright` in CI (Part 03+) |
| A4 | The cluster map has an equivalent accessible view: a structured list/tree of the same resources and relationships, with the same updates announced via a polite live region. | Part 15 tests |
| A5 | Terminal output is exposed to screen readers (xterm.js screen-reader mode on by default when a screen reader preference is set; toggle in settings). Command results are announced. | Part 14 manual QA with VoiceOver and NVDA |
| A6 | Respect `prefers-reduced-motion` and the in-app reduced-motion setting: no animated graph transitions, no auto-scrolling animations. | Part 03/15 tests |
| A7 | Layout works from 320 px wide with no horizontal page scroll (the terminal may scroll internally). Lab panels become tabs below 1024 px. | Playwright viewport tests |
| A8 | Forms have labels, inline errors linked with `aria-describedby`, and error summaries. | Part 06/08 tests |
| A9 | The Monaco editor is optional on touch devices: a plain `<textarea>` fallback with the same validation is available. | Part 13 |

## Performance

| ID | Requirement | Budget |
| --- | --- | --- |
| P1 | Marketing and catalog pages: Largest Contentful Paint | ≤ 2.5 s at p75 on a mid-range phone (Lighthouse mobile profile) |
| P2 | Interaction to Next Paint, all pages | ≤ 200 ms at p75 |
| P3 | Lab page JS: terminal, editor and graph loaded lazily per panel | Initial lab route JS ≤ 350 KB gzipped before panels load |
| P4 | Command round trip (WS send → `command.result`), server side | p95 ≤ 150 ms, p99 ≤ 400 ms for a 100-resource lab |
| P5 | Snapshot fetch | p95 ≤ 200 ms for a 100-resource lab |
| P6 | Validation (grading) | p95 ≤ 300 ms |
| P7 | Capacity for launch | 500 concurrent active labs, 50 commands/s sustained, on 2 API replicas |

## Security

Baseline: OWASP ASVS 5.0 level 2 for authentication, session management, access control, input validation and API sections. Detailed threats and controls: [threat-model.md](../security/threat-model.md).

| ID | Requirement |
| --- | --- |
| S1 | No user-supplied text is executed by a shell, interpreter or `os/exec`. The kubectl parser is an allowlist. |
| S2 | Every `/labs/{id}` route checks ownership in SQL; cross-user access returns 404. |
| S3 | Go trusts identity only from a valid Ed25519 assertion; role only from PostgreSQL. |
| S4 | Browser never receives internal assertions, OAuth tokens or WS tickets for labs it does not own. |
| S5 | All mutating BFF routes check `Origin` against `WEB_ORIGIN` and require the Auth.js session; Auth.js's own CSRF protection covers sign-in/out. |
| S6 | Rate limits (Redis): commands 100/min/user; lab creation 20/hour/user; validation 30/min/user; answer submissions 10/goal/session; WS upgrades 30/min/IP; provisioning 10/min per GitHub identity (every request reaches the API from the BFF, so per-IP limits belong at the edge). |
| S7 | Logs never contain: OAuth tokens, assertions, tickets, session cookies, Secret values, raw YAML file content. Command text is logged only as its parsed verb and kind. |
| S8 | Security headers on web: CSP with nonces (no `unsafe-eval` outside Monaco's worker origin), `X-Content-Type-Options`, `Referrer-Policy: strict-origin-when-cross-origin`, `frame-ancestors 'none'`, HSTS in production. |
| S9 | Dependencies scanned (Dependabot), secrets scanned (gitleaks) on every PR; known-critical CVEs block release. |

## Testing

| Layer | Tool | Required coverage |
| --- | --- | --- |
| Simulator, parser, evaluator | `go test` | Golden tests for every supported command form and every lesson's reference solution; property-style tests for determinism (same inputs → same bytes) |
| Go services and repositories | `go test` + ephemeral PostgreSQL | Every query; transactions roll back on error; ownership checks; idempotency |
| API contract | Go tests against [openapi.yaml](../api/openapi.yaml) | Every implemented route's responses validate against the spec |
| Web units | Vitest + Testing Library | Components with logic; BFF assertion minting |
| End to end | Playwright | Journeys J1–J3 in [PRD](../product/PRD.md); auth substitutes only in test builds |
| Accessibility | axe in Playwright | Every route |
| Race detector | `go test -race` | All Go packages in CI |
| Conformance | kind cluster in a private CI job (Part 19) | Each lesson's reference solution produces the same object shapes as the simulator |

`make check` runs lint, typecheck, unit and integration tests and must be green before merge.

## Privacy

| ID | Requirement |
| --- | --- |
| PR1 | Data collected at sign-in: GitHub numeric ID, login, display name, avatar URL. Email is not requested or stored. |
| PR2 | Profiles are private by default. Leaderboard is opt-in. Public profiles show handle, display name, avatar, XP, completed lessons and achievements only. |
| PR3 | Activity analytics are first-party, tied to the user ID, and contain event type, lesson ID and timestamps only—never command text, YAML or answers. Users can opt out in settings; opted-out users still get progress and XP. |
| PR4 | No third-party trackers or ad pixels. |
| PR5 | Users can export all their data as JSON from settings (Part 06 starts the flow, Part 19 completes it). |

## Data retention and deletion

| Data | Retention |
| --- | --- |
| Account, profile, settings | Until the user deletes the account |
| Lab resources, files, events, command executions, answers | Active sessions: until 14 days of inactivity (then `expired`). Ended sessions: purged 30 days after ending. Command executions additionally capped at 90 days. |
| Lab session rows, challenge attempts, hint usage | Life of the account (needed for progress) |
| XP ledger, totals, achievements | Life of the account |
| Activity analytics events | 180 days |
| Audit logs | 1 year; never contain command text or YAML |
| Application logs | 30 days |
| Backups | 30 days rolling; deleted users' data ages out of backups within 30 days |

**Account deletion:** the user confirms in settings → `users.deleted_at` set, `oauth_identities` row deleted, sessions ended, profile fields cleared immediately; a worker deletes the remaining personal rows (labs, attempts, XP, achievements, settings, analytics) within 30 days. Aggregated, non-identifying counts (for example "lesson X completed N times") are kept. The handle is reserved for 30 days. Audit log entries keep the user ID but no personal fields.

## Launch acceptance criteria

V1 launches only when all of the following hold:

1. All 30 lessons are published, each with a reference solution that passes in CI, and each has been completed by at least two people who did not write it.
2. Journeys J1–J4 pass in Playwright against a production-like environment.
3. `make check` and all CI jobs are green on the release commit; zero critical/serious axe violations.
4. Performance budgets P1–P7 met in a load test of the release candidate.
5. Threat model controls marked P0 are implemented and tested; an external reviewer has done a security pass on auth, the BFF, the WebSocket and the parser.
6. Backups are taken automatically and a restore has been tested.
7. Account export and deletion work end to end.
8. Privacy policy, terms and the simulator-differences page are published.
9. On-call runbook covers: database down, Redis down, stuck outbox, abusive user, simulator bug in a published lesson (unpublish procedure).
