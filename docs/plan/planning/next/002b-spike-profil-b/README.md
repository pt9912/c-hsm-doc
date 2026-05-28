# Spike — HSM-FMT-006 Profil B (Software-HMAC-Konstruktion) auf SoftHSM + Bouncy HSM

**Status:** Plan-Konsolidierung abgeschlossen (2026-05-28) — Pfad-H/K-
Aufspaltung, Helper-Schnitt, SoftHSM-Vorbehalt und Templates fixiert
in [ADR 0007 / 0008 / 0009 / 0010](../../../adr/). Sub-Verzeichnis +
Fixture stehen; Probe-Code (CGO-Pfade `extract_reimport.go`,
`expand_reimport.go`, `sign_b.go`, `hsm_test.go`) folgt.
**Datum:** 2026-05-28
**Bezug:**
[Slice 002b §Vorbedingung 4](../002b-pkcs11-encrypt-hexagon.md),
[ADR 0001 §2.4](../../../adr/0001-documentation-and-planning-structure.md),
[ADR 0005 §2.2](../../../adr/0005-planstruktur-open-trigger-und-spike-pattern.md),
[ADR 0006](../../../adr/0006-hkdf-profil-a-binding-und-bouncy-hsm.md),
[ADR 0007](../../../adr/0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md),
[Spezifikation HSM-FMT-006](../../../../../spec/spezifikation.md)

---

## 1. Zweck

Vorbedingung 4 für Slice 002b (eingeführt durch
[ADR 0007 §3](../../../adr/0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md))
schließen: Validieren, dass die Software-HMAC-Konstruktion aus
[HSM-FMT-006](../../../../../spec/spezifikation.md) §1 Profil B
gegen **beide CI-Module** läuft.

Profil B ist der M1-Default ([ADR 0007 §2.2](../../../adr/0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md));
`HSM-FA-HSM-001`-Akzeptanz hängt unmittelbar an diesem Pfad.
**SoftHSM-Status für Profil B ist Spike-Befund-abhängig
([ADR 0009 §2.2](../../../adr/0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md)):**
Die Spec-konforme `HMAC(salt, IKM)`-Realisierung mit nicht-
extrahierbarem IKM braucht einen Vendor-Mechanismus oder ein
Salt-as-Key-Pattern, das SoftHSM 2.6.1/2.7.0 nicht im Standard-
Repertoire hat. Spike erkundet pro Modul. Bouncy HSM hat
`CKM_HKDF_DERIVE` und ist damit per Definition geeignet
([ADR 0009 §5](../../../adr/0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md)).

Konkrete Konstruktion gemäß
[HSM-FMT-006 §1 Profil B](../../../../../spec/spezifikation.md)
(spec-konforme Argumentreihenfolge,
[ADR 0008 §2.1](../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)):

```
PRK        = HMAC-SHA256( salt, IKM )                  # Extract
header_key = HMAC-SHA256( PRK, info || 0x01 )          # Expand (L=32)
tag        = HMAC-SHA256( header_key, header_bytes )   # Header-HMAC
```

**Cross-Profil-Identität:** Diese Konstruktion ist identisch mit
RFC-5869-HKDF (L=32, ein Expand-Block). Profil A (natives
`CKM_HKDF_DERIVE`) und Profil B (zweistufige HMAC) liefern damit
**denselben** `header_key` über identische Inputs — der
Pure-Go-Vergleich nutzt
[`ExpectedHeaderMAC`](../002b-spike-hkdf/spike/verify.go) aus dem
Profil-A-Spike-Paket für beide Profile.

PKCS#11-Aufrufgruppen — die Helper aus
[ADR 0009 §2.1](../../../adr/0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md)
mit verbindlicher Pfad-Aufspaltung aus
[ADR 0010 §2.1](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md):

