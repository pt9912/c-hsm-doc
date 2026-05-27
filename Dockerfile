# syntax=docker/dockerfile:1.7

# ---------------------------------------------------------------------------
# c-hsm-doc-server — HSM-gestützte Dokumentverschlüsselung (Go-Server).
#
# Docker-only Build- und Lieferkette per ADR 0002. Host braucht nur
# Docker + make; keine Go-Toolchain.
#
# Slice 002a aktiviert die CGO-fähige Pipeline für die spätere PKCS#11-
# Anbindung (ADR 0004). Distroless-Static reicht nicht mehr — Vendor-`.so`-
# Module brauchen libc; Library-Closure wird deterministisch über
# `lddtree` ermittelt; ein Sentinel-COPY erzwingt die `closure-check`-
# Stage als Build-Voraussetzung.
#
# Stages (ADR 0002 §2.2 + ADR 0004 §2.3..§2.5):
#   deps                    — Go-Modul-Resolution (Cache-Layer)
#   compile                 — schnelles Compile-Feedback ohne Tests/Lint
#   lint                    — golangci-lint
#   test                    — go test ./... (CGO=0)
#   coverage                — go test -coverprofile + coverage-gate (CGO=0)
#   build                   — Server + pkcs11-dlopen-smoke (CGO=1)
#   deps-closure            — PKCS#11-Vendor-Modul + lddtree-Closure-Staging
#   runtime-without-check   — Distroless-Base + Server + Smoke + Closure
#   closure-check           — Library-Closure-Verifikation, touch /closure-check.ok
#   runtime                 — runtime-without-check + Sentinel-COPY aus closure-check
#
# Pin-Politik (ADR 0002 §2.4 + ADR 0004 §2.8): GO_VERSION,
# GOLANGCI_LINT_VERSION und (neu in 002a) LDDTREE_BASE_IMAGE,
# PKCS11_VENDOR_IMAGE sind routine pins; Updates dokumentieren sich
# im Commit-Body.
# ---------------------------------------------------------------------------

ARG GO_VERSION=1.26.3
ARG GOLANGCI_LINT_VERSION=v2.12.1
ARG DEBIAN_VERSION=bookworm

# Digest-Pinning (ADR 0002 §2.4 + ADR 0004 §2.8): Dev-Builds nutzen Tags;
# CI/Release-Builds setzen vollstaendige <tag>@sha256:...-Pins via --build-arg.
ARG GO_BASE_IMAGE=golang:${GO_VERSION}
ARG GOLANGCI_BASE_IMAGE=golangci/golangci-lint:${GOLANGCI_LINT_VERSION}-alpine
# ADR 0004 §2.1: Wechsel von distroless/static auf distroless/base für
# CGO/PKCS#11-Closure. ABI-kompatibel zur Debian-Builder-Stage
# (debian12/bookworm).
ARG RUNTIME_BASE_IMAGE=gcr.io/distroless/base-debian12:nonroot
# ADR 0004 §2.3: Closure-Ermittlung über pax-utils/lddtree gegen
# debian12-kompatiblen Rootfs. Auch fuer closure-check verwendet.
ARG LDDTREE_BASE_IMAGE=debian:${DEBIAN_VERSION}-slim
# ADR 0004 §2.6: Vendor-Quelle für das CI-PKCS#11-Modul. Default ist
# das Debian-Paket softhsm2; das in ADR 0004 gewählte zweite OSS-Modul
# (OpenCryptoki) wird in derselben Stage installiert.
ARG PKCS11_VENDOR_IMAGE=${LDDTREE_BASE_IMAGE}

# ---- deps ------------------------------------------------------------------
FROM ${GO_BASE_IMAGE} AS deps

WORKDIR /src
ENV GOFLAGS="-mod=readonly -buildvcs=false" \
    GOMODCACHE=/go/pkg/mod \
    GOCACHE=/root/.cache/go-build

COPY go.mod go.sum ./

# Strict-Mode (Open-Trigger 001): seit dem ersten produktiven Import
# (Slice 001) ist go.sum Pflicht und die Hash-Integrität wird beim
# Build geprüft.
RUN mkdir -p "$GOMODCACHE" && go mod download && go mod verify

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
# Slice 002a hält die Coverage-/Test-Mechanik unverändert gegenüber
# Slice 001 (ADR 0004 §Abgrenzung). Der CGO-pflichtige Adapter-Code
# und die Coverage-Aggregation kommen erst in 002b.
RUN CGO_ENABLED=0 go test ./...

# ---- coverage --------------------------------------------------------------
# Bootstrap-aware (ADR 0002 §2.5): solange ./internal/... leer ist,
# läuft der Stage mit COVERAGE_BOOTSTRAP=1 und akzeptiert leeren Input.
#
# `pipefail` ist explizit gesetzt, damit `go test … | tee …` den
# go-test-Exit-Code propagiert statt durch tee maskiert zu werden.
FROM deps AS coverage

SHELL ["/bin/bash", "-eo", "pipefail", "-c"]

