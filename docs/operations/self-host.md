# Self-host KubeDojo

Run your own KubeDojo from prebuilt images. You need a machine with Docker and the Compose plugin (v2.23 or newer); amd64 and arm64 are both supported. Nothing else: no Node, Go or source checkout.

The images are published weekly from `main` to GitHub Container Registry (`ghcr.io/kubedojolearn/kubedojo-api` and `-web`). Each release carries its lessons, so updating the images also updates the course content.

## Install

```sh
mkdir kubedojo && cd kubedojo
base=https://github.com/kubedojolearn/kubedojo/releases/latest/download
curl -fsSL -o compose.yaml "$base/compose.yaml"
curl -fsSL -o .env.example "$base/env.example"
curl -fsSL -o gen-env.mjs "$base/gen-env.mjs"

# Create .env with fresh secrets (runs the generator in a throwaway Node container).
docker run --rm -v "$PWD:/w" -w /w -u "$(id -u):$(id -g)" node:24-alpine node gen-env.mjs

docker compose up -d --wait
```

Open http://localhost:8000. On every start a one-shot `migrate` container applies database migrations and imports and publishes the lessons in the image, then the API, worker, web app and edge proxy start.

Keep `.env` private and backed up: it holds the database password and the keys that sign sessions.

## Make it reachable from other machines

The stack serves plain HTTP on `127.0.0.1:8000`. For anything beyond your own computer, put HTTPS in front of it, for example Caddy or nginx on the same host, or a Cloudflare Tunnel, forwarding to `127.0.0.1:8000`. Then set in `.env`:

```sh
WEB_ORIGIN=https://kubedojo.example.com   # exactly what users type, no trailing slash
```

and run `docker compose up -d` again. `EDGE_BIND=0.0.0.0` exposes the plain-HTTP port to your network; only do that on a trusted network.

## Sign-in

Without Asgardeo credentials KubeDojo runs, but nobody can sign in. To enable sign-in, create an Asgardeo application as described in [auth.md](auth.md), register `${WEB_ORIGIN}/api/auth/callback/asgardeo` as its redirect URL, fill in the `AUTH_ASGARDEO_*` values in `.env`, and run `docker compose up -d`.

## Update

Releases come out on Mondays. To update:

```sh
docker compose pull && docker compose up -d --wait
```

To update automatically, add a cron job on the host (here Tuesdays at 04:00, after Monday's release):

```cron
0 4 * * 2  cd /path/to/kubedojo && docker compose pull -q && docker compose up -d --wait
```

To stay on one release, set `KUBEDOJO_TAG` in `.env` to its date (for example `2026-11-09`; see the [releases page](https://github.com/kubedojolearn/kubedojo/releases)) instead of `latest`.

Database migrations only move forward. Going back to an older image after a newer one has migrated the database isn't supported, so take a backup before updating if you might want to undo it.

## Back up and restore

```sh
docker compose exec -T postgres pg_dump -U kubedojo -d kubedojo --clean --if-exists > kubedojo-$(date +%F).sql
docker compose exec -T postgres psql -U kubedojo -d kubedojo < kubedojo-2026-11-09.sql   # restore
```

Redis holds nothing worth backing up (ADR-0004). `docker compose down` keeps the database; `docker compose down -v` deletes it.

## For maintainers: how releases are made

[`.github/workflows/release.yml`](../../.github/workflows/release.yml) runs every Monday at 03:00 UTC and can be started by hand (`gh workflow run release.yml`, with `-f force=true` to release an unchanged commit). It:

1. Picks the newest commit on `main` whose CI run succeeded, and skips the week if the last `release-*` tag already points at it.
2. Builds the API image (with `content/` built in) and the web image natively on amd64 and arm64 runners.
3. Tags both multi-platform images `latest`, the release date and `sha-<commit>`.
4. Creates a GitHub release `release-<date>` with generated notes and the self-host files (`compose.yaml`, `env.example`, `gen-env.mjs`) attached.

Because self-hosters may update unattended, every change merged to `main` must be safe to apply on top of the previous release:

- Migrations must work on a database with real data from the previous release, without manual steps. The [database rules](database.md) (forward-only, never edit a merged migration) are what make this hold.
- Lessons are published on import, so anything merged under `content/` reaches every self-hosted copy in the next release.
- A new required environment variable breaks existing installs. Give it a default, or add it to `deploy/selfhost/` and say so in the release notes.

After the very first release, open each package on GitHub (organization → Packages → `kubedojo-api` and `kubedojo-web` → Package settings) and change its visibility to **Public**. New packages start private, and private images can't be pulled without logging in.
