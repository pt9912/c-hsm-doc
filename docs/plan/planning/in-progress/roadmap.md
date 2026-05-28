# Roadmap – `c-hsm-doc`

| Status      | Entwurf                                                                 |
| ----------- | ----------------------------------------------------------------------- |
| Version     | 0.1                                                                     |
| Datum       | 2026-05-26                                                              |
| Bezug       | [Lastenheft](../../../../spec/lastenheft.md), [Spezifikation](../../../../spec/spezifikation.md), [Architektur](../../../../spec/architecture.md) |

Lebende Übersicht der Meilensteine und ihrer Akzeptanzschnitte. Slice-
Pläne entstehen als eigene Einträge in
[`docs/plan/planning/next/`](../next/) bzw.
[`docs/plan/planning/in-progress/`](.) und referenzieren auf diese
Roadmap.

---

## Meilenstein M1 – MVP-Kern

**Ziel:** Funktionaler End-to-End-Stream Encrypt/Decrypt gegen SoftHSM v2,
ausgeliefert als Container mit Helm-Chart, mit append-only Audit-Log und
Java-Client ohne JNI.

**Bezug (vertraglich):** `HSM-MVP-001..006`.

**Scope (Lastenheft-IDs, die mit M1 abnehmbar werden):**

- Funktional: `HSM-FA-ENC-001..003`, `HSM-FA-DEC-001..002`,
  `HSM-FA-CHUNK-001..003`, `HSM-FA-STREAM-001..002`, `HSM-FA-HSM-001..003`,
  `HSM-FA-KEY-001..002`, `HSM-FA-QUEUE-001`, `HSM-FA-RETRY-001..002`,
  `HSM-FA-AUDIT-001..005`, `HSM-FA-FAIL-002`.
- Mandantenisolation: `HSM-FA-TENANT-001..002` minimal (Single-Tenant
  zulässig, mehrstufige Quotas folgen in M4).
- Schnittstellen: `HSM-API-JAVA-001`, `HSM-API-GRPC-001..003`,
  `HSM-API-P11-001`, `HSM-API-CFG-001..002`.
- Umgebung: `HSM-ENV-001..004` (Container, Kubernetes, lokale SoftHSM-
  Dev-Umgebung, Plattform-Neutralität gegen Mesh-Varianten; bereits durch
  ADR 0002 und Helm-Chart-Stub adressiert).
- NFA: `HSM-NFA-MEM-001..002`, `HSM-NFA-OPS-001..003`,
  `HSM-NFA-PORT-001`, `HSM-NFA-PORT-003`, `HSM-NFA-SEC-001`,
  `HSM-NFA-SEC-003`, `HSM-NFA-SEC-007..008`.
- Architektur: `HSM-ARCH-001..002`, `HSM-PRINC-001..003`.

**Aus dem MVP ausgeschlossen** (kommt in M2/M3/M4):

- `HSM-FA-AUDIT-006..008` Segment-Signatur, externe Verankerung,
  Chain-Rotation (die regulierten Detail-Verfahren oberhalb der in
  `HSM-FA-AUDIT-002` geforderten Basis-Hash-Chain),
- `HSM-FA-KEY-003` Schlüsselrotation,
- `HSM-FA-KEY-005..006` Usage-Limits (LH-Pflicht + SP-Detail),
- `HSM-FA-TENANT-003` Quotas pro Mandant,
- `HSM-FA-TENANT-004` vollständiger Mandantenkontext in Audit/Telemetrie
  (M1 trägt die Tenant-ID nur als Default-Wert),
- `HSM-FA-TENANT-005..006` Fair Scheduling und Tenant-Metriken
  (Spezifikation, kommen in M4),
- `HSM-NFA-PERF-001..004` Performance-Zielwerte (Messung erst in M3),
- `HSM-COMP-001..002` BSI-konforme Cipher-Suites (formaler Nachweis in
  M3 gegen Produktionsprofile),
- `HSM-NFA-SEC-005..006` SBOM + Image-Signierung (folgt in M2).

### Definition of Done — M1

Status-Konvention: `[ ]` = offen, `[x]` = erledigt. Tabellen-Checkboxen
rendern in GitHub nicht interaktiv; Status wird im PR-Commit gepflegt,
der den DoD-Punkt erfüllt.

