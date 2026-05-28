# Spike — `CKM_HKDF_DERIVE` auf SoftHSM v2 + OpenCryptoki

**Status:** Spike-Erfolg auf Bouncy HSM 2.1.0 — Pfad (a) Shim
End-to-End grün gegen `CKM_HKDF_DERIVE` + `CKM_SHA256_HMAC`,
HSM-Tag stimmt byteweise mit Pure-Go-RFC-5869-Referenz überein
(siehe §6.1). SoftHSM v2.6.1/2.7.0 + OpenCryptoki-Software-Token
bleiben blockiert (kein HKDF). Reproduktion via
`ci/keys-init/bouncyhsm.sh` + Bouncy-HSM-Image
([`ci/bouncyhsm/Dockerfile`](../../../../ci/bouncyhsm/Dockerfile));
Make-Target und Folge-ADR 0006 folgen.
**Datum:** 2026-05-28
**Bezug:**
[Slice 002b §Vorbedingungen](../002b-pkcs11-encrypt-hexagon.md),
[ADR 0001 §2.4](../../../adr/0001-documentation-and-planning-structure.md),
[ADR 0005 §2.2](../../../adr/0005-planstruktur-open-trigger-und-spike-pattern.md),
[ADR 0004 — Runtime-Base CGO/PKCS#11](../../../adr/0004-runtime-base-cgo-pkcs11.md),
[Spezifikation HSM-FMT-006](../../../../../spec/spezifikation.md)

---

## 1. Zweck

Vorbedingung 3 für Slice 002b
([`002b-pkcs11-encrypt-hexagon.md` §Vorbedingungen](../002b-pkcs11-encrypt-hexagon.md))
schließen: Validieren, dass der HKDF-Profil-A-Pfad aus
[HSM-FMT-006](../../../../../spec/spezifikation.md) — `C_DeriveKey`
mit `CKM_HKDF_DERIVE` + `CK_HKDF_PARAMS`, anschließendes
`C_SignInit`/`C_Sign` mit `CKM_SHA256_HMAC` auf dem abgeleiteten
session-ephemeren Header-Key — **gegen beide CI-Module** (SoftHSM v2
**und** das in [ADR 0004](../../../adr/0004-runtime-base-cgo-pkcs11.md)
gewählte Zweitmodul, Default OpenCryptoki) durchläuft. Das vorgeschriebene
Go-Binding [`github.com/miekg/pkcs11`](https://pkg.go.dev/github.com/miekg/pkcs11)
hat keine native `CK_HKDF_PARAMS`-Unterstützung in der öffentlichen API; der
Spike entscheidet zwischen drei möglichen Lösungspfaden.

Slice 002b wird **nicht** nach `in-progress/` migriert, solange dieser
Spike nicht grün ist und keine Folge-ADR
([ADR 0006 — HKDF-Profil-A-Binding](../../../adr/), geplant)
existiert.

---

## 2. Geprüfte Pfade

Identisch zu den drei Pfaden aus dem Slice-002b-Plan
([§Vorbedingungen, Pfad a/b/c](../002b-pkcs11-encrypt-hexagon.md));
hier nur kurz, Details im Slice-Plan.

### (a) Shim — `CK_HKDF_PARAMS` als `[]byte`

`CK_HKDF_PARAMS` wird gemäß PKCS#11 v3.0 §6.31.1 C-Struct-Layout
serialisiert (alignment-/endianness-aware) und über
`pkcs11.NewMechanism(CKM_HKDF_DERIVE, paramBytes)` an `C_DeriveKey`
übergeben. Erfolgskriterium: **beide CI-Module** akzeptieren den Aufruf
und liefern einen Header-Key-Handle mit `CKA_EXTRACTABLE=false`.

### (b) Forked Binding — natives `CK_HKDF_PARAMS`

Ein gepflegter Fork von `github.com/miekg/pkcs11` mit nativer
`CK_HKDF_PARAMS`-Unterstützung wird ausgewählt, die `replace`-Direktive
in `go.mod` dokumentiert (samt Commit-Hash + Maintainer-Hinweis);
Ableitung muss auf beiden Modulen funktionieren.

### (c) Fallback-Eskalation

Schlagen (a) und (b) auf einem der beiden Module fehl, geht
Slice 002b zurück in die Planung: entweder Profil B als M1-Pfad
(vendor-spezifische Non-Export-Konstruktion gemäß HSM-FMT-006 §1
Profil B), Binding-Wechsel-Entscheidung als eigener Open-Trigger,
oder Wechsel des Zweitmoduls (Folge-ADR zu ADR 0004 — bedingte
Zweitmodul-Korrektur).

---

## 3. Erfolgs-Kriterien

Spike gilt als **grün**, wenn alle Punkte gegen **beide CI-Module**
nachweisbar sind:

1. `C_DeriveKey(CKM_HKDF_DERIVE, params, masterHmacHandle, template)`
   liefert einen Header-Key-Handle mit:
   - `CKA_CLASS = CKO_SECRET_KEY`
   - `CKA_KEY_TYPE = CKK_GENERIC_SECRET` (oder moduläquivalentes
     HMAC-fähiges Secret-Key-Attribut, siehe Slice 002b §HeaderMAC-Port)
   - `CKA_VALUE_LEN = 32` (HSM-FMT-006 Profil A fordert 32 Byte
     Header-Key-Output; `CK_HKDF_PARAMS` selbst hat **kein**
     Output-Length-Feld, die Länge wird ausschließlich über dieses
     Template-Attribut gesteuert)
   - `CKA_SIGN = true`
   - `CKA_TOKEN = false`
   - `CKA_EXTRACTABLE = false`
   - `CKA_SENSITIVE = true`
2. `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)` + `C_Sign(headerBytes)`
   liefert einen 32-Byte-HMAC. Roundtrip: zweimal mit identischem Input
   liefert byteweise identischen Output (Determinismus).
3. Negativ-Test auf Schlüsselauslese: `C_GetAttributeValue` mit
   `CKA_VALUE` als angefordertem Attribut auf dem Header-Key-Handle
   liefert `CKR_ATTRIBUTE_SENSITIVE` (belegt `CKA_SENSITIVE=true`,
   PKCS#11 v3.0 §5.2). Der Test verzichtet bewusst auf `C_WrapKey`,
   weil das einen zusätzlichen Wrapping-Key mit `CKA_WRAP=true`
   und einen modul-unterstützten Wrap-Mechanismus voraussetzen
   würde; ein Modul könnte sonst vorher mit `CKR_KEY_HANDLE_INVALID`
   oder `CKR_MECHANISM_INVALID` aussteigen und der Test wäre nicht
   aussagekräftig für `CKA_EXTRACTABLE`/`CKA_SENSITIVE`. Die
   Durchsetzung von `CKA_EXTRACTABLE=false` über einen
   Wrap-Negativtest mit ephemerem Wrapping-Key bleibt
   Slice-002b-Akzeptanzscope (außerhalb des Spike).
4. `C_DestroyObject(headerKeyHandle)` ist erfolgreich; nachfolgendes
   `C_SignInit` mit demselben Handle liefert `CKR_OBJECT_HANDLE_INVALID`.
5. Die HKDF-Parameter werden korrekt durchgereicht: Salt
   `key_id || key_version` (über `CK_HKDF_PARAMS.pSalt/ulSaltLen` als
   `CKF_HKDF_SALT_DATA`), Info `"c-hsm-doc/header-hmac/v1"` (über
   `pInfo/ulInfoLen`), Output-Länge 32 Byte (über das
   `C_DeriveKey`-Template, **nicht** über `CK_HKDF_PARAMS`).
   Verifikation: Spike-Test vergleicht den `C_Sign`-Output aus
   Punkt 2 byteweise gegen
   `HMAC-SHA256(HKDF-Extract+Expand(SHA-256, IKM=fixture, salt, info,
   L=32), headerBytes)`, berechnet rein in Pure-Go. Der HKDF-Output
   ist Zwischen-Ergebnis und nicht selbst der HSM-Output — der HSM
   gibt den HMAC-Tag über die headerBytes mit dem abgeleiteten
   Header-Key zurück.
   Das Pure-Go-IKM stammt aus einem **dediziertem Test-Fixture-IKM**: das CI-Setup-Skript pro Modul
   (`ci/keys-init/{softhsm,opencryptoki}.sh`) importiert ein bekanntes
   32-Byte-Hex-IKM (Konstante im Spike-Code, niemals produktives
   Material) per `C_CreateObject` als Master-HMAC-Key. Das Import-
   Template setzt `CKA_VALUE=<fixture>`, `CKA_DERIVE=true`,
   `CKA_EXTRACTABLE=false` und `CKA_SENSITIVE=true` im selben
   PKCS#11-Aufruf; ein nachträgliches Umschalten dieser Attribute ist
   kein zulässiger Spike-Pfad. Der Pure-Go-Referenz-HKDF im Spike
   kennt das Fixture-IKM direkt aus dem Test-Quellcode. Differenzen
   sind harter Fehler. **Wichtig:**
   Diese Fixture-Hilfskonstruktion ist Spike-/Test-only —
   produktiver Adapter-Code unter `internal/adapter/driven/pkcs11/`
   hat keinen Zugriff auf Software-HKDF oder Software-HMAC über
   echtes Master-Material; der Lookup-Pfad dort sieht ausschließlich
   den nicht-extrahierbaren Master-Key per Label.
6. PKCS#11-Trace (`pkcs11-spy` oder Modul-Log) zeigt pro Lauf genau
   die in [`trace/README.md`](trace/README.md) definierte kanonische
   Aufruffolge (`C_Initialize` … `C_Finalize`). Abweichungen zur
   kanonischen Sequenz sind Spike-Befunde und gehören in §6 Ergebnis.

---

## 4. Layout

```
docs/plan/planning/next/002b-spike-hkdf/
├── README.md             (dieser Plan; nach Lauf um §6 „Ergebnis" ergänzt)
├── spike/
│   ├── README.md         (Konventionen, Datei-Inventar)
│   ├── doc.go            (Paket-Doc, Build-Tag-Klammer)
│   ├── fixture.go        (FixtureIKM + HeaderHMACInfo-Konstante)
│   ├── mechanism.go      (CK_HKDF_PARAMS-Serialisierer, Pfad a Shim)
│   ├── mechanism_test.go (Hex-Dump-Referenz + Validierungs-Tests)
│   ├── verify.go         (Pure-Go-HKDF + HMAC-SHA256-Referenz)
│   └── verify_test.go    (RFC-5869-A.1-Vektor + Determinismus-Tests)
└── trace/
    └── README.md         (kanonische PKCS#11-Aufruffolge; single source of truth)
```

Geplant, mit dem ersten HSM-gestützten Spike-Lauf:
`spike/derive.go`, `spike/sign.go`, `spike/hsm_test.go`,
`ci/keys-init/{softhsm,opencryptoki}.sh` und
`trace/<modul>-<pfad>.log` pro Modul (siehe
[`spike/README.md`](spike/README.md) §Geplant).

**Build-Tag-Isolation:** Aller Go-Code unter `spike/` trägt
`//go:build spike` als erste Build-Tag-Zeile. Der reguläre Repo-Build
(`go build ./...`, `make ci`) sieht den Spike-Code nicht. Spike-Läufe
verwenden dedizierte Docker-only-Make-Targets: aktuell
`make spike-hkdf-test` für die Pure-Go-Serialisierer-Tests, später
`make spike-hkdf-run` für den HSM-gestützten Lauf gegen beide Module
(gemäß [ADR 0002](../../../adr/0002-docker-only-build-pipeline.md)).

**Docker-only:** Alle Build- und Lauf-Aufrufe laufen über Docker,
keine direkten Aufrufe auf dem Host. Das CI-Build-Image aus
[Slice 002a](../../done/002a-cgo-build-pipeline.md) bringt SoftHSM v2
und OpenCryptoki bereits mit.

---

## 5. Vorgehen

1. **Modul-Setup-Skripte** unter `ci/keys-init/` pro Modul anlegen
   (`softhsm.sh`, `opencryptoki.sh`); jedes Skript initialisiert
   einen Master-HMAC-Key (Typ pro Modul gemäß Slice-002b-Akzeptanz)
   mit `CKA_DERIVE=true`, `CKA_EXTRACTABLE=false`, `CKA_SENSITIVE=true`.
2. **Pfad (a) Shim** zuerst — kleinster Eingriff, kein Fork nötig.
   `CK_HKDF_PARAMS`-Serialisierer als Pure-Go-Helper im `spike/`-Paket;
   Unit-Test der Byte-Reihenfolge gegen einen Hex-Dump-Referenzwert.
   Anschließend Lauf gegen beide Module mit `pkcs11-spy`-Wrapping.
3. **Pfad (b) Fork** nur, wenn (a) auf einem der Module fehlschlägt.
   Fork-Kandidaten in `spike/README.md` §Fork-Kandidaten dokumentieren
   (Commit-Hash, Maintainer-Aktivität, offene Issues zu HKDF).
4. **Pfad (c) Fallback** dokumentieren, sobald (a) **und** (b) auf einem
   Modul fehlschlagen. Slice 002b geht in dem Fall zurück nach Planung;
   die Eskalations-Entscheidung wird in der Spike-README §6 Ergebnis
   protokolliert und der Roadmap-Slice-Tabelle als Status-Hinweis
   beigegeben.

---

## 6. Ergebnis

### 6.1 Zwischenstand 2026-05-28 — Modul-Mechanismus-Lücke

**Spike-Befund (live verifiziert + Code-Recherche):**

| Modul                          | `CKM_HKDF_DERIVE`? | Beleg                                                      |
| ------------------------------ | ------------------ | ---------------------------------------------------------- |
| SoftHSM v2.6.1 (Debian 12)     | **nein**           | `C_DeriveKey` → `CKR_MECHANISM_INVALID` (0x70), live       |
| SoftHSM v2.7.0 (latest, Jan 26)| **nein**           | NEWS-Datei + GitHub-Source-Search: nur `pkcs11.h`-Konstante, keine Implementierung |
| OpenCryptoki (Software-Token)  | **nein**           | Source-Search: `HKDF` nur in `usr/lib/ep11_stdll/ep11.h` (IBM-EP11-Hardware-Backend) |
| Bouncy HSM                     | **ja**             | `src/Src/BouncyHsm.Core/Services/Contracts/Generators/HkdfDeriveKeyGenerator.cs`, aktiv (letzte Aktualisierung 2026-05-14) |

**Pfad (a) Shim — Binding-Seite:** Korrekt validiert.
`CK_HKDF_PARAMS`-Serialisierung kommt im Modul an
([`spike/derive.go`](spike/derive.go) + [`spike/mechanism.go`](spike/mechanism.go)),
`miekg/pkcs11.NewMechanism(uint(CKM_HKDF_DERIVE), paramBytes)` reicht den
64-Byte-Block durch. Das HSM-seitige Fail ist **keine** Binding-Lücke,
sondern eine Modul-Mechanismus-Lücke.

**Folge:** Die im Slice-002b-Plan vorgesehene Modul-Kombination
(SoftHSM v2 + OpenCryptoki, [ADR 0004](../../../adr/0004-runtime-base-cgo-pkcs11.md))
trägt Profil A in beiden Software-Modulen **nicht**. Das ist
Pfad-(c)-Eskalationsmaterial aus dem Slice-Plan §Vorbedingung 3: „mit
einem anderen Zweitmodul". Bouncy HSM ist der einzige Open-Source-
Software-HSM mit HKDF-Implementierung.

**Verteidigung des laufenden Pure-Go-Codes:** Der `make spike-hkdf-test`-
Lauf bleibt grün — `TestHKDFEndToEndAgainstHSM` führt jetzt einen
Pre-Flight `HasMechanism`-Check ([`spike/connect.go`](spike/connect.go))
durch und skippt mit klarer Meldung, wenn das Modul HKDF nicht
anbietet. Pure-Go-HKDF-Referenz (RFC-5869-A.1) bleibt unangetastet
und grün.

### 6.2 Bouncy-HSM-Erfolg (2026-05-28)

End-to-End-Lauf grün gegen Bouncy HSM 2.1.0 (siehe
[`ci/bouncyhsm/Dockerfile`](../../../../ci/bouncyhsm/Dockerfile)
für das Image, [`ci/keys-init/bouncyhsm.sh`](../../../../ci/keys-init/bouncyhsm.sh)
für das Setup):

- **C_DeriveKey** mit `CKM_HKDF_DERIVE` + `CK_HKDF_PARAMS`-Shim
  + Template (`CKA_VALUE_LEN=32`, `CKA_SIGN=true`,
  `CKA_EXTRACTABLE=false`, `CKA_SENSITIVE=true`) liefert
  Header-Key-Handle.
- **CKA_VALUE-Auslese** auf dem Header-Key →
  `CKR_ATTRIBUTE_SENSITIVE` (deckt §3 Punkt 3).
- **C_Sign** mit `CKM_SHA256_HMAC` über headerBytes liefert
  32-Byte-HMAC-Tag.
- **byteweiser Vergleich** gegen `ExpectedHeaderMAC` aus
  [`spike/verify.go`](spike/verify.go) (Pure-Go-HKDF+HMAC mit
  identischem Fixture-IKM) — **identisch** (deckt §3 Punkt 5).
- **C_DestroyObject** + **Post-Destroy-C_SignInit** liefert
  `CKR_OBJECT_HANDLE_INVALID` (deckt §3 Punkt 4).
- Reproduktion: Bouncy-HSM-Server-Container + Setup-Container
  + Test-Container über Docker-Network; `BOUNCY_HSM_CFG_STRING`
  zeigt die PKCS#11-Library auf den TCP-Endpoint (8765).

**Damit ist Pfad (a) Shim auf einem CI-Modul validiert.**

### 6.3 Nächste Spike-Schritte

1. **Make-Target `spike-hkdf-bouncyhsm`** — landed
   ([`scripts/spike-hkdf-bouncyhsm.sh`](../../../../scripts/spike-hkdf-bouncyhsm.sh),
   Makefile-Target `spike-hkdf-bouncyhsm`). Reproduziert den gesamten
   Lauf: Image-Build → Docker-Network → Server-Start → Ready-Probe
   → Lib-Extraktion → Init-Skript → Go-Test → Cleanup (Trap, läuft
   auch bei Fehler). Host bleibt clean (ADR 0002).
2. Folge-ADR zu ADR 0004 (geplant: ADR 0006 — HKDF-Profil-A-
   Binding + Bouncy-HSM-Modulwahl): begründet Modul-Wechsel von
   OpenCryptoki auf Bouncy HSM, dokumentiert SoftHSM-Profil-A-
   Lücke + Bouncy-HSM-Erfolg.
3. Slice-002b-Plan auf Bouncy HSM als Zweitmodul aktualisieren
   (additive Erweiterung; Plan ist noch in `next/`, nicht
   `Accepted`).

### 6.3 Closure-Vorlage (wird nach Phase 2 befüllt)

- **Spike-Datum:** YYYY-MM-DD
- **Geprüfte Pfade:** a / b / c — je Modul
- **Gewählter Pfad:** a (Shim) | b (Fork) | c (Fallback)
- **Trace-Belege:** Verweis auf `trace/<modul>-<pfad>.log`
- **Folge-ADR:** ADR 0006 — HKDF-Profil-A-Binding, Status `Proposed`/`Accepted`
- **Roadmap-Update:** Slice-002b-Vorbedingung-3 abgehakt, Slice nach
  `in-progress/` migrierbar

---

## 7. Lebenszyklus dieses Sub-Verzeichnisses

Gemäß [ADR 0005 §2.2](../../../adr/0005-planstruktur-open-trigger-und-spike-pattern.md):

- Sub-Verzeichnis wandert mit Slice 002b nach `in-progress/` (sobald der
  Spike grün ist), später nach `done/` (mit Slice-Closure).
- Beim Slice-Closure entscheidet die Closure-Notiz, ob das
  Sub-Verzeichnis archiviert (`docs/archive/`) oder gelöscht wird. Der
  Trace-Output hat Reproduktions-Wert (PKCS#11-Aufruffolge je Modul)
  und sollte tendenziell archiviert werden; der Probe-Code selbst hat
  nach Auflösung in den produktiven PKCS#11-Adapter keinen
  eigenständigen Wert mehr.
- Bei Fallback (Pfad c) bleibt das Sub-Verzeichnis bei Slice 002b und
  wird mit dem nächsten Aktivierungsversuch wieder gefüllt — oder
  archiviert, falls Slice 002b grundlegend neu geschnitten wird.
