# Spike-Probe-Code (Profil B)

**Status:** Plan + Fixture landed; CGO-Pfade
(`extract.go`, `reimport.go`, `sign_b.go`, `hsm_test.go`)
folgen mit dem nächsten Inkrement.
**Bezug:** [Spike-README](../README.md),
[ADR 0007 §4](../../../../adr/0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md)
(PRK-Zeroize-Pflicht-Invariante)

---

Hier landet der Go-Code für den Profil-B-Spike-Lauf gegen
SoftHSM + Bouncy HSM. Konventionen:

- **Build-Tag:** Jede Go-Datei trägt
  `//go:build spike && cgo && (amd64 || arm64)` als erste Zeile,
  analog zum Profil-A-Spike-Paket.
- **Paketname:** `package profilbspike` (vermeidet Symbol-
  Kollision mit `hkdfspike` aus dem Profil-A-Spike).
- **Imports:** Standard-Library + `github.com/miekg/pkcs11` für
  CGO-PKCS#11-Pfade. **Wiederverwendung der Pure-Go-Referenz:**
  `ExpectedHeaderMAC` und `DeriveHeaderKey` werden direkt aus
  `github.com/pt9912/c-hsm-doc/docs/plan/planning/next/002b-spike-hkdf/spike`
  importiert (Cross-Spike-Import unter demselben Build-Tag-Set
  ist zulässig). Keine eigene HKDF-Implementation in diesem
  Paket — dasselbe HKDF-Pure-Go-Modul deckt Profil A und Profil B.
- **Lauf (geplant):** `make spike-profil-b-test` für die Compile-/
  Mock-Tests; `make spike-profil-b-bouncyhsm` und
  `make spike-profil-b-softhsm` für die HSM-Läufe.
- **Trace-Capture:** Aufrufe laufen mit `pkcs11-spy` als Wrapper;
  Output wandert nach `../trace/<modul>-profil-b.log` (siehe
  [trace/README.md](../trace/README.md)).

## Aktuelle Dateien

- `doc.go` — Paket-Doc, Build-Tag-Klammer.
- `fixture.go` — `FixtureIKM` + `HeaderHMACInfo` synchron zu
  [`../../002b-spike-hkdf/spike/fixture.go`](../../002b-spike-hkdf/spike/fixture.go).
  Wenn sich eine Konstante hier ändert, MUSS die Profil-A-Spike-
  Konstante synchron mitziehen — sonst weichen die Pure-Go-Referenz
  und der HSM-Tag voneinander ab.

## Geplant (CGO + HSM-Pfade)

- `extract.go` — `Extract(ctx, session, masterKey, salt) ([]byte,
  error)`. Führt `C_SignInit(CKM_SHA256_HMAC, master)` +
  `C_Sign(salt)` aus, liefert die 32-Byte-PRK in einem
  neu allokierten `[]byte`.
- `reimport.go` — `ReimportPRK(ctx, session, prk []byte) (
  pkcs11.ObjectHandle, error)`. `C_CreateObject` mit dem
  CKA-Template aus Spike-README §3 Punkt 2. Pfad (a) zuerst,
  Pfad (b) Vendor-Variante als Fallback wenn
  `CKR_TEMPLATE_INCONSISTENT`. **Zeroize:** der Aufrufer
  ist verpflichtet, `prk` unmittelbar nach Rückkehr aus
  dieser Funktion zu zeroizen — die Adapter-Funktion selbst
  ruft `C_CreateObject` und kehrt zurück, der Test/Use-Case
  setzt den Loop.
- `sign_b.go` — `SignHeaderProfileB(ctx, session, headerKey,
  headerBytes []byte) ([]byte, error)`. Identisch zu
  `SignHeaderHMAC` aus dem Profil-A-Spike (gleiche Operation),
  aber lokal definiert um Build-Tag-Sauberkeit zu wahren.
- `hsm_test.go` — End-to-End-Integrationstest. Skip wenn
  `SPIKE_PKCS11_MODULE` fehlt **oder** das Modul
  `CKM_SHA256_HMAC` nicht anbietet (sollte universell sein —
  ein Modul ohne SHA-256-HMAC wäre disqualifiziert für M1).
  Vergleicht den HSM-`C_Sign`-Output gegen
  `hkdfspike.ExpectedHeaderMAC` mit identischen Inputs.
  Zeroize-Check: PRK-Buffer ist nach dem `ReimportPRK`-Aufruf
  ausschließlich Null-Bytes.
