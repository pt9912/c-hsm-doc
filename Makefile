# c-hsm-doc — Build- und Lieferkette per ADR 0002.
#
# Docker-only Workflow: Build/Lint/Test/Coverage/Image-Build laufen in
# Containern. Host braucht nur Docker und `make`.
#
# Quality Gates:
#   make gates       — inner-loop gates (lint + test + coverage-gate + docs-check)
#   make ci          — gates plus govulncheck
#   make fullbuild   — ci plus build (runtime image)

IMAGE                   ?= c-hsm-doc-server
GO_VERSION              ?= 1.26.3
GOLANGCI_LINT_VERSION   ?= v2.12.1
GOVULNCHECK_VERSION     ?= v1.1.4
BUF_VERSION             ?= 1.47.2
PYTHON_VERSION          ?= 3.13-slim
DEBIAN_VERSION          ?= bookworm
TRIVY_VERSION           ?= 0.55.2
# Slice 001 hat das Coverage-Gate aus dem Bootstrap-Modus gehoben
# (ADR 0002 §2.5). Default ist 80 %; höhere Werte per Override:
#   make coverage-gate THRESHOLD=85
THRESHOLD               ?= 80

# Digest-Pinning fuer Supply-Chain (ADR 0002 §2.4 + ADR 0004 §2.8):
# Default-Pfad nutzt nur den Tag; Releases setzen vollstaendige
# <tag>@sha256:...-Strings, z. B.:
#   make ci GO_BASE_IMAGE=golang:1.26.3@sha256:abc... \
#           GOLANGCI_BASE_IMAGE=golangci/golangci-lint:v2.12.1-alpine@sha256:def... \
#           RUNTIME_BASE_IMAGE=gcr.io/distroless/base-debian12:nonroot@sha256:ghi...
GO_BASE_IMAGE           ?= golang:$(GO_VERSION)
GOLANGCI_BASE_IMAGE     ?= golangci/golangci-lint:$(GOLANGCI_LINT_VERSION)-alpine
# ADR 0004 §2.1: Distroless-base statt distroless/static fuer CGO/PKCS#11.
RUNTIME_BASE_IMAGE      ?= gcr.io/distroless/base-debian12:nonroot
# ADR 0004 §2.3/§2.6: pax-utils-Closure-Stage und Vendor-Quelle (debian12-Slim).
LDDTREE_BASE_IMAGE      ?= debian:$(DEBIAN_VERSION)-slim
PKCS11_VENDOR_IMAGE     ?= $(LDDTREE_BASE_IMAGE)
BUF_BASE_IMAGE          ?= bufbuild/buf:$(BUF_VERSION)
PYTHON_BASE_IMAGE       ?= python:$(PYTHON_VERSION)
TRIVY_BASE_IMAGE        ?= aquasec/trivy:$(TRIVY_VERSION)

# --no-cache-filter zwingt BuildKit, die Stage neu zu evaluieren, ohne
# den deps-Cache zu invalidieren. Verhindert, dass stale Layer Lint-/
# Test-/Coverage-/Closure-Fehler maskieren.
NO_CACHE_FILTER_TEST          := --no-cache-filter test
NO_CACHE_FILTER_LINT          := --no-cache-filter lint
NO_CACHE_FILTER_COVERAGE      := --no-cache-filter coverage
NO_CACHE_FILTER_CLOSURE_CHECK := --no-cache-filter closure-check

# In CI gibt --progress=plain vollstaendige Logs; lokal bleibt der
# default-progress (auto) fuer kompaktere Ausgabe.
PROGRESS_FLAG :=
ifeq ($(CI),1)
PROGRESS_FLAG := --progress=plain
endif

DOCKER_BUILD := docker build $(PROGRESS_FLAG) \
    --build-arg GO_VERSION=$(GO_VERSION) \
    --build-arg GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION) \
    --build-arg DEBIAN_VERSION=$(DEBIAN_VERSION) \
    --build-arg GO_BASE_IMAGE=$(GO_BASE_IMAGE) \
    --build-arg GOLANGCI_BASE_IMAGE=$(GOLANGCI_BASE_IMAGE) \
    --build-arg RUNTIME_BASE_IMAGE=$(RUNTIME_BASE_IMAGE) \
    --build-arg LDDTREE_BASE_IMAGE=$(LDDTREE_BASE_IMAGE) \
    --build-arg PKCS11_VENDOR_IMAGE=$(PKCS11_VENDOR_IMAGE)

