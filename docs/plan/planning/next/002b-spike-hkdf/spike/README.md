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

## Aktuelle Dateien (Pfad a Shim, Pure-Go)

- `doc.go` — Paket-Doc, Build-Tag-Klammer.
- `mechanism.go` — `CK_HKDF_PARAMS`-Serialisierer (LP64/LE Layout,
  Konstanten aus PKCS#11 v3.0 §6.30/§6.31, Salt-Type-Validierung).
- `mechanism_test.go` — Hex-Dump-Referenzwert-Test + Layout-Asserts
  + Salt-Type-Validierungs-Tests.

## Geplant (HSM-Aufrufpfade, folgen mit dem ersten Spike-Lauf)

- `derive.go` — `C_DeriveKey`-Aufruf mit `CKM_HKDF_DERIVE`, CGO-Memory
  für `pSalt`/`pInfo` (typisch `C.CBytes` + `C.free`), Template-Setzen
  (`CKA_EXTRACTABLE=false`, `CKA_SIGN=true`, …).
- `sign.go` — `C_SignInit`/`C_Sign`-Roundtrip mit `CKM_SHA256_HMAC` auf
  dem abgeleiteten Header-Key-Handle.
- `verify.go` — Vergleich gegen Pure-Go-HKDF-Referenz
  (`golang.org/x/crypto/hkdf`).
- `hsm_test.go` — Integrationstests pro Modul (Modulpfad als
  Env-Variable: `SPIKE_PKCS11_MODULE`, `SPIKE_PKCS11_TOKEN`,
  `SPIKE_PKCS11_PIN`, `SPIKE_MASTER_HMAC_LABEL`).
