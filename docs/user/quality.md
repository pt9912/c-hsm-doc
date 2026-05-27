# Quality Gates — `c-hsm-doc`

| Dokument    | Quality-Gates-Übersicht |
| ----------- | ----------------------- |
| Projektname | `c-hsm-doc` |
| Bezug       | `HSM-ENV-001..003`, `HSM-NFA-SEC-005..008`, `HSM-NFA-OPS-001..003`, `HSM-COMP-001..002` in [`spec/lastenheft.md`](../../spec/lastenheft.md) |
| ADR         | [`docs/plan/adr/0002-docker-only-build-pipeline.md`](../plan/adr/0002-docker-only-build-pipeline.md) |
| Status      | Entwurf 0.1.0 |
| Datum       | 2026-05-27 |

## Zweck

Dieses Dokument beschreibt die verbindlichen Quality-Gates der
`c-hsm-doc`-Codebase: welche Checks lokal und in CI laufen sollen,
welche Docker-Stages sie ausführen und welche Bootstrap-Ausnahmen
aktuell gelten. Die Architekturentscheidung lebt in
[`docs/plan/adr/0002-docker-only-build-pipeline.md`](../plan/adr/0002-docker-only-build-pipeline.md);
dieses Dokument ist die nutzernahe Übersicht für Entwicklung und
Review.

---

## 1. Grundregel: Docker-only

Der Go-Server wird ausschließlich über Docker-gekapselte Targets gebaut,
geprüft und ausgeliefert. Der Host braucht nur Docker Engine und GNU
`make`; es gibt keine Pflicht für lokale Go-, `golangci-lint`- oder
SoftHSM-Installationen.

Verbindliche Einstiege:

```bash
make lint
make test
make coverage-gate
make docs-check
make gates
make ci
make fullbuild
```

Die Targets sind dünne Wrapper um `Dockerfile`-Stages oder explizite
`docker run`-Aufrufe. CI MUSS dieselben Targets verwenden wie der
Entwickler-Host; ein CI-only-Pfad ist nicht vorgesehen.

---

## 2. Statische Analyse (`golangci-lint`)

Statische Analyse läuft über die `lint`-Stage des Top-Level-
[`Dockerfile`](../../Dockerfile):

```bash
make lint
```

Die Stage führt `golangci-lint run ./...` in einem gepinnten
`golangci/golangci-lint`-Container aus. Die Version wird über
`GOLANGCI_LINT_VERSION` im [`Makefile`](../../Makefile) gesetzt und als
Build-Argument an das Dockerfile weitergegeben.

Aktueller Bootstrap-Stand:

- Es gibt noch keine projektspezifische `.golangci.yml`.
- Damit gilt das Default-Profil der gepinnten `golangci-lint`-Version.
- Sobald der erste produktive M1-Slice echte Pakete unter `internal/`
  einzieht, SOLL das Projektprofil um Architekturregeln ergänzt werden
  (insbesondere `depguard` für die hexagonalen Schichtgrenzen aus
  [`spec/architecture.md`](../../spec/architecture.md)).

Verstöße brechen den Build.

---

## 3. Tests

Tests laufen über die `test`-Stage:

```bash
make test
```

Die Stage führt `CGO_ENABLED=0 go test ./...` aus. Im Bootstrap-Stand
prüft sie den Platzhalter unter [`cmd/hsmdoc/`](../../cmd/hsmdoc/).

Mit M1 werden die Testpflichten erweitert:

- Unit-Tests für Domain- und Application-Logik unter `internal/`.
- Adaptertests für gRPC, PKCS#11 und Storage dort, wo der jeweilige
  Slice produktiven Code einzieht.
- Integrationstests gegen SoftHSM für HSM-nahe Akzeptanzpfade.

---

## 4. Coverage

Coverage läuft über die `coverage`-Stage:

```bash
make coverage-gate
make coverage-gate THRESHOLD=80
```

Die Stage erzeugt eine Go-Coverage-Datei für `./internal/...` und ruft
[`scripts/coverage-gate.sh`](../../scripts/coverage-gate.sh) auf.

Default-Schwellwert seit Slice 001:

