# KubeDojo developer commands. `make help` lists them.
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help
MAKEFLAGS += --no-print-directory

COMPOSE := docker compose
API_DIR := apps/api
GOLANGCI_LINT_VERSION := v2.15.0
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GO_LDFLAGS := -s -w \
  -X github.com/kubedojolearn/kubedojo/apps/api/internal/platform/buildinfo.Version=$(VERSION) \
  -X github.com/kubedojolearn/kubedojo/apps/api/internal/platform/buildinfo.Commit=$(COMMIT)

# Load .env (if present) for targets that talk to local services.
ifneq (,$(wildcard .env))
include .env
export
endif

##@ Setup and running

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)} /^[a-zA-Z0-9_-]+:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: setup
setup: ## Check toolchain, install dependencies, create .env with dev secrets
	@./scripts/setup.sh

.PHONY: dev
dev: ## Postgres/Redis/edge in Docker, API + web on the host -> http://localhost:8000
	@./scripts/dev.sh

.PHONY: up
up: ## Build and run the full stack in Docker -> http://localhost:8000
	VERSION=$(VERSION) COMMIT=$(COMMIT) KUBEDOJO_VERSION=$(VERSION) KUBEDOJO_COMMIT=$(COMMIT) \
	  $(COMPOSE) --profile app up -d --build --wait
	@echo "KubeDojo is running at http://localhost:$${EDGE_PORT:-8000}"

.PHONY: down
down: ## Stop all containers (data volumes are kept)
	$(COMPOSE) --profile app down

.PHONY: deps-up
deps-up: ## Start only Postgres and Redis
	$(COMPOSE) up -d --wait postgres redis

.PHONY: logs
logs: ## Follow container logs
	$(COMPOSE) --profile app logs -f

##@ Quality gates

.PHONY: check
check: lint typecheck test test-integration licenses ## Everything CI runs except e2e and image builds

.PHONY: lint
lint: lint-web lint-api lint-contracts ## All linters

.PHONY: lint-web
lint-web:
	pnpm lint

.PHONY: lint-api
lint-api:
	@cd $(API_DIR) && unformatted="$$(gofmt -l .)" && if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd $(API_DIR) && go vet ./... && go vet -tags=integration ./...
	cd $(API_DIR) && $(GOLANGCI_LINT) run ./...

.PHONY: lint-contracts
lint-contracts:
	pnpm openapi:lint

.PHONY: typecheck
typecheck: ## TypeScript and Go type checks
	pnpm typecheck
	cd $(API_DIR) && go build ./...

.PHONY: test
test: test-web test-api test-contracts ## Unit tests

.PHONY: test-web
test-web:
	pnpm --filter @kubedojo/web test

.PHONY: test-api
test-api:
	cd $(API_DIR) && go test -race ./...

.PHONY: test-contracts
test-contracts:
	pnpm --filter @kubedojo/content-schema test
	pnpm content:validate

.PHONY: test-integration
test-integration: deps-up ## Go integration tests against Compose Postgres/Redis
	cd $(API_DIR) && go test -race -tags=integration -count=1 ./...

.PHONY: test-e2e
test-e2e: ## Playwright end-to-end tests (builds and starts the web app)
	pnpm --filter @kubedojo/web exec playwright install chromium
	pnpm --filter @kubedojo/web test:e2e

PLAYWRIGHT_IMAGE := mcr.microsoft.com/playwright:v1.64.1-noble
VISUAL_RUN := docker run --rm --ipc=host -v "$(CURDIR):/repo" -w /repo/apps/web -e CI=1 -e E2E_SKIP_BUILD=1 \
  $(PLAYWRIGHT_IMAGE) node node_modules/@playwright/test/cli.js test --project=visual

.PHONY: test-visual
test-visual: ## Visual regression screenshots (inside the pinned Playwright container)
	pnpm --filter @kubedojo/web build
	$(VISUAL_RUN)

.PHONY: test-visual-update
test-visual-update: ## Re-record visual baselines; review the image diffs before committing
	pnpm --filter @kubedojo/web build
	$(VISUAL_RUN) --update-snapshots

.PHONY: licenses
licenses: ## Check production dependency licenses
	pnpm licenses:check

##@ Build

.PHONY: build
build: ## Build the web app and Go binaries (apps/api/bin)
	pnpm build
	cd $(API_DIR) && CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/api ./cmd/api
	cd $(API_DIR) && CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/worker ./cmd/worker

.PHONY: images
images: ## Build container images
	VERSION=$(VERSION) COMMIT=$(COMMIT) KUBEDOJO_VERSION=$(VERSION) KUBEDOJO_COMMIT=$(COMMIT) $(COMPOSE) --profile app build