1. **`ExtractAndReimportPRK`** liefert `prkHandle`. Pfad-Wahl
   modulabhängig:
   - **Pfad H — Native-Derive (Bouncy HSM):**
     `C_DeriveKey(CKM_HKDF_DERIVE, extractParams, masterKey,
     prkTemplate)` → `prkHandle` direkt. **Kein Klartext-PRK
     im Server-RAM, kein `C_CreateObject`, kein
     `defer zeroize`.**
   - **Pfad K — Klartext-Reimport (modulabhängig,
     Compliance-Vorbehalt):** lokaler `prk := make([]byte, 32)`
     + `defer zeroize(prk)` + vendor-konforme
     `HMAC(salt, IKM)`-Erzeugung + internes `C_CreateObject(CKA_VALUE=
     prk, …)`. Nur zulässig, wenn der Modul-Pfad die Spec-Garantie
     „ohne Klartext-Export des Master-Materials" hält
     ([ADR 0010 §2.1 Pfad K](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)).
2. **`ExpandAndReimportHeaderKey`** liefert `headerKeyHandle`.
   Analoge Pfad-H/Pfad-K-Aufspaltung; bei Bouncy HSM via
   `CKM_HKDF_DERIVE` mit `prkHandle` als Base-Key.
   `C_DestroyObject(prkHandle)` direkt danach.
3. **Header-HMAC** = `C_SignInit(CKM_SHA256_HMAC,
   headerKeyHandle)` + `C_Sign(headerBytes)` → 32-Byte-Tag.
4. **Cleanup:** `C_DestroyObject(headerKeyHandle)`.

Auf Pfad H entstehen weder PRK- noch Header-Key-Klartext im
Server-RAM — alle Derivate leben ausschließlich im HSM. Auf
Pfad K leben beide Klartexte mikrosekunden innerhalb der
Helper-Funktionen und werden durch das `defer zeroize`-Pattern
gelöscht; Aufrufer sehen die Klartexte nie.

Slice 002b wird **nicht** nach `in-progress/` migriert, solange dieser
Spike nicht grün ist.

---

## 2. Geprüfte Pfade

Die zwei verbindlichen Implementierungs-Pfade aus
[ADR 0010 §2.1](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md);
der Spike erkundet pro CI-Modul, welcher Pfad realisierbar ist.

### Pfad H — Native-Derive (Handle direkt)

Helper rufen `C_DeriveKey(CKM_HKDF_DERIVE, …)` zweimal —
`bExtract=true, bExpand=false, masterKey` → `prkHandle`, dann
`bExtract=false, bExpand=true, prkHandle` → `headerKeyHandle`.
**Kein Klartext-PRK, kein Klartext-Header-Key, kein
`C_CreateObject(CKA_VALUE=…)`, kein `defer zeroize`.** Erfolgs-
kriterium: das HSM akzeptiert die zwei `C_DeriveKey`-Aufrufe mit
den jeweiligen Pfad-H-Templates aus
[ADR 0010 §2.1](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md);
der finale `C_Sign(headerBytes)`-Tag stimmt byteweise mit der
Pure-Go-Referenz überein.

**Bouncy HSM 2.x** ist über Pfad H verifiziert (Profil-A-Spike
hat `CKM_HKDF_DERIVE` bereits live belegt, siehe
[002b-spike-hkdf §6.2](../002b-spike-hkdf/README.md)).

### Pfad K — Klartext-Reimport (vendor-konform, Spike-Befund pro Modul)

Helper allokiert lokal einen 32-Byte-Klartext-Buffer, setzt
`defer zeroize(buf)`, erzeugt PRK / Header-Key über einen
modulabhängigen vendor-konformen Pfad (z. B. Vendor-Mechanismus,
der den Master als Base-Key nimmt und einen Klartext-Ableitung
liefert), und ruft `C_CreateObject(CKA_VALUE=buf, prkTemplate
bzw. headerKeyTemplate)`. **Nur zulässig**, wenn der Modul-
Befund die Nicht-Export-Garantie aus HSM-FMT-006 hält
([ADR 0010 §2.1 Pfad K](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)).
Reine Software-`HMAC(salt, IKM)`-Konstruktion mit Master-Export
ist verboten.

