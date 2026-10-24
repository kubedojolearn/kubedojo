# ADR-0003: Auth.js BFF, signed internal assertions, edge proxy and explicit provisioning

- **Status:** Accepted · **Date:** 2026-10-22
- Resolves plan-review items 4 and 5.

## Context

Users sign in with GitHub. The Go API must know who is calling without ever trusting the browser, and the browser must never hold a credential that works directly against Go. The lab terminal needs WebSockets, which Next.js route handlers cannot proxy. Go needs profile data (GitHub login, name, avatar) to create an account, but the request assertion should carry identity only.

## Decision

**Sessions.** Auth.js in Next.js runs the GitHub OAuth flow and stores the immutable GitHub account ID in its encrypted, HttpOnly, `Secure`, `SameSite=Lax` session cookie. The subject is always `github:<numeric account id>`, never the editable username or email.

**Request assertions.** For each BFF call to Go, Next.js mints a JWT signed with Ed25519 (`alg: EdDSA`, `kid` header):

| Claim | Value |
| --- | --- |
| `iss` | `kubedojo-web` |
| `aud` | `kubedojo-api` |
| `sub` | `github:<id>` |
| `iat`, `exp` | `exp - iat` ≤ 60 s |
| `jti` | random UUID, logged for correlation |

Go accepts only `EdDSA`, only `kid`s in `API_JWT_PUBLIC_KEYS`, requires every claim above, allows ±10 s clock skew, rejects lifetimes over 60 s, and ignores any identity or role sent any other way. Roles come from PostgreSQL. Go does **not** keep a `jti` replay store: assertions travel only on the private network between Next.js and Go over TLS (in production), live ≤ 60 s, and every state-changing endpoint is idempotent or uses idempotency keys, so a replayed assertion cannot do anything its holder could not already do. If the network assumption changes, add a Redis `SET NX` on `jti`.

**Provisioning.** Accounts are created only by an explicit call at sign-in:

1. In the Auth.js `signIn` callback, Next.js mints a *provisioning assertion*: same issuer and key, `aud: kubedojo-api-provision`, ≤ 60 s, plus `login`, `name`, `avatar_url` from the GitHub profile returned by the OAuth exchange.
2. It calls `POST /api/v1/auth/provision`. Only that endpoint accepts the provisioning audience; no other endpoint accepts it.
3. Go upserts `oauth_identities` + `users` in one transaction (`ON CONFLICT (provider, provider_subject)`), derives a handle from `login` on first creation (case-insensitive unique, numeric suffix on collision), and stores length-limited, sanitized `display_name`/`avatar_url`. Later sign-ins refresh only fields the user has not customized.
4. If provisioning fails, sign-in fails with a visible error.
5. A request assertion whose `sub` has no linked identity gets `401 UNAUTHENTICATED`; the BFF ends the session. Go never creates users on ordinary requests.

**Deleted accounts.** Deletion tombstones `users` (`deleted_at`), deletes the `oauth_identities` link and erases personal data per the retention policy. Signing in again provisions a new, empty account. The old handle stays reserved for 30 days.

**Network shape.** One public origin, fronted by a reverse proxy (Caddy in Docker Compose, an ingress in production):

- `/api/v1/ws/*` → Go realtime endpoint (the only Go route reachable from the internet)
- everything else → Next.js

Go's REST port listens on the private network only. This is defence in depth; the controls are the assertion check on every REST call and the Origin + one-use ticket check on every WebSocket.

**WebSocket tickets.** `POST /api/v1/labs/{id}/ws-ticket` (via the BFF) returns a random 256-bit ticket stored in Redis as `wsticket:<sha256(ticket)>` → `{userId, labId}` with a 30 s TTL. The browser opens `wss://<origin>/api/v1/ws/labs/{id}` and must send `{"type":"auth","ticket":…}` within 3 s. Go checks `Origin` against `WS_ALLOWED_ORIGINS`, redeems the ticket with `GETDEL`, and requires the ticket's lab to match the URL.

## Consequences

- The browser never sees an internal credential; a stolen session cookie still works only through the BFF, which applies CSRF and Origin checks.
- Key rotation: Go accepts a set of public keys by `kid`; Next.js signs with one. Rotation = add new public key → deploy Go → switch the signing key → deploy web → remove the old public key.
- Every BFF request costs one Ed25519 signature (~tens of µs); negligible.
- Adding Google later requires explicit account linking, never email matching.

## Alternatives considered

- **Forward the Auth.js session cookie to Go:** Go would need Auth.js's encryption secret and format; couples the services and widens the secret's blast radius.
- **Profile claims in every request assertion, upsert on any request:** simpler, but every request becomes a potential write and profile sync rules get murky.
- **Proxy WebSockets through a custom Next.js server:** loses the standard Next.js deployment model and puts long-lived connections on the web tier.
