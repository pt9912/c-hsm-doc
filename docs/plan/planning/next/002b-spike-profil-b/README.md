# Spike — HSM-FMT-006 Profil B (Software-HMAC-Konstruktion) auf SoftHSM + Bouncy HSM

**Status:** geplant (Sub-Verzeichnis angelegt, Probe-Code folgt)
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
SoftHSM bricht im Profil-A-Pfad am Mechanism-Check ab ([Spike 002b-HKDF §6.1](../002b-spike-hkdf/README.md)),
trägt aber Profil B vollständig — `CKM_SHA256_HMAC` ist universell.

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

PKCS#11-Aufruffolge — drei HMAC-Operationen, zwei Re-Imports, zwei
`defer zeroize`-Loops über die Helper-Funktionen aus
[ADR 0008 §2.3](../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md):

1. **Extract** = `HMAC-SHA256(salt, IKM)` realisiert per
   PKCS#11-Aufruf, der `salt` als HMAC-Key und `IKM` als Daten
   nutzt. Die genaue Realisierung ist Spike-Erkundungs-Material
   ([ADR 0008 §2.2](../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)):
   Vendor-HKDF-Mechanismus, Salt-as-Key-Pattern via `C_DeriveKey`,
   oder Modul-Disqualifikation für Profil B. Output: PRK als
   32-Byte-Klartext im Server-RAM. `defer zeroize(prk)` setzt
   **sofort** nach Rückkehr aus `C_Sign`/`C_DeriveKey` ein.
2. **PRK-Re-Import** = `C_CreateObject(CKK_GENERIC_SECRET,
   CKA_VALUE=PRK, CKA_SIGN=true, CKA_TOKEN=false,
   CKA_EXTRACTABLE=false, CKA_SENSITIVE=true,
   CKA_MODIFIABLE=false)` → `prkHandle`. Aufrufer übergibt den
   `Extract`-Output direkt; der Helper-`defer`-Loop läuft am
   Stack-Frame-Ende.
3. **Expand** = `HMAC-SHA256(PRK, info || 0x01)` realisiert per
   `C_SignInit(CKM_SHA256_HMAC, prkHandle)` + `C_Sign(info ||
   0x01)`. Output: Header-Key als 32-Byte-Klartext. `defer
   zeroize(headerKey)` setzt **sofort** nach `C_Sign` ein.
4. **Header-Key-Re-Import** = `C_CreateObject(…, CKA_VALUE=
   headerKey, …)` → `headerKeyHandle`. `C_DestroyObject(prkHandle)`
   direkt danach.
5. **Header-HMAC** = `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)`
   + `C_Sign(headerBytes)` → 32-Byte-Tag.
6. **Cleanup:** `C_DestroyObject(headerKeyHandle)`.

Beide Klartext-Werte (PRK + Header-Key) leben mikrosekunden im
Server-RAM und werden durch das `defer`-Pattern in `Extract`/
`Expand` zeroized — keine Owner-Konflikte, Error-Pfad-fest.

Slice 002b wird **nicht** nach `in-progress/` migriert, solange dieser
Spike nicht grün ist.

---

## 2. Geprüfte Pfade

### (a) PRK-Re-Import als Header-Key (Hauptweg)

Standard-Konstruktion gemäß HSM-FMT-006 §1 Profil B. Erfolgskriterium:
beide CI-Module akzeptieren `C_CreateObject` mit `CKA_VALUE=PRK` für
ein `CKK_GENERIC_SECRET` und stellen `CKA_SENSITIVE=true`/
`CKA_EXTRACTABLE=false` durch. Der abgeleitete `C_Sign`-Tag muss
byteweise mit der Pure-Go-Referenz übereinstimmen.

### (b) Modul-spezifischer Re-Import-Mechanismus (Variante)