Bei `CKR_TEMPLATE_INCONSISTENT` oder
`CKR_ATTRIBUTE_VALUE_INVALID` versucht der Helper automatisch
Re-Import-Varianten (z. B. über `CKM_GENERIC_SECRET_KEY_GEN`
statt direktem `C_CreateObject`) und protokolliert die
erfolgreiche Variante pro Modul.

### Fallback-Eskalation

Findet der Spike für ein Modul **weder** Pfad H **noch** einen
vendor-konformen Pfad K, geht Slice 002b zurück in die Planung:
Profil-B-Implementierung wird auf das andere CI-Modul beschränkt,
oder Modul-Wechsel als eigene Folge-ADR. Bei SoftHSM ohne
realisierbaren Pfad ist `HSM-FA-HSM-001` betroffen (siehe
[ADR 0010 §2.3](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)).

---

## 3. Erfolgs-Kriterien

Der Spike gilt als **grün**, wenn alle Punkte gegen **beide CI-Module**
(SoftHSM v2 und Bouncy HSM 2.x) nachweisbar sind:

1. **`ExtractAndReimportPRK` liefert `prkHandle`** (pfad-spezifisch,
   siehe §2 + [ADR 0010 §2.1](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)):
   - **Pfad H:** ein `C_DeriveKey(CKM_HKDF_DERIVE,
     extractParams={bExtract=true, bExpand=false}, masterKey,
     prkTemplate)` → `prkHandle` direkt. Kein internes
     `C_CreateObject`.
   - **Pfad K:** vendor-konforme Klartext-PRK-Erzeugung +
     `C_CreateObject(CKK_GENERIC_SECRET, CKA_VALUE=PRK,
     prkTemplate)`.
   PRK-Template setzt `CKA_DERIVE=true`, `CKA_SIGN=false`,
   `CKA_TOKEN=false`, `CKA_EXTRACTABLE=false`,
   `CKA_SENSITIVE=true`, `CKA_MODIFIABLE=false` (siehe
   [ADR 0010 §2.1](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)).
   Die zugesicherten Bits werden über `C_GetAttributeValue` auf
   `prkHandle` verifiziert. Master ist der mit
   `softhsm.sh`/`bouncyhsm.sh` importierte nicht-extrahierbare
   `CKK_GENERIC_SECRET`-Key.
2. **`ExpandAndReimportHeaderKey` liefert `headerKeyHandle`**:
   - **Pfad H:** ein `C_DeriveKey(CKM_HKDF_DERIVE,
     expandParams={bExtract=false, bExpand=true}, prkHandle,
     headerKeyTemplate)` → `headerKeyHandle` direkt.
   - **Pfad K:** `C_SignInit(CKM_SHA256_HMAC, prkHandle)` +
     `C_Sign(info || 0x01)` + internes `C_CreateObject(…,
     CKA_VALUE=headerKey, headerKeyTemplate)`.
   Header-Key-Template setzt `CKA_DERIVE=false`,
   `CKA_SIGN=true`, restliche Bits wie `prkTemplate`.
3. **Zeroize-Owner-Vertrag (pfad-spezifisch,
   [ADR 0010 §2.2](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)):**
   - **Pfad H** allokiert keinen Klartext-Buffer; kein
     `defer zeroize` und kein Mock-Hook nötig. Spike-Test
     prüft, dass der Adapter-Code keine `make([]byte, 32)`/
     `C_Sign`/`C_CreateObject`-Sequenz im H-Pfad enthält
     (Code-Review-Akzeptanz).
   - **Pfad K** allokiert lokalen Klartext-Buffer; Helper setzt
     `defer zeroize(buf)` **unmittelbar** nach dem HSM-Aufruf,
     ruft anschließend `C_CreateObject(CKA_VALUE=buf, …)`. Der
     `defer`-Loop läuft am Funktions-Stack-Frame-Ende, also
     **nach** dem `C_CreateObject` — der HSM hat die Bytes
     bereits ins importierte Objekt übernommen, bevor der
     Klartext-Buffer überschrieben wird. Der Aufrufer sieht
     ausschließlich das Object-Handle (in Go ist die in
     [ADR 0008 §2.3](../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)
     vorgesehene Helper-gibt-Kopie-zurück-Konstruktion nicht
     sicher umsetzbar, siehe
     [ADR 0009 §1.1](../../../adr/0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md)).
     Spike-Test injiziert einen Mock-Hook zwischen
     Klartext-Erzeugung und `C_CreateObject`, greift den
     Klartext-Snapshot ab und prüft: nach Helper-Rückkehr ist
     der lokale Buffer null. Kein Logging, kein Trace, kein
     temp-File (Code-Review- + `gosec`-Gate).
