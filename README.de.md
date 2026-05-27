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

**Bootstrap — noch kein Produktiv-Code.** Spezifikation, Architektur
und Planung stehen; die Build-Pipeline läuft gegen einen Platzhalter-
`main.go`. Die Umsetzung startet mit Meilenstein M1
(siehe [`docs/plan/planning/in-progress/roadmap.md`](docs/plan/planning/in-progress/roadmap.md)).

| Phase                       | Status            | Quelle                                                                                  |
| --------------------------- | ----------------- | --------------------------------------------------------------------------------------- |
| Lastenheft (vertraglich)    | Entwurf 0.2       | [`spec/lastenheft.md`](spec/lastenheft.md)                                              |
| Spezifikation (technisch)   | Entwurf 0.2       | [`spec/spezifikation.md`](spec/spezifikation.md)                                        |
| Architektur                 | Entwurf 0.1       | [`spec/architecture.md`](spec/architecture.md)                                          |
| Architekturentscheidungen   | 2 ADRs            | [`docs/plan/adr/`](docs/plan/adr/)                                                      |
| Roadmap                     | M1..M4 definiert  | [`docs/plan/planning/in-progress/roadmap.md`](docs/plan/planning/in-progress/roadmap.md)|

## Quickstart

Der Build ist **Docker-only** (ADR 0002): keine Go-Toolchain am Host
erforderlich. Nur Docker und `make` müssen installiert sein.

```bash
make help            # alle Targets auflisten
make build           # Runtime-Image bauen (distroless static, nonroot)
make run             # Smoketest: docker run c-hsm-doc-server --version
```

Inner-Loop-Quality-Gates:

```bash
make lint            # golangci-lint
make test            # go test ./...
make coverage-gate   # Coverage-Gate (bootstrap-aware, ADR 0002 §2.5)
make gates           # lint + test + coverage-gate + docs-check
make ci              # gates + govulncheck
make fullbuild       # ci + build (vollständiger Closure-Lauf)
```

Lokaler SoftHSM-Token für Entwicklung (HSM-ENV-003):

```bash
make dev-softhsm     # SoftHSM-Token im Compose-Volume initialisieren
make dev-down        # Compose-Umgebung herunterfahren (Volume bleibt)
```

## Repository-Layout

```text
.
├── cmd/hsmdoc/             # Go-Server-Entry-Point (Bootstrap-Platzhalter)
├── internal/               # hexagonales Layout (siehe spec/architecture.md)
├── scripts/                # Build-Helfer (coverage-gate.sh)
├── dev/softhsm/            # Dev-only SoftHSM-Init-Container
├── spec/                   # Lastenheft, Spezifikation, Architektur
├── docs/                   # ADRs und Planung (Kanban-Buckets)
├── Dockerfile              # Multi-Stage Go-Build (ADR 0002)
├── Makefile                # Docker-only-Workflow
├── .dockerignore           # Build-Kontext-Filter
├── docker-compose.dev.yml  # lokale SoftHSM-Dev-Umgebung (HSM-ENV-003)
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
