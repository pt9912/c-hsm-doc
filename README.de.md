# c-hsm-doc

**HSM-gestützte Dokumentverschlüsselung — Go-Server, Java-Client.**

`c-hsm-doc` ist ein hardwaregestützter Krypto-Dienst, der Dokumente
beliebiger Größe mittels AES-256-GCM verschlüsselt und entschlüsselt.
Alle AES-Operationen laufen via PKCS#11 vollständig im HSM; das
Schlüsselmaterial verlässt das HSM niemals. Der Go-basierte Server
kommuniziert per gRPC; eine Java-21-Client-Bibliothek streamt
Dokumente zu und von ihm.

> **Language:** The English version of this README is at
> [`README.md`](README.md). Lastenheft, Spezifikation und Architektur
> sind auf Deutsch verfasst (siehe `spec/`).

## Status

**M1 aktiv.** Slice 001 (gRPC-Skeleton, TLS 1.3, Health-/Ready-
Endpoints, 12-Factor-Config) ist geliefert. Slice 002a (CGO-fähige
Build-Pipeline mit `distroless/base`, transitive `lddtree`-
Library-Closure, `pkcs11-dlopen-smoke`-Helfer, OpenCryptoki als
zweites herstellerfremdes PKCS#11-Modul) ist aktiv und `make ci`
ist grün
(siehe [`docs/plan/planning/in-progress/roadmap.md`](docs/plan/planning/in-progress/roadmap.md)).
Slice 002b (PKCS#11-Driven-Adapter, Encrypt-Hexagon, durabler
Audit-Sink, Key-Registry) wartet in
[`docs/plan/planning/next/`](docs/plan/planning/next/).

| Phase                       | Status                                | Quelle                                                                                  |
| --------------------------- | ------------------------------------- | --------------------------------------------------------------------------------------- |
| Lastenheft (vertraglich)    | Entwurf 0.2                           | [`spec/lastenheft.md`](spec/lastenheft.md)                                              |
| Spezifikation (technisch)   | Entwurf 0.2                           | [`spec/spezifikation.md`](spec/spezifikation.md)                                        |
| Architektur                 | Entwurf 0.1                           | [`spec/architecture.md`](spec/architecture.md)                                          |
| Architekturentscheidungen   | 5 ADRs (0001–0005)                    | [`docs/plan/adr/`](docs/plan/adr/)                                                      |
| Roadmap                     | M1 aktiv (001 done, 002a in-progress) | [`docs/plan/planning/in-progress/roadmap.md`](docs/plan/planning/in-progress/roadmap.md)|

## Quickstart

Der Build ist **Docker-only** (ADR 0002): keine Go-Toolchain am Host
erforderlich. Nur Docker und `make` müssen installiert sein.

```bash
make help            # alle Targets auflisten
make build           # Runtime-Image bauen (distroless base, nonroot, CGO; ADR 0004)
make run             # Smoketest: docker run c-hsm-doc-server --version
```

Inner-Loop-Quality-Gates:

```bash
make lint            # golangci-lint
make test            # go test ./... (CGO_ENABLED=0)
make coverage-gate   # Coverage-Gate (Schwelle 80 %, ADR 0002 §2.5)
make gates           # lint + test + coverage-gate + docs-check
make ci              # gates + govulncheck + Image-Pipeline-Gates (ADR 0004)
make fullbuild       # ci + build (vollständiger Closure-Lauf)
```

Image-Pipeline-Gates aus Slice 002a (ADR 0004) — auch in `make ci`
enthalten:

```bash
make closure-check   # lddtree-Verifikation gegen Runtime-Rootfs neu erzwingen
make smoke-dlopen    # PKCS#11-Modul im Runtime-Image dlopen()
make image-scan      # Trivy HIGH/CRITICAL (schreibt out/security/trivy-runtime.json)
make image-size      # Runtime-Image-Größe protokollieren (out/security/image-size.txt)
```

Lokaler SoftHSM-Token für Entwicklung (HSM-ENV-003):

```bash
make dev-softhsm     # SoftHSM-Token im Compose-Volume initialisieren
make dev-down        # Compose-Umgebung herunterfahren (Volume bleibt)
```

## Repository-Layout

```text
.
├── cmd/hsmdoc/                  # Go-Server-Entry-Point (Slice 001)
├── cmd/pkcs11-dlopen-smoke/     # PKCS#11-dlopen-Helfer (Slice 002a, ADR 0004 §2.5)
├── internal/                    # hexagonales Layout (siehe spec/architecture.md)
├── scripts/                     # Build-Helfer (coverage-gate.sh)
├── dev/softhsm/                 # Dev-only SoftHSM-Init-Container
├── spec/                        # Lastenheft, Spezifikation, Architektur
├── docs/                        # ADRs und Planung (Kanban-Buckets)
├── Dockerfile                   # Multi-Stage Go-Build (ADR 0002 + 0004)
├── Makefile                     # Docker-only-Workflow
├── .dockerignore                # Build-Kontext-Filter
├── docker-compose.dev.yml       # lokale SoftHSM-Dev-Umgebung (HSM-ENV-003)
└── go.mod
```

## Dokumentation

- **Lastenheft** (vertraglich abnahmebindend):
  [`spec/lastenheft.md`](spec/lastenheft.md).
- **Technische Spezifikation** (technisch verbindlich, ohne Lastenheft-
  Änderung fortschreibbar per `HSM-LESE-004`):
  [`spec/spezifikation.md`](spec/spezifikation.md).
- **Architektur-Sicht** (Komponenten, Deployment, Vertrauensgrenzen,
  Sequenzen):
  [`spec/architecture.md`](spec/architecture.md).
- **Architecture Decision Records:**
  [`docs/plan/adr/`](docs/plan/adr/).
- **Planning-Artefakte (Slices, Roadmap):**
  [`docs/plan/planning/{open,next,in-progress,done}/`](docs/plan/planning/).
- **Quality-Gates:**
  [`docs/user/quality.md`](docs/user/quality.md).

## Voraussetzungen

Für den Bau aus den Quellen:

- Docker Engine
- GNU `make`

Es ist bewusst keine Go-Toolchain am Host erforderlich (ADR 0002).

## Lizenz

MIT — siehe [`LICENSE`](LICENSE).