4. **Header-HMAC:** `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)`
   + `C_Sign(headerBytes)` liefert einen 32-Byte-HMAC. Roundtrip:
   zweimal mit identischem Input liefert byteweise identischen
   Output (Determinismus).
5. **Pure-Go-Vergleich (cross-profil):** Der HSM-`C_Sign`-
   Output stimmt byteweise mit
   `ExpectedHeaderMAC(FixtureIKM, salt, []byte(HeaderHMACInfo),
   headerBytes)` aus
   [`../002b-spike-hkdf/spike/verify.go`](../002b-spike-hkdf/spike/verify.go)
   überein. Spec sagt `header_key = HKDF-SHA-256(...)` für beide
   Profile; die Profil-B-Konstruktion aus §1 ist HKDF mit L=32
   und einem Expand-Block, also identisch zum Profil-A-Output.
   Damit gilt Cross-Profil-Identität ([ADR 0008 §2.1](../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md))
   und ein Container, der mit Profil A erzeugt wurde, ist mit
   Profil B verifizierbar (und umgekehrt).
6. **Sensitive-Durchsetzung (beide Handles):** Sowohl auf
   `prkHandle` als auch auf `headerKeyHandle` liefert
   `C_GetAttributeValue(CKA_VALUE)` leeren Wert oder
   `CKR_ATTRIBUTE_SENSITIVE` (analog
   [Profil-A-Spike-Trace Schritt 7](../002b-spike-hkdf/trace/README.md)).
7. **Modul-spezifische Quirks:** Der Spike protokolliert pro
   Modul, welche Re-Import-Variante (a/b) verwendet wird, ob
   `CKR_TEMPLATE_INCONSISTENT`- oder
   `CKR_ATTRIBUTE_VALUE_INVALID`-Pfade getriggert wurden, und
   welche `CKK_*`/`CKM_*`-Werte aktiv sind. Ergebnis wandert in §6
   „Ergebnis" und in den Slice-002b-Plan §HeaderMAC-Port-Profil-B-
   Block.

---

## 4. Layout

```
docs/plan/planning/next/002b-spike-profil-b/
├── README.md             (dieser Plan; nach Lauf um §6 „Ergebnis" ergänzt)
├── spike/
│   ├── README.md         (Konventionen + Datei-Inventar)
│   ├── doc.go            (Paket-Doc, Build-Tag-Klammer)
│   ├── fixture.go        (FixtureIKM + HeaderHMACInfo, synchron zu 002b-spike-hkdf)
│   └── (geplant: extract_reimport.go, expand_reimport.go,
│        sign_b.go, hsm_test.go)
└── trace/
    └── README.md         (kanonische PKCS#11-Aufruffolge; single source of truth)
```

**Build-Tag-Isolation:** Aller Go-Code unter `spike/` trägt
`//go:build spike && cgo && (amd64 || arm64)`. Der reguläre Repo-
Build (`go build ./...`, `make ci`) sieht den Spike-Code nicht. Das
Profil-B-Spike-Paket heißt `profilbspike` (analog `hkdfspike`).

