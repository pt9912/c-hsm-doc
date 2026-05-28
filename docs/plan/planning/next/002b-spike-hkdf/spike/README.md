# Spike-Probe-Code

**Status:** Pfad (a) Shim — `CK_HKDF_PARAMS`-Serialisierer + Pure-Go-Unit-Tests landed;
`C_DeriveKey`/`C_SignInit`-Aufrufpfade folgen mit dem ersten Spike-Lauf gegen
SoftHSM v2 + OpenCryptoki.
**Bezug:** [Spike-README](../README.md)

---

Hier landet der minimale Go-Code, der den HKDF-Profil-A-Pfad gegen
SoftHSM v2 und OpenCryptoki exerziert. Konventionen:

- **Build-Tag:** Jede Go-Datei trägt `//go:build spike` als erste Zeile.
  Der reguläre `go build ./...` und `make ci` sehen den Code nicht.
- **Paketname:** `package hkdfspike` (oder `package main` mit
  `cmd/<probe>/main.go`-Struktur, falls als ausführbares Binary
  geschnitten).
- **Kein Application-Code:** Imports beschränkt auf Standard-Library,
  `github.com/miekg/pkcs11` (Pfad a Shim) bzw. Fork-Pfad (Pfad b) und
  `golang.org/x/crypto/hkdf` ausschließlich für den Test-Fixture-
  Vergleich. Keine Imports aus `internal/hexagon/**` oder
  `internal/adapter/**`.
- **Lauf:** `make spike-hkdf-test` (Docker-only,
  [ADR 0002](../../../../adr/0002-docker-only-build-pipeline.md)) führt
  die Pure-Go-Unit-Tests des Serialisierers aus. Ein zweites Target
  (`make spike-hkdf-run` o. ä.) für den HSM-gestützten Lauf gegen
  SoftHSM v2 + OpenCryptoki wird mit dem ersten `C_DeriveKey`-Probe-Commit
  angelegt.
- **Trace-Capture:** Aufrufe laufen mit `pkcs11-spy` als Wrapper; der
  Spy-Output wandert nach `../trace/<modul>-<pfad>.log` (siehe
  [trace/README.md](../trace/README.md)).

## Aktuelle Dateien

Pure-Go (Pfad a Shim + HKDF-Referenz):

- `doc.go` — Paket-Doc, Build-Tag-Klammer.
- `fixture.go` — `FixtureIKM` (32 Byte, niemals produktiv) +
  `HeaderHMACInfo` (HSM-FMT-006 Profil A Info-String). CI-Init-
  Skripte importieren das Fixture-IKM ins HSM (siehe
  [`../README.md` §3 Punkt 5](../README.md)).
- `mechanism.go` — `CK_HKDF_PARAMS`-Serialisierer (LP64/LE Layout,
  Konstanten aus PKCS#11 v3.0 §6.30/§6.31, Salt-Type- und
  Info-Pointer-Validierung).
- `mechanism_test.go` — Hex-Dump-Referenzwert-Test + Layout-Asserts
  + Salt-/Info-Validierungs-Tests + Mechanism-Literal-Guards.
- `verify.go` — Pure-Go-HKDF-Referenz (`golang.org/x/crypto/hkdf`)
  mit `DeriveHeaderKey` + `ExpectedHeaderMAC`. Liefert den
  Vergleichswert, gegen den der HSM-`C_Sign`-Output validiert wird.
- `verify_test.go` — RFC-5869-A.1-Testvektor für die HKDF-Stufe
  + Determinismus-/Salt-Sensitivity-/Längen-Tests.

CGO (HSM-Aufrufpfade gegen `miekg/pkcs11`):

- `connect.go` — `LoadModule`, `Close`, `FindTokenSlot`,
  `LoginUser`, `FindSecretKey`, `HasMechanism`-Pre-Flight-Check.
- `derive.go` — `DeriveHeaderKeyHSM` mit `C_DeriveKey` +
  `CKM_HKDF_DERIVE` + `CK_HKDF_PARAMS`-Shim, C-Memory für
  `pSalt`/`pInfo`, Template mit `CKA_VALUE_LEN=32`.
- `sign.go` — `SignHeaderHMAC` mit `C_SignInit`+`C_Sign`+
  `CKM_SHA256_HMAC`.
- `hsm_test.go` — End-to-End-Integrationstest. Skippt sauber, wenn
  `SPIKE_PKCS11_MODULE` fehlt **oder** das Modul `CKM_HKDF_DERIVE`
  nicht anbietet (Spike-Befund §6.1: SoftHSM 2.x + OpenCryptoki
  haben den Mechanismus nicht). Vergleicht den HSM-`C_Sign`-Output
  gegen `ExpectedHeaderMAC`.