COMPOSE_DEV := docker compose -f docker-compose.dev.yml

# Pfad zum PKCS#11-Modul im Runtime-Image fuer smoke-dlopen. Default
# zeigt auf SoftHSM v2 (multiarch-Pfad aus Debian 12); OpenCryptoki
# ueber Override (SMOKE_PKCS11_MODULE=/usr/lib/x86_64-linux-gnu/pkcs11/PKCS11_API.so).
SMOKE_PKCS11_MODULE     ?= /usr/lib/x86_64-linux-gnu/softhsm/libsofthsm2.so

.DEFAULT_GOAL := help

.PHONY: help deps compile lint test coverage coverage-gate build run clean \
        gates ci fullbuild govulncheck docs-check proto-gen proto-check \
        closure-check smoke-dlopen image-scan image-size \
        dev-softhsm dev-down dev-purge spike-hkdf-test spike-hkdf-bouncyhsm

# Spike-Verzeichnis aus Slice 002b §Vorbedingung 3 (ADR 0005 §2.2).
SPIKE_HKDF_PKG := ./docs/plan/planning/next/002b-spike-hkdf/spike/...

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

build: ## Build the runtime image (distroless base, nonroot, CGO, ADR 0004).
	$(DOCKER_BUILD) --target runtime -t $(IMAGE):latest .

run: build ## Smoke test: run the built image with --version.
	docker run --rm $(IMAGE):latest --version

# ---- 002a image-pipeline gates (ADR 0004) ---------------------------------

closure-check: ## Force-rebuild the closure-check stage (lddtree against runtime rootfs, ADR 0004 §2.4).
	$(DOCKER_BUILD) $(NO_CACHE_FILTER_CLOSURE_CHECK) \
	    --target closure-check -t $(IMAGE):closure-check .

smoke-dlopen: build ## Run pkcs11-dlopen-smoke inside the runtime image against $(SMOKE_PKCS11_MODULE).
	docker run --rm \
	    --entrypoint /usr/local/bin/pkcs11-dlopen-smoke \
	    $(IMAGE):latest \
	    $(SMOKE_PKCS11_MODULE)

image-scan: build ## Trivy HIGH/CRITICAL scan against the runtime image (ADR 0004 §2.7).
	@mkdir -p out/security
	docker run --rm \
	    -v /var/run/docker.sock:/var/run/docker.sock \
	    -v "$(CURDIR)/out/security":/out \
	    $(TRIVY_BASE_IMAGE) image \
	        --format json --output /out/trivy-runtime.json \
	        --severity HIGH,CRITICAL \
	        --exit-code 1 \
	        $(IMAGE):latest
	@echo "[image-scan] trivy report at out/security/trivy-runtime.json"

image-size: build ## Record runtime image size (ADR 0004 §2.7).
	@mkdir -p out/security
	@docker image inspect $(IMAGE):latest --format '{{.Size}}' > out/security/image-size.txt
	@echo "[image-size] $(IMAGE):latest = $$(cat out/security/image-size.txt) bytes"

# ---- security gates --------------------------------------------------------

govulncheck: ## Run govulncheck against the project (pinned, ADR 0002 §2.4).
	docker run --rm \
	    -v "$(CURDIR)":/src -w /src \
	    -e GOFLAGS=-buildvcs=false \
	    $(GO_BASE_IMAGE) \
	    sh -c "go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) && govulncheck ./..."

# ---- docs gates ------------------------------------------------------------

docs-check: ## Markdown link validator (docker-gekapseltes Python).
	docker run --rm \
	    -v "$(CURDIR)":/src -w /src \
	    $(PYTHON_BASE_IMAGE) \
	    python tools/check_refs.py

# ---- code generation -------------------------------------------------------
#
# Proto-Generierung läuft als one-off docker run gegen ein gepinntes
# buf-Image. Generierte Go-Dateien (internal/gen/) sind im Repo
# eingecheckt (Slice 001, Vorbedingung 1: "Generated Protobuf-Code
# eingecheckt vs. via Dockerfile-Stage generiert. Empfehlung:
# einchecken, kein neuer Build-Stage; spart Toolchain-Dep.").

