# ADR 0006 — HKDF-Profil-A-Binding und Bouncy HSM als Spike-Zweitmodul

**Status:** Accepted
**Datum:** 2026-05-28
**Bezug:** [Lastenheft](../../../spec/lastenheft.md) (`HSM-FA-HSM-001`,
`HSM-API-P11-003`),
[Spezifikation](../../../spec/spezifikation.md) (`HSM-FMT-006`),
[ADR 0001](0001-documentation-and-planning-structure.md),
[ADR 0004](0004-runtime-base-cgo-pkcs11.md)
(geschärft durch diese ADR — §2.6 Modulwahl und §1 HKDF-Erwähnung),
[Slice 002b](../planning/next/002b-pkcs11-encrypt-hexagon.md),
[Spike 002b-HKDF §6](../planning/next/002b-spike-hkdf/README.md)

---

## 1. Kontext

Slice 002b §HeaderMAC-Port macht
[`HSM-FMT-006`](../../../spec/spezifikation.md) Profil A (`C_DeriveKey`
mit `CKM_HKDF_DERIVE` + `CK_HKDF_PARAMS`, anschließendes `C_SignInit`/
`C_Sign` mit `CKM_SHA256_HMAC` auf dem session-ephemeren Header-Key)
zum einzigen M1-Pfad für den Header-HMAC. Vorbedingung 3 des Slice-Plans
verlangt einen Spike, der diesen Pfad gegen **zwei** CI-Module validiert
— SoftHSM v2 als Erstmodul und das in
[ADR 0004 §2.6](0004-runtime-base-cgo-pkcs11.md) als Default gewählte
OpenCryptoki als Zweitmodul.

Der Spike (siehe
[Spike-README §6](../planning/next/002b-spike-hkdf/README.md))
hat zwei Befunde produziert, die ADR 0004 §2.6 und die zugehörige
HKDF-Annahme aus §1 schärfungsbedürftig machen:

### 1.1 Binding-Lücke im vorgeschriebenen Go-Binding

