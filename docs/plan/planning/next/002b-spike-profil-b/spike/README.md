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
  CGO-PKCS#11-Pfade. **Pure-Go-Referenz wiederverwendet:**
  `hkdfspike.ExpectedHeaderMAC` aus dem Profil-A-Spike-Paket
  ([`../../002b-spike-hkdf/spike/verify.go`](../../002b-spike-hkdf/spike/verify.go))
  liefert den Vergleichswert. Spec-konforme Profil-B-Konstruktion
  ist identisch zu RFC-5869-HKDF; beide Profile produzieren
  denselben Header-Key ([ADR 0008 §2.1](../../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)).
  Cross-Spike-Import ist zulässig — `cgo`-Build-Tag verengt die
  Build-Bedingungen, hindert den Import von Pure-Go-Funktionen
  aus dem breiteren Tag-Set aber nicht.
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
  salt []byte) ([]byte, error)`. Realisiert die spec-konforme
  Extract-Stufe `HMAC(salt, IKM)`. Die genaue PKCS#11-Aufruffolge
  wird pro Modul erkundet ([ADR 0008 §2.2](../../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)):
  Vendor-HKDF-Mechanismus (`CKM_NSS_HKDF`,
  `CKM_SP800_108_COUNTER_KDF`, …), Salt-as-Key-Pattern via
  `C_DeriveKey`, oder Modul-Disqualifikation für Profil B.
  **Zeroize-Owner-Vertrag ([ADR 0008 §2.3](../../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)):**
  `defer zeroize(buf)` steht unmittelbar nach dem HSM-Aufruf;
  Helper ist alleiniger Owner. Aufrufer ruft `ReimportPRK` direkt
  mit der zurückgegebenen Kopie — kein zusätzliches Zeroize.
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
  Fallback bei `CKR_TEMPLATE_INCONSISTENT`. **Kein Zeroize:**
  Owner bleibt der `Extract`/`Expand`-Helper über das
  `defer`-Pattern; doppeltes Zeroize ist verboten
  ([ADR 0008 §2.3](../../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)).
- `sign_b.go` — `SignHeader(ctx, session, headerKeyHandle
  pkcs11.ObjectHandle, headerBytes []byte) ([]byte, error)`.
  `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)` +
  `C_Sign(headerBytes)`.
- `hsm_test.go` — End-to-End-Integrationstest. Skip wenn
  `SPIKE_PKCS11_MODULE` fehlt **oder** das Modul keinen
  erkundeten Extract-Pfad bietet (siehe `extract.go`).
  Vergleicht den HSM-`C_Sign`-Output gegen
  `hkdfspike.ExpectedHeaderMAC` (Cross-Profil-identisch zum
  Profil-A-Output). Zeroize-Check: Mock-Hook zwischen `C_Sign`
  und Zeroize abgreifen, nach Rückkehr aus dem Helper muss der
  Buffer null sein.