**Docker-only:** Alle Build- und Lauf-Aufrufe laufen über Docker
(ADR 0002). Die Setup-Skripte `ci/keys-init/{softhsm,bouncyhsm}.sh`
sind unverändert nutzbar — Profil B braucht nur den Master-HMAC-Key
(vorhanden), keine zusätzlichen Mechanismen.

**Pure-Go-Referenz wiederverwendet:** Spec-konforme Profil-B-
Konstruktion ist HKDF mit L=32 und einem Expand-Block, also
identisch zum Profil-A-Output. Der Spike importiert
`ExpectedHeaderMAC` aus
[`../002b-spike-hkdf/spike/verify.go`](../002b-spike-hkdf/spike/verify.go)
direkt aus dem `hkdfspike`-Paket. Cross-Spike-Import unter
demselben Build-Tag-Set (`spike && (amd64 || arm64)`-Subset von
`spike && cgo && (amd64 || arm64)`) ist zulässig — der `cgo`-Tag
verengt die Build-Bedingungen, hindert aber den Import von
Pure-Go-Funktionen aus dem breiteren Tag-Set nicht.

---

## 5. Vorgehen

1. **Pure-Go-Referenz wiederverwendet:**
   `hkdfspike.ExpectedHeaderMAC` aus dem Profil-A-Spike-Paket
   liefert den Vergleichswert. Spec-konforme Profil-B-Konstruktion
   ist identisch zu RFC-5869-HKDF (L=32, ein Expand-Block) —
   beide Profile produzieren denselben Header-Key
   ([ADR 0008 §2.1](../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)).
2. **`ExtractAndReimportPRK` — Spike-Erkundung pro Modul:**
   Helper-Signatur aus
   [ADR 0009 §2.1](../../../adr/0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md):
   `ExtractAndReimportPRK(ctx, session, masterKey, salt)
   (prkHandle, error)`. Interne Realisation modulabhängig
   (Pfad H/K aus
   [ADR 0010 §2.1](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)):
   - **Pfad H (Bouncy HSM):** ein `C_DeriveKey(CKM_HKDF_DERIVE,
     bExtract=true/bExpand=false, masterKey, prkTemplate)` →
     `prkHandle`. Kein internes `C_CreateObject`, kein
     Klartext-Buffer, kein `defer zeroize`.
   - **Pfad K (Spike-Erkundung pro Modul):** vendor-konforme
     Klartext-PRK-Erzeugung + internes `C_CreateObject(CKA_VALUE=
     PRK, prkTemplate)` + `defer zeroize(prkBuf)`. Findet der
     Spike keinen Pfad → Modul für Profil B nicht freigegeben.
3. **`ExpandAndReimportHeaderKey` gegen beide Module testen:**
   `ExpandAndReimportHeaderKey(ctx, session, prkHandle, info)
   (headerKeyHandle, error)`. Selbe Pfad-H/K-Aufspaltung:
   - **Pfad H:** ein `C_DeriveKey(CKM_HKDF_DERIVE,
     bExtract=false/bExpand=true, prkHandle, headerKeyTemplate)`
     → `headerKeyHandle`. Kein Klartext, kein Zeroize.
   - **Pfad K:** `C_SignInit(CKM_SHA256_HMAC, prkHandle)` +
     `C_Sign(info || 0x01)` + internes `C_CreateObject(CKA_VALUE=
     headerKey, headerKeyTemplate)` + `defer zeroize(headerKeyBuf)`.
   `C_DestroyObject(prkHandle)` direkt nach Rückkehr aus beiden
   Pfaden.
4. **Pfad-/Template-Quirks dokumentieren:** Bei Pfad K mit
   `CKR_TEMPLATE_INCONSISTENT` versuchen die Helper automatisch
   Re-Import-Varianten (z. B. `CKM_GENERIC_SECRET_KEY_GEN` statt
   direktem `C_CreateObject`) und protokollieren pro Modul,
   welche Variante erfolgreich war. **Kein eigener Reimport-
   Helper außerhalb von Extract/Expand** — siehe
   [ADR 0009 §3](../../../adr/0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md).
