# KubeDojo Helm chart

Runs [KubeDojo](https://github.com/kubedojolearn/kubedojo) on Kubernetes: the web app, the API (which applies migrations and imports the lessons on start) and the background worker, plus an optional bundled PostgreSQL and Redis.

The chart is published with every weekly release at `oci://ghcr.io/kubedojolearn/charts/kubedojo`. Each chart version deploys the images of the same release.

Requirements: Kubernetes 1.29+, Helm 3.8+ (OCI support), and for public access an Ingress controller. Lab terminals use a WebSocket at `/api/v1/ws`, which the Ingress routes to the API.

## Install

**1. Create the Secret.** Generate fresh secrets with the same files the Docker Compose setup uses, then load them into a Secret:

```sh
base=https://github.com/kubedojolearn/kubedojo/releases/latest/download
curl -fsSL -o .env.example "$base/env.example"
curl -fsSL -o gen-env.mjs "$base/gen-env.mjs"
docker run --rm -v "$PWD:/w" -w /w -u "$(id -u):$(id -g)" node:24-alpine node gen-env.mjs   # or: node gen-env.mjs

kubectl create namespace kubedojo
kubectl -n kubedojo create secret generic kubedojo --from-env-file=.env
```

Keep `.env` somewhere safe (or delete it once the Secret exists): it holds the database password and the keys that sign sessions. Settings in it that only Compose uses are ignored.

**2. Install the chart:**

```sh
helm install kubedojo oci://ghcr.io/kubedojolearn/charts/kubedojo -n kubedojo \
  --set webOrigin=https://kubedojo.example.com \
  --set ingress.enabled=true \
  --set ingress.host=kubedojo.example.com \
  --set ingress.className=nginx
```

For HTTPS, add `--set ingress.tls.enabled=true --set ingress.tls.secretName=kubedojo-tls` with a certificate in that Secret (cert-manager can issue it: put its annotation in `ingress.annotations`).

The API's first start waits for PostgreSQL, applies the migrations and imports the lessons, so allow a minute or two before every pod is ready.

**Just trying it?** Skip the Ingress and use a port-forward:

```sh
helm install kubedojo oci://ghcr.io/kubedojolearn/charts/kubedojo -n kubedojo --set webOrigin=http://localhost:8000
kubectl -n kubedojo port-forward svc/kubedojo-web 8000:3000   # then open http://localhost:8000
```

## Upgrade

```sh
helm upgrade kubedojo oci://ghcr.io/kubedojolearn/charts/kubedojo -n kubedojo --reuse-values
```

That installs the newest release. Pin one with `--version <chart version>` (listed in each [GitHub release](https://github.com/kubedojolearn/kubedojo/releases)). New API pods apply migrations and import the new lessons before they start serving. Migrations only move forward, so back up the database before upgrading if you might want to go back.

## The Secret

`existingSecret` (default `kubedojo`) names a Secret with these keys. `.env` generated as above has all the required ones.

| Key | Required | Used for |
| --- | --- | --- |
| `AUTH_SECRET` | yes | Session encryption |
| `BFF_SIGNING_PRIVATE_KEY`, `BFF_SIGNING_KID` | yes | The web app's signed requests to the API |
| `API_JWT_PUBLIC_KEYS` | yes | The API's check of those requests (must match the private key) |
| `POSTGRES_PASSWORD` | with the bundled PostgreSQL | Database password |
| `DATABASE_URL` | with `postgresql.enabled=false` | Your PostgreSQL, e.g. `postgres://user:pass@host:5432/kubedojo?sslmode=require` |
| `REDIS_URL` | with `redis.enabled=false` | Your Redis, `rediss://` (TLS) |
| `AUTH_ASGARDEO_ID`, `AUTH_ASGARDEO_SECRET`, `AUTH_ASGARDEO_ISSUER` | for sign-in | Asgardeo application ([auth.md](../../../docs/operations/auth.md)); the redirect URL is `<webOrigin>/api/auth/callback/asgardeo` |

After changing the Secret, restart the pods: `kubectl -n kubedojo rollout restart deploy`.

## Your own PostgreSQL and Redis

The bundled PostgreSQL is a single pod with a PersistentVolumeClaim: fine to start with, but it has no backups or failover. For anything you can't afford to lose, use a managed database:

```sh
kubectl -n kubedojo create secret generic kubedojo --from-env-file=.env \
  --from-literal=DATABASE_URL='postgres://…' --from-literal=REDIS_URL='rediss://…' \
  --dry-run=client -o yaml | kubectl apply -f -
helm upgrade kubedojo oci://ghcr.io/kubedojolearn/charts/kubedojo -n kubedojo --reuse-values \
  --set postgresql.enabled=false --set redis.enabled=false
```

Moving existing data from the bundled database is a `pg_dump` from its pod and a restore into the new one.

## Values

| Value | Default | Description |
| --- | --- | --- |
| `webOrigin` | (required) | The URL people open, without a trailing slash |
| `existingSecret` | `kubedojo` | Secret with the keys above |
| `image.registry` / `image.tag` | `ghcr.io/kubedojolearn` / the chart's appVersion | Images `kubedojo-api` and `kubedojo-web` |
| `api.replicas`, `web.replicas` | `1` | The worker always runs one replica |
| `api.resources`, `web.resources`, `worker.resources` | small requests, memory limits | |
| `ingress.enabled`, `.className`, `.host`, `.annotations`, `.tls` | off | Routes `/api/v1/ws` to the API, everything else to the web app |
| `postgresql.enabled`, `.persistence.size`, `.persistence.storageClass` | on, `8Gi`, default class | Bundled PostgreSQL 18 |
| `redis.enabled` | on | Bundled Redis 8, no persistence (it holds only disposable data) |
| `networkPolicy.enabled` | on | Only the API and worker may reach the bundled PostgreSQL and Redis |
| `auth.asgardeoGoogleConnection`, `.asgardeoGithubConnection` | `Google`, `GitHub` | Connection names in the Asgardeo console |
| `logLevel` | `info` | `debug`, `info`, `warn` or `error` |
| `nodeSelector`, `tolerations`, `affinity`, `imagePullSecrets` | empty | Applied to every pod |

See [values.yaml](values.yaml) for every value with comments.

## Security

- Every KubeDojo container runs as a non-root user with a read-only root filesystem, no Linux capabilities and the runtime's default seccomp profile. Pods get no Kubernetes API token.
- Only `/api/v1/ws` of the API is routed by the Ingress (ADR-0003); its REST endpoints are reached only by the web app inside the cluster.
- The bundled Redis has no password, and the API talks to it without TLS. The NetworkPolicies keep everything except the API and worker away from it, but only if your cluster's network plugin enforces NetworkPolicy. Otherwise, use your own Redis with TLS.

## Uninstall

```sh
helm uninstall kubedojo -n kubedojo
```

The bundled database's PersistentVolumeClaim (`data-kubedojo-postgresql-0`) and the Secret are kept. Delete them to remove all data.

## Developing the chart

`make chart-check` lints and renders the chart with each `ci/*-values.yaml` file and validates the manifests against the Kubernetes schemas. `make test-chart` installs it on a throwaway [kind](https://kind.sigs.k8s.io) cluster with images built from your checkout and checks that KubeDojo comes up. CI runs both.
