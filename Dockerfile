# syntax=docker/dockerfile:1.7

# ---------------------------------------------------------------------------
# c-hsm-doc-server — HSM-gestützte Dokumentverschlüsselung (Go-Server).
#
# Docker-only Build- und Lieferkette per ADR 0002. Host braucht nur
# Docker + make; keine Go-Toolchain.
#
# Stages (ADR 0002 §2.2):
#   deps      — Go-Modul-Resolution (Cache-Layer)
#   compile   — schnelles Compile-Feedback ohne Tests/Lint
#   lint      — golangci-lint
#   test      — go test ./...
#   coverage  — go test -coverprofile + coverage-gate.sh (bootstrap-aware)
#   build     — statisches Binary (CGO=0, -ldflags="-s -w")
#   runtime   — distroless/static:nonroot (HSM-NFA-SEC-007, -008)
#
# Pin-Politik (ADR 0002 §2.4): GO_VERSION und GOLANGCI_LINT_VERSION
# sind routine pins; Updates dokumentieren sich im Commit-Body.
# ---------------------------------------------------------------------------

ARG GO_VERSION=1.26.3
ARG GOLANGCI_LINT_VERSION=v2.12.1

# Digest-Pinning (ADR 0002 §2.4): Dev-Builds nutzen den Tag; CI/Release-
# Builds setzen vollstaendige <tag>@sha256:...-Pins via --build-arg.
ARG GO_BASE_IMAGE=golang:${GO_VERSION}
ARG GOLANGCI_BASE_IMAGE=golangci/golangci-lint:${GOLANGCI_LINT_VERSION}-alpine
ARG RUNTIME_BASE_IMAGE=gcr.io/distroless/static-debian12:nonroot

# ---- deps ------------------------------------------------------------------
FROM ${GO_BASE_IMAGE} AS deps

WORKDIR /src
ENV GOFLAGS="-mod=readonly -buildvcs=false" \
    GOMODCACHE=/go/pkg/mod \
    GOCACHE=/root/.cache/go-build

COPY go.mod ./
# go.sum kann fehlen (pre-`go mod tidy`-Bootstrap); [m] matched nichts,
# wenn die Datei nicht existiert.
COPY go.su[m] ./

RUN mkdir -p "$GOMODCACHE" && go mod download

# ---- compile ---------------------------------------------------------------
FROM deps AS compile

COPY . .
RUN CGO_ENABLED=0 go build -o /tmp/c-hsm-doc-server ./cmd/hsmdoc

# ---- lint ------------------------------------------------------------------
FROM ${GOLANGCI_BASE_IMAGE} AS lint

WORKDIR /src
COPY --from=deps /go/pkg/mod /go/pkg/mod
COPY . .
RUN golangci-lint run ./...

# ---- test ------------------------------------------------------------------
FROM deps AS test

COPY . .
RUN CGO_ENABLED=0 go test ./...

# ---- coverage --------------------------------------------------------------
# Bootstrap-aware (ADR 0002 §2.5): solange ./internal/... leer ist,
# läuft der Stage mit COVERAGE_BOOTSTRAP=1 und akzeptiert leeren Input.
#
# `pipefail` ist explizit gesetzt, damit `go test … | tee …` den
# go-test-Exit-Code propagiert statt durch tee maskiert zu werden.
FROM deps AS coverage

SHELL ["/bin/bash", "-eo", "pipefail", "-c"]

ARG COVERAGE_THRESHOLD=0
ENV COVERAGE_THRESHOLD=${COVERAGE_THRESHOLD}

COPY . .
RUN mkdir -p /out && \
    COVERPKG=$(go list ./internal/... 2>/dev/null | tr '\n' ',' | sed 's/,$//') && \
    if [ -z "$COVERPKG" ]; then \
        echo "coverage: keine Produktiv-Pakete in ./internal/... — bootstrap mode"; \
        : > /out/coverage.out; \
        : > /out/coverage-func.txt; \
        export COVERAGE_BOOTSTRAP=1; \
    else \
        CGO_ENABLED=0 go test \
            -coverpkg="$COVERPKG" \
            -coverprofile=/out/coverage.out \
            -covermode=atomic \
            ./... && \
        go tool cover -func=/out/coverage.out | tee /out/coverage-func.txt; \
        export COVERAGE_BOOTSTRAP=0; \
    fi && \
    bash scripts/coverage-gate.sh /out/coverage-func.txt "$COVERAGE_THRESHOLD"

# ---- build -----------------------------------------------------------------
FROM deps AS build

COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w" \
    -o /out/c-hsm-doc-server \
    ./cmd/hsmdoc

# ---- runtime ---------------------------------------------------------------
FROM ${RUNTIME_BASE_IMAGE} AS runtime

LABEL org.opencontainers.image.source="https://github.com/pt9912/c-hsm-doc" \
      org.opencontainers.image.description="c-hsm-doc — HSM-gestützte Dokumentverschlüsselung (Go-Server)." \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.title="c-hsm-doc-server" \
      org.opencontainers.image.vendor="pt9912"

COPY --from=build /out/c-hsm-doc-server /usr/local/bin/c-hsm-doc-server

USER 65532:65532

# HEALTHCHECK in exec-Form (kein Shell noetig, kompatibel mit distroless).
# Solange cmd/hsmdoc nur Bootstrap ist, ueberprueft --version, dass das
# Binary lauffaehig ist. M1 ersetzt das durch einen echten Health-Check
# gegen den gRPC-Endpoint.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/c-hsm-doc-server", "--version"]

ENTRYPOINT ["/usr/local/bin/c-hsm-doc-server"]
