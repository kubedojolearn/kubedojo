#!/usr/bin/env bash
# Installs the Helm chart on a throwaway kind cluster with images built from this checkout and
# checks that KubeDojo comes up: lessons imported, API ready, web app serving, a second API pod
# bootstrapping safely. Needs docker, kind, helm, kubectl and node.
#
#   scripts/test-chart-kind.sh                   test, then delete the cluster
#   KEEP_CLUSTER=1 scripts/test-chart-kind.sh    leave the cluster running to inspect it
set -euo pipefail
cd "$(dirname "$0")/.."
root=$PWD

cluster=${KIND_CLUSTER:-kubedojo-chart-test}
ns=kubedojo
tag=chart-test
chart=deploy/helm/kubedojo
work=$(mktemp -d)
pids=()

cleanup() {
  for pid in ${pids[@]+"${pids[@]}"}; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$work"
  if [ "${KEEP_CLUSTER:-0}" != 1 ]; then kind delete cluster --name "$cluster" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT

# port_forward SERVICE LOCAL:REMOTE waits until the local port answers.
port_forward() {
  kubectl -n "$ns" port-forward "svc/$1" "$2" >/dev/null 2>&1 &
  pids+=($!)
  local port=${2%%:*}
  for _ in $(seq 1 30); do
    if curl -s -o /dev/null "http://localhost:$port"; then return 0; fi
    sleep 1
  done
  echo "port-forward to $1 never answered" >&2
  return 1
}

echo "→ Building images"
docker build -q --build-context content=content -t "ghcr.io/kubedojolearn/kubedojo-api:$tag" apps/api >/dev/null
docker build -q -f apps/web/Dockerfile -t "ghcr.io/kubedojolearn/kubedojo-web:$tag" . >/dev/null

echo "→ Creating kind cluster $cluster"
kind create cluster --name "$cluster" --wait 120s
kind load docker-image --name "$cluster" "ghcr.io/kubedojolearn/kubedojo-api:$tag" "ghcr.io/kubedojolearn/kubedojo-web:$tag"

echo "→ Creating the Secret from a generated .env, as the chart README describes"
cp deploy/selfhost/.env.example "$work/.env.example"
(cd "$work" && node "$root/scripts/gen-env.mjs" >/dev/null)
kubectl create namespace "$ns"
kubectl -n "$ns" create secret generic kubedojo --from-env-file="$work/.env"

echo "→ Installing the chart"
helm install kubedojo "$chart" -n "$ns" -f "$chart/ci/default-values.yaml" \
  --set image.tag="$tag" --set image.pullPolicy=Never --wait --timeout 6m

echo "→ Checking the bootstrap imported the lessons"
kubectl -n "$ns" logs deploy/kubedojo-api -c bootstrap | grep -E '"content imported".*"courses":[1-9]'

echo "→ Checking the API is ready and the web app serves pages"
port_forward kubedojo-api 18080:8080
curl -fsS http://localhost:18080/api/v1/readyz | grep -q '"status":"ready"'
port_forward kubedojo-web 18000:3000
curl -fsS http://localhost:18000/api/healthz | grep -q '"status":"ok"'
code=$(curl -s -o /dev/null -w '%{http_code}' http://localhost:18000/learn)
[ "$code" = 200 ] || { echo "/learn returned $code" >&2; exit 1; }

echo "→ Scaling the API to 2: the new pod bootstraps under the database lock"
helm upgrade kubedojo "$chart" -n "$ns" --reuse-values --set api.replicas=2 --wait --timeout 4m
kubectl -n "$ns" get pods -l app.kubernetes.io/instance=kubedojo

echo "✓ Helm chart installs and serves KubeDojo on kind"
