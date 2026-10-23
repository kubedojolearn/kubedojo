# Authentication

Design: [ADR-0003](../adr/0003-auth-bff.md). Implemented in Part 06.

## How a request is authenticated

1. **Sign-in.** Auth.js (`apps/web/lib/auth/config.ts`) runs GitHub OAuth with the `read:user` scope only; no email is requested. In the sign-in callback the web app mints a *provisioning assertion* and calls `POST /api/v1/auth/provision`. If that fails, sign-in fails with a visible error, so no session exists without an account.
2. **Session.** Auth.js stores an encrypted JWT in an HttpOnly, `SameSite=Lax` cookie (`Secure` over HTTPS). It contains only `subject = github:<numeric id>`. Profile and role are always read from the API.
3. **Browser → BFF.** The browser calls `/api/bff/v1/...` with its cookie. The BFF (`apps/web/lib/api/bff.ts`):
   - requires a session, except for public catalog, leaderboard and profile reads;
   - requires the mutation's `Origin` (or `Referer`) to be `WEB_ORIGIN`, and JSON bodies (CSRF);
   - allows only allowlisted API paths (never provisioning or WebSockets);
   - mints a fresh 60-second assertion per call and forwards only `Content-Type`, `Idempotency-Key`, `Accept` and a request ID. Cookies never reach the API.
4. **Server components** use `getCurrentUser()` (`lib/auth/current-user.ts`), which calls `GET /users/me` with an assertion. Reads happen inside `<Suspense>` boundaries (Cache Components).
5. **Route protection.** `proxy.ts` redirects signed-out visitors away from `/dashboard`, `/settings`, `/lab` and `/admin`. This is a convenience; the Go API enforces access on every call.

The browser never receives an internal assertion. `pnpm --filter @kubedojo/web check:bundle` (run in CI) fails if a secret's name or value appears in client bundles.

## Local development with GitHub

1. Create an OAuth app at <https://github.com/settings/developers> → **New OAuth App**:
   - Homepage URL: `http://localhost:8000`
   - Authorization callback URL: `http://localhost:8000/api/auth/callback/github`
2. Put the client ID and a generated client secret in `.env` as `AUTH_GITHUB_ID` and `AUTH_GITHUB_SECRET`.
3. `make dev` (or `make up`) and sign in at <http://localhost:8000/login>.

### Without GitHub: test sign-in

Set `AUTH_TEST_MODE=1` in `.env` to add a **Test sign-in** form to `/login` that accepts any numeric GitHub ID and login. It still goes through provisioning and the BFF exactly like a real sign-in; only the OAuth hop is skipped. The web app **refuses to start** with `AUTH_TEST_MODE=1` and `APP_ENV=production`. Automated stack tests (`make test-stack`) depend on it.

## Production

- Callback URL: `https://<your domain>/api/auth/callback/github`, registered in a **separate** GitHub OAuth app from development.
- Set `WEB_ORIGIN` to the public origin. Auth.js trusts the forwarded host from the edge proxy (`trustHost`), so the edge must be the only way in.
- Secrets (`AUTH_SECRET`, `AUTH_GITHUB_SECRET`, `BFF_SIGNING_PRIVATE_KEY`) come from the platform's secret store. Rotation: [keys.md](keys.md).

## Rotating `AUTH_SECRET`

Auth.js accepts a list of secrets, newest first. Set `AUTH_SECRET` to the new value and keep the old one available for decryption during the session lifetime (30 days), then remove it. Rotating without overlap signs everyone out, which is acceptable in an emergency.

## Error states

`/login?error=<code>` shows plain-language messages for `AccessDenied` (cancelled on GitHub), `OAuthCallbackError`, `OAuthSignin`, `Configuration`, `ProvisioningFailed` and `SessionRequired`.

## Account export and deletion

The settings page shows these as coming soon. The data model, deletion steps and retention are documented in [database.md](database.md#account-deletion); the endpoints arrive with hardening (Part 19).