# Slice 001 hat das Coverage-Gate aus dem Bootstrap-Modus gehoben
# (ADR 0002 §2.5). Default ist 80 %; höhere Werte per
# --build-arg COVERAGE_THRESHOLD=… überschreibbar.
ARG COVERAGE_THRESHOLD=80
ENV COVERAGE_THRESHOLD=${COVERAGE_THRESHOLD}

COPY . .
# Generated code aus dem coverpkg ausschließen (internal/gen/**). Die
# zugehörigen .pb.go-Dateien tragen "// Code generated ... DO NOT EDIT."
# am Anfang; Coverage über sie ist nicht aussagekräftig. Der Regex ist
# am vollen Modulpfad geankert, damit ein zukünftiges Paket wie
# `internal/dependency-generator/` nicht versehentlich mitgefiltert wird.
RUN mkdir -p /out && \
    COVERPKG=$(go list ./internal/... 2>/dev/null | grep -v '^github.com/pt9912/c-hsm-doc/internal/gen/' | tr '\n' ',' | sed 's/,$//') && \
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
# ADR 0004 §2.2: CGO_ENABLED=1 für den späteren PKCS#11-Adapter.
# Auch das pkcs11-dlopen-smoke-Helper-Binary (ADR 0004 §2.5) wird hier
# gebaut, damit Linker/libc-Variante mit der späteren Adapter-Linkage
# konsistent bleibt.
FROM deps AS build

COPY . .
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w" \
    -o /out/c-hsm-doc-server \
    ./cmd/hsmdoc && \
    CGO_ENABLED=1 go build \
    -ldflags="-s -w" \
    -o /out/pkcs11-dlopen-smoke \
    ./cmd/pkcs11-dlopen-smoke

# ---- deps-closure ----------------------------------------------------------
# ADR 0004 §2.3: PKCS#11-Vendor-Modul + transitive Library-Closure
# deterministisch via lddtree (pax-utils), nicht via ldd-Parsing.
# Stückliste landet in /build/pkcs11-libs.list, die Files unter
# /staging/pkcs11-rootfs/ mit Pfaderhalt.
FROM ${PKCS11_VENDOR_IMAGE} AS deps-closure

SHELL ["/bin/bash", "-eo", "pipefail", "-c"]

# Vendor-Module: SoftHSM v2 (Default) plus das in ADR 0004 §2.6
# gewählte zweite herstellerfremde OSS-Modul (OpenCryptoki ICA).
# Beide sind im Closure-Staging, weil der HSM-FA-HSM-001-Vendor-Smoke
# beide gegen das Runtime-Image fahren muss.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        pax-utils \
        softhsm2 \
        opencryptoki \
        && rm -rf /var/lib/apt/lists/*

# SoftHSM v2 in Debian 12 wohnt unter /usr/lib/softhsm/libsofthsm2.so.
# OpenCryptoki bringt /usr/lib/pkcs11/PKCS11_API.so. lddtree --list
# expandiert NEEDED rekursiv und respektiert RPATH/RUNPATH.
RUN mkdir -p /build /staging/pkcs11-rootfs && \
    # softhsm2-Paket bringt nur CLI-Tools; das .so-Modul lebt in
    # libsofthsm2. Statt Paket-Lookup direkt nach File suchen — robust
    # gegen Paket-Aufteilung.
    # Module sind oft Symlinks (z. B. /usr/lib/.../pkcs11/PKCS11_API.so
    # → libopencryptoki.so.0). -L lässt find Symlinks folgen.
    SOFTHSM_MODULE=$(find -L /usr/lib -name 'libsofthsm2.so' -type f 2>/dev/null | head -n1) && \
    OPENCRYPTOKI_MODULE=$(find -L /usr/lib -path '*/pkcs11/PKCS11_API.so' 2>/dev/null | head -n1) && \
    if [ -z "$SOFTHSM_MODULE" ] || [ -z "$OPENCRYPTOKI_MODULE" ]; then \
        echo "deps-closure: konnte Vendor-Modul-Pfade nicht aufloesen" >&2; \
        echo "  SOFTHSM_MODULE=$SOFTHSM_MODULE" >&2; \
        echo "  OPENCRYPTOKI_MODULE=$OPENCRYPTOKI_MODULE" >&2; \
        exit 1; \
    fi && \
    echo "deps-closure: softhsm=$SOFTHSM_MODULE opencryptoki=$OPENCRYPTOKI_MODULE" && \
    echo "$SOFTHSM_MODULE" > /build/pkcs11-modules.list && \
    echo "$OPENCRYPTOKI_MODULE" >> /build/pkcs11-modules.list && \
    lddtree --list --skip-non-elfs "$SOFTHSM_MODULE" "$OPENCRYPTOKI_MODULE" | \
        sort -u > /build/pkcs11-libs.list && \
    cat /build/pkcs11-modules.list >> /build/pkcs11-libs.list && \
    sort -u /build/pkcs11-libs.list -o /build/pkcs11-libs.list && \
    while IFS= read -r path; do \
        if [ -z "$path" ]; then continue; fi; \
        # lddtree liefert manchmal symlink-Quellen; cp -L derefenziert.
        target="/staging/pkcs11-rootfs${path}"; \
        mkdir -p "$(dirname "$target")"; \
        cp -L "$path" "$target"; \
    done < /build/pkcs11-libs.list && \
    echo "deps-closure: $(wc -l < /build/pkcs11-libs.list) Pfade in /staging/pkcs11-rootfs/"

