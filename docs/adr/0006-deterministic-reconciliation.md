# ADR-0006: Deterministic reconciliation on a logical clock

- **Status:** Accepted · **Date:** 2026-10-22
- Resolves plan-review item 3.

## Context

Real Kubernetes converges asynchronously over wall-clock time. If the simulator did the same, the same commands could produce different output depending on timing, golden tests would be flaky, and grading would depend on how long a learner waited. If it ignored time entirely, `AGE` would always read `0s`, rollouts would never show progress and CrashLoopBackOff restart counts could not grow.

## Decision

**Logical clock.** Each lab session has a simulated clock `simNow`, stored on `lab_sessions.sim_time`.

- It starts at the fixture epoch (`2026-01-01T00:00:00Z` unless the fixture sets `epoch`). Fixture objects may set `ageSeconds` to backdate `creationTimestamp`.
- **Every mutating command advances it by a fixed tick** of 10 s (`simulator.Tick`). The tick is part of a simulator version, not configuration: making it configurable would let two deployments produce different output for the same commands. Read-only commands do not advance it and never change any state, which keeps Part 10's rule "a read-only command never mutates state" true.
- Wall-clock time never enters the simulator. `AGE`, event timestamps and restart counts are all computed from `simNow`.

**Reconcile to a fixed point after every mutation.** After a mutating command is applied, controllers run in a fixed order until nothing changes:

1. Namespace lifecycle (deleting a namespace deletes its contents; there is no Terminating phase)
2. Node addresses (stable InternalIPs in name order)
3. Deployment controller (ReplicaSets, rolling update steps, revision history)
4. ReplicaSet controller (create/delete Pods to match replicas)
5. Scheduler (bind Pending Pods to Nodes)
6. Kubelet simulation (Pod IPs, image resolution, config references, crash profiles, readiness)
7. Service addresses (ClusterIPs, NodePorts)
8. EndpointSlice controller (Service → Pod endpoints with readiness)
9. Garbage collector (owner-reference cascade)

A pass that changes nothing ends the loop. More than 256 passes (`MaxReconcilePasses`) is a simulator bug: the transaction rolls back and the learner sees an internal error (with the request ID), never a half-reconciled cluster.

*Amended 2026-10-23 (Part 09):* the cap was first set at 32. A rolling update moves one step per pass, and the largest allowed Deployment (50 replicas with `maxSurge: 1, maxUnavailable: 0`) needs about 100 passes, so the cap is 256. It still stops a controller bug within milliseconds.

**Intermediate steps are visible.** The reconcile loop records each step as a synthetic Event (e.g. `ScalingReplicaSet`, `SuccessfulCreate`, `Scheduled`). `kubectl rollout status` prints the recorded rollout steps followed by the final result, so learners see a rolling update happen even though it completed within one transaction.

**Time-dependent behavior uses `simNow`:**

- CrashLoopBackOff restart counts follow the kubelet backoff (10 s doubling to 300 s) evaluated against elapsed simulated time each time the clock advances.
- Nothing else in V1 is time-dependent. There are no timers firing between commands; the cluster state shown to the learner is always fully reconciled.

**Determinism inputs.** Output is a pure function of (fixture, simulator version, session seed, ordered command list). UIDs, Pod name suffixes, Pod IPs, ClusterIPs and NodePorts come from a seeded generator stored on the session. Lists are sorted by namespace then name.

## Consequences

- Golden tests are exact; replaying a command list reproduces state byte for byte, which also powers lesson-solution CI.
- `kubectl get pods -w` (watch) and `kubectl wait` are not supported in V1; learners never need to wait for convergence. Lesson text explains that real clusters converge over seconds.
- A learner who only reads doesn't see restart counts grow. Crash lessons tell the learner to look at `RESTARTS` and logs, which are present from the start.

## Alternatives considered

- **Wall-clock-driven async controllers:** most realistic, non-deterministic, hard to test and grade.
- **Clock advances on every command including reads:** restart counts grow while reading, but reads would mutate session state and break the read-only invariant.
- **Explicit "advance time" button:** adds a concept learners don't have on real clusters; may return in V2 for timing-focused lessons.
