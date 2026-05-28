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

- `extract_reimport.go` — `ExtractAndReimportPRK(ctx, session,
  masterKey pkcs11.ObjectHandle, salt []byte) (pkcs11.ObjectHandle,
  error)`. Helper-Signatur verbindlich aus
  [ADR 0009 §2.1](../../../../adr/0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md);
  interne Pfad-H/K-Aufspaltung aus
  [ADR 0010 §2.1](../../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md).
  Auswahl erfolgt beim Aufbau aus der `C_GetMechanismList`-Liste:
  - **Pfad H (Bouncy HSM, `CKM_HKDF_DERIVE` verfügbar):** ein
    `C_DeriveKey(CKM_HKDF_DERIVE, extractParams={bExtract=true,
    bExpand=false, salt, info=nil}, masterKey, prkTemplate)`.
    Kein Klartext-Buffer, kein internes `C_CreateObject`, kein
    `defer zeroize`. PRK-Template setzt `CKA_DERIVE=true`,
    `CKA_SIGN=false` (siehe ADR 0010 §2.1).
  - **Pfad K (Modul ohne HKDF-Derive, vendor-konformer Klartext-
    Pfad):** lokaler `prk := make([]byte, 32)` + `defer zeroize(prk)`
    + vendor-konforme `HMAC(salt, IKM)`-Erzeugung + internes
    `C_CreateObject(CKA_VALUE=prk, prkTemplate)`. Aufrufer sieht
    ausschließlich `prkHandle` und kann den Klartext nirgends
    abgreifen.
- `expand_reimport.go` — `ExpandAndReimportHeaderKey(ctx, session,
  prkHandle pkcs11.ObjectHandle, info []byte) (pkcs11.ObjectHandle,
  error)`. Selbes Pfad-H/K-Pattern:
  - **Pfad H:** ein `C_DeriveKey(CKM_HKDF_DERIVE,
    expandParams={bExtract=false, bExpand=true, salt=nil, info},
    prkHandle, headerKeyTemplate)`. Header-Key-Template setzt
    `CKA_SIGN=true`, `CKA_DERIVE=false`.
  - **Pfad K:** `C_SignInit(CKM_SHA256_HMAC, prkHandle)` +
    `C_Sign(info || 0x01)` + internes `C_CreateObject(
    CKA_VALUE=headerKey, headerKeyTemplate)` mit demselben
    `defer zeroize`-Pattern wie in `ExtractAndReimportPRK` Pfad K.
  Aufrufer ist verpflichtet, `C_DestroyObject(prkHandle)` nach
  Rückkehr auszuführen (für beide Pfade).
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
