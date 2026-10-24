# Risk register

Delivery, product and operational risks. Security threats are in [threat-model.md](../security/threat-model.md). Likelihood (L) and impact (I) are rated 1–3; score = L × I. Review at every milestone.

| ID | Risk | L | I | Score | Mitigation | Trigger to act |
| --- | --- | --- | --- | --- | --- | --- |
| R1 | **Simulator scope creep**: each lesson pulls in "just one more" behavior until the simulator never stabilizes | 3 | 3 | 9 | Compatibility matrix is the contract; new behaviors need a lesson that requires them and a matrix PR; P2 behaviors wait for V2 | Any Part 11–13 PR adds a behavior not in the matrix |
| R2 | **Late integration**: the first playable lesson arrives only at Part 16, so design flaws in the simulator/lab loop surface late | 3 | 3 | 9 | Build a thin vertical slice of `workloads/scaling-deployments` (fixture → `scale` → validate) immediately after Part 09–10, before polishing UI | Part 10 merged without a lesson being playable through the API |
| R3 | **Simulator diverges from real Kubernetes** and teaches wrong behavior | 2 | 3 | 6 | Conformance suite against kind (Part 19); content states differences; reviewers with Kubernetes experience on lesson PRs | Conformance mismatch or learner report |
| R4 | **Lesson content takes longer than code** (30 lessons with reference solutions and fixtures) | 3 | 2 | 6 | Lesson templates in Part 07; write lessons in parallel with Parts 11–16 once the matrix is frozen | Fewer than 10 lessons drafted when Part 14 starts |
| R5 | **Lab UI is unusable on small screens** (terminal + editor + graph) | 2 | 2 | 4 | Tabs below 1024 px; textarea fallback for the editor; mobile is "works", desktop is "great" | Mobile Playwright journeys fail |
| R6 | **Auth.js major-version churn** (v5 had a long beta) | 2 | 2 | 4 | Pin exact version in Part 02; keep Auth.js usage behind `apps/web/lib/auth`; the Go side doesn't depend on Auth.js formats | Breaking release during V1 |
| R7 | **Kubernetes trademark / brand confusion** (name "KubeDojo", blue palette) | 2 | 2 | 4 | "Not affiliated with CNCF or The Linux Foundation" notice in footer and About; logo must not imitate the Kubernetes helm wheel; check the Linux Foundation trademark guidelines before launch | Before public launch |
| R8 | **Single maintainer bottleneck** on security-sensitive code | 2 | 2 | 4 | CODEOWNERS; ADRs and threat model let reviewers check against written intent | — |
| R9 | **Redis or worker outage stalls notifications** | 1 | 2 | 2 | Clients refresh on reconnect, on command results (they include the version) and on version gaps; correctness doesn't depend on notifications | Outbox lag alert > 30 s |
| R10 | **Hosting cost spikes from abuse** | 1 | 2 | 2 | Rate limits and per-lab caps; no per-learner compute | Spend alert |
