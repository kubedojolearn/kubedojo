# API conventions (Go service)

Implemented in `apps/api/internal/platform/httpx` and `apps/api/internal/auth` (Part 04). The contract is [openapi.yaml](openapi.yaml); `internal/httpapi/contract_test.go` fails if an implemented route is missing from it or returns a response it doesn't describe.

## Request lifecycle

Middleware, outermost first:

1. **Request ID:** accepts a well-formed `X-Request-ID` (8–64 of `[A-Za-z0-9._-]`) from the BFF or generates a UUID. It is echoed in the response header and in `meta.requestId`, and attached to every log line.
2. **Panic recovery:** returns a 500 `INTERNAL` envelope; the stack is logged and never sent.
3. **Deadline:** request context timeout (`REQUEST_TIMEOUT`, default 5 s).
4. **Body cap:** `MAX_BODY_BYTES` (default 128 KiB) → 413.
5. **Access log:** method, route pattern, status, bytes, duration. Never headers, query strings or bodies.
6. **Security headers:** `nosniff`, `DENY`, `no-referrer`, `default-src 'none'`.
7. Per route group: **authentication** (request assertion) → **per-user rate limit** → **authorization** (role from DB) → handler.

## Authentication

- `Authorization: Bearer <assertion>` where the assertion is an EdDSA JWT from the BFF ([ADR-0003](../adr/0003-auth-bff.md)). Accepted only if: alg is `EdDSA`; `kid` is in `API_JWT_PUBLIC_KEYS`; `iss=kubedojo-web`; `aud` is exactly the endpoint's audience; `exp`, `iat`, `jti` present; lifetime ≤ 60 s; ±10 s clock skew; `sub` matches `github:<numeric id>`; request assertions carry no profile claims.
- Every failure is the same `401 UNAUTHENTICATED` with a generic message; the reason is logged at info level.
- Identity and role are never read from headers (`X-User-Id`, `X-Role`, …), query strings or bodies. Raw GitHub tokens are not accepted.
- A verified identity with no linked user (not provisioned, or deleted) is 401.

## Errors

Envelope: `{"error":{"code","message","details":[{field,message}]},"meta":{"requestId"}}`.

| Status | Code | When |
| --- | --- | --- |
| 400 | `VALIDATION_FAILED` | Malformed input, unknown body fields, bad cursor or limit |
| 401 | `UNAUTHENTICATED` | Missing/invalid assertion, unlinked identity |
| 403 | `FORBIDDEN` | Authenticated but lacking the role |
| 404 | `NOT_FOUND` | Unknown route, missing resource, **or a resource owned by someone else** |
| 405 | `VALIDATION_FAILED` | Method not allowed |
| 409 | `CONFLICT` / `STALE_VERSION` | State conflicts |
| 410 | `SESSION_EXPIRED` | Lab expired |
| 413 | `VALIDATION_FAILED` | Body too large |
| 415 | `VALIDATION_FAILED` | Body is not `application/json` |
| 429 | `RATE_LIMITED` | With `Retry-After` |
| 500 | `INTERNAL` | Anything unexpected; details only in logs |
| 503 | `RATE_LIMITED` | A fail-closed limiter is unavailable |

## Request bodies

`httpx.DecodeJSON` requires `Content-Type: application/json`, rejects unknown fields (so `role`, `xp` or `userId` can't be smuggled in), rejects trailing data, and reports field-level errors.

## Pagination

`?cursor=&limit=` with `limit` 1–100 (default 20). Cursors are opaque, HMAC-signed positions; a tampered or foreign cursor is a 400. The last page has `meta.nextCursor: null`.

## Rate limits

Fixed windows in Redis (`internal/platform/ratelimit`). All authenticated routes share a general limit of 300 requests/minute/user; route-specific limits (commands, lab creation, validation, answers) are added with those routes per [requirements S6](../architecture/requirements.md#security). General limits fail open if Redis is down; abuse-sensitive ones fail closed. Per-IP limits use `X-Forwarded-For` only from `TRUSTED_PROXY_CIDRS`.

## Network restriction

The Go REST API must be reachable only from the Next.js BFF:

- **Compose:** the API has no published port in the `app` profile; only the edge (`/api/v1/ws/*`) and the web container reach it. In `make dev` it listens on the host for convenience.
- **Production:** run the API on a private network or service mesh with ingress limited to the web tier, and route only `/api/v1/ws/*` from the public edge (see the Caddyfile). This is defence in depth: the assertion check is the control, and every route behaves as if it were public.