| DoD   | Kennung    | Beschreibung                                                 | Belegtyp                          | Bezug                                              |
| ----- | ---------- | ------------------------------------------------------------ | --------------------------------- | -------------------------------------------------- |
| `[ ]` | `M1-DoD-01` | Funktionale Abnahme gegen SoftHSM erfüllt                    | Integrationstest in CI            | `HSM-ACCEPT-001`                                   |
| `[ ]` | `M1-DoD-02` | Betriebsabnahme (Helm-Chart auf Kind-Cluster) erfüllt        | Helm-Smoke-Test                   | `HSM-ACCEPT-005`                                   |
| `[ ]` | `M1-DoD-03` | 1-GiB-Demo: Encrypt-Decrypt mit identischer SHA-256-Summe    | `demo/encrypt-decrypt.sh`         | `HSM-MVP-001`                                      |
| `[ ]` | `M1-DoD-04` | Java-Beispielprogramm läuft gegen Demo-Service               | `examples/`-Modul + Live-Lauf     | `HSM-MVP-006`, `HSM-API-JAVA-001`                  |
| `[x]` | `M1-DoD-05` | `make ci` grün mit `internal/`-Coverage ≥ 80 % (kein Bootstrap) | CI-Job-Status                     | ADR 0002 §2.5                                      |
| `[x]` | `M1-DoD-06` | Open-Trigger 001 (`go.sum` Strict-Mode) nach `done/` migriert | Repo-State                        | [`done/001`](../done/001-gosum-strict-mode.md)     |
| `[x]` | `M1-DoD-07` | Open-Trigger 002 (CGO-Base-Switch) nach `done/` migriert      | Repo-State                        | [`done/002`](../done/002-distroless-base-fuer-cgo.md), eingelöst durch [Slice 002a](../done/002a-cgo-build-pipeline.md) |

**Verifikationspfad:** Integrationstests in CI gegen SoftHSM, Helm-
Smoke-Test gegen Kind, Maven-Build-Analyse für Java-Client.

**Slice-Bestand:** wird durch konkrete Slice-Pläne in
[`next/`](../next/) bzw. [`in-progress/`](.) befüllt.

### Einstiegspunkt M1

Der erste M1-Slice ist
[`done/001-grpc-skeleton.md`](../done/001-grpc-skeleton.md) (gRPC-
Skeleton mit allen vier Service-Methoden als `UNIMPLEMENTED`-Stubs,
TLS 1.3, Health-/Ready-Endpoints, 12-Factor-Konfiguration; geliefert
am 2026-05-27). Open-Trigger 001 (`go.sum` Strict-Mode) ist mit
diesem Slice eingelöst und nach
[`done/001-gosum-strict-mode.md`](../done/001-gosum-strict-mode.md)
migriert. Slice 002a (CGO-Build-Pipeline) ist mit Closure-PR vom
2026-05-28 nach [`done/002a-cgo-build-pipeline.md`](../done/002a-cgo-build-pipeline.md)
migriert; M1-DoD-07 ist damit abgehakt. Aktiv-Schlitz ist offen
für Slice 002b (PKCS#11-Adapter + Encrypt-Hexagon,
[`next/002b-pkcs11-encrypt-hexagon.md`](../next/002b-pkcs11-encrypt-hexagon.md));
geplante Folge-Slices stehen in der Slice-Tabelle unten.

**Tagesabschluss 2026-05-28 (002b-Vorbedingungen):**
- **Vorbedingung 3 — HKDF-Spike (Profil A):** abgeschlossen. Pfad
  (a) Shim End-to-End grün gegen Bouncy HSM 2.1.0 via
  `make spike-hkdf-bouncyhsm`; Pure-Go-Referenz gegen
  RFC-5869 Appendix A.1 verifiziert; Folge-ADRs
  [ADR 0006](../../adr/0006-hkdf-profil-a-binding-und-bouncy-hsm.md)
  `Accepted`. SoftHSM 2.6.1/2.7.0 + OpenCryptoki-Software-Token
  ohne `CKM_HKDF_DERIVE` dokumentiert (Spike §6.1).
