# KubeDojo

**Master Kubernetes, one command at a time.**

KubeDojo is an open-source, browser-based platform for learning Kubernetes by doing: interactive lessons, a safe simulated cluster you drive with real `kubectl` commands, a visual map of what your commands changed, and automatically graded challenges.

> **Status: early development.** The repository currently contains the specification (Part 01), development infrastructure (Part 02), and the foundations that follow. Learners can't use it yet. See the [plan](docs/master-plan.md) and the [documentation index](docs/README.md).

The cluster is **simulated**: no containers run and nothing touches a real cluster. The simulator implements a documented subset of Kubernetes v1.37 and says so whenever you step outside it ([compatibility matrix](docs/architecture/simulator-compatibility.md)).

KubeDojo is an independent project and is not affiliated with the Cloud Native Computing Foundation or The Linux Foundation. Kubernetes is a registered trademark of The Linux Foundation.

## Quick start

Requirements: **Node.js 24 LTS**, **pnpm 10**, **Go 1.26+**, **Docker** (Desktop or Engine with the Compose plugin), `make`. Versions are pinned in [.config/mise.toml](.config/mise.toml) if you use [mise](https://mise.jdx.dev).

```sh
git clone https://github.com/kubedojolearn/kubedojo.git
cd kubedojo
make setup   # checks tools, installs dependencies, creates .env with fresh dev secrets
make dev     # Postgres + Redis + edge proxy in Docker; API and web app on your machine
```

Open http://localhost:8000. Other ways to run it:

| Command | What runs | URL |
| --- | --- | --- |
| `make dev` | Postgres, Redis and the edge proxy in Docker; Go API (`go run`) and Next.js (`next dev`) on the host | http://localhost:8000 |
| `make up` / `make down` | Everything in containers, as in production | http://localhost:8000 |
| `make deps-up` | Only Postgres and Redis | — |

Health checks: `curl localhost:8000/api/healthz` (web) and, in `make dev`, `curl localhost:8080/api/v1/readyz` (API). In the container stack the Go REST API is deliberately not reachable from the edge; see [ADR-0003](docs/adr/0003-auth-bff.md).

### Quality gates

```sh
make check      # lint + typecheck + unit + integration tests + license check (what CI runs)
make test-e2e   # Playwright end-to-end tests (accessibility, responsive, keyboard, navigation)
make test-visual  # screenshot comparisons inside the pinned Playwright container
make help       # every target
```

### Windows

Use WSL 2 with Ubuntu 24.04, Docker Desktop's WSL integration, and clone the repository inside the WSL filesystem (not `/mnt/c`). All commands above then work unchanged.

## Repository layout

```text
apps/web                Next.js app (pages, BFF under app/api)
apps/api                Go API and worker (Chi, pgx)
packages/content-schema JSON Schemas + validator for lesson content
docs                    Plan, product spec, ADRs, API contracts, threat model
infra/compose           Edge proxy and Postgres init for Docker Compose
scripts                 Setup, dev runner, env generation, license check
```

## Contributing

See [CONTRIBUTING.md](.github/CONTRIBUTING.md). Report security issues privately as described in [SECURITY.md](.github/SECURITY.md).

## License

[Apache License 2.0](LICENSE).
