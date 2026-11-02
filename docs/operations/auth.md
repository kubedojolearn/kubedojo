# Authentication

Design: [ADR-0003](../adr/0003-auth-bff.md) and [ADR-0008](../adr/0008-asgardeo-identity-broker.md). Implemented in Part 06; sign-in moved to Asgardeo afterwards.

## How a request is authenticated

1. **Sign-in.** Auth.js (`apps/web/lib/auth/config.ts`) signs in with Asgardeo over OpenID Connect, scope `openid profile` only; no email is requested. `/login` has a button each for Google, GitHub and email. Google and GitHub pass Asgardeo's `fidp` parameter to skip its login page; email opens it. In the sign-in callback the web app mints a *provisioning assertion* and calls `POST /api/v1/auth/provision`. If that fails, sign-in fails with a visible error, so no session exists without an account.
2. **Session.** Auth.js stores an encrypted JWT in an HttpOnly, `SameSite=Lax` cookie (`Secure` over HTTPS). It contains only `subject = asgardeo:<user id>`. Profile and role are always read from the API.
3. **Browser → BFF.** The browser calls `/api/bff/v1/...` with its cookie. The BFF (`apps/web/lib/api/bff.ts`):
   - requires a session, except for public catalog, leaderboard and profile reads;
   - requires the mutation's `Origin` (or `Referer`) to be `WEB_ORIGIN`, and JSON bodies (CSRF);
   - allows only allowlisted API paths (never provisioning or WebSockets);
   - mints a fresh 60-second assertion per call and forwards only `Content-Type`, `Idempotency-Key`, `Accept` and a request ID. Cookies never reach the API.
4. **Server components** use `getCurrentUser()` (`lib/auth/current-user.ts`), which calls `GET /users/me` with an assertion. Reads happen inside `<Suspense>` boundaries (Cache Components).
5. **Route protection.** `proxy.ts` redirects signed-out visitors away from `/dashboard`, `/settings`, `/lab` and `/admin`. This is a convenience; the Go API enforces access on every call.

The browser never receives an internal assertion. `pnpm --filter @kubedojo/web check:bundle` (run in CI) fails if a secret's name or value appears in client bundles.

## Setting up Asgardeo

Do this once per environment, with a **separate** Asgardeo organization (or at least a separate application) for development and production.

1. **Organization.** Sign up at <https://asgardeo.io> and create an organization, e.g. `kubedojo-dev`. Its issuer is `https://api.asgardeo.io/t/<organization>/oauth2/token`.
2. **Application.** Console → **Applications** → **New Application** → **Traditional Web Application**, protocol **OpenID Connect**.
   - Authorized redirect URL: `http://localhost:8000/api/auth/callback/asgardeo` (production: `https://<your domain>/api/auth/callback/asgardeo`).
   - Allowed origin: `http://localhost:8000` (production: your origin).
3. **Protocol tab.** Copy the client ID and secret into `.env` as `AUTH_ASGARDEO_ID` and `AUTH_ASGARDEO_SECRET`, and set `AUTH_ASGARDEO_ISSUER` to the issuer above. Keep **Authorization Code** as the grant type.
4. **User attributes tab.** Request the `profile` scope attributes only: name, given name, family name, username and picture. Do **not** request email. Set the **subject attribute** to the **User ID** (the default), so `sub` is a UUID; KubeDojo refuses any other subject.
5. **Connections.** Console → **Connections** → **New Connection**:
   - **Google**: create an OAuth client in Google Cloud with the redirect URL Asgardeo shows, then paste its client ID and secret. Name the connection `Google`.
   - **GitHub**: create a GitHub OAuth app with the redirect URL Asgardeo shows, then paste its client ID and secret. Name the connection `GitHub`.
   - Using other names? Set `AUTH_ASGARDEO_GOOGLE_CONNECTION` / `AUTH_ASGARDEO_GITHUB_CONNECTION` to them.
   - In each connection's **Advanced** settings, keep **Just-in-Time User Provisioning** on, so federated sign-ins get an Asgardeo user ID.
6. **Login flow.** Application → **Login Flow**: add *Username & Password*, *Google* and *GitHub* as options of the first step.
7. **Self-registration and linking** (threat T18). Console → **Login & Registration**:
   - Turn on **Self Registration** with **Account Verification** (email confirmation), so nobody can register someone else's address.
   - Don't enable automatic association of federated users with existing local accounts by email. One Asgardeo user is one KubeDojo account.
8. `make dev` (or `make up`) and sign in at <http://localhost:8000/login>.

If sign-in comes back with "Sign-in isn't configured correctly", check the web logs: the usual causes are a wrong issuer (it must end in `/oauth2/token`), a redirect URL mismatch, or a subject attribute other than the user ID.

**Sign-out.** Signing out ends the KubeDojo session only, not the Asgardeo session. Every sign-in sends `prompt=login`, so Asgardeo asks again instead of silently reusing that session.

### Without Asgardeo: test sign-in

Set `AUTH_TEST_MODE=1` in `.env` to add a **Test sign-in** form to `/login` that accepts any numeric test ID and username. The test ID is turned into a stable UUID, so the same ID always signs into the same account. It still goes through provisioning and the BFF exactly like a real sign-in; only the Asgardeo hop is skipped. The web app **refuses to start** with `AUTH_TEST_MODE=1` and `APP_ENV=production`. Automated stack tests (`make test-stack`) depend on it.

## Production

- Callback URL: `https://<your domain>/api/auth/callback/asgardeo`, registered in a **separate** Asgardeo application from development.
- Set `WEB_ORIGIN` to the public origin. Auth.js trusts the forwarded host from the edge proxy (`trustHost`), so the edge must be the only way in.
- Secrets (`AUTH_SECRET`, `AUTH_ASGARDEO_SECRET`, `BFF_SIGNING_PRIVATE_KEY`) come from the platform's secret store. Rotation: [keys.md](keys.md).

## Rotating `AUTH_SECRET`

Auth.js accepts a list of secrets, newest first. Set `AUTH_SECRET` to the new value and keep the old one available for decryption during the session lifetime (30 days), then remove it. Rotating without overlap signs everyone out, which is acceptable in an emergency.

## Error states

`/login?error=<code>` shows plain-language messages for `AccessDenied` (cancelled at the sign-in page), `OAuthCallbackError`, `OAuthSignin`, `Configuration` (including a subject that isn't an Asgardeo user ID), `ProvisioningFailed`, `SignInNotConfigured` and `SessionRequired`.

## Account export and deletion

The settings page shows these as coming soon. The data model, deletion steps and retention are documented in [database.md](database.md#account-deletion); the endpoints arrive with hardening (Part 19).
