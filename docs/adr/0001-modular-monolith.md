# ADR-0001: Go modular monolith behind a Next.js BFF

- **Status:** Accepted · **Date:** 2026-10-22

## Context

V1 is built by a very small team. The domain has a few tightly coupled parts (lab state, the simulator, grading, XP) that must change together inside one database transaction. The web front end needs server-side rendering for SEO and a trusted place to hold the user's session.

## Decision

- One Go service (`apps/api`, Chi router) contains every backend domain as an internal package: `auth`, `users`, `courses`, `progress`, `labs`, `simulator`, `commands`, `validators`, `gamification`, `admin`, `realtime`. Packages talk through Go interfaces and function calls, never over the network.
- A second binary from the same module (`cmd/worker`) runs background jobs (outbox publishing, session expiry, retention). It shares the code and database; it has no HTTP API of its own.
- Next.js (`apps/web`) renders pages and acts as a backend-for-frontend (BFF): the only place browser sessions are checked and the only caller of the Go REST API.
- The simulator package (`internal/simulator`) is pure: no SQL, no network, no wall clock. Everything it needs is passed in.

## Consequences

- One deploy unit for the backend; a cross-domain change is one PR and one transaction.
- Package boundaries must be enforced by review (and an import-boundary lint in Part 02 CI), since the compiler will not stop `labs` from reaching into `gamification` internals.
- Scaling is vertical first, then horizontal replicas of the same binary. Lab writes serialize on a per-session row lock, so replicas are safe.

## Alternatives considered

- **Microservices** (separate simulator, grading, XP services): distributed transactions for every command; far more operational work than the team can carry.
- **Next.js only (TypeScript backend):** workable, but the deterministic simulator benefits from Go's typing, testing and performance, and keeping it out of the web runtime keeps the trust boundary clear.
