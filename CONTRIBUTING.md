# Contributing to KubeDojo

Thanks for helping people learn Kubernetes. This guide covers how work is organized, how to set up, and what a mergeable pull request looks like.

## How work is organized

V1 is built in 20 parts described in [docs/master-plan.md](docs/master-plan.md). Each part is one branch and one pull request, reviewed before the next part starts. Design decisions live in [docs/adr](docs/adr/README.md); read the relevant ADRs before changing architecture.

## Setup

Follow the [README quick start](README.md#quick-start). `make setup` is idempotent; run it again after pulling dependency changes.

`.env` is created with random development secrets and is git-ignored. Never commit it, and never reuse development keys in production ([key management](docs/operations/keys.md)).

## Branches and commits

- Branch names: `feat/part-05-database`, `fix/terminal-reconnect`, `docs/authoring-guide`.
- Commit messages: [Conventional Commits](https://www.conventionalcommits.org/) are encouraged (`feat(api): …`, `fix(web): …`, `docs: …`) but not enforced.
- Keep pull requests focused. Unrelated refactors go in their own PR.

## Before you open a pull request

```sh
make check      # must pass
make test-e2e   # if you changed the web app
```

Git hooks (installed by `pnpm install` via [lefthook](lefthook.yml)) format staged files and run quick checks. CI runs everything regardless.

The pull request template links the [review checklist](docs/architecture/review-checklist.md). Security-sensitive paths (auth, the BFF, the command parser, the simulator, migrations, CI) need a maintainer review; see [.github/CODEOWNERS](.github/CODEOWNERS).

## Code guidelines

- **Server is authoritative.** Never trust identity, role, XP, scores or completion from the client.
- **No process execution.** Nothing in `apps/api` may import `os/exec`; the linter enforces this.
- **Contracts first.** API changes start in [docs/api/openapi.yaml](docs/api/openapi.yaml); content format changes start in [packages/content-schema](packages/content-schema/README.md).
- **No fake success.** Unsupported behavior returns a clear error, never a silent no-op.
- **Tests for every behavior**, including failure paths. Integration tests use the real Postgres/Redis from Compose and must never be skipped silently.
- Match the surrounding code's style; formatters (Prettier, gofmt/goimports) are the source of truth.

## Lesson content

Lessons live under `content/` and are validated against [packages/content-schema](packages/content-schema/README.md). The authoring guide arrives with Part 07.

## Code of conduct

Participation is governed by our [Code of Conduct](CODE_OF_CONDUCT.md).