proto-gen: ## (Re)generate Go code from spec/proto/**/*.proto into internal/gen/.
	docker run --rm \
	    -v "$(CURDIR)":/workspace -w /workspace \
	    -e HOME=/workspace/.buf-cache \
	    --user "$$(id -u):$$(id -g)" \
	    $(BUF_BASE_IMAGE) generate
	@rm -rf "$(CURDIR)/.buf-cache"
	@echo "[proto-gen] regenerated internal/gen/ from spec/proto/"

proto-check: ## Fail if checked-in generated code drifts from spec/proto/.
	@snapshot="$$(mktemp -d -t c-hsm-doc-proto-snapshot-XXXXXX)" ; \
	cp -a internal/gen "$$snapshot/gen" ; \
	$(MAKE) --no-print-directory proto-gen >/dev/null ; \
	if ! diff -r internal/gen "$$snapshot/gen" >/dev/null 2>&1 ; then \
	    echo "[proto-check] internal/gen/ is out of sync with spec/proto/ — run 'make proto-gen' and commit." ; \
	    rm -rf internal/gen ; \
	    cp -a "$$snapshot/gen" internal/gen ; \
	    rm -rf "$$snapshot" ; \
	    exit 1 ; \
	fi ; \
	rm -rf "$$snapshot" ; \
	echo "[proto-check] internal/gen/ matches spec/proto/"

# ---- aggregators -----------------------------------------------------------

gates: lint test coverage-gate docs-check ## Inner-loop mandatory gates.
	@echo "[gates] lint + test + coverage-gate + docs-check green"

# Slice 002a (ADR 0004) zieht die Image-Pipeline-Gates in `make ci`,
# damit keine 002a-Akzeptanz hinter manuellem Aufruf versteckt bleibt:
# closure-check forciert die lddtree-Verifikation; smoke-dlopen und
# image-scan haengen als Make-Dependencies an `build` und ziehen das
# Runtime-Image automatisch nach.
ci: gates govulncheck closure-check smoke-dlopen image-scan image-size ## Gates + govulncheck + 002a image-pipeline gates (ADR 0004).
	@echo "[ci] gates + govulncheck + closure-check + smoke-dlopen + image-scan + image-size green"

fullbuild: ci build ## CI plus runtime image (full closure).
	@echo "[fullbuild] ci + runtime image green"

# ---- 002b spike (HKDF Profil A) -------------------------------------------

spike-hkdf-test: ## Pure-Go-Unit-Tests des CK_HKDF_PARAMS-Serialisierers (Spike 002b, Pfad a; ADR 0005 §2.2).
	docker run --rm \
	    -v "$(CURDIR)":/src -w /src \
	    -e GOFLAGS="-mod=readonly -buildvcs=false" \
	    $(GO_BASE_IMAGE) \
	    go test -tags=spike $(SPIKE_HKDF_PKG)

spike-hkdf-bouncyhsm: ## Reproducible E2E run: Bouncy HSM image + setup + Go HKDF test (Spike 002b §6.2).
	bash scripts/spike-hkdf-bouncyhsm.sh

# ---- local dev environment -------------------------------------------------

dev-softhsm: ## Initialize SoftHSM token in the local compose volume (HSM-ENV-003).
	$(COMPOSE_DEV) up --build softhsm-init
	@echo "[dev-softhsm] SoftHSM token initialized in compose volume"

dev-down: ## Tear down the local compose environment (volume preserved).
	$(COMPOSE_DEV) down

dev-purge: ## Tear down compose AND remove volumes (destructive!).
	$(COMPOSE_DEV) down --volumes
	@echo "[dev-purge] compose environment and SoftHSM tokens removed"

# ---- maintenance -----------------------------------------------------------

clean: ## Remove local build artefacts and built images.
	@rm -rf out coverage *.out *.test
	@-docker image rm \
	    $(IMAGE):latest $(IMAGE):deps $(IMAGE):compile \
	    $(IMAGE):lint $(IMAGE):test $(IMAGE):coverage 2>/dev/null || true
	@echo "[clean] artefacts and images removed"
