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
- Umgebung: `HSM-ENV-001..003` (Container, Kubernetes, lokale SoftHSM-
  Dev-Umgebung; bereits durch ADR 0002 und Helm-Chart-Stub adressiert).
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

- [ ] `HSM-ACCEPT-001` (Funktionale Abnahme gegen SoftHSM) ist erfüllt.
- [ ] `HSM-ACCEPT-005` (Betriebsabnahme, Helm-Chart auf Kind-Cluster)
      ist erfüllt.
- [ ] Demo-Skript verschlüsselt + entschlüsselt eine 1-GiB-Datei mit
      identischer SHA-256-Summe.
- [ ] Java-Beispielprogramm läuft gegen den Demo-Service.
- [ ] `make ci` ist mit `internal/`-Coverage ≥ 80 % grün (Bootstrap-
      Mode deaktiviert).
- [ ] Open-Trigger 001 (`go.sum` Strict-Mode) ist nach `done/` migriert.
- [ ] Open-Trigger 002 (CGO-Base-Switch) ist nach `done/` migriert.

**Verifikationspfad:** Integrationstests in CI gegen SoftHSM, Helm-
Smoke-Test gegen Kind, Maven-Build-Analyse für Java-Client.

**Slice-Bestand:** wird durch konkrete Slice-Pläne in
[`next/`](../next/) bzw. [`in-progress/`](.) befüllt.

### Einstiegspunkt M1

Der erste M1-Slice ist als
[`next/001-grpc-skeleton.md`](../next/001-grpc-skeleton.md) hinterlegt
(gRPC-Skeleton mit allen vier Service-Methoden als `UNIMPLEMENTED`-
Stubs, TLS 1.3, Health-/Ready-Endpoints, 12-Factor-Konfiguration). Er
aktiviert Open-Trigger 001 (`go.sum` Strict-Mode) durch die ersten
echten Imports. Geplante Folge-Slices und Aktivierungspfade siehe
dort.

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

- [ ] `HSM-ACCEPT-003` (Security-Abnahme) ist erfüllt.
- [ ] `HSM-ACCEPT-004` (Audit-Abnahme) ist erfüllt.
- [ ] Manipulation eines Audit-Eintrags wird vom Verify-Tool erkannt.
- [ ] Vollständiger Neuschreib der Audit-Datei wird vom Verify-Tool
      anhand der externen Verankerung erkannt.
- [ ] Token-Removal-Test: Service wird automatisch wieder ready, ohne
      Pod-Restart.
- [ ] mTLS-Test schlägt für Clients ohne gültiges Zertifikat fehl.
- [ ] SBOM (CycloneDX oder SPDX) liegt je Release vor (`HSM-NFA-SEC-005`).
- [ ] Container-Images sind signiert (`HSM-NFA-SEC-006`).
- [ ] Schlüsselrotation während eines aktiven Streams bricht den Stream
      nicht ab (`HSM-FA-KEY-003`).

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

- [ ] `HSM-ACCEPT-002` (Performance-Abnahme) ist für mindestens ein
      Produktionsprofil erfüllt.
- [ ] `HSM-ACCEPT-006` (Compliance-Abnahme) ist für dasselbe
      Produktionsprofil erfüllt.
- [ ] Performance-Messprotokoll mit p50/p95/p99-Latenz und Durchsatz
      liegt pro Profil im Repository.
- [ ] HKDF-Profil aus Spezifikation `HSM-FMT-006` ist für das
      Produktionsprofil validiert und im Profil-Dokument festgehalten.
- [ ] BSI-TR-02102/TR-03116-Cipher-Suite-Nachweis liegt vor.
- [ ] HSM-Zertifizierungsnachweis (FIPS 140-3 Level 3 oder CC EAL4+)
      ist im Profil-Dokument referenziert (`HSM-COMP-004`).

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

- [ ] Quota-Test rejektiert über Limit mit `RESOURCE_EXHAUSTED` +
      `TENANT_QUOTA`.
- [ ] Fair-Scheduling-Test (aggressiver Mandant A vs. moderater
      Mandant B) hält p99 für B innerhalb des in
      `HSM-FA-TENANT-005` definierten Korridors (≤ Faktor 3 ggü.
      ungeladenem Referenz-p99).
- [ ] Cross-Tenant-Decrypt-Versuch schlägt mit
      `FAILED_PRECONDITION` + Fehlerklasse `KEY_NOT_FOUND` fehl und
      wird im Audit-Log als `result=error` festgehalten.
- [ ] `tenant_id` (oder Hash) erscheint in allen Pflicht-Metriken
      und Audit-Einträgen (`HSM-FA-TENANT-004`).

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

- [`001-gosum-strict-mode`](../open/001-gosum-strict-mode.md) — wird
  durch den M1-Einstiegs-Slice 001 automatisch aktiviert.
- [`002-distroless-base-fuer-cgo`](../open/002-distroless-base-fuer-cgo.md)
  — wird durch den ersten Slice aktiviert, der `github.com/miekg/pkcs11`
  importiert (geplant: M1-Slice 002).

Beispiele für künftige Trigger, die noch keinen Eintrag haben:

- Wahl des Audit-Persistenz-Backends pro Produktionsprofil (eigene ADR).
- Wahl des Secret-Backends (Kubernetes Secret vs. Vault) — eigene ADR.
- Wahl der CI/CD-Pipeline + Image-Registry — eigene ADR.
- Confidential-Compute-Pfad als Mitigation für `HSM-THREAT-008`.

---

## Status der Roadmap

| Meilenstein | Status                                                    |
| ----------- | --------------------------------------------------------- |
| M1          | Einstiegs-Slice 001-grpc-skeleton im Bestand von `next/`. |
| M2          | wartet auf M1-Closure.                                    |
| M3          | wartet auf M2-Closure und Verfügbarkeit Produktions-HSM.  |
| M4          | wartet auf M3-Closure.                                    |

Sobald der erste Slice von `next/` nach `in-progress/` wandert, wird
dieser Abschnitt um eine Slice-Tabelle ergänzt
(Slice → Status → Owner → letzter Touchpoint).