- **Vorbedingung 4 — Profil-B-Spike:** Sub-Verzeichnis
  [`next/002b-spike-profil-b/`](../next/002b-spike-profil-b/) angelegt
  inklusive Pfad-H/Pfad-K-Aufspaltung; Probe-Code folgt. Helper-Schnitt
  + Spec-Konstruktion + SoftHSM-Vorbehalt fixiert in
  [ADR 0007](../../adr/0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md),
  [ADR 0008](../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md),
  [ADR 0009](../../adr/0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md),
  [ADR 0010](../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)
  (alle `Accepted`).
- **`HSM-FA-HSM-001`-Status:** offen — Bouncy HSM für Profil B
  via Pfad H freigegeben; SoftHSM-Tragfähigkeit ist Profil-B-
  Spike-Befund-abhängig (Pfad K mit vendor-konformem
  Klartext-PRK-Pfad oder Modul-Disqualifikation). Lastenheft
  verlangt Service-Start gegen SoftHSM v2 + Zweitmodul;
  Bouncy-HSM-Doppel-Profil ist kein Ersatz. M1-Closure ist
  blockiert bis Spike-Befund grün gegen SoftHSM oder
  `HSM-LESE-004`-Lastenheft-Change.

---

## Meilenstein M2 – Härtung und Auditierbarkeit

**Ziel:** Der Service erfüllt die Audit- und Liefer-Anforderungen, die
für Behörden- und regulierte Umgebungen typisch sind.

**Scope (zusätzlich zu M1):**

- `HSM-FA-AUDIT-006..008` Detail-Verfahren aus Spezifikation Kapitel 7
  (Segment-Signatur, externe Verankerung, Chain-Rotation; ergänzt die
  bereits in M1 implementierte Basis-Hash-Chain aus `HSM-FA-AUDIT-002`).
- `HSM-NFA-SEC-005..006` SBOM + Image-Signierung.
- `HSM-FA-FAIL-001` voll umgesetzt (alle PKCS#11-Fehlerklassen behandelt,
  Circuit Breaker, Re-Login-Throttle, Token-Removal-Recovery,
  Netzwerkpartition).
- `HSM-NFA-HA-002..003` Rolling Restart und HSM-Failover.
- `HSM-FA-KEY-003` Schlüsselrotation ohne Stream-Abbruch.
- `HSM-FA-KEY-005` + `HSM-FA-KEY-006` Key-Usage-Limits.

### Definition of Done — M2

| DoD   | Kennung    | Beschreibung                                                          | Belegtyp                          | Bezug                              |
| ----- | ---------- | --------------------------------------------------------------------- | --------------------------------- | ---------------------------------- |
| `[ ]` | `M2-DoD-01` | Security-Abnahme erfüllt                                              | mTLS-Reject-Test + PIN-Scan       | `HSM-ACCEPT-003`                   |
| `[ ]` | `M2-DoD-02` | Audit-Abnahme erfüllt                                                 | Verify-Tool-Lauf                  | `HSM-ACCEPT-004`                   |
| `[ ]` | `M2-DoD-03` | Manipulation eines Audit-Eintrags wird vom Verify-Tool erkannt         | Failure-Injection-Test            | `HSM-FA-AUDIT-002`                 |
| `[ ]` | `M2-DoD-04` | Vollständiger Audit-Datei-Neuschreib wird über externe Verankerung erkannt | Verify-Tool gegen TSA/Rekor       | `HSM-FA-AUDIT-007`                 |
| `[ ]` | `M2-DoD-05` | Token-Removal-Test: Service wieder ready ohne Pod-Restart              | Failure-Injection in Kind         | `HSM-FA-FAIL-001`, `HSM-FA-FAIL-006` |
| `[ ]` | `M2-DoD-06` | mTLS-Reject-Test zweigleisig: (a) Identitätsquelle `mtls-subject` in Modus 1+2 — Client ohne Zertifikat → `UNAUTHENTICATED`; (b) Identitätsquelle `header` in Modus 4 — Anfrage von Peer außerhalb Allowlist → `UNAUTHENTICATED`, von vertrauenswürdigem Peer → Header-Identität als `caller` im Audit | Integrationstest (Modus 1+2) + Mesh-Integrationstest (Modus 4) | `HSM-API-GRPC-003`, `HSM-ENV-004`  |
| `[ ]` | `M2-DoD-07` | SBOM (CycloneDX oder SPDX) liegt je Release vor                        | Release-Artefakt im Repo          | `HSM-NFA-SEC-005`                  |
| `[ ]` | `M2-DoD-08` | Container-Images signiert (cosign)                                     | Signaturprüfung im Deployment     | `HSM-NFA-SEC-006`                  |
| `[ ]` | `M2-DoD-09` | Schlüsselrotation während aktivem Stream bricht Stream nicht ab        | Rotation-Test gegen laufenden Stream | `HSM-FA-KEY-003`                |