Falls (a) auf einem Modul mit `CKR_TEMPLATE_INCONSISTENT` oder
`CKR_ATTRIBUTE_VALUE_INVALID` scheitert: Re-Import über
`CKM_GENERIC_SECRET_KEY_GEN` mit `CKA_VALUE`-Befüllung statt
direktem `C_CreateObject`. Modul-spezifisches Quirk-Pattern wird
pro Modul dokumentiert (Slice-Plan §HeaderMAC-Port-Profil-B
„Modul-spezifische Quirks").

### (c) Fallback-Eskalation

Schlagen (a) und (b) auf SoftHSM oder Bouncy HSM fehl, geht Slice
002b zurück in die Planung: entweder Profil-B-Implementation auf das
betroffene Modul beschränken, oder Modul-Wechsel als eigene Folge-ADR.
Lastenheft-Akzeptanz `HSM-FA-HSM-001` ist betroffen.

---

## 3. Erfolgs-Kriterien

Der Spike gilt als **grün**, wenn alle Punkte gegen **beide CI-Module**
(SoftHSM v2 und Bouncy HSM 2.x) nachweisbar sind:

1. **Extract + PRK-Re-Import:** `C_SignInit(CKM_SHA256_HMAC, master)`
   + `C_Sign(salt)` liefert genau 32 Byte (HMAC-SHA-256-Tag-Länge);
   anschließend `C_CreateObject(CKK_GENERIC_SECRET, CKA_VALUE=PRK,
   CKA_SIGN=true, CKA_TOKEN=false, CKA_EXTRACTABLE=false,
   CKA_SENSITIVE=true, CKA_MODIFIABLE=false)` liefert ein gültiges
   `prkHandle`. Die zugesicherten Bits werden über
   `C_GetAttributeValue` verifiziert. Master ist der mit
   `softhsm.sh`/`bouncyhsm.sh` importierte nicht-extrahierbare
   `CKK_GENERIC_SECRET`-Key.
2. **Expand + Header-Key-Re-Import:**
   `C_SignInit(CKM_SHA256_HMAC, prkHandle)` +
   `C_Sign(info || 0x01)` liefert 32 Byte Header-Key-Klartext;
   `C_CreateObject(CKK_GENERIC_SECRET, CKA_VALUE=headerKey, …)`
   liefert `headerKeyHandle` (gleiche Attribut-Pflicht wie PRK).
3. **Zeroize-Owner-Vertrag = Helper-`defer`-Pattern
   ([ADR 0008 §2.3](../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)):**
   `Extract` und `Expand` sind die alleinigen Owner ihrer
   Klartext-Buffer. Beide allokieren den Buffer, rufen `C_Sign`
   und setzen `defer zeroize(buf)` **unmittelbar** danach, bevor
   sie eine Kopie an den Aufrufer zurückgeben. Der Aufrufer ruft
   `Reimport*` direkt mit dem zurückgegebenen Buffer; nach
   Rückkehr aus `Reimport*` läuft der `defer`-Loop am Stack-
   Frame-Ende des Helpers — kein Klartext überlebt die Helper-
   Grenze. Doppel-Zeroize im Aufrufer ist überflüssig (und
   verboten, weil es Eigentümerschaft verwischt). Spike-Test
   injiziert einen Mock-Hook zwischen `C_Sign` und Zeroize,
   greift einen Klartext-Snapshot ab und prüft: nach Rückkehr
   aus dem Helper ist der Buffer null. Kein Logging, kein
   Trace, kein temp-File (Code-Review- + `gosec`-Gate).
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
│   └── (geplant: extract.go, expand.go, reimport.go, sign_b.go,
│        hsm_test.go)
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
2. **Extract — Spike-Erkundung pro Modul:** CGO-Helper
   `Extract(ctx, session, masterKey, salt []byte) ([]byte,
   error)` realisiert `HMAC(salt, IKM)`. Die genaue
   PKCS#11-Aufruffolge ist pro Modul zu erkunden
   ([ADR 0008 §2.2](../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)):
   Vendor-HKDF-Mechanismus, Salt-as-Key-Pattern via `C_DeriveKey`,
   oder Modul-Disqualifikation. **Zeroize-Owner:** Helper setzt
   `defer zeroize(buf)` unmittelbar nach `C_Sign`/`C_DeriveKey`
   und gibt eine Kopie an den Aufrufer zurück.
3. **Expand gegen beide Module testen:** CGO-Helper
   `Expand(ctx, session, prkHandle, info []byte) ([]byte, error)`,
   `C_SignInit(CKM_SHA256_HMAC, prkHandle)` + `C_Sign(info ||
   0x01)`. Selbes `defer`-Pattern wie `Extract`.
4. **Re-Import gegen beide Module:**
   `ReimportPRK(ctx, session, prk []byte) (handle, error)` und
   `ReimportHeaderKey(ctx, session, hk []byte) (handle, error)`
   rufen `C_CreateObject` mit dem PKCS#11-Template. Pfad (a)
   zuerst; bei `CKR_TEMPLATE_INCONSISTENT` automatisch (b)
   probieren und pro Modul protokollieren. **Kein eigener
   Zeroize-Schritt:** der Klartext-Buffer wurde von
   `Extract`/`Expand` allokiert; deren `defer`-Loop greift am
   Helper-Stack-Frame-Ende nach Rückkehr aus `Reimport*`.
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

Strukturvorlage:

- **Spike-Datum:** YYYY-MM-DD
- **Geprüfte Pfade:** a (Re-Import) / b (Vendor-Variante) / c (Fallback) — je Modul
- **Gewählter Pfad:** a (Re-Import) | b (Vendor-Mechanismus auf Modul X) | c (Fallback)
- **Trace-Belege:** Verweis auf `trace/<modul>-profil-b.log`
- **Modul-spezifische Quirks:** Re-Import-Mechanismus pro Modul,
  Attribut-Template-Abweichungen, Mechanism-Listen-Befund
- **Folge-ADR:** geplant nur, falls Pfad (b) oder (c) gegangen
  wird — sonst reicht das Spike-README + Slice-Plan-Anpassung
- **Roadmap-Update:** Slice-002b-Vorbedingung-4 abgehakt, Slice
  nach `in-progress/` migrierbar

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
