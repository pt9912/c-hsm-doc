# Offene Arbeitsfäden — Roadmap-Lücken

**Status:** lebende Trackerliste (in-progress)
**Datum:** 2026-05-27
**Bezug:** [roadmap.md](roadmap.md),
[ADR 0003](../../adr/0003-plattform-und-mesh-neutralitaet.md),
Code-Review zu Slice 001

---

## Zweck

Sammelpunkt für Arbeitspakete, die heute in keiner anderen
Planungsdatei (`roadmap.md`, Slice-Plan, Open-Trigger, ADR) sauber
erfasst sind. **Diese Datei ist Routing-Stelle, nicht Endlager:**
Sobald ein Item ein dauerhaftes Zuhause hat (eigene Slice-Datei,
Open-Trigger, ADR), wird es hier gestrichen und durch einen
Verweis auf den neuen Ort ersetzt.

Wenn alle Items geroutet sind, fliegt die Datei nach `done/`.

---

## 1. Aufgeschobene Code-Review-Items aus Slice 001

Aus dem code-reviewer-Lauf vom 2026-05-27 als „bewusst aufgeschoben"
markiert (siehe Commits `9c4f59c` + `dcc1758`).

### 1.1 Cross-Adapter Sibling-Regel in `.golangci.yml`

- **Zustand:** Reviewer hatte vorgeschlagen, jeden Sibling-Import
  aus `internal/adapter/*` außerhalb von `cmd/` zu blocken (um zu
  verhindern, dass ein neues `adapter/shared/` zur heimlichen
  Brücke wird). Ich habe es als overkill eingestuft, weil ein
  legitimer `adapter/shared/` damit auch blockiert wäre.
- **Risiko, wenn nicht gefixt:** Ein neu eingezogener
  `adapter/<sibling>/` mit unerwünschter Kopplung würde nicht
  vom Linter erkannt; nur Code-Review fängt es.
- **Routing:** Wenn Slice 002 ein zweites Adapter-Sibling
  (`driven/pkcs11/`) einzieht, im selben Slice prüfen, ob das
  Pattern es nahelegt, einen Sibling-Filter zu ergänzen.
  Andernfalls hier stehen lassen.

### 1.2 Threshold Two-Sources-of-Truth

- **Zustand:** `THRESHOLD ?= 80` im
  [`Makefile`](../../../../Makefile) **und**
  `ARG COVERAGE_THRESHOLD=80` im
  [`Dockerfile`](../../../../Dockerfile). Wer `docker build --target
  coverage` direkt aufruft (statt `make coverage-gate`), sieht den
  Makefile-Override nicht.
- **Routing:** kleiner Hygiene-Commit jederzeit machbar — Makefile
  reicht `--build-arg COVERAGE_THRESHOLD=$(THRESHOLD)` durch,
  Dockerfile-Default geht auf `0`. Wird hier gelöscht, sobald
  umgesetzt.

---

## 2. TODO-Marker im Code (Slice-006-Scope)

### 2.1 `MaxRecvMsgSize` / Keepalive für gRPC-Server — geroutet

- Geroutet in [`../next/002-pkcs11-encrypt.md`](../next/002-pkcs11-encrypt.md)
  §gRPC-Adapter (Scope) und §Akzeptanzkriterien
  (`TODO(slice-002)` aus `cmd/hsmdoc/main.go` muss mit dem Slice
  beseitigt sein). Eintrag bleibt als Routing-Marker stehen, bis
  Slice 002 nach `done/` migriert ist.

### 2.2 TLS-Material-Reload ohne Prozess-Restart

- **Zustand:** `cmd/hsmdoc/main.go:113` trägt
  `// TODO(slice-006)`. mTLS-Identitäts-Material muss rotiert
  werden können, ohne den Pod neu zu starten.
- **Routing:** Slice 006 (Identity-Source) liegt bereits in
  [`../next/006-identity-source-und-peer-allowlist.md`](../next/006-identity-source-und-peer-allowlist.md).
  Beim Aktivieren des Slice 006 prüfen, ob Reload-Mechanik in den
  Scope passt oder einen eigenen Sub-Slice braucht.

---

## 3. M2-Slice-Pläne 007+ noch nicht skizziert

