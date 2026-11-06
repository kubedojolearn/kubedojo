# Deploy the web app to Vercel

Vercel hosts the Next.js app in `apps/web`. The Go API, worker, PostgreSQL and Redis still need a separately hosted environment. The web app calls the API from server-side routes and server components, so `API_INTERNAL_URL` must be a reachable HTTPS URL from Vercel Functions; a Docker Compose hostname such as `http://api:8080` will not work.

## Create the Vercel project

1. Import this repository in Vercel and select **Next.js** as the framework (normally detected automatically).
2. Set **Root Directory** to `apps/web`.
3. In the Root Directory settings, enable **Include source files outside of the Root Directory in the Build Step**. `apps/web` depends on the workspace package `packages/api-client`.
4. Leave the install and build commands at their detected defaults. The root `pnpm-lock.yaml`, `pnpm-workspace.yaml`, and `packageManager` field select pnpm; the app's build script is `next build`.
5. Set the Node.js version to **24.x**, matching the repository's supported runtime.

The Next config uses Vercel's native build output on Vercel and keeps standalone output for the Docker image.

## Configure environment variables

Add the following in **Project Settings → Environment Variables**. Use separate production and preview values, and keep secrets out of Git.

| Variable | Value |
| --- | --- |
| `APP_ENV` | `production` in Vercel deployments |
| `API_INTERNAL_URL` | HTTPS origin of the deployed Go API, for example `https://api.example.com` |
| `AUTH_SECRET` | Random 32-byte-or-longer secret; generate with `openssl rand -base64 32` |
| `AUTH_ASGARDEO_ID` | Client ID of the Asgardeo OIDC application for this environment |
| `AUTH_ASGARDEO_SECRET` | Client secret of that application |
| `AUTH_ASGARDEO_ISSUER` | Issuer URL ending in `/oauth2/token` |
| `BFF_SIGNING_PRIVATE_KEY` | Production Ed25519 private key in base64 PKCS#8 DER format |
| `BFF_SIGNING_KID` | Key ID paired with the private key; the API must trust its public key |
| `WEB_ORIGIN` | Production canonical origin, such as `https://learn.example.com`; leave unset for previews to use their `VERCEL_URL` |

Generate a separate production signing keypair using `node scripts/gen-env.mjs --print-keypair <kid>`. Put its private key in Vercel and add its public key to the API's `API_JWT_PUBLIC_KEYS`. Follow [keys.md](keys.md) for key format and rotation.

Do not set `AUTH_TEST_MODE=1` in Vercel. `AUTH_SECRET`, Asgardeo credentials and signing keys must be server-only variables; do not prefix them with `NEXT_PUBLIC_`.

## Configure the API and Asgardeo

- Run the Go API, worker, PostgreSQL and Redis on infrastructure reachable by the API. Set the API's `APP_ENV=production` and `API_JWT_PUBLIC_KEYS` to the matching public key. Keep database and Redis credentials in that platform's secret store.
- In Asgardeo, register `https://<production-domain>/api/auth/callback/asgardeo` as an authorized redirect URL and the production origin as an allowed origin. For previews, register the preview callback URL too; exact redirect allowlists do not accept every unique preview URL automatically.
- Configure Preview to use a staging API and staging Asgardeo application. Preview deployments derive their origin from `VERCEL_URL` when `WEB_ORIGIN` is unset. If you use a stable preview alias, set `WEB_ORIGIN` to that alias and register its callback in Asgardeo.
- The browser calls the web app's BFF, which calls the API server-side; the API needs a public HTTPS endpoint reachable by Vercel, but browser CORS is not needed for that BFF traffic.

## Verify a deployment

After the first deployment, check `/api/healthz`, `/robots.txt` and `/sitemap.xml`. Then sign in through Asgardeo and verify a page that reads the API. A successful Vercel build alone does not confirm that the external API, database, Redis, or identity provider is reachable and configured.
