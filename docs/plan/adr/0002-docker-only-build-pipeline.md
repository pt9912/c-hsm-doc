# ADR 0002 — Docker-only Build- und Lieferkette für den Go-Server

**Status:** Accepted
**Datum:** 2026-05-26
**Bezug:** [Lastenheft](../../../spec/lastenheft.md) (`HSM-ENV-001..003`,
`HSM-TECH-001`, `HSM-NFA-PORT-001..003`, `HSM-NFA-SEC-005..008`,
`HSM-NFA-OPS-001..003`),
[Spezifikation](../../../spec/spezifikation.md),
[Architektur](../../../spec/architecture.md),
[ADR 0001](0001-documentation-and-planning-structure.md)

---

## 1. Kontext

Das Lastenheft verlangt für den Go-Server:

- Containerauslieferung (`HSM-ENV-001`), Kubernetes-Kompatibilität
  (`HSM-ENV-002`) und lokale Dev-Umgebung mit SoftHSM v2 plus
  `docker-compose.dev.yml` (`HSM-ENV-003`),
- distroless oder vergleichbares Base-Image (`HSM-NFA-SEC-007`),
- Pod-Härtung mit `runAsNonRoot`, `readOnlyRootFilesystem`,
  `allowPrivilegeEscalation=false` (`HSM-NFA-SEC-008`),
- SBOM und CVE-Scan je Release (`HSM-NFA-SEC-005`),
- signierte Images (`HSM-NFA-SEC-006`),
- Go ≥ 1.22 (`HSM-TECH-001`),
- OCI-konforme Images (`HSM-NFA-PORT-003`).

Das Repository hat zwei Sprachen:

- **Go-Server**: Image, distroless-Runtime in Kubernetes.
- **Java-Client**: JAR-Bibliothek, klassisch via Maven/Gradle; kein
  Container.

Offene Frage: Welche Build-Toolchain für den Go-Server?

---

## 2. Entscheidung

### 2.1 Docker-only Build für den Go-Server

Alle Build-, Lint-, Test-, Coverage- und Image-Build-Schritte des
Go-Servers laufen in Containern. Das Repository setzt **keine
Go-Toolchain am Host** voraus.

Host-Voraussetzungen für Mitwirkende:

- Docker Engine,
- GNU `make`.

GNU `make` ist ein bewusster Carveout: es ist Build-Werkzeug am
Entwickler-Host und nicht Bestandteil des Runtime-Pfads, also kein
Verstoß gegen `HSM-NFA-PORT-001..002`.

### 2.2 Multi-Stage Dockerfile

Das `Dockerfile` definiert folgende Stages:

| Stage      | Zweck                                                                 |
| ---------- | --------------------------------------------------------------------- |
| `deps`     | Go-Modul-Resolution (Cache-Layer)                                     |
| `compile`  | schnelles Compile-Feedback ohne Tests/Lint                            |
| `lint`     | golangci-lint mit Projekt-Profil                                      |
| `test`     | `go test ./...` (Unit-Tests)                                          |
| `coverage` | `go test -coverprofile` + `coverage-gate.sh` (bootstrap-aware)        |
| `build`    | statisch gelinktes Binary (`CGO_ENABLED=0`, `-ldflags="-s -w"`)       |
| `runtime`  | `gcr.io/distroless/static-debian12:nonroot`, Binary unter `/usr/local/bin/c-hsm-doc-server` |

### 2.3 Makefile als dünner Wrapper

Das `Makefile` bietet ergonomische Targets, die jeweils
`docker build --target <stage>` aufrufen. Inner Loop: `make deps`,
`make compile`, `make lint`, `make test`, `make coverage[-gate]`,
`make build`, `make run`. Aggregatoren: `make gates` (lint + test +
coverage-gate), `make ci` (gates + govulncheck), `make fullbuild`
(ci + build). Dev-Helfer: `make dev-softhsm`, `make dev-down`.

CI ruft dieselben Targets wie der Entwickler-Host. Es gibt **keinen
CI-only-Pfad**.

### 2.4 Pin-Politik

Toolchain-Versionen werden als `ARG` im Dockerfile gepinnt:

- `GO_VERSION`
- `GOLANGCI_LINT_VERSION`

