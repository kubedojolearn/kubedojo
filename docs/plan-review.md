# KubeDojo V1 Master Plan — Review

- **Reviewed document:** [docs/master-plan.md](master-plan.md) (executive specification + Parts 01–10)
- **Review date:** 22 October 2026
- **Scope note:** Parts 11–20 were not in the source document and are not covered by this review.

## Summary

The foundations are solid. The trust boundary is right: the browser signs in, the Next.js server makes short-lived signed tokens, and Go checks roles in its own database. The server is authoritative, XP comes from an append-only ledger with idempotency keys, typed commands are parsed against an allowlist and never run in a shell, and state is versioned so a reconnect just refetches it. The problems are in grading, a few contradictions, and the order of the parts.

## Fix before Part 01 is finalized

### 1. About a third of the lessons can't be graded

The grader only checks cluster state, never which commands were typed. But lessons like "listing resources", "inspect Pod", "namespace navigation", "inspect Nodes", "events and diagnostics" and "architecture overview" don't change anything, so a learner who does nothing and a learner who does the work look the same. Part 01's gate ("every lesson maps to one or more state assertions") fails for about 8–10 of the 30.

**Recommendation:** add a second kind of goal to the assertion language, either:

- an **answer goal** ("what node is `web-2` on?"), checked on the server against the current state, or
- a check that a **certain kind of command succeeded on a certain object**, using the parsed command records rather than matching the raw text.

### 2. Some lessons need behavior the simulator doesn't have

Part 09 doesn't define any of these:

| Lesson | Missing simulator behavior |
| --- | --- |
| Debug missing endpoints; selectors/endpoints | An Endpoints or EndpointSlice resource |
| Troubleshoot Pending Pod; resource limits and scheduling | A small scheduler: node capacity, requests, and nodeSelector or taints |
| Image pull failure; CrashLoopBackOff | A fixed list of images that work, and how the others fail |
| Namespace navigation | `kubectl config set-context --namespace`, which means a simulated kubeconfig |
| Labels/annotations | `kubectl label` and `kubectl annotate` (not listed anywhere) |
| Safe Secrets awareness | `kubectl get secrets` (Secret is a supported type in Part 09, but Part 10 leaves it out of `get`) |

**Recommendation:** the command and resource matrix in Part 01 must cover all of these before lesson slugs are frozen.

### 3. Simulated time isn't defined

"status.readyReplicas = 3", rollout status, AGE and CrashLoopBackOff all depend on how time moves. If state settles instantly after each command, AGE is always 0s and a rollout never shows progress. If it follows the real clock, tests aren't repeatable.

**Recommendation:** settle state right after each command (with a cap on loop iterations) and advance a logical clock by each command's simulated duration. Use real-time progress only for scripted failures. Record this in ADR-0006.

### 4. The Go API is called private, but the browser connects to it directly over WebSocket

Next.js route handlers can't proxy WebSocket upgrades, so you need a reverse proxy (Caddy, nginx or an ingress) that exposes only `/api/v1/ws/*` publicly. That affects the Part 02 Docker Compose setup and deployment.

**Recommendation:** add the reverse proxy to the architecture and to Docker Compose. Once that path is public, the signed token check is the real protection, and the threat model should say so.

### 5. Go has no way to get profile data at first sign-in

The token carries only `sub`, but Go creates the user on the first request and needs a handle, name and avatar.

**Recommendation:** either put signed `login`/`name`/`avatar` fields in the token, or add a separate provisioning call. Also decide what happens when a deleted user signs in again.

## Contradictions and loose ends

- **Two sources of content.** Lessons live in Git under `content/`, but "an admin can publish and revise lesson content". Make Git the only source, with the admin screen limited to publishing or unpublishing bundles that passed validation. That also mostly removes the "admin content injection" risk.
- **Two models of lab state.** The plan has both "ClusterSnapshot" and per-resource rows (`lab_resources`). Pick the rows as the source of truth, treat the snapshot as a computed view, and store the starting snapshot separately for reset.
- **Unique token IDs are never checked.** The token carries a unique ID (`jti`) and Part 04 tests "replay windows", but nothing stores used IDs. Either keep a Redis set of used IDs, or accept 60-second reuse and record that choice in an ADR.
- **Streaks have no time zone.** `user_totals.last_completion_date` needs one, so add a time zone to `user_settings`.
- **Sessions per lesson aren't limited.** Add a partial unique index so each user has at most one active session per challenge. Otherwise "resume" is ambiguous.
- **"100 commands/minute/user" is a rate limit, not a lab budget.** It belongs in Redis rate limiting, not the per-lab resource budget in Part 09.

## Order and scope

- **Part 09's dependencies are wrong.** It actually needs Part 06 (owned sessions require users) and Part 07 (starting state comes from published challenges), not just Part 05.
- **Part 08 comes too early.** Its dashboard shows XP, streaks and achievements before Part 18 builds them, which conflicts with "no hardcoded fake learner statistics". Ship it with empty states, or move those widgets to Part 18.
- **The first playable lesson doesn't arrive until Part 16.** About 15 PRs pass before anyone can play one lesson end to end, and the simulator's design risks show up late. Build one thin version of the scale-deployment lab right after Part 06, rough but working end to end, and then deepen it. Split Part 03 too: design tokens and the app shell early, marketing pages later.
- **Mobile lab experience.** Monaco works poorly on phones, and a terminal, editor and graph on one phone screen isn't realistic. Define mobile as a tabbed layout, or read and review only.
- **`kubectl explain`** needs a written body of field documentation. That's real content work, so give it its own budget or reduce its scope.
- **Simplification option:** with one API instance in V1, the outbox, worker and Redis pub/sub could become a notification sent in-process after commit, or Postgres LISTEN/NOTIFY. Keep the outbox if you expect several instances at launch.

## Smaller items

- The plan uses the exact Kubernetes brand blue (#326CE5) and "Kube" in the name. Add a "not affiliated with CNCF/The Linux Foundation" notice and check the Linux Foundation's trademark guidelines.
- Part 03 says to "import approved project logo", but there's no logo in the repo yet.
- The repo is already Apache-2.0 licensed, which matches the plan.
- Auth.js v5 spent a long time in beta. Confirm it has a stable release when Part 02 pins versions.
