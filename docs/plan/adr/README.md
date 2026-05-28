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
| 0006 | [HKDF-Profil-A-Binding und Bouncy HSM als Spike-Zweitmodul](0006-hkdf-profil-a-binding-und-bouncy-hsm.md) | Accepted | 2026-05-28 | **HSM-FA-HSM-001-Lücke-Kette:** [ADR 0007](0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md) greift §2.2 auf (Profil B als M1-Default + Profil-Wahl als Config); [ADR 0009 §2.2](0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md) macht SoftHSM-Tragfähigkeit Spike-Befund-abhängig; [ADR 0010 §2.3](0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md) fixiert verbindliche Sprachregelung **„HSM-FA-HSM-001 nicht erfüllt"** — offen bis Profil-B-Spike gegen SoftHSM grün oder Lastenheft-Change |
| 0007 | [Profil B als M1-Header-HMAC-Default und konfigurierbare Profil-Wahl](0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md) | Accepted | 2026-05-28 | [ADR 0008](0008-profil-b-spec-konstruktion-zeroize-owner.md) (fixiert Profil-B-Spec-Konstruktion HMAC(salt, IKM); schärft §2.1 Cross-Profil-Identität + §4 zwei Klartext-Werte + Zeroize-Owner-Vertrag via `defer`); [ADR 0009](0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md) (schärft §2.2: SoftHSM-Pauschalaussage durch Spike-Befund-Vorbehalt ersetzt) |
| 0008 | [Profil-B-Konstruktion gemäß HSM-FMT-006 und Zeroize-Owner-Vertrag](0008-profil-b-spec-konstruktion-zeroize-owner.md) | Accepted | 2026-05-28 | [ADR 0009](0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md) (schärft §2.3: Helper-Schnitt von „Extract+Reimport separat mit Klartext-Rückgabe" auf kombinierte `ExtractAndReimportPRK`/`ExpandAndReimportHeaderKey`-Helper, weil Go den separaten Pfad nicht sicher umsetzen kann) |
| 0009 | [Profil-B-Extract/Expand-Reimport-Helper und SoftHSM-Vorbehalt](0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md) | Accepted | 2026-05-28 | [ADR 0010](0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md) (schärft §2.1: Helper-Realisation in Pfad H (native Derive, Handle direkt) und Pfad K (Klartext-Reimport mit defer zeroize) getrennt; §3: HSM-FA-HSM-001 als „nicht erfüllt" verbindlich) |
| 0010 | [Profil-B-Helper-Zwei-Pfade und HSM-FA-HSM-001-Status](0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md) | Accepted | 2026-05-28 | —                        |

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
