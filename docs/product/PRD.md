# KubeDojo V1 — Product Requirements

- **Status:** Accepted for implementation (Part 01)
- **Last updated:** 2026-10-22
- **Simulator target:** a documented subset of Kubernetes v1.37 (v1.37.1 was the latest patch on 2026-10-22)

## 1. Problem

People learning Kubernetes are stuck between two bad options. Reading docs and watching videos teaches vocabulary but not the feel of a cluster reacting to you. Real clusters (minikube, kind, cloud trials) teach that feel but cost setup time and money, break in confusing ways, and give no feedback about whether you did the exercise right. Beginners quit in the gap.

KubeDojo gives learners a cluster that reacts the way Kubernetes does to the commands they type, shows them *why* (desired state, controllers, ownership), and tells them when they have achieved the goal—without them installing anything.

## 2. Audience

| Persona | Starting point | What they want from KubeDojo |
| --- | --- | --- |
| **Complete beginner** ("Priya") | Knows what a container is, has never used kubectl | A guided path from "what is a Pod" to running and exposing an app, with no setup |
| **Application developer** ("Marco") | Deploys via CI, uses kubectl occasionally, panics when things break | Confidence reading cluster state and fixing common failures (ImagePullBackOff, Pending, missing endpoints) |
| **DevOps learner** ("Sam") | Comfortable with Linux and Docker, moving into platform work | Fast, repeatable practice of core workflows; a way to see controllers at work |

Not the audience for V1: experienced cluster operators, people who need a real cluster for production experiments, exam candidates who need full CKA/CKAD/CKS coverage.

## 3. Goals and non-goals

### Goals

1. A learner with no setup completes their first hands-on lesson within 10 minutes of landing on the site.
2. Learners build an accurate mental model of desired vs. observed state, controllers, labels/selectors and namespaces—not just command recall.
3. Every lesson is graded automatically and deterministically from the state the learner produced (or the answer they gave), never by matching typed text.
4. Learners come back: progress, streaks and achievements reflect real, server-verified work.
5. The project is credible open source: anyone can run it locally with one command, read why it is built the way it is, and contribute lessons.

### Non-goals (V1)

Unrestricted shell, SSH, real containers or networking, a production-grade control plane, the full kubectl CLI, CRDs/Helm/operators, a secrets vault, real-cluster sandboxes, paid plans, AI mentoring, community-submitted challenges, and any certification guarantee. See [simulator-compatibility.md](../architecture/simulator-compatibility.md) for the exact boundary.

## 4. Honesty principles

These are product requirements, not engineering preferences:

- **Labelled simulation.** The lab UI always shows that the cluster is simulated. Synthetic logs and events are marked. Lesson content explains where the simulator differs from a real cluster when that difference matters to the lesson.
- **No fake success.** A command the simulator does not support fails with a clear `UNSUPPORTED` message that says what to do instead. It never silently does nothing or pretends to succeed.
- **No fake numbers.** Dashboards show only real, server-computed statistics. A new user sees empty states, not sample data.

## 5. User journeys

### J1 — First visit to first completed lesson (Priya)

1. Lands on `/`, reads what KubeDojo is and that it is free and simulated. Clicks **Start learning**.
2. Browses `/learn`, opens *Kubernetes fundamentals → Cluster architecture* and reads the public lesson preview.
3. Clicks **Start lab**, is asked to sign in with GitHub, approves, and returns to the same lesson.
4. Completes a three-question onboarding (experience level, goal, optional handle). Can skip.
5. The lab opens: lesson text on the left; terminal, YAML editor and cluster map on the right (tabs on small screens).
6. Types `kubectl get nodes`. Output appears in the terminal; the nodes are highlighted in the cluster map.
7. Answers the lesson question ("How many nodes does this cluster have?") in the goals panel.
8. Clicks **Check my work**. Each goal shows pass/fail with a short explanation. All pass: the lesson is marked complete and XP is shown.
9. Sees **Next lesson** and a dashboard link showing 1/30 lessons complete and a 1-day streak.

### J2 — Fixing a broken deployment (Marco)

1. Opens *Troubleshooting → Image pull failure* from the dashboard's "recommended next".
2. The lab starts with a Deployment whose Pods are in `ImagePullBackOff`; the cluster map shows them in an error state.
3. Runs `kubectl describe pod …` and `kubectl get events`, finds the typo in the image name.
4. Either edits the manifest in the YAML editor and applies it, or runs `kubectl set image …`. Both are accepted because grading checks state.
5. Watches the old ReplicaSet scale down and the new one scale up in the cluster map.
6. Checks work, passes, earns XP minus a penalty for the one hint they revealed.

