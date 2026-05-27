# 003 — Cloud-KMS-Adapter (Envelope-Pattern) als zweiter Driven-Pfad

**Trigger:** Architektur-Diskussion bei Slice 002 Review (2026-05-27;
Slice 002 wurde nach dem Review in 002a + 002b gesplittet — die hier
beschriebene hexagonale Architektur entsteht in 002b).

## Beobachtung

Die hexagonale Architektur aus Slice 002b
([`next/002b-pkcs11-encrypt-hexagon.md`](../next/002b-pkcs11-encrypt-hexagon.md))
schneidet `ChunkSealer` und `HeaderMAC` als Driven-Ports.
`internal/adapter/driven/pkcs11/` implementiert beide gegen
PKCS#11 mit **Pro-Chunk-AEAD** ([`HSM-FA-ENC-006`](../../../../spec/spezifikation.md)):
eine `C_EncryptInit`+`C_Encrypt`-Operation je Chunk gegen das HSM,
Schlüssel verlassen das HSM nie.

Diese Architektur ist die richtige Default für lokale/dedizierte HSMs
(SoftHSM, Utimaco, Thales, Cloud-HSM via PKCS#11). Sie passt aber
**nicht** zu Cloud-KMS-Services (AWS KMS, GCP KMS, Azure Key Vault,
Vault Transit) im Standard-Modus, weil:

- Cloud-KMS bieten typischerweise **Envelope-Encryption**: die App
  generiert lokal einen Daten-Key (DEK), verschlüsselt Daten mit dem
  DEK in der Anwendung, und der KMS verschlüsselt nur den DEK
  (Key-Encrypting-Key, KEK).
- Pro-Chunk-AEAD-Calls gegen ein Cloud-KMS wären Netzwerk-RTT-gebunden:
  bei 4-MiB-Default-Chunks ([`HSM-FA-CHUNK-008`](../../../../spec/spezifikation.md))
  und dem 10-GiB-Akzeptanztest aus
  [`HSM-FA-ENC-003`](../../../../spec/lastenheft.md) wären das
  ~2560 Cloud-API-Roundtrips pro Stream — Latenz, API-Kosten und
  Quota-Druck sind erheblich.

Die heutige Spec verlangt explizit Pro-Chunk-AEAD und lässt für
Envelope-Pattern keinen Pfad. **Cloud-HSM mit PKCS#11-Modul** (AWS
CloudHSM, Azure Managed HSM, GCP Cloud HSM) ist hingegen schon
abgedeckt — anderes `.so`-Modul, gleicher Adapter, gleiche Spec.
Dieser Trigger zielt also nicht auf Cloud-HSM, sondern auf
Cloud-**KMS** als zweiten Verschlüsselungsmodus.

## Aktivierungsbedingung

Mindestens **eine** der folgenden Bedingungen liegt vor und ist
dokumentiert (Betreiber-Anfrage, Threat-Model-Review oder Compliance-
Audit):

1. **Operativer Druck:** Betreiber/Tenant fordert explizit
   Envelope-Pattern gegen einen Cloud-KMS-Service (typisch:
   Cloud-only-Workloads ohne dedizierte HSM-Anbindung, oder
   Multi-Region-Deployments, wo Cloud-KMS regionale Verfügbarkeit
   liefert).
2. **Wirtschaftlichkeit / Latenz:** In einem konkreten Produktions-
   profil ([`HSM-TECH-006`](../../../../spec/lastenheft.md)) sind
   die Pro-Chunk-HSM-Calls nachweislich der dominante Kosten- oder
   Latenztreiber, und der Envelope-Pfad würde das spürbar
   verbessern. Messdaten aus M3 (`HSM-NFA-PERF-001..004`) liegen vor.
3. **Regulatorisch:** Eine Compliance-Auflage (Vertrag, Branchen-
   Standard, behördliche Vorgabe) verlangt einen spezifischen
   Cloud-KMS — und nicht das PKCS#11-Modul desselben Anbieters.

Ohne mindestens einen dieser drei Punkte bleibt der Trigger hier
stehen. Spekulative Cloud-Bereitschaft ist kein Aktivierungsgrund —
die Ports sind schon so geschnitten, dass die Implementierung
nachgeholt werden kann, wenn der Bedarf konkret ist.

## Ergebnis

Wenn aktiviert, wird folgendes ausgelöst:

