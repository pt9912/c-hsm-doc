# ADR 0005 — Planstruktur: Open-Trigger-Lifecycle und Spike-Sub-Verzeichnisse

**Status:** Accepted
**Datum:** 2026-05-27
**Bezug:** [ADR 0001](0001-documentation-and-planning-structure.md)
(geschärft durch diese ADR — §2.4),
[Slice 001](../planning/done/001-grpc-skeleton.md),
[Slice 002a](../planning/in-progress/002a-cgo-build-pipeline.md),
[Slice 002b](../planning/next/002b-pkcs11-encrypt-hexagon.md)

---

## 1. Kontext

ADR 0001 §2.4 definiert den Lebenszyklus eines Plan-Eintrags als:

> `open/` (Trigger entsteht) → `next/` (Scope skizziert) →
> `in-progress/` (Slice-Plan aktiv) → `done/` (geliefert,
> Closure-Notiz).

In der Praxis sind beim Bootstrap und der M1-Slice-Arbeit zwei
Muster entstanden, die ADR 0001 nicht explizit benennt:

### 1.1 Open-Trigger-Lifecycle (Migration aus dem einlösenden Slice)

Ein Open-Trigger (`docs/plan/planning/open/NNN-…md`) markiert eine
Vorab-Klärung, die durch einen späteren Slice-PR aktiviert und
geschlossen wird. Beispiele:

- Open-Trigger 001 (`go.sum` Strict-Mode) wurde durch Slice 001
  eingelöst und im Schluss-PR von Slice 001 nach `done/` migriert
  (Commit `9c4f59c` Vorlauf; Trigger heute unter
  `docs/plan/planning/done/001-gosum-strict-mode.md`).
- Open-Trigger 002 (CGO-Base-Switch) wird durch Slice 002a aktiviert
  (aktuell `in-progress`) und mit dem Slice-002a-Closure-PR nach
  `done/` migriert; M1-DoD-07 hängt daran.

ADR 0001 §2.4 sagt nicht, **welcher** PR die Trigger-Migration trägt
und **wie** sie sich zum Slice-Closure verhält. Die Konvention ist
in der Praxis implizit eingespielt; ohne ADR-Anker droht Drift
(z. B. eigener Mini-PR für Trigger-Migration vs. zusammen mit
Slice-Closure).

### 1.2 `next/<slice>/`-Sub-Verzeichnis für Spikes und Sub-Artefakte

ADR 0001 §2.4 erlaubt nur einzelne Dateien pro Slice
(`next/NNN-titel.md`). Slice 002b führt aber einen
`CKM_HKDF_DERIVE`-Spike als Vorbedingung 3 ein, der mehrere
Artefakte produziert (Spike-Plan, Probe-Code unter
`next/002b-spike-hkdf/`, Spike-Ergebnis-Notiz, Test-Outputs gegen
SoftHSM v2 und das in ADR 0004 gewählte Zweitmodul). Diese
Sub-Artefakte gehören semantisch zu Slice 002b, sind aber zu
voluminös für die Slice-Plan-Datei selbst und überleben den
Slice-Closure nicht (Spike ist eine ein-Zeit-Untersuchung,
kein dauerhafter Code).

ADR 0001 §2.4 sagt nichts dazu, wo Spike-Sub-Artefakte zwischen
„Scope skizziert" und „Slice aktiv" wohnen. Ohne explizites
Pattern droht entweder Wildwuchs (`spike/`, `experiments/`,
`scratch/` … parallel zu `planning/`) oder das Verschieben des
Spike-Inhalts in den Slice-Plan selbst (was die Slice-Plan-Datei
unleserlich aufbläht).

ADR 0001 ist `Accepted` und nach §2.3 inhaltlich unveränderlich.
Beide Konventionen werden deshalb in dieser Folge-ADR fixiert.

---

## 2. Entscheidung

### 2.1 Open-Trigger-Lifecycle

