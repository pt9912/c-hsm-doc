# Spike — `CKM_HKDF_DERIVE` auf SoftHSM v2 + OpenCryptoki

**Status:** geplant (Sub-Verzeichnis angelegt, Probe-Code folgt)
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
   - `CKA_SIGN = true`
   - `CKA_TOKEN = false`
   - `CKA_EXTRACTABLE = false`
   - `CKA_SENSITIVE = true`
2. `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)` + `C_Sign(headerBytes)`
   liefert einen 32-Byte-HMAC. Roundtrip: zweimal mit identischem Input
   liefert byteweise identischen Output (Determinismus).
3. `C_WrapKey` auf dem Header-Key-Handle antwortet mit
   `CKR_KEY_UNEXTRACTABLE` (belegt `CKA_EXTRACTABLE=false`).
4. `C_DestroyObject(headerKeyHandle)` ist erfolgreich; nachfolgendes
   `C_SignInit` mit demselben Handle liefert `CKR_OBJECT_HANDLE_INVALID`.
5. Die HKDF-Parameter (Salt `key_id || key_version`, Info
   `"c-hsm-doc/header-hmac/v1"`, L=32) werden korrekt durchgereicht — Test
   vergleicht den `C_Sign`-Output gegen eine Pure-Go-HKDF-Referenzimplementierung
   (`golang.org/x/crypto/hkdf`) mit identischen Inputs auf demselben
   Master-HMAC-Material. Differenzen sind harter Fehler.
6. PKCS#11-Trace (`pkcs11-spy` oder Modul-Log) zeigt pro Lauf genau
   die erwartete Aufruffolge: `C_OpenSession`, `C_Login`, `C_FindObjects*`
   (Master-HMAC-Lookup), `C_DeriveKey`, `C_SignInit`, `C_Sign`,
   `C_DestroyObject`, `C_Logout`, `C_CloseSession`.

---

## 4. Layout

```
docs/plan/planning/next/002b-spike-hkdf/
├── README.md       (dieser Plan; nach Lauf um §6 „Ergebnis" ergänzt)
├── spike/
│   └── README.md   (Probe-Code-Stub; siehe spike/README.md)
└── trace/
    └── README.md   (Trace-Capture-Stub; siehe trace/README.md)
```

**Build-Tag-Isolation:** Aller Go-Code unter `spike/` trägt
`//go:build spike` als erste Build-Tag-Zeile. Der reguläre Repo-Build
(`go build ./...`, `make ci`) sieht den Spike-Code nicht. Spike-Läufe
verwenden `go run -tags=spike ./spike/...` bzw. ein dediziertes
Make-Target (Vorschlag: `make spike-hkdf`, Docker-only gemäß
[ADR 0002](../../../adr/0002-docker-only-build-pipeline.md)).

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

_(wird nach dem Spike-Lauf befüllt)_

Strukturvorlage:

- **Spike-Datum:** YYYY-MM-DD
- **Geprüfte Pfade:** a / b / c — je Modul (SoftHSM v2, OpenCryptoki)
- **Gewählter Pfad:** a (Shim) | b (Fork: `<repo>@<commit>`) | c (Fallback,
  Slice 002b zurück nach Planung)
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