# ---- runtime-without-check -------------------------------------------------
# ADR 0004 §2.1: Distroless-base statt distroless/static. Enthält Server,
# Smoke-Binary, Vendor-Module + Closure. Sentinel-COPY aus closure-check
# kommt in der finalen `runtime`-Stage, damit Stage-Graph zyklenfrei
# bleibt.
FROM ${RUNTIME_BASE_IMAGE} AS runtime-without-check

LABEL org.opencontainers.image.source="https://github.com/pt9912/c-hsm-doc" \
      org.opencontainers.image.description="c-hsm-doc — HSM-gestützte Dokumentverschlüsselung (Go-Server)." \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.title="c-hsm-doc-server" \
      org.opencontainers.image.vendor="pt9912"

# PKCS#11-Vendor-Module + transitive Closure (ADR 0004 §2.3).
COPY --from=deps-closure /staging/pkcs11-rootfs/ /
COPY --from=deps-closure /build/pkcs11-libs.list /etc/hsmdoc/pkcs11-libs.txt

# Server- und Smoke-Binary aus der gemeinsamen CGO-Build-Stage.
COPY --from=build /out/c-hsm-doc-server /usr/local/bin/c-hsm-doc-server
COPY --from=build /out/pkcs11-dlopen-smoke /usr/local/bin/pkcs11-dlopen-smoke

USER 65532:65532

# HEALTHCHECK in exec-Form (kein Shell noetig, kompatibel mit distroless).
# Solange cmd/hsmdoc nur Bootstrap ist, ueberprueft --version, dass das
# Binary lauffaehig ist. M1 ersetzt das durch einen echten Health-Check
# gegen den gRPC-Endpoint.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/c-hsm-doc-server", "--version"]

ENTRYPOINT ["/usr/local/bin/c-hsm-doc-server"]

# ---- closure-check ---------------------------------------------------------
# ADR 0004 §2.4: Library-Closure-Verifikation gegen das fertige
# Runtime-Rootfs. touch /closure-check.ok ist Erfolgs-Marker, den die
# finale runtime-Stage via Sentinel-COPY zieht. closure-check nimmt
# runtime-without-check als Input — damit invalidiert jeder
# Runtime-Wechsel den BuildKit-Cache dieser Stage.
FROM ${LDDTREE_BASE_IMAGE} AS closure-check

SHELL ["/bin/bash", "-eo", "pipefail", "-c"]

RUN apt-get update && \
    apt-get install -y --no-install-recommends pax-utils && \
    rm -rf /var/lib/apt/lists/*

COPY --from=runtime-without-check / /rootfs

# Verifiziert pro Vendor-Modul, dass lddtree gegen das Runtime-Rootfs
# keine "not found"-Einträge meldet. Stückliste aus /etc/hsmdoc/
# (vom Runtime-Image mitkopiert) ist die Pflicht-Quelle.
RUN if [ ! -s /rootfs/etc/hsmdoc/pkcs11-libs.txt ]; then \
        echo "closure-check: /etc/hsmdoc/pkcs11-libs.txt fehlt oder leer" >&2; \
        exit 1; \
    fi && \
    fail=0; \
    # lddtree --root /rootfs interpretiert Argument-Pfade relativ zur
    # Rootfs-Wurzel; ein absolutes /rootfs-Prefix wuerde sich
    # verdoppeln. File-Existenz-Check braucht dagegen den absoluten
    # Host-Pfad.
    while IFS= read -r module; do \
        if [ -z "$module" ]; then continue; fi; \
        case "$module" in *.so|*.so.*) ;; *) continue ;; esac; \
        if [ ! -f "/rootfs${module}" ]; then continue; fi; \
        output=$(lddtree --root /rootfs --list --skip-non-elfs "${module}" 2>&1) || { \
            echo "closure-check: lddtree failed for $module:" >&2; \
            echo "$output" >&2; \
            fail=1; \
            continue; \
        }; \
        if echo "$output" | grep -E 'not found|=> not found' >/dev/null; then \
            echo "closure-check: missing libs for $module:" >&2; \
            echo "$output" | grep -E 'not found' >&2; \
            fail=1; \
        fi; \
    done < /rootfs/etc/hsmdoc/pkcs11-libs.txt && \
    if [ "$fail" -ne 0 ]; then exit 1; fi && \
    touch /closure-check.ok && \
    echo "closure-check: alle Vendor-Module aufloesbar gegen das Runtime-Rootfs"

# ---- runtime ---------------------------------------------------------------
# Finale Stage: runtime-without-check + Sentinel-COPY aus closure-check.
# Der Sentinel macht die closure-check-Stage zur harten Build-Voraussetzung
# für jeden runtime-Build (ADR 0004 §2.4).
FROM runtime-without-check AS runtime

COPY --from=closure-check /closure-check.ok /etc/hsmdoc/closure-check.ok
