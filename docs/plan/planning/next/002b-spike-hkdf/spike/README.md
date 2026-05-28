# Spike-Probe-Code (Platzhalter)

**Status:** leer (Probe-Code folgt im nächsten Schritt)
**Bezug:** [Spike-README](../README.md)

---

Hier landet der minimale Go-Code, der den HKDF-Profil-A-Pfad gegen
SoftHSM v2 und OpenCryptoki exerziert. Konventionen:

- **Build-Tag:** Jede Go-Datei trägt `//go:build spike` als erste Zeile.
  Der reguläre `go build ./...` und `make ci` sehen den Code nicht.
- **Paketname:** `package hkdfspike` (oder `package main` mit
  `cmd/<probe>/main.go`-Struktur, falls als ausführbares Binary
  geschnitten).
- **Kein Application-Code:** Imports beschränkt auf Standard-Library +
  `github.com/miekg/pkcs11` (Pfad a Shim) bzw. Fork-Pfad (Pfad b).
  Keine Imports aus `internal/hexagon/**` oder `internal/adapter/**`.
- **Lauf:** `make spike-hkdf` (Docker-only, [ADR 0002](../../../../adr/0002-docker-only-build-pipeline.md)).
  Das Make-Target wird mit dem ersten Probe-Code-Commit zusammen
  angelegt; bis dahin gibt es noch keinen Lauf-Befehl.
- **Trace-Capture:** Aufrufe laufen mit `pkcs11-spy` als Wrapper; der
  Spy-Output wandert nach `../trace/<modul>-<pfad>.log` (siehe
  [trace/README.md](../trace/README.md)).

## Geplante Dateien

- `mechanism.go` — `CK_HKDF_PARAMS`-Serialisierer (Pfad a) mit Hex-Dump-
  Referenzwert-Unit-Test.
- `derive.go` — `C_DeriveKey`-Aufruf mit `CKM_HKDF_DERIVE` und
  Template-Setzen (`CKA_EXTRACTABLE=false`, `CKA_SIGN=true`, …).
- `sign.go` — `C_SignInit`/`C_Sign`-Roundtrip mit `CKM_SHA256_HMAC` auf
  dem abgeleiteten Header-Key-Handle.
- `verify.go` — Vergleich gegen Pure-Go-HKDF-Referenz
  (`golang.org/x/crypto/hkdf`).
- `main_test.go` — Tabellengetriebene Tests pro Modul (Modulpfad als
  Env-Variable: `SPIKE_PKCS11_MODULE`, `SPIKE_PKCS11_TOKEN`,
  `SPIKE_PKCS11_PIN`, `SPIKE_MASTER_HMAC_LABEL`).