**Verifikationspfad:** Failure-Injection-Tests, Audit-Verify-Tool,
SBOM-Check im Release-Workflow, Image-Signaturprüfung im Deployment.

---

## Meilenstein M3 – Produktionsprofile und Performance

**Ziel:** Der Service ist gegen mindestens ein produktives HSM-Profil
(Utimaco oder Thales) verifiziert; Performance-Ziele sind dokumentiert.

**Scope (zusätzlich zu M2):**

- `HSM-NFA-PERF-001..004` Messung in der Referenzumgebung und gegen
  ein Netzwerk-HSM.
- `HSM-TECH-006` mindestens ein Produktionsprofil mit Konfigurations-
  vorlage und Smoke-Test.
- `HSM-COMP-001..002` formaler BSI-Cipher-Suite-Nachweis.
- `HSM-COMP-004` HSM-Zertifizierungsnachweis im Profil-Dokument.
- HKDF-Profil aus Spezifikation `HSM-FMT-006` für das Produktionsprofil
  validiert.

### Definition of Done — M3

| DoD   | Kennung    | Beschreibung                                                            | Belegtyp                          | Bezug                              |
| ----- | ---------- | ----------------------------------------------------------------------- | --------------------------------- | ---------------------------------- |
| `[ ]` | `M3-DoD-01` | Performance-Abnahme für mindestens ein Produktionsprofil erfüllt         | Benchmark-Messprotokoll           | `HSM-ACCEPT-002`                   |
| `[ ]` | `M3-DoD-02` | Compliance-Abnahme für dasselbe Produktionsprofil erfüllt                | Konfigurations- + Test-Beleg      | `HSM-ACCEPT-006`                   |
| `[ ]` | `M3-DoD-03` | Performance-Messprotokoll (p50/p95/p99-Latenz + Durchsatz) liegt vor    | Messprotokoll pro Profil im Repo  | `HSM-NFA-PERF-001..004`            |
| `[ ]` | `M3-DoD-04` | HKDF-Profil für Produktionsprofil validiert (Profil A/B/C aus FMT-006)  | Profil-Dokument + Smoke-Test      | `HSM-FMT-006`                      |
| `[ ]` | `M3-DoD-05` | BSI-Cipher-Suite-Nachweis liegt vor                                      | TLS-Konfig + TR-Cipher-Mapping    | `HSM-COMP-001`, `HSM-COMP-002`     |
| `[ ]` | `M3-DoD-06` | HSM-Zertifizierungsnachweis (FIPS 140-3 L3 oder CC EAL4+) referenziert   | Profil-Dokument mit Verweis       | `HSM-COMP-004`                     |

**Verifikationspfad:** Profilspezifischer Test-Stack (HSM in Test-Lab),
Performance-Benchmark als CI-Job (optional, profilspezifisch getriggert).

---

## Meilenstein M4 – Mandantenfähigkeit produktiv

**Ziel:** Mehrmandantenbetrieb ist abnahmefähig; ein Mandant kann den
Service nicht für andere blockieren.

**Scope (zusätzlich zu M3):**

- `HSM-FA-TENANT-003` Quotas pro Mandant.
- `HSM-FA-TENANT-004` Mandantenkontext in Audit und Telemetrie
  vollständig.
- `HSM-FA-TENANT-005` Fair Scheduling (Spezifikation).
- `HSM-FA-TENANT-006` Tenant-Metriken (Spezifikation).
- Mandantenspezifische Key-Lookup-Filterung (`HSM-FA-TENANT-002`)
  als Härtungs-Test.

