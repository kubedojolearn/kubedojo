#!/usr/bin/env bash
# One-time (and idempotent) developer setup. Run via `make setup`.
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "✗ $1 not found — $2"
    fail=1
  fi
}
need node "install Node.js 24 LTS (https://nodejs.org or mise/nvm; see .config/mise.toml)"
need pnpm "install pnpm 10 (corepack enable, or https://pnpm.io/installation)"
need go "install Go 1.26+ (https://go.dev/dl)"
need docker "install Docker Desktop or Docker Engine with the compose plugin"
[ "$fail" = 0 ] || exit 1

node_major="$(node -p 'process.versions.node.split(".")[0]')"
if [ "$node_major" != "24" ]; then
  echo "! Node $(node -v) detected; Node 24 LTS is the supported version (CI uses it). Continuing."
fi
if ! docker info >/dev/null 2>&1; then
  echo "✗ Docker is installed but the daemon is not running. Start Docker and re-run make setup."
  exit 1
fi

echo "→ Installing JavaScript dependencies"
pnpm install --frozen-lockfile

echo "→ Downloading Go modules"
(cd apps/api && go mod download)

echo "→ Creating .env"
node scripts/gen-env.mjs

echo "✓ Setup complete. Next: make dev (or make up for the full stack in containers)."