Ein Open-Trigger wandert mit dem **Closure-PR des einlösenden
Slices** von `open/` nach `done/`, **nicht** in einem eigenen
Mini-PR und **nicht** bei der Slice-Aktivierung (Migration
`next/` → `in-progress/`). Die Trigger-Migration ist Akzeptanz-
kriterium des Slice-Closure-PR.

Konkret:

- **Trigger entsteht** (`open/NNN-titel.md`): wird einzeln per PR
  angelegt, sobald ein Beobachtungs- oder Klärungsbedarf
  formuliert ist. Header trägt **Trigger-Bedingung** (z. B. „erster
  Slice, der X importiert" oder „nach Threat-Model-Review").
- **Slice aktiviert den Trigger** (Slice geht von `next/` →
  `in-progress/`): Roadmap-`Offene Trigger`-Block streicht den
  Trigger aus dem „aktueller Bestand"; die Trigger-Datei selbst
  bleibt aber in `open/`, weil der Slice noch nicht abgeschlossen
  ist.
- **Slice schließt** (`in-progress/` → `done/`): derselbe PR
  migriert die Trigger-Datei von `open/` nach `done/` und ergänzt
  in deren Header eine Closure-Notiz („eingelöst durch Slice NNN
  am YYYY-MM-DD"). Die Trigger-Datei behält ihren Inhalt und
  bekommt eine eindeutige Status-Zeile (`Status: done`).
- **Roadmap-DoD-Tabelle** trägt für jeden Trigger einen DoD-Eintrag
  (z. B. `M1-DoD-07` für Trigger 002); der Eintrag wird mit dem
  Slice-Closure abgehakt.

Wenn ein Trigger zwischenzeitlich neu bewertet und verworfen wird,
wandert er nach `docs/archive/` (analog zu verworfenen Slice-Plänen
aus ADR 0001 §2.4). Eine Re-Aktivierung erzeugt einen neuen Trigger
mit neuer Nummer und Verweis auf den archivierten Eintrag.

### 2.2 `next/<slice>/`-Sub-Verzeichnis für Spikes und Sub-Artefakte

Ein Slice in `next/` darf ein Sub-Verzeichnis
`next/<slice-id>-<topic>/` anlegen, das Sub-Artefakte enthält, die:

- semantisch zur Slice-Vorbereitung gehören (typisch: Spikes,
  Konfigurations-Probes, Mess-Skripte),
- den Slice-Closure **nicht** überleben sollen (also nicht in
  `internal/`, `cmd/`, `docs/`, `spec/` landen),
- zu voluminös sind, um in der Slice-Plan-Datei zu wohnen.

Konvention:

- **Pfad:** `docs/plan/planning/next/<slice-id>-<topic>/`. Beispiel:
  `docs/plan/planning/next/002b-spike-hkdf/` für den HKDF-Spike
  aus Slice 002b Vorbedingung 3.
- **Pflicht-Inhalte:** Ein `README.md` im Sub-Verzeichnis, das den
  Zweck, die Erfolgs-Kriterien und das Ergebnis-Schema benennt.
- **Lebenszyklus:** Das Sub-Verzeichnis wird **vor** dem
  `next/` → `in-progress/`-Wechsel des Slices gefüllt. Beim
  Slice-Closure-PR wird das Sub-Verzeichnis nach `docs/archive/`
  verschoben oder gelöscht, je nach Reproduktionswert; die
  Entscheidung steht in der Slice-Closure-Notiz.
- **Inhaltliche Form:** Spike-Code (Go, Shell, Dockerfile-Stages)
  liegt unter dem Sub-Verzeichnis, **nicht** unter `internal/`
  oder `cmd/`. Produktiver Code, der aus einem Spike-Ergebnis
  entsteht, wandert in den regulären Slice-Implementierungs-PR
  (in `internal/`/`cmd/`).
- **Slice-Plan-Bezug:** Der Slice-Plan referenziert das Sub-
  Verzeichnis explizit als Vorbedingung mit Pfad und Erfolgs-
  Kriterien (so wie Slice 002b Vorbedingung 3 dies tut).

Spikes mit dauerhafter Reproduktionsrelevanz (z. B. eine
Performance-Messung, die im M3-Profilsmoke wieder gebraucht wird)
wandern beim Slice-Closure nicht ins Archiv, sondern in einen
geeigneten Pfad unter `docs/` (z. B. `docs/operations/spikes/`)
mit eigener README und Pflege-Hinweis.

### 2.3 ADR 0001 §2.4 bleibt textlich unverändert

ADR 0001 ist `Accepted` und §2.3 verbietet inhaltliche Edits. Die
zwei Konventionen aus §2.1 und §2.2 dieser ADR sind die maßgebliche
Schärfung; ADR 0001 §2.4 bleibt historisch gültig für die
allgemeine `open/ → next/ → in-progress/ → done/`-Bewegung.

Der ADR-Index (`docs/plan/adr/README.md`) trägt diese ADR 0005 als
Schärfung von ADR 0001 in der „Schärfungen"-Spalte ein.

---

## 3. Konsequenzen

- Open-Trigger-Migration ist deterministisch im Slice-Closure-PR.
  Reviewer können beim Slice-Closure-Review prüfen, ob die
  Trigger-Datei korrekt verschoben und die Closure-Notiz vorhanden
  ist; M1-DoD-Tabelle hat ein verbindliches Verifikations-Anker.
- Spike-Sub-Verzeichnisse legalisieren das Spike-Pattern aus Slice
  002b, ohne die Slice-Plan-Datei aufzublähen. Sie haben einen
  klaren Lebenszyklus: Anlage im `next/`-Zustand, Verwendung in
  Vorbedingungs-Check, Archivierung beim Closure.
- Das Planungs-Repository bleibt frei von `spike/` /
  `experiments/` / `scratch/`-Parallelstrukturen — alles, was zur
  Slice-Vorbereitung gehört, lebt unter
  `docs/plan/planning/next/<slice-id>-<topic>/`.
- Slice-Reviewer können beim Vorbedingungs-Check (`next/` →
  `in-progress/`) verlässlich erwarten, dass Sub-Artefakte
  unterhalb des Sub-Verzeichnisses sitzen, nicht in der
  Slice-Plan-Datei.

---

## 4. Pflege-Regeln

- Open-Trigger ohne einlösenden Slice innerhalb der ihm
  zugeordneten Meilenstein-Frist (z. B. M1) wandern mit dem
  Meilenstein-Closure entweder nach `done/` (falls inzwischen
  obsolet) oder in den nächsten Meilenstein-Scope (mit
  aktualisierter Trigger-Bedingung).
- Spike-Sub-Verzeichnisse, die nach Slice-Closure nicht ins Archiv
  gewandert sind und keinen Folge-Slice haben, sind innerhalb eines
  Quartals zu archivieren oder zu löschen — Wartungs-Pflicht des
  Repository-Maintainers.
- Neue Konventionen, die das Planungs-Lifecycle betreffen, kommen
  als weitere Folge-ADR zu ADR 0001 (oder Schärfung dieser ADR
  0005), nicht als Edit der bestehenden Texte.

---

## 5. Nicht Gegenstand dieser ADR

- Allgemeine ADR-Schreibkonventionen (Header-Pflichtfelder, ID-
  Zuordnung Lastenheft vs. Spezifikation) — bleiben in ADR 0001
  §2.5 und §2.6.
- Verworfene Slice-Pläne und ihre Archivierung
  (`docs/archive/`) — bleibt in ADR 0001 §2.4.
- Slice-Schneidungs-Heuristiken (was rechtfertigt einen Split wie
  002 → 002a + 002b?) — keine ADR-Pflicht; Slice-Autor
  entscheidet, Review prüft.
- Konkrete Inhalte einzelner Spike-Sub-Verzeichnisse — gehört in
  den jeweiligen Slice-Plan.