### Definition of Done — M4

| DoD   | Kennung    | Beschreibung                                                            | Belegtyp                          | Bezug                              |
| ----- | ---------- | ----------------------------------------------------------------------- | --------------------------------- | ---------------------------------- |
| `[ ]` | `M4-DoD-01` | Quota-Überschreitung → `RESOURCE_EXHAUSTED` + Fehlerklasse `TENANT_QUOTA` | Quota-Test                        | `HSM-FA-TENANT-003`                |
| `[ ]` | `M4-DoD-02` | Fair-Scheduling: p99 für moderaten Mandanten ≤ Faktor 3 ggü. Referenz   | Synthetischer Lasttest A vs. B    | `HSM-FA-TENANT-005`                |
| `[ ]` | `M4-DoD-03` | Cross-Tenant-Decrypt → `FAILED_PRECONDITION` + `KEY_NOT_FOUND`, im Audit als `result=error` | Negativ-Integrationstest          | `HSM-FA-TENANT-002`, `HSM-FA-AUDIT-001` |
| `[ ]` | `M4-DoD-04` | `tenant_id` (oder Hash) in allen Pflicht-Metriken und Audit-Einträgen   | Metrik-/Audit-Stichprobe          | `HSM-FA-TENANT-004`, `HSM-FA-TENANT-006` |

---

## Querschnitt (über alle Meilensteine)

| Thema                | Bearbeitung                                                         |
| -------------------- | ------------------------------------------------------------------- |
| ADR-Pflege           | bei jeder langfristigen Entscheidung neue ADR; Index aktuell halten |
| Spezifikationsdrift  | jede Implementierung referenziert Spezifikations-ID; Drift = Bug    |
| Lastenheft-Schutz    | keine technischen Detailänderungen am Lastenheft (per HSM-LESE-004) |
| Security-Review      | vor jedem Release: mTLS, PIN-Scan, SBOM, Image-Signatur             |
| Threat-Model-Pflege  | bei jedem neuen Adapter / jeder neuen externen Senke prüfen         |

---

## Offene Trigger und Vorabklärungen

Liste lebt in [`docs/plan/planning/open/`](../open/). Aktueller Bestand:

- _(keiner — alle bisher offenen Trigger sind eingelöst und nach
  `done/` migriert.)_

Erledigte Trigger:

- [`001-gosum-strict-mode`](../done/001-gosum-strict-mode.md) —
  eingelöst durch Slice 001 am 2026-05-27 (Dockerfile-Strict-Copy +
  `go mod verify`).
- [`002-distroless-base-fuer-cgo`](../done/002-distroless-base-fuer-cgo.md)
  — eingelöst durch [Slice 002a](../done/002a-cgo-build-pipeline.md)
  am 2026-05-27 (Runtime-Base auf `distroless/base-debian12:nonroot`,
  CGO-Pipeline mit `lddtree`-basierter Library-Closure und
  `pkcs11-dlopen-smoke`-Verifikation); Closure-Migration nach `done/`
  am 2026-05-28.

Beispiele für künftige Trigger, die noch keinen Eintrag haben:

- Wahl des Audit-Persistenz-Backends pro Produktionsprofil (eigene ADR).
- Wahl des Secret-Backends (Kubernetes Secret vs. Vault) — eigene ADR.
- Wahl der CI/CD-Pipeline + Image-Registry — eigene ADR.
- Confidential-Compute-Pfad als Mitigation für `HSM-THREAT-008`.

---

## Offene Arbeitsfäden

Items, die in keiner Slice-/Trigger-/ADR-Datei sauber erfasst sind,
laufen über die Routing-Liste
[`offene-arbeitsfaeden.md`](offene-arbeitsfaeden.md). Aktueller Inhalt:
zwei aufgeschobene Review-Items aus Slice 001 (cross-adapter rule,
Threshold-Two-Sources), zwei TODO-Code-Marker (MaxRecvMsgSize für
Slice 002b, TLS-Reload für Slice 006), die noch nicht skizzierten
M2-Slices 007+, ein anstehender SPIFFE/SPIRE-Open-Trigger und der
Helm-Chart-NetworkPolicy-Sub-Scope für Slice 005. Jeder Eintrag dort
trägt sein dauerhaftes Zuhause als Routing-Vermerk; sobald geroutet,
wird er aus der Liste gestrichen.