### J3 — Returning learner resumes (Sam)

1. Closed the tab mid-lab yesterday. Opens `/dashboard`, clicks **Continue**.
2. The same lab session resumes with identical state, terminal history and editor contents (sessions persist server-side).
3. Their network drops briefly; the terminal shows "Reconnecting…", reconnects, refetches the snapshot and continues. No command is replayed.

### J4 — Admin publishes a content revision

1. A contributor opens a PR that changes `content/challenges/workloads/scaling-deployments.yaml`. CI validates it against the JSON Schema and replays its reference solution in the simulator.
2. After merge and deploy, the new version is imported as a *draft* challenge version.
3. An admin opens `/admin/content`, reviews the diff and content hash, and publishes. New labs use the new version; labs already in progress keep the version they started with.

## 6. Feature requirements and acceptance criteria

| # | Feature | Priority | Acceptance criteria | Part |
| --- | --- | --- | --- | --- |
| F1 | Marketing site | P0 | `/`, `/learn`, `/about`, `/faq`, `/privacy`, `/terms`, `/changelog` render with real copy, are responsive and have no critical axe violations | 03 |
| F2 | GitHub sign-in | P0 | New GitHub account → exactly one KubeDojo user; repeat sign-in reuses it; failed provisioning blocks sign-in with a visible error | 06 |
| F3 | Onboarding | P1 | ≤ 3 questions, all skippable; completes in under 60 s | 08 |
| F4 | Course catalog and lesson reader | P0 | 6 courses / 30 lessons browsable without signing in; hidden goals and solutions never appear in public responses or bundles | 07, 17 |
| F5 | Persistent simulated lab | P0 | Lab state survives reload, API restart and reconnect; one active session per user per lesson | 09 |
| F6 | Terminal | P0 | Supported commands produce kubectl-like output; unsupported ones produce `UNSUPPORTED` with guidance; shell syntax is rejected | 10, 11, 14 |
| F7 | YAML editor | P0 | Edit virtual manifests with schema diagnostics; `kubectl apply -f <file>` applies them | 13 |
| F8 | Cluster map | P0 | Shows Nodes, Pods, ReplicaSets, Deployments, Services and their relationships; updates after each command; has an equivalent accessible list view | 15 |
| F9 | Grading | P0 | Deterministic; state, answer and observation goals ([ADR-0007](../adr/0007-challenge-goal-types.md)); per-goal feedback; completion awards XP exactly once | 16 |
| F10 | Hints | P0 | Revealed in order; each reveal recorded once; penalty shown before reveal | 16 |
| F11 | Progress and dashboard | P0 | Continue, recommended next, per-course completion, streak, XP; new users see empty states | 08, 18 |
| F12 | XP, streaks, achievements | P1 | Derived from an append-only ledger; streak uses the learner's time zone | 18 |
| F13 | Leaderboard | P1 | Only opted-in users appear; default is opted out | 18 |
| F14 | Admin content | P1 | Admins publish/unpublish imported content versions and read the audit log; cannot edit content text | 18 |
| F15 | Account export and deletion | P0 | Export returns the user's data as JSON; deletion erases personal data within 30 days | 06, 19 |

## 7. Success metrics

Measured from first-party, privacy-preserving activity events (no raw terminal input is collected; see [requirements.md](../architecture/requirements.md#privacy)).

| Metric | Definition | V1 target (90 days after launch) |
| --- | --- | --- |
| Activation | % of new sign-ins that complete at least one lesson within 24 h | ≥ 50% |
| Time to first completion | Median time from sign-in to first lesson completed | ≤ 10 min |
| Lesson completion rate | Lessons completed / labs started (per lesson) | ≥ 60% for every lesson; lessons below are revised |
| Week-1 retention | % of activated learners who complete a lesson 7–13 days after activation | ≥ 25% |
| Simulator honesty | `UNSUPPORTED` responses per 100 commands, broken down by command | Tracked; top 5 reviewed monthly to decide what to build or explain |
| Reliability | Successful command executions / all command submissions (excluding user errors) | ≥ 99.5% |

## 8. Release scope

V1 is done when the [definition of done in the master plan](../master-plan.md#v1-definition-of-done) holds and the launch criteria in [requirements.md](../architecture/requirements.md#launch-acceptance-criteria) pass.