1. **Eigener Slice-Plan** unter `docs/plan/planning/next/` (Nummer
   nach M2/M3-Schneidung). Slice-Scope:
   - Zweiter Driven-Adapter `internal/adapter/driven/kms/<provider>/`
     parallel zu `internal/adapter/driven/pkcs11/`. Implementiert
     `ChunkSealer` und `HeaderMAC` über Envelope-Pattern (lokaler
     DEK, KMS verschlüsselt KEK).
   - Adapter-Auswahl per Konfiguration (z. B. `HSMDOC_SEAL_MODE` ∈
     `{pkcs11, kms-envelope}`); pro Service-Instanz genau ein
     Modus aktiv.
   - DEK-Lebenszyklus: pro Stream oder pro Tenant; DEK darf nur in
     definiertem Speicher-Scope leben (Memory-only, keine Persistenz,
     `mlock` oder Äquivalent, explizites Zeroize).
2. **Spec-Erweiterung** in `spec/spezifikation.md`:
   - Neuer Modus „Envelope" als alternativer Encrypt-Pfad neben
     Pro-Chunk-AEAD ([`HSM-FA-ENC-006`](../../../../spec/spezifikation.md)).
   - Container-Format-Erweiterung: Header trägt den per KEK
     verschlüsselten DEK (zusätzliche Felder in
     [`HSM-FMT-001`](../../../../spec/spezifikation.md), Version-Bump
     auf `0x02`).
   - Neue Fehlerklassen für KMS-spezifische Ausfälle
     (Throttle/Quota, Auth-Fehler, regionale Nichtverfügbarkeit).
3. **ADR** unter `docs/plan/adr/` (Nummer nach Stand) mit
   konkretem Entscheidungsinhalt:
   - Welcher KMS-Provider (oder mehrere via Sub-Adapter)?
   - Envelope-Pattern-Details (DEK-Lebensdauer, Re-Key-Strategie,
     KEK-Rotation).
   - Wie wird die Modus-Wahl (pkcs11 vs. kms-envelope) operativ
     getroffen — pro Deployment, pro Tenant, pro Key?
   - Threat-Model-Delta gegenüber Pro-Chunk-PKCS#11 (DEK lebt
     temporär im Prozess-Speicher — bewusste Härtung notwendig).
4. **Audit-/Compliance-Bewertung:** prüfen, ob das Envelope-Modell
   die regulatorischen Anforderungen der Zielumgebung (z. B.
   FIPS-Mode, CC-Zertifizierung) genauso erfüllt wie der
   Pro-Chunk-PKCS#11-Pfad. Ergebnis im ADR.

Sub-Trigger, der separat überlegt werden sollte, sobald 003 aktiv
wird: **Vault Transit** als hybrider Pfad (Vault als KMS mit
HSM-Backing); das hätte Charakteristika sowohl von Cloud-KMS als
auch von einer PKCS#11-ähnlichen Operation und könnte ein eigener
Sub-Adapter werden.

## Abgrenzung

- **Kein Ersatz für PKCS#11.** Der bestehende Adapter und alle
  Spec-Anforderungen rund um Pro-Chunk-AEAD bleiben Default und
  Pflicht-Pfad. Envelope wäre additiv.
- **Kein Cloud-HSM-Trigger.** AWS CloudHSM/Azure Managed HSM/GCP
  Cloud HSM laufen über ihr jeweiliges PKCS#11-Modul innerhalb
  des bestehenden Adapters — kein Spec-Eingriff, kein neuer
  Adapter, kein Trigger nötig. Das gehört in den
  HSM-Profil-Smoke ([`HSM-TECH-006`](../../../../spec/lastenheft.md),
  M3-Scope).
- **Kein PIN-/Secret-Store-Trigger.** Vault-Agent oder K8s-Secret-CSI
  als PIN-Quelle ist eine andere Diskussion (siehe Slice 002b
  §Abgrenzung „Keine Vault-/K8s-CSI-Secret-Backends als eigener
  Adapter"). Wird separat verfolgt, wenn relevant.

## Bezug

- [`next/002b-pkcs11-encrypt-hexagon.md`](../next/002b-pkcs11-encrypt-hexagon.md) §ChunkSealer-Port, §HeaderMAC-Port
- [`spec/spezifikation.md`](../../../../spec/spezifikation.md) HSM-FA-ENC-006 (Pro-Chunk-AEAD), HSM-FMT-001..006 (Container-Layout)
- [`spec/lastenheft.md`](../../../../spec/lastenheft.md) HSM-FA-ENC-001..003 (AES-256-GCM, HSM-resident), HSM-TECH-006 (HSM-Profile), HSM-NFA-PERF-001..004 (Performance)
- [ADR 0001 §2.4 — Open-Trigger-Lebenszyklus](../../adr/0001-documentation-and-planning-structure.md)
- [Roadmap M3 — Produktionsprofile und Performance](../in-progress/roadmap.md)