---

## Status der Roadmap

| Meilenstein | Status                                                                          |
| ----------- | ------------------------------------------------------------------------------- |
| M1          | Slices 001 + 002a in `done/`; Slice 002b in `next/` (Aktiv-Schlitz offen); M1-DoD-05/06/07 abgehakt. **M1-Closure blockiert** durch offenen `HSM-FA-HSM-001`-Akzeptanzpunkt (siehe [Tagesabschluss 2026-05-28](#einstiegspunkt-m1) + [ADR 0010 §2.3](../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)). |
| M2          | wartet auf M1-Closure; Slice 006 (Identity-Source) in `next/` vorbereitet.       |
| M3          | wartet auf M2-Closure und Verfügbarkeit Produktions-HSM.                        |
| M4          | wartet auf M3-Closure.                                                          |

### Slice-Tabelle

Lebende Übersicht der Slice-Pläne über alle Verzeichnis-Zustände
(`next/`, `in-progress/`, `done/`). Owner heute generisch
(Repo-Maintainer); wird konkretisiert, sobald mehrere Mitwirkende
parallel arbeiten.

| Slice | Titel                                              | Ort           | Status             | Letzter Touchpoint           |
| ----- | -------------------------------------------------- | ------------- | ------------------ | ---------------------------- |
| 001   | [gRPC-Skeleton](../done/001-grpc-skeleton.md)      | `done`        | Akzeptanzkriterien erfüllt, Closure-Notiz im Slice-Dokument; M1-DoD-05/06 abgehakt | Closure-Commit (2026-05-27)  |
| 002a  | [CGO-Build-Pipeline](../done/002a-cgo-build-pipeline.md) | `done`        | Akzeptanzkriterien erfüllt, Closure-Notiz im Slice-Dokument; ADR 0004 + 0005 `Accepted`; Open-Trigger 002 nach `done/` migriert; M1-DoD-07 abgehakt | Closure-Commit (2026-05-28) |
| 002b  | [PKCS#11-Adapter + Encrypt-Hexagon](../next/002b-pkcs11-encrypt-hexagon.md) | `next`        | Vorbedingung 3 (HKDF-Spike Profil A) abgeschlossen — `make spike-hkdf-bouncyhsm` grün, [ADR 0006](../../adr/0006-hkdf-profil-a-binding-und-bouncy-hsm.md) `Accepted`. Vorbedingung 4 (Profil-B-Spike) als Sub-Verzeichnis [`002b-spike-profil-b/`](../next/002b-spike-profil-b/) angelegt; Helper-Schnitt + Pfad-H/K + SoftHSM-Vorbehalt in [ADR 0007](../../adr/0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md) / [ADR 0008](../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md) / [ADR 0009](../../adr/0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md) / [ADR 0010](../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md) `Accepted`. Probe-Code (Spike Phase 1+2) + Slice-Aktivierung offen. | Plan-Konsolidierungs-Commit (2026-05-28) |
| 003   | Container-Codec + Decrypt                          | _ungeschnitten_ | geplant; hängt an 002b (Container-Encoder + Pro-Chunk-AAD)         | —                            |
| 004   | Basis-Audit-Log mit Hash-Chain                     | _ungeschnitten_ | geplant                                          | —                            |
| 005   | Helm-Chart + Kind-Smoke                            | _ungeschnitten_ | geplant; trägt Sub-Scope NetworkPolicy-Defaults aus [`offene-arbeitsfaeden.md`](offene-arbeitsfaeden.md) §5 | — |
| 006   | [Identity-Source und Peer-Allowlist](../next/006-identity-source-und-peer-allowlist.md) | `next` | wartet auf Slices 001+004; setzt `HSM-API-GRPC-006..008` um | Commit `9de091d` (2026-05-27) |
| 007–011 | M2-DoDs 02..05, 07–09                            | _ungeschnitten_ | Schneidung skizziert in [`offene-arbeitsfaeden.md`](offene-arbeitsfaeden.md) §3 | — |