5. **Header-HMAC + Vergleich:**
   `SignHeader(ctx, session, headerKeyHandle, headerBytes []byte)`
   ruft `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)` +
   `C_Sign(headerBytes)`. Vergleich gegen
   `hkdfspike.ExpectedHeaderMAC(FixtureIKM, salt,
   []byte(HeaderHMACInfo), headerBytes)`.
6. **Sensitive-Negativtest + Cleanup:** auf beiden Handles
   (`prkHandle`, `headerKeyHandle`) und mit
   `C_DestroyObject` auf beiden.
7. **Make-Targets** (`make spike-profil-b-test`,
   `spike-profil-b-bouncyhsm`, `spike-profil-b-softhsm`) folgen
   im Probe-Code-Inkrement; analog `make spike-hkdf-bouncyhsm`.

---

## 6. Ergebnis

_(wird nach dem Spike-Lauf befüllt)_

Strukturvorlage je CI-Modul (SoftHSM v2, Bouncy HSM 2.x):

- **Spike-Datum:** YYYY-MM-DD
- **Modul:** `<modul-id>` (Modulpfad, Versionsstand, Mechanismus-
  Liste aus `C_GetMechanismList`)
- **Realisierter Pfad** (gemäß
  [ADR 0010 §2.1](../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)):
  Pfad H (native `C_DeriveKey(CKM_HKDF_DERIVE)` zweimal) |
  Pfad K (vendor-konformer Klartext-Reimport, Mechanismus-
  Liste) | kein Pfad (Modul nicht freigegeben)
- **Trace-Beleg:** Verweis auf `trace/<modul>-profil-b.log`
- **Templates:** PRK-Template + Header-Key-Template-Befund pro
  Modul (Attribute, abgelehnte Bits, `CKR_TEMPLATE_INCONSISTENT`-
  Quirks)
- **Modul-Freigabe-Status für Profil B:**
  - Freigegeben (Pfad H), oder
  - Freigegeben mit Pfad K + Compliance-Befund (Klartext-PRK-
    Erzeugung verletzt Nicht-Export-Garantie nicht), oder
  - Nicht freigegeben (kein Pfad realisierbar)
- **`HSM-FA-HSM-001`-Status:** Erfüllt (SoftHSM + Bouncy HSM
  beide für Profil B freigegeben) | offen (SoftHSM nicht
  freigegeben — Folge-ADR via Modul-Wechsel oder
  `HSM-LESE-004`-Lastenheft-Change)
- **Folge-ADR:** geplant nur, falls (a) der Modul-Freigabe-Status
  von SoftHSM offen bleibt, oder (b) ein neues Modul aufgenommen
  wird, oder (c) eine Lastenheft-Änderung den Akzeptanzpfad
  ändert
- **Roadmap-Update:** Slice-002b-Vorbedingung-4 abgehakt, Slice
  nach `in-progress/` migrierbar — wenn `HSM-FA-HSM-001` offen
  bleibt, blockiert das den M1-Closure (nicht die Slice-
  Aktivierung)

---

## 7. Lebenszyklus dieses Sub-Verzeichnisses

Gemäß [ADR 0005 §2.2](../../../adr/0005-planstruktur-open-trigger-und-spike-pattern.md):

- Sub-Verzeichnis wandert mit Slice 002b nach `in-progress/`
  (sobald der Spike grün ist), später nach `done/` (mit
  Slice-Closure).
- Beim Slice-Closure entscheidet die Closure-Notiz, ob das
  Sub-Verzeichnis archiviert (`docs/archive/`) oder gelöscht
  wird. Der Trace-Output hat Reproduktions-Wert (Modul-Quirks
  je Modul) und sollte tendenziell archiviert werden.
- Bei Fallback (Pfad c) bleibt das Sub-Verzeichnis bei Slice
  002b und wird mit dem nächsten Aktivierungsversuch wieder
  gefüllt — oder archiviert, falls Slice 002b grundlegend
  neu geschnitten wird (z. B. anderer Default-Profil-Pfad).
