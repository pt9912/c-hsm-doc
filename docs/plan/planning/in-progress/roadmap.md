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
  zulässig, mehrstufige Quotas folgen in M3).
- Schnittstellen: `HSM-API-JAVA-001`, `HSM-API-GRPC-001..003`,
  `HSM-API-P11-001`, `HSM-API-CFG-001..002`.
- NFA: `HSM-NFA-MEM-001..002`, `HSM-NFA-OPS-001..003`,
  `HSM-NFA-PORT-001`, `HSM-NFA-PORT-003`, `HSM-NFA-SEC-001`,
  `HSM-NFA-SEC-003`, `HSM-NFA-SEC-007..008`.
- Architektur: `HSM-ARCH-001..002`, `HSM-PRINC-001..003`.

**Aus dem MVP ausgeschlossen** (kommt in M2/M3):

- `HSM-FA-AUDIT-002` Hash-Chain + externe Verankerung,
- `HSM-FA-KEY-003` Schlüsselrotation,
- `HSM-FA-KEY-005` Usage-Limits,
- `HSM-FA-TENANT-003..004` Quotas + Fair Scheduling,
- `HSM-NFA-PERF-001..004` Performance-Zielwerte (Messung erst in M3),
- `HSM-COMP-001..002` BSI-konforme Cipher-Suites (formaler Nachweis in
  M3 gegen Produktionsprofile),
- `HSM-NFA-SEC-005..006` SBOM + Image-Signierung (folgt in M2).

**Akzeptanz M1:**

- `HSM-ACCEPT-001` (Funktionale Abnahme gegen SoftHSM) ist erfüllt.
- `HSM-ACCEPT-005` (Betriebsabnahme, Helm-Chart auf Kind-Cluster) ist
  erfüllt.
- Demo-Skript verschlüsselt + entschlüsselt eine 1-GiB-Datei mit
  identischer SHA-256-Summe.
- Java-Beispielprogramm läuft gegen den Demo-Service.

**Verifikationspfad:** Integrationstests in CI gegen SoftHSM, Helm-
Smoke-Test gegen Kind, Maven-Build-Analyse für Java-Client.

**Slice-Bestand:** wird durch konkrete Slice-Pläne in
[`next/`](../next/) bzw. [`in-progress/`](.) befüllt.

---

## Meilenstein M2 – Härtung und Auditierbarkeit

**Ziel:** Der Service erfüllt die Audit- und Liefer-Anforderungen, die
für Behörden- und regulierte Umgebungen typisch sind.

**Scope (zusätzlich zu M1):**

- `HSM-FA-AUDIT-002` Hash-Chain + Detail-Verfahren aus Spezifikation
  Kapitel 7 (Segmentsignatur, Verankerung, Chain-Rotation, Durability,
  zulässige Senken).
- `HSM-NFA-SEC-005..006` SBOM + Image-Signierung.
- `HSM-FA-FAIL-001` voll umgesetzt (alle PKCS#11-Fehlerklassen behandelt,
  Circuit Breaker, Re-Login-Throttle, Token-Removal-Recovery,
  Netzwerkpartition).
- `HSM-NFA-HA-002..003` Rolling Restart und HSM-Failover.
- `HSM-FA-KEY-003` Schlüsselrotation ohne Stream-Abbruch.
- `HSM-FA-KEY-005` + `HSM-FA-KEY-006` Key-Usage-Limits.

**Akzeptanz M2:**

- `HSM-ACCEPT-003` (Security-Abnahme) und `HSM-ACCEPT-004` (Audit-
  Abnahme) sind erfüllt.
- Manipulation eines Audit-Eintrags + vollständiger Neuschreib werden
  vom Verify-Tool erkannt.
- Token-Removal-Test: Service wird automatisch wieder ready, ohne Pod-
  Restart.
- mTLS-Test schlägt für Clients ohne gültiges Zertifikat fehl.

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

**Akzeptanz M3:**

- `HSM-ACCEPT-002` (Performance-Abnahme) und `HSM-ACCEPT-006`
  (Compliance-Abnahme) sind für mindestens ein Produktionsprofil
  erfüllt.
- Performance-Messprotokoll mit p50/p95/p99-Latenz und Durchsatz pro
  Profil liegt im Repository.

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

**Akzeptanz M4:**

- Quota-Test rejektiert über Limit mit `RESOURCE_EXHAUSTED` +
  `TENANT_QUOTA`.
- Fair-Scheduling-Test (aggressiver Mandant A vs. moderater Mandant B)
  hält p99 für B im definierten Korridor.
- Cross-Tenant-Decrypt-Versuch schlägt mit dokumentierter Fehlerklasse
  fehl.

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

Liste lebt in [`docs/plan/planning/open/`](../open/). Heute leer
(`open/README.md` enthält nur die Konvention). Beispiele für künftige
Trigger:

- Wahl des Audit-Persistenz-Backends pro Produktionsprofil (eigene ADR).
- Wahl des Secret-Backends (Kubernetes Secret vs. Vault) – eigene ADR.
- Wahl der CI/CD-Pipeline + Image-Registry – eigene ADR.
- Confidential-Compute-Pfad als Mitigation für `HSM-THREAT-008` – open-Eintrag.

---

## Status der Roadmap

- M1: noch nicht gestartet (kein Code im Repository).
- M2–M4: nachgelagert, hängen an M1.

Wenn M1 startet, wird dieser Abschnitt durch eine kurze Status-Tabelle
ersetzt (Slice → Status → Owner → letzter Touchpoint).
