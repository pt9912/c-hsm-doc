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
- **Imports:** Standard-Library (insbesondere `crypto/hmac` +
  `crypto/sha256` für die Profil-B-Pure-Go-Referenz) +
  `github.com/miekg/pkcs11` für CGO-PKCS#11-Pfade.
  **Eigene Pure-Go-Referenz** in `verify_b.go` — die Profil-B-
  Spec-Konstruktion (`HMAC(HMAC(HMAC(IKM, salt), info||0x01),
  headerBytes)`) ist nicht identisch mit RFC-5869-HKDF, deshalb
  kein Re-Use des `hkdfspike.ExpectedHeaderMAC`. Cross-Spike-
  Import bleibt verboten.
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

- `extract.go` — `Extract(ctx, session, masterKey pkcs11.ObjectHandle,
  salt []byte) ([]byte, error)`. Führt
  `C_SignInit(CKM_SHA256_HMAC, masterKey)` + `C_Sign(salt)` aus.
  **Zeroize-Owner-Vertrag:** `defer zeroize(buf)` unmittelbar nach
  `C_Sign`; die Funktion gibt eine Kopie an den Aufrufer zurück,
  der seinerseits `defer zeroize(prkCopy)` setzt. Damit ist der
  Klartext nirgends „naked" über mehrere Funktionsaufrufe hinweg.
- `expand.go` — `Expand(ctx, session, prkHandle pkcs11.ObjectHandle,
  info []byte) ([]byte, error)`. Führt
  `C_SignInit(CKM_SHA256_HMAC, prkHandle)` + `C_Sign(info || 0x01)`
  aus. Selbes Zeroize-Owner-Pattern wie `Extract`.
- `reimport.go` — `ReimportPRK(ctx, session, prk []byte) (
  pkcs11.ObjectHandle, error)` und
  `ReimportHeaderKey(ctx, session, hk []byte) (pkcs11.ObjectHandle,
  error)`. Beide rufen `C_CreateObject` mit dem CKA-Template aus
  Spike-README §3 Punkt 1 (für PRK) bzw. §3 Punkt 2 (für
  Header-Key). Pfad (a) zuerst, Pfad (b) Vendor-Variante als
  Fallback bei `CKR_TEMPLATE_INCONSISTENT`. **Zeroize:** der
  übergebene Buffer wird in dieser Funktion **nicht** zeroized —
  Owner bleibt die `Extract`/`Expand`-Funktion über das
  `defer`-Pattern.
- `sign_b.go` — `SignHeader(ctx, session, headerKeyHandle
  pkcs11.ObjectHandle, headerBytes []byte) ([]byte, error)`.
  `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)` +
  `C_Sign(headerBytes)`.
- `verify_b.go` — `ExpectedHeaderMACProfileB(ikm, salt, info,
  headerBytes []byte) []byte`. Drei nested `hmac.New(sha256.New,
  …)` aus `crypto/hmac`. Test gegen RFC-5869-A.1 ist
  **nicht** sinnvoll, weil die Konstruktion abweicht — stattdessen
  ein Snapshot-Test mit eingefrorenen Hex-Werten.
- `hsm_test.go` — End-to-End-Integrationstest. Skip wenn
  `SPIKE_PKCS11_MODULE` fehlt (jedes ernsthafte HSM mit
  `CKM_SHA256_HMAC` qualifiziert). Vergleicht den HSM-`C_Sign`-
  Output gegen `ExpectedHeaderMACProfileB`. Zeroize-Check:
  Mock-Funktion zwischen `C_Sign` und Zeroize abgreifen, nach
  `return` der Helper-Funktion muss der Buffer null sein.
