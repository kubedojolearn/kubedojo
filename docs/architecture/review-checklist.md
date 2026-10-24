# Open tradeoffs and review checklist

## Unresolved design tradeoffs

These are known open questions. None blocks Parts 02–08; each names the part by which it must be settled.

| # | Question | Options | Current lean | Settle by |
| --- | --- | --- | --- | --- |
| U1 | Should the outbox/worker be replaced by in-process notification while V1 runs one API replica? | Keep outbox (multi-replica safe) · in-process notify after commit · Postgres `LISTEN/NOTIFY` | Keep outbox (ADR-0004); revisit if the worker is the only reason to run a second process | Part 14 |
| U2 | Which hosting platform for production? | Fly.io · Render · a small Kubernetes cluster · a single VM with Compose | Undecided; must support a private network between web and API, managed PostgreSQL with PITR, and WebSockets through the edge | Part 19 |
| U3 | XP formula details | Plan's `base + firstTry − hints × penalty` · add time bonus | Plan's formula, floor at 10% of base; no time bonus (rewards rushing) | Part 18 |
| U4 | Streak rules | Calendar day in user's time zone · rolling 24 h | Calendar day in user's IANA time zone (`user_settings.time_zone`, default `UTC`); one completion per day keeps the streak | Part 18 |
| U5 | Should `kubectl explain` text be written by hand or generated from upstream OpenAPI? | Hand-written subset · generated + filtered | Generated from upstream v1.37 OpenAPI, filtered to supported fields, with a license check (Apache-2.0, attribution) | Part 10 |
| U6 | Visual regression tooling | Storybook + test runner · route-based component gallery + Playwright screenshots in a pinned container | Gallery + Playwright in the official Playwright container so baselines match locally and in CI | Part 03 |
| U7 | Should completed labs stay explorable (read-only commands allowed)? | Read-only after completion · fully frozen | Read-only commands allowed; mutations rejected with `CONFLICT` and a "this lab is complete — start it again to practise" message | Part 16 |
| U8 | Content import trigger | API startup · explicit operator command in the deploy pipeline | Explicit `kubedojo-api content import` in the deploy pipeline (startup imports hide failures) | Part 07 |

## Review checklist (every part's PR)

### Scope and contracts

- [ ] Implements only the requested part plus unavoidable dependencies.
- [ ] Changes to API shape are in [openapi.yaml](../api/openapi.yaml) first; generated client regenerated; contract tests updated.
- [ ] Changes to the WebSocket protocol bump `protocolVersion` if not backward compatible.
- [ ] New simulator behavior appears in [simulator-compatibility.md](simulator-compatibility.md) with its support level.
- [ ] New decisions with lasting consequences have an ADR.

### Correctness and safety

- [ ] Server remains authoritative: no client-supplied role, user ID, XP, score or completion is trusted.
- [ ] Every lab query enforces ownership in SQL.
- [ ] Mutations that must be atomic are in one transaction; failure paths roll back.
- [ ] Unsupported inputs return `UNSUPPORTED`/`UNSUPPORTED_COMMAND`, never fake success.
- [ ] No `os/exec`, shell or interpreter on user input.
- [ ] Threat model's security PR checklist completed for sensitive paths.

### Quality

- [ ] Tests added for new behavior, including failure paths; `make check` green; results reported honestly.
- [ ] No placeholder TODOs or sample data in production paths; incomplete UI is visibly marked.
- [ ] Accessibility: keyboard, focus, contrast, axe clean, reduced motion.
- [ ] Logs are structured, include the request ID, and contain no secrets.
- [ ] Docs updated (README, operations, authoring) where behavior changed.