`HSM-API-P11-003` schreibt
[`github.com/miekg/pkcs11`](https://pkg.go.dev/github.com/miekg/pkcs11)
als Binding vor. Die Version 1.1.2 exportiert weder die Konstante
`CKM_HKDF_DERIVE` (0x0000402A, PKCS#11 v3.0 §6.30) noch einen
Go-Typ für `CK_HKDF_PARAMS` (PKCS#11 v3.0 §6.31.1). Ohne Adapter ist
`C_DeriveKey` mit HKDF aus dem Binding nicht aufrufbar. Slice 002b
nennt drei Auswege (Spike-Plan §Vorbedingung 3):
(a) Shim — `CK_HKDF_PARAMS` lokal als `[]byte` serialisieren und über
`pkcs11.NewMechanism(uint(CKM_HKDF_DERIVE), paramBytes)` durchreichen;
(b) Fork — gepflegter Fork des Bindings mit nativer Unterstützung;
(c) Fallback — Profil B oder Modul-Wechsel.

### 1.2 Modul-Mechanismus-Lücke in SoftHSM und OpenCryptoki

Der Spike hat gegen beide vorgesehenen Software-HSMs HKDF gesucht:

| Modul                          | `CKM_HKDF_DERIVE`? | Beleg                                                      |
| ------------------------------ | ------------------ | ---------------------------------------------------------- |
| SoftHSM v2.6.1 (Debian 12)     | **nein**           | `C_DeriveKey` → `CKR_MECHANISM_INVALID` (0x70), live       |
| SoftHSM v2.7.0 (latest, Jan 26)| **nein**           | NEWS-Datei + GitHub-Source-Search: HKDF nur in `pkcs11.h` als Konstante, keine Implementierung |
| OpenCryptoki (Software-Token)  | **nein**           | Source-Search: `HKDF` nur in `usr/lib/ep11_stdll/ep11.h` (IBM-EP11-Hardware-Backend) |
| Bouncy HSM 2.1.0               | **ja**             | `src/Src/BouncyHsm.Core/Services/Contracts/Generators/HkdfDeriveKeyGenerator.cs`, aktiv (letzte Aktualisierung 2026-05-14) |

Damit ist die in ADR 0004 §2.6 angedeutete Alternative
„Mozilla-NSS-Softoken" weder erforderlich noch ausreichend — NSS-Softoken
hat ebenfalls kein `CKM_HKDF_DERIVE`. Bouncy HSM ist der einzige
Open-Source-Software-HSM, der den Mechanismus nativ implementiert.

ADR 0004 ist `Accepted` und nach
[ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
inhaltlich unveränderlich. Die zwei Schärfungen (Binding-Pfad +
Modul-Wechsel) werden deshalb in dieser Folge-ADR fixiert.

---

## 2. Entscheidung

### 2.1 Binding-Pfad: (a) Shim

Slice 002b nutzt den **Shim-Pfad** (a) aus dem Spike-Plan: ein lokales
Marshal von `CK_HKDF_PARAMS` nach PKCS#11 v3.0 §6.31.1 (LP64-Layout,
Little-Endian) ergibt einen 64-Byte-Parameterblock, der direkt als
`pkcs11.NewMechanism(uint(CKM_HKDF_DERIVE), paramBytes)` an `C_DeriveKey`
übergeben wird. C-Memory für `pSalt`/`pInfo` wird vom Aufrufer über
`C.CBytes`/`C.free` verwaltet, weil Go-Slice-Adressen über CGO-Aufrufe
hinweg nicht stabil sind.

Validiert durch:

- **Pure-Go-Schiene:** `Marshal` ist Test-bar gegen einen Hex-Dump-
  Referenzwert + Layout-Asserts; alle 10 verwendeten PKCS#11-Konstanten
  (`CKM_HKDF_DERIVE = 0x402A` u. a.) werden gegen Literale geprüft, damit
  ein Tippfehler nicht durch alle Tests rutscht
  ([`spike/mechanism_test.go`](../planning/next/002b-spike-hkdf/spike/mechanism_test.go)).
- **RFC-5869-Schiene:** `DeriveHeaderKey` reproduziert
  RFC-5869 Appendix A.1 Test Case 1 (SHA-256) byteweise
  ([`spike/verify_test.go`](../planning/next/002b-spike-hkdf/spike/verify_test.go)).
- **HSM-Schiene:** Der End-to-End-Test gegen Bouncy HSM 2.1.0 vergleicht
  den HSM-`C_Sign`-Tag byteweise gegen
  `HMAC-SHA256(HKDF-Extract+Expand(IKM_fixture, salt, info, L=32),
  headerBytes)` aus der Pure-Go-Referenz — Übereinstimmung
  ([`spike/hsm_test.go`](../planning/next/002b-spike-hkdf/spike/hsm_test.go),
  live-Lauf via `make spike-hkdf-bouncyhsm`).

Pfad (b) Fork ist damit unnötig und (c) Fallback (Profil B als M1-Pfad)
unverbindlich — Profil B bleibt M3-Scope wie im Slice-002b-Plan
geregelt.

Konkret:

- `mechanism.go` definiert `Params`, `Marshal`, die Field-Offsets und
  die Mechanism-/Salt-Type-Konstanten. Validierung: Salt-Type-/
  Pointer-Konsistenz, Info-Pointer-/Längen-Konsistenz, mindestens
  eines von `Extract`/`Expand` gesetzt.
- `derive.go` ruft `Marshal` + `pkcs11.NewMechanism(uint(CKM_HKDF_DERIVE),
  params)`; das Template setzt `CKA_VALUE_LEN=32` (CK_HKDF_PARAMS hat
  kein Output-Length-Feld, die 32-Byte-Vorgabe aus HSM-FMT-006 Profil A
  kommt ausschließlich aus dem Template).
- `verify.go` liefert die Pure-Go-Vergleichsreferenz, die produktiv
  niemals neben dem HSM-Pfad existieren wird — sie ist Spike-/Test-only
  (siehe Spike-README §3 Punkt 5).

### 2.2 Spike-Zweitmodul: Bouncy HSM 2.x statt OpenCryptoki

Für die HKDF-Profil-A-Akzeptanz wird das in ADR 0004 §2.6 Default
gesetzte Zweitmodul **Bouncy HSM 2.x** statt **OpenCryptoki**.
Begründung: OpenCryptoki-Software-Token implementiert `CKM_HKDF_DERIVE`
nicht; ein Modul ohne den Mechanismus kann die HSM-FMT-006-Profil-A-
Akzeptanz nicht tragen, gleichgültig wie stabil es im CI läuft.

Konkrete Modulwahl: **Bouncy HSM 2.1.0**, gebaut aus dem
offiziellen Release-Tarball (`BouncyHsm.zip`) via
[`ci/bouncyhsm/Dockerfile`](../../../ci/bouncyhsm/Dockerfile). Das
Dockerfile pinnt den Release-Tarball per SHA-256-Checksum (`ADD
--checksum=sha256:…`) — Supply-Chain-Pinning analog
[ADR 0002 §2.4](0002-docker-only-build-pipeline.md), auch für
Spike-/Test-only-Artefakte. Eine Version-Aktualisierung verlangt
explizites Update von `BOUNCYHSM_VERSION` **und**
`BOUNCYHSM_SHA256` im Dockerfile; ein unbemerkter Tarball-Wechsel
ist damit ausgeschlossen.

Setup-Skript
[`ci/keys-init/bouncyhsm.sh`](../../../ci/keys-init/bouncyhsm.sh)
legt Slot + Token über die REST-API an (`POST /Slot`) und importiert
den 32-Byte-Fixture-IKM per PyKCS11 mit dem vom Server bereitgestellten
nativen `BouncyHsm.Pkcs11Lib.so`. Alle CKA-Attribute werden in einem
`C_CreateObject` gesetzt (Slice-Plan §3 Punkt 5; kein nachträgliches
Umschalten).

**Wichtig — `HSM-FA-HSM-001`-Status:** Diese ADR fixiert die
Spike-/Profil-A-Modulwahl; sie **schließt die
`HSM-FA-HSM-001`-Akzeptanz nicht**. Das Lastenheft
([`spec/lastenheft.md`](../../../spec/lastenheft.md), `HSM-FA-HSM-001`)
verlangt erfolgreichen Service-Start gegen SoftHSM v2 **und** ein
zweites herstellerfremdes Modul ohne Codeänderung. Solange der
Service auf Profil A festgelegt ist und SoftHSM `CKM_HKDF_DERIVE`
nicht anbietet, wird ein Service-Start gegen SoftHSM am
Mechanism-Check ([`HSM-FA-HSM-005`](../../../spec/spezifikation.md))
deterministisch abbrechen — die Akzeptanz ist damit **offen**.

Diese Lücke ist nicht durch ADR-Eingriff schließbar — ADR-Sharpening
darf Lastenheft-Akzeptanz nicht aufweichen. Sie muss in einem der
folgenden Pfade gelöst werden, und das ist explizit **kein** Scope
dieser ADR:

1. **Slice-002b-Plan-Update** (Spike-README §6.3 Punkt 3): Erfasst
   den jetzigen Stand klar und führt den Lösungspfad
   per Plan-Schärfung herbei.
2. **Profil-B-Fallback als M1-Header-HMAC-Konfig** (Software-HMAC-
   Konstruktion, derzeit Slice-002b §HeaderMAC-Port-Profil-B-Block
   M3-Scope). Wenn als M1-Pfad gezogen, wäre `HSM-FA-HSM-001` mit
   SoftHSM + Bouncy HSM erfüllbar; eigene Folge-ADR.
3. **Profil-Wahl als Service-Konfiguration**: Mechanism-Check
   prüft je Modul nur die für das konfigurierte Profil notwendigen
   Mechanismen. Erweiterung der Spec (`HSM-FA-HSM-005`); eigene
   Folge-ADR.
4. **Modul-Tausch in der `HSM-FA-HSM-001`-Akzeptanz** (z. B. zwei
   beliebige HKDF-fähige Module): Lastenheft-Änderung über
   `HSM-LESE-004`-Pfad; ADR alleine reicht nicht.

Bis einer dieser Pfade gegangen ist, ist `HSM-FA-HSM-001` für M1
**dokumentierter offener Akzeptanzpunkt**, nicht erfüllt.

### 2.3 SoftHSM bleibt Erstmodul für nicht-HKDF-Pfade

SoftHSM v2 wird nicht aus dem CI-Stack entfernt. Es bedient weiterhin
alle Pfade, die ohne HKDF auskommen:

- AES-GCM-Encrypt/Decrypt (`HSM-FA-ENC-006`),
- Session-Pool, Re-Login-Throttle, Token-Removal-Smoke
  (`HSM-FA-HSM-003..005`, `HSM-FA-FAIL-001..008`),
- Key-Lookup, Key-Registry (`HSM-FA-KEY-001..002`).

Der Profil-A-Spike-Test
[`spike/hsm_test.go`](../planning/next/002b-spike-hkdf/spike/hsm_test.go)
skippt deterministisch, wenn das angesprochene Modul `CKM_HKDF_DERIVE`
nicht anbietet — die kanonische `make spike-hkdf-test`-Linie bleibt
auch ohne HSM grün.

### 2.4 OpenCryptoki und NSS-Softoken sind nicht im Spike-Scope

ADR 0004 §2.6 nannte OpenCryptoki als Default und Mozilla-NSS-Softoken
als Alternative. Beide sind für **Profil A** disqualifiziert (kein
CKM_HKDF_DERIVE). Sie können in einem späteren Slice für nicht-HKDF-
Akzeptanzpfade reaktiviert werden, falls Bouncy HSM ausfällt — das
ist dann eine eigene Folge-ADR.

### 2.5 ADR 0004 bleibt textlich unverändert

ADR 0004 §1 (HKDF-Pflicht in den CI-Modulen) und §2.6 (OpenCryptoki-
Default) bleiben nach [ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
inhaltlich unverändert. Die maßgebliche Fassung dieser Stellen ist ab
heute diese ADR 0006; der ADR-Index trägt die Schärfungs-Beziehung.

---

## 3. Konsequenzen

- **Slice 002b ist HKDF-fähig** auf Bouncy HSM. Die Slice-Akzeptanz
  §3 Punkt 5 (Vergleich gegen Pure-Go-HKDF) ist live-grün gegen
  Bouncy HSM 2.1.0. Die `HSM-FA-HSM-001`-Akzeptanz für M1 ist damit
  **nicht** geschlossen (siehe §2.2-Schluss); Slice-Plan-Update +
  einer der vier dort genannten Pfade müssen folgen.
- **CI-Stack wird breiter:** SoftHSM-Container für nicht-HKDF-Pfade,
  Bouncy-HSM-Container (.NET aspnet:10.0) für den HKDF-Pfad. Der
  Spike-Lauf `make spike-hkdf-bouncyhsm` bringt beide Container in
  einem Docker-Network zusammen; der produktive CI-Pipeline-Eintrag
  folgt mit der Slice-002b-Aktivierung.
- **Pfad (a) Shim ist die langfristige Binding-Strategie**, solange
  `miekg/pkcs11` keine native `CK_HKDF_PARAMS`-Unterstützung
  ausliefert. Sollte ein gepflegter Fork mit nativer Unterstützung
  verfügbar werden, ist ein Wechsel auf Pfad (b) optional, nicht
  zwingend — das Marshal-Layout ist klein, getestet und in der
  Domain isoliert.
- **Bouncy HSM ist Spike-/Test-only.** Das Modul ist nicht für
  produktive Schlüsselhaltung freigegeben (Repo-README:
  „not intended for production data"). M3-Produktionsprofile
  (Utimaco, Thales) validieren Profil A je gegen ihre eigenen
  Vendor-Module; HSM-TECH-006 bleibt unverändert.
- **OpenCryptoki kann in Slice 002b nicht mehr blockieren.** Das
  Setup-Skript `ci/keys-init/opencryptoki.sh` aus ADR 0004 §2.6 wird
  nicht angelegt; das CI-Build-Image enthält OpenCryptoki nur, solange
  ADR 0004 §2.7 (Image-Größe) das nicht teurer macht als den Nutzen
  rechtfertigt.

---

## 4. Pflege-Regeln

- Wenn `miekg/pkcs11` in einer künftigen Version `CK_HKDF_PARAMS`
  nativ unterstützt, kann der Shim entfallen. Die Entscheidung über
  den Wechsel auf Pfad (b) wird als eigene Folge-ADR dokumentiert
  (Original-Marshal bleibt historisch nutzbar).
- Wenn Bouncy HSM seine HKDF-Implementierung entfernt oder das Projekt
  inaktiv wird, wird die Modulwahl in einer eigenen Folge-ADR neu
  begründet. Erste Fallback-Kandidaten: SoftHSM-Fork mit HKDF-Patches,
  oder Profil B als M1-Pfad.
- Der Pure-Go-Vergleichscode in `spike/verify.go` bleibt **strikt
  Spike-/Test-only**. Ein Refactor, der ihn in den produktiven
  Adapter-Pfad zieht, ist nicht zulässig — der produktive
  Adapter kennt das IKM nie (HSM-FMT-006 Profil A).
- Folge-ADRs zu ADR 0006 dürfen die Binding- oder Modulwahl schärfen;
  diese ADR selbst bleibt nach [ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
  unverändert.

---

## 5. Nicht Gegenstand dieser ADR

- **Profil B** (Software-HMAC-Konstruktion ohne `CKM_HKDF_DERIVE`)
  bleibt M3-Scope wie im Slice-002b-Plan §HeaderMAC-Port-Profil B
  geregelt. Bei einem Bouncy-HSM-Ausfall vor Slice-002b-Closure wird
  diese Entscheidung separat geprüft.
- **Produktive HSM-Profile (M3)** bleiben in der separaten Profil-
  Validierung je Vendor-HSM. Der Binding-Pfad (a) folgt dem
  Client-Prozess-ABI (LP64-Little-Endian auf den vom Spike unterstützten
  Architekturen `amd64`/`arm64`, siehe Build-Tag-Klausel in
  [`spike/doc.go`](../planning/next/002b-spike-hkdf/spike/doc.go)). Ein
  Vendor-Modul, das auf demselben ABI ein abweichendes
  `CK_HKDF_PARAMS`-Encoding (Big-Endian, abweichendes Padding etc.)
  verlangt, ist Modul-Bug, kein Profil-Variant — sowas würde
  PKCS#11-Konformität verletzen. Marshal-Anpassungen sind
  ausschließlich bei einem **Target-ABI-Wechsel** zulässig (z. B.
  Build für 32-Bit-LP32 oder eine Big-Endian-Architektur wie
  AIX-PPC64); dann als eigener Build-Tag + eigener Marshal-Pfad, mit
  Folge-ADR.
- **Reaktivierung von OpenCryptoki/NSS-Softoken** für nicht-HKDF-
  Pfade ist außerhalb dieser ADR; falls relevant, eigene Folge-ADR.
- **Slice-002b-Plan-Update** (Bouncy HSM als Zweitmodul im
  Slice-Plan eintragen, OpenCryptoki-Akzeptanzen entfernen) ist
  Plan-Pflege, kein ADR-Eingriff. Slice 002b ist in `next/`, also
  noch nicht `Accepted` — additive Pflege ist zulässig.
