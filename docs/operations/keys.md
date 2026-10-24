# Secrets and key management

| Secret | Used by | Format | Generate |
| --- | --- | --- | --- |
| `AUTH_SECRET` | Next.js (Auth.js session encryption) | 32+ random bytes, base64 | `openssl rand -base64 32` |
| `BFF_SIGNING_PRIVATE_KEY` + `BFF_SIGNING_KID` | Next.js BFF (signs internal assertions) | Ed25519, base64 of PKCS#8 DER | `node scripts/gen-env.mjs --print-keypair <kid>` |
| `API_JWT_PUBLIC_KEYS` | Go API (verifies assertions) | comma-separated `kid:base64(SPKI DER)` | printed with the keypair above |
| `AUTH_GITHUB_ID` / `AUTH_GITHUB_SECRET` | Next.js (GitHub OAuth) | from the GitHub OAuth app | GitHub → Settings → Developer settings |
| `POSTGRES_PASSWORD` / `DATABASE_URL` | API, worker | — | platform-managed in production |

`make setup` generates development values automatically. **Development and production keys must differ.** Production secrets live only in the deployment platform's secret store, never in Git, images or logs.

## Rotating the BFF signing key

The API accepts several public keys at once (by `kid`), so rotation has no downtime:

1. Generate a new pair: `node scripts/gen-env.mjs --print-keypair prod-2026-11`.
2. **Add** the new public key to `API_JWT_PUBLIC_KEYS` (keep the old one) and deploy the API.
3. Switch the web app to the new `BFF_SIGNING_PRIVATE_KEY` and `BFF_SIGNING_KID` and deploy it.
4. Wait at least 60 seconds (the maximum assertion lifetime), then **remove** the old public key from `API_JWT_PUBLIC_KEYS` and deploy the API.

If a private key leaks, do the same steps immediately and treat step 4 as urgent.

## Rotating `AUTH_SECRET`

Changing it signs everyone out. Auth.js supports a list of secrets for gradual rotation; the procedure is documented with the authentication work (Part 06).
