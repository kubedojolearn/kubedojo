# ADR-0002: Simulate Kubernetes instead of hosting real clusters

- **Status:** Accepted · **Date:** 2026-10-22

## Context

Hosting a real cluster (or a vcluster/kind sandbox) per learner gives perfect fidelity but costs money per session, takes tens of seconds to start, needs strong isolation against container escape and crypto-mining abuse, and produces non-deterministic behavior that is hard to grade and test.

## Decision

V1 runs a **Kubernetes-inspired simulator** implementing a documented subset of Kubernetes v1.37 behavior ([simulator-compatibility.md](../architecture/simulator-compatibility.md)):

- Resources are typed Go objects stored per session in PostgreSQL. No container, Pod or network is ever created.
- Terminal input is parsed by an allowlisted kubectl grammar. It is never passed to a shell, `os/exec`, Docker or any interpreter.
- Behavior outside the subset fails with `UNSUPPORTED` and an explanation. The simulator never invents semantics to look complete.
- All simulated output (logs, events, IPs, timestamps) is labelled as simulated in the UI.
- Real clusters are used only in a private CI conformance suite (Part 19) that runs each lesson's reference solution against kind and compares the resulting object shapes with the simulator's, to catch drift.

## Consequences

- Zero marginal infrastructure cost per learner; labs start instantly and are deterministic, so grading and golden tests are exact.
- Fidelity is bounded. Lessons must be designed around supported behavior, and content must say where real clusters differ.
- Each new behavior costs simulator engineering; the compatibility matrix is a living product surface.

## Alternatives considered

- **Per-learner real clusters (kind/vcluster/k3s):** deferred to V2 as an optional "real lab" mode for advanced learners.
- **Recorded terminal sessions:** cheap but not interactive; learners can't explore.