Die `roadmap.md`-DoD-Tabelle für M2 (Zeile 113–125) listet neun
DoD-Items. Mit Slice 006 ist `M2-DoD-06` abgedeckt; die übrigen
acht haben keine eigene Slice-Datei in `next/`.

Vorgeschlagene Slice-Schneidung:

| Slice | DoD-Items                                  | Bezug                                    |
| ----- | ------------------------------------------ | ---------------------------------------- |
| 007   | M2-DoD-02 + M2-DoD-03 (Audit-Verify-Tool, Hash-Chain-Manipulation) | `HSM-FA-AUDIT-002`, `HSM-ACCEPT-004` |
| 008   | M2-DoD-04 (externe Verankerung TSA/Rekor) | `HSM-FA-AUDIT-006..008`                  |
| 009   | M2-DoD-05 (Token-Removal-Recovery, Circuit Breaker, Re-Login-Throttle) | `HSM-FA-FAIL-001`, `HSM-FA-FAIL-006..008` |
| 010   | M2-DoD-07 + M2-DoD-08 (SBOM + Cosign)     | `HSM-NFA-SEC-005..006`                   |
| 011   | M2-DoD-09 (Schlüsselrotation ohne Stream-Abbruch) | `HSM-FA-KEY-003`                  |

**Routing:** Jeder dieser Slices bekommt eine eigene Datei in
`next/`, **sobald M1 abgeschlossen ist oder klar wird, dass M2-Arbeit
parallel zu M1 startet**. Vorab-Schneidung jetzt schon einzufrieren
würde Detailwissen einlocken, das noch nicht stabil ist —
Just-in-Time-Anlage ist der bewusste Trade-off.

Mit jedem angelegten Slice-Plan wird die zugehörige Zeile hier
gestrichen.

---

## 4. SPIFFE/SPIRE-Folge-ADR

- **Zustand:** [ADR 0003 §2.6](../../adr/0003-plattform-und-mesh-neutralitaet.md)
  hat das explizit als zukünftige Folge-Arbeit markiert: „eigene
  Folge-ADR, wenn und falls ein Betreiber diese Härtungsstufe
  nachfragt." Heute weder Trigger-Datei noch ADR-Stub.
- **Routing:** Open-Trigger in `docs/plan/planning/open/`
  („003-spiffe-spire-haertung.md" o. ä.), der formuliert: aktiviert
  durch (a) Betreiber-Anfrage nach SPIFFE-ID-Bindung oder
  (b) Priorisierung von `HSM-THREAT-002` (Cluster-Admin-Sidecar-
  Injektion) in einem Threat-Model-Review. Wird hier gelöscht,
  sobald der Trigger angelegt ist.

---

## 5. Helm-Chart NetworkPolicy-Defaults

- **Zustand:** [ADR 0003 §2.5](../../adr/0003-plattform-und-mesh-neutralitaet.md)
  sagt „Mesh-spezifische Konfigurationsbeispiele liegen als
  Beispiel-Manifeste neben dem Helm-Chart, sind aber nicht
  Bestandteil des Chart-Defaults." Für **Modus 2** (K8s ohne Mesh)
  sollte das Helm-Chart aber sehr wohl sinnvolle NetworkPolicy-
  Defaults mitbringen — der Mesh-Datenebene-Schutz fehlt dort,
  also muss das Chart das einbringen.
- **Routing:** Sub-Scope von Slice 005 (Helm-Chart + Kind-Smoke).
  Sobald Slice 005 als Datei in `next/` angelegt wird, einen
  Abschnitt „NetworkPolicy-Defaults für Modus 2" hinzufügen, der
  auf [`docs/operations/mesh-examples/mode-2-networkpolicy.yaml`](../../../operations/mesh-examples/mode-2-networkpolicy.yaml)
  als Vorlage referenziert. Wird hier gelöscht, sobald Slice 005
  diesen Sub-Scope explizit aufgenommen hat.

---

## Lebenszyklus

- Jedes Item hat eine Route, kein Item bleibt hier dauerhaft.
- Wenn ein Item in `next/` (Slice-Plan) oder `open/` (Trigger) oder
  als ADR-Entwurf gelandet ist, wird der Abschnitt hier gestrichen
  und durch einen Einzeiler mit Link ersetzt.
- Sobald alle fünf Abschnitte verschwunden sind, wandert diese Datei
  nach `done/` mit einem Closure-Datum.
