# AGENTS.md — Briefing für AI-Coding-Agenten

Onboarding-Dokument für jede AI-Session, die in diesem Repo Code oder
Dokumentation ändert. Trägt die **harten Regeln** und **Pointer auf die
kanonischen Quellen**, nicht deren Inhalt — Drift zwischen `AGENTS.md`
und ADRs/Spec-Dokumenten wird so vermieden.

Ergänzend zu diesem Dokument:
[`README.md`](README.md) (Projekt-Überblick, englisch),
[`README.de.md`](README.de.md) (deutsche Variante),
[`spec/lastenheft.md`](spec/lastenheft.md) (vertraglich abnahmebindend),
[`spec/spezifikation.md`](spec/spezifikation.md) (technisch verbindlich, fortschreibbar),
[`spec/architecture.md`](spec/architecture.md) (Komponenten- und Sequenzsicht),
[`docs/plan/adr/README.md`](docs/plan/adr/README.md) (ADR-Index),
[`docs/plan/planning/in-progress/roadmap.md`](docs/plan/planning/in-progress/roadmap.md)
(Meilensteine M1..M4).

---

## 1. Repo-Layout

| Pfad                                                  | Inhalt                                                                                         |
| ----------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| [`spec/lastenheft.md`](spec/lastenheft.md)            | vertraglich abnahmebindend (`HSM-*`-IDs); Änderungen = Change Request                          |
| [`spec/spezifikation.md`](spec/spezifikation.md)      | technisch verbindlich, ohne LH-Änderung fortschreibbar (`HSM-LESE-004`)                        |
| [`spec/architecture.md`](spec/architecture.md)        | Komponenten-, Deployment-, Sequenz-Diagramme; **keine** eigenen Anforderungen                  |
| [`docs/plan/adr/`](docs/plan/adr/)                    | Architecture Decision Records, Schema `NNNN-kurz-titel.md`                                     |
| [`docs/plan/planning/`](docs/plan/planning/)          | Kanban-Buckets `open/` → `next/` → `in-progress/` → `done/`                                    |
| [`cmd/hsmdoc/`](cmd/hsmdoc/)                          | Go-Server-Entry-Point (`main.go`) — Wiring-Schicht                                             |
| [`internal/`](internal/)                              | hexagonales Layout (siehe `spec/architecture.md`); bootstrap                                   |
| [`scripts/`](scripts/)                                | Build-Helfer (`coverage-gate.sh`)                                                              |
| [`tools/`](tools/)                                    | Repo-Tooling (`check_refs.py`)                                                                 |
| [`dev/softhsm/`](dev/softhsm/)                        | Dev-only SoftHSM-Init-Container (`HSM-ENV-003`)                                                |
| [`Dockerfile`](Dockerfile) + [`Makefile`](Makefile)   | Docker-only Build, alle Quality Gates (ADR 0002)                                               |
| [`docker-compose.dev.yml`](docker-compose.dev.yml)    | Lokale Dev-Umgebung mit SoftHSM v2                                                             |

---

## 2. Harte Regeln

### 2.1 Lastenheft ist vertraglich, Spezifikation ist technisch

- [`spec/lastenheft.md`](spec/lastenheft.md) ist vertraglich
  abnahmebindend. Inhaltliche Änderungen folgen einem Change-Request-
  Prozess und sind keine „kosmetische Anpassung".
- [`spec/spezifikation.md`](spec/spezifikation.md) ist technisch
  verbindlich, aber ohne Lastenheft-Änderung fortschreibbar — Algorithmen,
  Datenstrukturen, Codes, Defaults, Metriknamen, Protokolldetails.
- Bei Konflikt zwischen den beiden gewinnt das Lastenheft (`HSM-LESE-004`).
- ADRs DÜRFEN NICHT Lastenheft-Anforderungen schärfen (ADR 0001 §2.6).
  ADRs DÜRFEN die Spezifikation schärfen, solange sie keine Lastenheft-
  Anforderung verletzen.
- IDs sind global eindeutig: eine `HSM-*`-ID lebt in genau einem Dokument
  (Lastenheft **oder** Spezifikation). Cross-Refs funktionieren beidseitig.

### 2.2 Docker-only Build (ADR 0002)

