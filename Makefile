# c-hsm-doc — Build- und Lieferkette per ADR 0002.
#
# Docker-only Workflow: Build/Lint/Test/Coverage/Image-Build laufen in
# Containern. Host braucht nur Docker und `make`.
#
# Quality Gates:
#   make gates       — inner-loop gates (lint + test + coverage-gate)
#   make ci          — gates plus govulncheck
#   make fullbuild   — ci plus build (runtime image)

IMAGE                   ?= c-hsm-doc-server
GO_VERSION              ?= 1.26.3
GOLANGCI_LINT_VERSION   ?= v2.12.1
THRESHOLD               ?= 0

# --no-cache-filter zwingt BuildKit, die Stage neu zu evaluieren, ohne
# den deps-Cache zu invalidieren. Verhindert, dass stale Layer Lint-/
# Test-/Coverage-Fehler maskieren.
NO_CACHE_FILTER_TEST     := --no-cache-filter test
NO_CACHE_FILTER_LINT     := --no-cache-filter lint
NO_CACHE_FILTER_COVERAGE := --no-cache-filter coverage

DOCKER_BUILD := docker build \
    --build-arg GO_VERSION=$(GO_VERSION) \
    --build-arg GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION)

COMPOSE_DEV := docker compose -f docker-compose.dev.yml

.DEFAULT_GOAL := help

.PHONY: help deps compile lint test coverage coverage-gate build run clean \
        gates ci fullbuild govulncheck docs-check dev-softhsm dev-down

help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ---- inner-loop ------------------------------------------------------------

deps: ## Resolve Go module dependencies (deps-cache layer).
	$(DOCKER_BUILD) --target deps -t $(IMAGE):deps .

compile: ## Fast compile feedback (no tests/lint).
	$(DOCKER_BUILD) --target compile -t $(IMAGE):compile .

lint: ## golangci-lint with the project profile.
	$(DOCKER_BUILD) $(NO_CACHE_FILTER_LINT) --target lint -t $(IMAGE):lint .

test: ## Run `go test ./...` inside Docker.
	$(DOCKER_BUILD) $(NO_CACHE_FILTER_TEST) --target test -t $(IMAGE):test .

coverage-gate: ## Coverage threshold gate (bootstrap-aware, ADR 0002 §2.5).
	$(DOCKER_BUILD) $(NO_CACHE_FILTER_COVERAGE) \
	    --target coverage \
	    --build-arg COVERAGE_THRESHOLD=$(THRESHOLD) \
	    -t $(IMAGE):coverage .

coverage: coverage-gate ## Alias for coverage-gate.

build: ## Build the runtime image (distroless static, nonroot).
	$(DOCKER_BUILD) --target runtime -t $(IMAGE):latest .

run: build ## Smoke test: run the built image with --version.
	docker run --rm $(IMAGE):latest --version

# ---- security gates --------------------------------------------------------

govulncheck: ## Run govulncheck against the project.
	docker run --rm \
	    -v "$(CURDIR)":/src -w /src \
	    -e GOFLAGS=-buildvcs=false \
	    golang:$(GO_VERSION) \
	    sh -c "go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./..."

# ---- docs gates ------------------------------------------------------------

docs-check: ## Markdown link validator (docker-gekapseltes Python).
	docker run --rm \
	    -v "$(CURDIR)":/src -w /src \
	    python:3.13-slim \
	    python tools/check_refs.py

# ---- aggregators -----------------------------------------------------------

gates: lint test coverage-gate docs-check ## Inner-loop mandatory gates.
	@echo "[gates] lint + test + coverage-gate + docs-check green"

ci: gates govulncheck ## Gates plus govulncheck.
	@echo "[ci] gates + govulncheck green"

fullbuild: ci build ## CI plus runtime image (full closure).
	@echo "[fullbuild] ci + runtime image green"

# ---- local dev environment -------------------------------------------------

dev-softhsm: ## Initialize SoftHSM token in the local compose volume (HSM-ENV-003).
	$(COMPOSE_DEV) up --build softhsm-init
	@echo "[dev-softhsm] SoftHSM token initialized in compose volume"

dev-down: ## Tear down the local compose environment (volume preserved).
	$(COMPOSE_DEV) down

# ---- maintenance -----------------------------------------------------------

clean: ## Remove local build artefacts and built images.
	@rm -rf out coverage *.out *.test
	@-docker image rm \
	    $(IMAGE):latest $(IMAGE):deps $(IMAGE):compile \
	    $(IMAGE):lint $(IMAGE):test $(IMAGE):coverage 2>/dev/null || true
	@echo "[clean] artefacts and images removed"