- `THRESHOLD ?= 80` im [`Makefile`](../../Makefile);
  `ARG COVERAGE_THRESHOLD=80` im [`Dockerfile`](../../Dockerfile).
- Generierter Protobuf-Code unter `internal/gen/` wird vom `-coverpkg`
  ausgeschlossen (Dockerfile `coverage`-Stage), da `.pb.go`-Dateien
  als `// DO NOT EDIT` gekennzeichnet sind.
- Der Bootstrap-Bypass über `COVERAGE_BOOTSTRAP=1` greift nur, solange
  `./internal/...` keine produktiven Pakete listet — seit Slice 001
  ist das nicht mehr der Fall.

Höhere Schwellwerte sind per Override ohne Code-Change möglich:

```bash
make coverage-gate THRESHOLD=85
```

---

## 5. Dokumentations-Gate

Markdown-Links werden Docker-gekapselt geprüft:

```bash
make docs-check
```

Das Target startet einen gepinnten Python-Container und führt
[`tools/check_refs.py`](../../tools/check_refs.py) aus. Gebrochene
relative Links brechen `make docs-check` und damit auch `make gates`.

Pflicht für neue Dokumentation:

- Relative Links müssen auf existierende Dateien oder Anchors zeigen.
- Pfade, Dateinamen und IDs werden in Markdown als Code formatiert.
- Kanonische Inhalte bleiben in Lastenheft, Spezifikation, Architektur
  oder ADRs; Überblicksdokumente verlinken diese Quellen nur.

---

## 6. Security-Gates

Go-Modul-Schwachstellen werden über `govulncheck` geprüft:

```bash
make govulncheck
make ci
```

`make govulncheck` startet einen Go-Container, installiert die im
[`Makefile`](../../Makefile) gepinnte `GOVULNCHECK_VERSION` und führt
`govulncheck ./...` aus. `make ci` aggregiert `make gates` plus
`make govulncheck`.

Release-nahe Security-Gates sind in `ADR 0002` vorbereitet, aber noch
nicht vollständig verdrahtet:

- SBOM-Erzeugung (`HSM-NFA-SEC-005`).
- Image-Signatur (`HSM-NFA-SEC-006`).
- Image-/Dependency-Scan über ein noch festzulegendes Tool.

Diese Gates werden mit dem jeweiligen M2-/Release-Slice ergänzt.

---

## 7. Architektur-Enforcement

Die Zielarchitektur ist hexagonal:
[`spec/architecture.md`](../../spec/architecture.md) beschreibt die
Schichten und erlaubten Abhängigkeiten.

Aktueller Bootstrap-Stand:

- Die Schichtstruktur ist dokumentiert.
- Produktive Pakete unter `internal/` entstehen erst mit M1.
- Automatisches Import-Enforcement per `golangci-lint depguard` ist
  deshalb vorbereitet, aber noch nicht projektspezifisch konfiguriert.

Ab dem ersten produktiven Slice gilt:

- Domain- und Application-Pakete importieren keine Infrastruktur wie
  PKCS#11, gRPC, Storage oder Vendor-Adapter.
- PKCS#11-Code lebt ausschließlich im vorgesehenen Driven-Adapter.
- Architekturverstöße werden als Lint-Fehler behandelt.

---

## 8. Aggregierte Gates

| Target           | Inhalt |
| ---------------- | ------ |
| `make gates`     | `lint` + `test` + `coverage-gate` + `docs-check` |
| `make ci`        | `gates` + `govulncheck` |
| `make fullbuild` | `ci` + Runtime-Image-Build |

Vor jedem Push soll mindestens `make gates` grün sein. Vor
Meilenstein-Closure oder release-naher Arbeit ist `make fullbuild`
Pflicht.

---

## 9. CI-Pipeline

Ein CI-System ist noch nicht als Workflow-Datei verdrahtet. Sobald CI
eingeführt wird, MUSS sie den Docker-only-Pfad aus `ADR 0002`
verwenden:

- keine Host-Go-Installation,
- kein Host-`golangci-lint`,
- keine CI-only-Sonderlogik,
- mindestens `make gates` für Pull Requests,
- `make ci` oder `make fullbuild` für `main` und Release-Kandidaten.
