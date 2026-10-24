# Architecture decision records

Each ADR records one decision, the context that forced it and what it costs. ADRs are immutable once accepted; a later ADR supersedes an earlier one instead of editing it.

| ADR | Title | Status |
| --- | --- | --- |
| [0001](0001-modular-monolith.md) | Go modular monolith behind a Next.js BFF | Accepted |
| [0002](0002-simulator-not-real-cluster.md) | Simulate Kubernetes instead of hosting real clusters | Accepted |
| [0003](0003-auth-bff.md) | Auth.js BFF, signed internal assertions, edge proxy and explicit provisioning | Accepted |
| [0004](0004-pg-state-redis-events.md) | PostgreSQL holds all state; Redis only coordinates | Accepted |
| [0005](0005-versioned-content.md) | Lesson content lives in Git and is published as immutable versions | Accepted |
| [0006](0006-deterministic-reconciliation.md) | Deterministic reconciliation on a logical clock | Accepted |
| [0007](0007-challenge-goal-types.md) | Three challenge goal types: state, answer, observation | Accepted |

Template: Context → Decision → Consequences → Alternatives considered.