Kein lokales `go install`, kein Host-`golangci-lint`, kein
`go build` am Host. Alles läuft über `make` (das wiederum Docker
nutzt). Host braucht nur Docker + GNU `make`.

Falsch: `go test ./...` direkt am Host.
Richtig: `make test` (läuft im Dockerfile-Stage `test`).

Begründung: Toolchain-Reproduzierbarkeit + Supply-Chain-Defense +
einheitlicher Pfad für Entwickler und CI.

### 2.3 Architektur ist meilensteinfrei

[`spec/architecture.md`](spec/architecture.md) referenziert ADRs und
`HSM-*`-IDs, aber **keine** Meilensteine, Commit-Hashes, Status-Notizen
oder Closure-Daten. Die zeitliche Schicht lebt in
[`docs/plan/planning/in-progress/roadmap.md`](docs/plan/planning/in-progress/roadmap.md)
und den späteren `M*-results.md`-Closure-Notizen.

### 2.4 ADRs sind nach `Accepted` immutable (ADR 0001 §2.3)

Eine ADR mit Status `Accepted` wird nicht inhaltlich überschrieben.
Spätere Korrekturen oder Schärfungen entstehen als neue ADR mit
explizitem Verweis auf die abgelöste oder geschärfte Vorgängerin. Der
ADR-Index ([`docs/plan/adr/README.md`](docs/plan/adr/README.md)) trägt
für jede geschärfte ADR die Spalte „Schärfungen / Folge-ADRs".

Pflicht-Header: `Status`, `Datum`, `Bezug` (siehe ADR 0001 §2.5).

### 2.5 `git mv` + Inhalts-Rewrite: zwei Commits

Wenn eine Datei verschoben **und** der Inhalt umgeschrieben wird:

1. `git mv source target` → eigener Commit (Git erkennt `R`-Rename).
2. Inhalt umschreiben → zweiter Commit.

Sonst fällt die Rename-Detection unter die 50 %-Similarity-Schwelle und
`git log --follow` wird unzuverlässig.

### 2.6 Markdown-Konvention

- Dateinamen, Pfade und Kennungen stehen in Backticks (Monospace).
- Klickbare Pfade als `` [`foo.md`](foo.md) ``-Markdown-Link in
  Backticks.
- [`tools/check_refs.py`](tools/check_refs.py) prüft alle relativen
  Markdown-Links im Repo — ein gebrochener Link bricht `make docs-check`
  und damit `make gates`.

### 2.7 Secrets und HSM-PINs

- Die produktive HSM-PIN DARF NICHT in Code, Container-Image, Konfigurations-
  dateien des Images oder Logs erscheinen (`HSM-FA-HSM-002`, `HSM-NFA-SEC-003`).
- Default-PINs in [`dev/softhsm/`](dev/softhsm/) sind **ausschließlich**
  für die lokale Dev-Umgebung gegen SoftHSM. Sie DÜRFEN NICHT auf eine
  produktive HSM-Konfiguration übertragen werden.
- Beim Hinzufügen neuer Secret-Quellen: Pfad nach
  [`.dockerignore`](.dockerignore) und [`.gitignore`](.gitignore) prüfen.

---

## 3. Quality Gates

| Befehl              | Was es prüft                                                                          |
| ------------------- | ------------------------------------------------------------------------------------- |
| `make gates`        | `lint` + `test` + `coverage-gate` + `docs-check`                                       |
| `make ci`           | `gates` + `govulncheck`                                                                |
| `make fullbuild`    | `ci` + Runtime-Image-Build                                                             |
| `make docs-check`   | Markdown-Link-Validator ([`tools/check_refs.py`](tools/check_refs.py)) Docker-gekapselt |
| `make dev-softhsm`  | SoftHSM-Token im Compose-Volume initialisieren                                         |

Das Coverage-Gate läuft **bootstrap-aware** (ADR 0002 §2.5): solange
[`internal/`](internal/) keine produktiven Pakete enthält, akzeptiert es
leeren Input mit Schwellwert 0. Mit dem Einzug echter Logik wird die
Bootstrap-Schaltung deaktiviert und ein echter Schwellwert per
`make coverage-gate THRESHOLD=…` gesetzt (separater Slice-Plan in der
Roadmap).

**Vor jedem Push:** mindestens `make gates` grün. Vor Meilenstein-
Closure zusätzlich `make fullbuild`.

---

