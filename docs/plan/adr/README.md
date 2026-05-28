# ADR-Index — c-hsm-doc

Lebende Übersicht über alle ADRs, ihren Status und die Vorrang-/
Schärfungs-Beziehungen zwischen ihnen. Diese Datei ist **kein**
ADR-Entscheidungstext — sie ist eine Service-Notiz für Reviewer, damit
Drift zwischen `Accepted`-Texten (die per
[`ADR 0001`](0001-documentation-and-planning-structure.md) §2.3 immutable
sind) und nachgelagerten Folge-ADRs sichtbar bleibt.

Reihenfolge: aufsteigend nach ADR-Nummer. Eine ADR mit Schärfung durch
eine Folge-ADR trägt eine Spalte „Schärfungen" mit Verweisen — die
Folge-ADR ist verbindlich, der Original-Text historisch für die
geschärfte Stelle.

---

## Aktive ADRs

| ADR  | Titel                                                                                | Status   | Datum      | Schärfungen / Folge-ADRs |
| ---- | ------------------------------------------------------------------------------------ | -------- | ---------- | ------------------------ |
| 0001 | [Dokumentations- und Planungsstruktur](0001-documentation-and-planning-structure.md) | Accepted | 2026-05-26 | [ADR 0005](0005-planstruktur-open-trigger-und-spike-pattern.md) (schärft §2.4: Open-Trigger-Lifecycle + Spike-Sub-Verzeichnisse) |
| 0002 | [Docker-only Build- und Lieferkette für den Go-Server](0002-docker-only-build-pipeline.md) | Accepted | 2026-05-26 | [ADR 0004](0004-runtime-base-cgo-pkcs11.md) (schärft §2.7: Runtime-Base auf `distroless/base` für CGO/PKCS#11) |
| 0003 | [Plattform- und Service-Mesh-Neutralität](0003-plattform-und-mesh-neutralitaet.md)    | Accepted | 2026-05-27 | —                        |
| 0004 | [Runtime-Base für CGO/PKCS#11](0004-runtime-base-cgo-pkcs11.md)                       | Accepted | 2026-05-27 | [ADR 0006](0006-hkdf-profil-a-binding-und-bouncy-hsm.md) (schärft §2.6: Zweitmodul-Default OpenCryptoki → Bouncy HSM, plus HKDF-Binding-Pfad) |
| 0005 | [Planstruktur: Open-Trigger-Lifecycle und Spike-Sub-Verzeichnisse](0005-planstruktur-open-trigger-und-spike-pattern.md) | Accepted | 2026-05-27 | —                        |
| 0006 | [HKDF-Profil-A-Binding und Bouncy HSM als Spike-Zweitmodul](0006-hkdf-profil-a-binding-und-bouncy-hsm.md) | Accepted | 2026-05-28 | [ADR 0007](0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md) (schließt §2.2 HSM-FA-HSM-001-Lücke: Profil B als M1-Default + Profil-Wahl als Config) |
| 0007 | [Profil B als M1-Header-HMAC-Default und konfigurierbare Profil-Wahl](0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md) | Accepted | 2026-05-28 | —                        |

---

## Lese-Reihenfolge bei Drift

Wenn Code, Tests oder Slice-Pläne auf einen Vertrag referenzieren, der
in einer älteren `Accepted`-ADR steht, **immer prüfen, ob eine Folge-ADR
in der „Schärfungen"-Spalte oben die Stelle schärft.** Im Zweifel:

1. Folge-ADR lesen — sie trägt die maßgebliche Fassung.
2. Original-ADR-Stelle bleibt historisch (kein Edit per
   [`ADR 0001`](0001-documentation-and-planning-structure.md) §2.3).
3. Code- und Modul-Docstrings zitieren beide ADRs über den
   `ADR NNNN`-Tag.

---

## Konvention

- Neuer ADR-Eintrag in dieser Tabelle ist Pflicht bei jeder neuen ADR.
  Reihenfolge: aufsteigend nach Nummer.
- Wenn eine ADR eine andere ablöst oder schärft, wird die
  „Schärfungen"-Spalte der **alten** ADR aktualisiert. Die alte ADR
  selbst bleibt textlich unverändert (per
  [`ADR 0001`](0001-documentation-and-planning-structure.md) §2.3).
- Statuswechsel (z. B. `Provisional → Accepted`) werden in der ADR-Datei
  selbst dokumentiert (Header-Pflichtfelder per
  [`ADR 0001`](0001-documentation-and-planning-structure.md) §2.5);
  diese Tabelle reflektiert sie.