Routine-Updates dieser Pins benötigen keine eigene ADR; die Hebung
wird im Commit-Body begründet. Major-Wechsel (z. B. Go 2, Wechsel des
Lint-Tools) lösen eine Folge-ADR aus.

### 2.5 Bootstrap-fähiges Coverage-Gate

`scripts/coverage-gate.sh` ist bootstrap-fähig: solange `./internal/...`
keine produktiven Pakete enthält, akzeptiert es leeren Coverage-Input
mit Schwellwert 0 (`COVERAGE_BOOTSTRAP=1`). Sobald produktiver Code in
`internal/` landet, wird die Bootstrap-Schaltung deaktiviert und ein
echter Schwellwert (per `make coverage-gate THRESHOLD=…`) erzwungen.

### 2.6 Lokale Dev-Umgebung mit SoftHSM v2

`docker-compose.dev.yml` orchestriert:

- einen `softhsm-init`-Service, der einen SoftHSM-Token in einem
  benannten Volume initialisiert (Token-Label, USER-PIN, SO-PIN über
  Env-Variablen),
- (vorbereitet, auskommentiert) einen `server-dev`-Service, der das
  Volume samt PKCS#11-Modul attached, sobald `cmd/hsmdoc` einen
  echten gRPC-Server bereitstellt.

Damit erfüllt das Repository `HSM-ENV-003` bereits im Bootstrap-Zustand.

### 2.7 Distroless-nonroot als Runtime-Base

Die `runtime`-Stage basiert auf
`gcr.io/distroless/static-debian12:nonroot`. Sie enthält keine Shell,
kein `apt`, keinen Paketmanager. Sie erfüllt `HSM-NFA-SEC-007` und
macht die Pod-Härtung aus `HSM-NFA-SEC-008` (`runAsNonRoot`,
`readOnlyRootFilesystem`) ohne Zusatzkonfiguration möglich.

### 2.8 Java-Client-Build ist nicht Gegenstand dieser ADR

Der Java-Client wird in einer eigenen Scaffold-Runde mit eigener ADR
aufgesetzt, sobald `client/` entsteht. Wahl zwischen Maven und Gradle,
JDK-Pinning, Publishing-Pfad (Maven Central oder interne Registry)
sind dort zu klären.

---

## 3. Konsequenzen

- Mitwirkende brauchen nur Docker + `make` am Host; kein Go SDK, kein
  golangci-lint lokal, kein softhsm2 am Host.
- CI ruft dieselben `make`-Targets wie der Entwickler-Host.
- Build-Cache lebt in BuildKit; Re-Builds sind schnell, solange
  `go.mod` stabil bleibt.
- Toolchain-Updates sind isoliert in ARG-Pins, kein Eingriff in
  Quellcode.
- Lokale Dev-Umgebung gegen SoftHSM ist mit
  `docker compose -f docker-compose.dev.yml up softhsm-init`
  einsatzbereit.

---

## 4. Pflege-Regeln

- Versions-Hebungen (`GO_VERSION`, `GOLANGCI_LINT_VERSION`) sind
  routine. Begründung in den Commit-Body, kein ADR-Pflicht.
- Neue Stages werden im Dockerfile ergänzt und im Makefile
  gleichwertig exponiert.
- Neue Quality-Gates (z. B. `govulncheck`, `gosec`, `semgrep`) folgen
  demselben Pattern: eigener Stage oder eigener `docker run`-Aufruf,
  Makefile-Target, in `make ci` aggregiert.
- Coverage-Gate-Schwelle wird mit dem Einzug echter Logik in
  `internal/` aktiviert (separater Slice-Plan in der Roadmap).

---

## 5. Nicht Gegenstand dieser ADR

- Wahl des Java-Client-Build-Tools (Maven vs. Gradle) — eigene ADR
  (M1- oder M2-zeitig).
- Wahl des CI-Systems (GitHub Actions, GitLab, Jenkins) — eigene ADR.
- Image-Registry und Tagging-Strategie — eigene ADR.
- SBOM- und Image-Signing-Werkzeuge (cosign-Variante etc.) — eigene
  ADR (zieht in M2 ein).
- Bazel als Alternative — explizit nicht gewählt: zwei Sprachen plus
  überschaubarer Service rechtfertigt den Bazel-Overhead nicht.