## 4. ADR- und Slice-Plan-Lifecycle

- **ADR-Lifecycle:** `Proposed` → `Provisional` → `Accepted`; spätere
  Änderungen kommen als neue ADR mit Verweis auf die Vorgängerin
  (ADR 0001 §2.3).
- **ADR-Index:** [`docs/plan/adr/README.md`](docs/plan/adr/README.md) —
  Pflicht-Eintrag bei jeder neuen ADR.
- **Plan-Lebenszyklus** (ADR 0001 §2.4):
  `open/` (Trigger entsteht) → `next/` (Scope skizziert) →
  `in-progress/` (Slice aktiv) → `done/` (geliefert, Closure-Notiz).
  Verworfen → `docs/archive/` (wird bei Bedarf angelegt).
- **Slice-Naming:** `NNN-kurz-titel.md` mit dreistelliger Nummer;
  Roadmap- und Meilenstein-Dokumente dürfen sprechende Namen tragen
  (z. B. `roadmap.md`, `M1-mvp-kern.md`).

---

## 5. Commit-Konvention

- **Format:** `<type>(<scope>): <kurze-headline>` (Conventional
  Commits). Typen: `feat`, `fix`, `chore`, `docs`, `test`, `build`,
  `spec`.
- **Co-Authored-By:** Bei AI-assistierten Commits Co-Authored-By-
  Trailer setzen (z. B. `Co-Authored-By: Claude Opus 4.7 (1M context)
  <noreply@anthropic.com>`).
- **HEREDOC für Multi-Line-Messages:** Bei Bash-`git commit -m` immer
  `$(cat <<'EOF' ... EOF)` nutzen, damit die Formatierung erhalten
  bleibt.
- **Sicherheit:** keine destruktiven Git-Operationen ohne explizite
  User-Freigabe (`push --force`, `reset --hard`, `checkout .`,
  `branch -D`, `--no-verify`).

---

## 6. Go-Konvention (greift mit M1)

Der Go-Server-Code existiert noch nicht (Platzhalter in
[`cmd/hsmdoc/main.go`](cmd/hsmdoc/main.go)). Sobald produktiver Code
mit M1 entsteht, gelten:

- **Hexagonale Architektur** (`HSM-ARCH-001`):
  `internal/hexagon/{domain,application,port/{driving,driven}}/` +
  `internal/adapter/{driving,driven}/`. Siehe
  [`spec/architecture.md`](spec/architecture.md) Kapitel 2.
- **Domain hängt nicht von Infrastruktur ab:** keine Imports von
  PKCS#11, gRPC, Storage oder Vendor-Modulen im `domain/`- oder
  `application/`-Paket. Enforcement über `golangci-lint depguard`,
  sobald der erste echte Slice landet.
- **`//nolint`-Marker** nur mit nachvollziehbarer Begründung im
  Kommentar daneben.
- **PKCS#11 nur in `internal/adapter/driven/pkcs11/`**; Domain spricht
  über Ports.
- **Per-Chunk-AEAD-Pflicht** (`HSM-FA-ENC-006`): genau ein
  `C_EncryptInit`/`C_Encrypt` pro Chunk, keine `C_EncryptUpdate`-Ketten
  über Chunk-Grenzen hinweg.

---

## 7. Was NICHT in `AGENTS.md` gehört

- **Konkrete ADR-Inhalte** — ADRs haben einen eigenen Lifecycle.
- **Slice-Plan-Status oder Commit-Hashes** — leben in
  [`roadmap.md`](docs/plan/planning/in-progress/roadmap.md) und den
  Planning-Buckets.
- **Architektur-Beschreibungen** — leben in
  [`spec/architecture.md`](spec/architecture.md).
- **Performance-Werte oder konkrete Defaults** — leben in
  [`spec/spezifikation.md`](spec/spezifikation.md).
- **PKCS#11-Returncode-Mappings, Container-Header-Layout,
  Audit-Hash-Chain-Verfahren** — leben in
  [`spec/spezifikation.md`](spec/spezifikation.md).

`AGENTS.md` trägt nur Pointer auf diese Quellen, nie deren Inhalt in
Kopie. Bei Konflikt zwischen `AGENTS.md` und der kanonischen Quelle
gewinnt **immer** die Quelle, und `AGENTS.md` wird nachgezogen.
