#!/usr/bin/env bash
# Development mode: Postgres, Redis and the edge proxy in Docker; the API and web app on the host
# with their own reloaders. Open http://localhost:${EDGE_PORT:-8000}. Ctrl-C stops everything.
set -euo pipefail
cd "$(dirname "$0")/.."

[ -f .env ] || { echo "No .env — run make setup first."; exit 1; }
set -a
# shellcheck disable=SC1091
source .env
set +a

WEB_UPSTREAM=host.docker.internal:3000 API_UPSTREAM=host.docker.internal:8080 \
  docker compose up -d --wait postgres redis edge

pids=()
cleanup() {
  trap - INT TERM EXIT
  echo
  echo "→ stopping dev processes"
  for pid in ${pids[@]+"${pids[@]}"}; do kill "$pid" 2>/dev/null || true; done
  wait 2>/dev/null || true
}
trap cleanup INT TERM EXIT

prefix() { sed -u "s/^/[$1] /"; }

(cd apps/api && go run ./cmd/api serve 2>&1 | prefix api) &
pids+=($!)
(pnpm --filter @kubedojo/web dev 2>&1 | prefix web) &
pids+=($!)

echo "→ KubeDojo dev: http://localhost:${EDGE_PORT:-8000}  (API direct: http://localhost:8080/api/v1/healthz)"
# Portable replacement for `wait -n` (macOS ships bash 3.2): stop when either process exits.
while :; do
  for pid in "${pids[@]}"; do
    kill -0 "$pid" 2>/dev/null || exit 1
  done
  sleep 1
done
