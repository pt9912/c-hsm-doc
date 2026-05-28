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
[HSM-FMT-006 §1 Profil B](../../../../../spec/spezifikation.md):
**zwei HMAC-SHA-256-Schritte** rekonstruieren HKDF, **danach**
folgt der eigentliche Header-HMAC. Insgesamt drei HMAC-Operationen,
zwei Re-Imports, zwei Zeroize-Loops:

1. **HKDF-Extract** = `C_SignInit(CKM_SHA256_HMAC, master)` +
   `C_Sign(salt)` → PRK als 32-Byte-Klartext im Server-RAM.
   `defer zeroize(prk)` setzt **sofort** nach `C_Sign` ein
   (Owner-Vertrag, siehe §3 Punkt 3).
2. **PRK-Re-Import** = `C_CreateObject(CKK_GENERIC_SECRET,
   CKA_VALUE=PRK, CKA_SIGN=true, CKA_TOKEN=false,
   CKA_EXTRACTABLE=false, CKA_SENSITIVE=true,
   CKA_MODIFIABLE=false)` → `prkHandle`.
3. **HKDF-Expand** = `C_SignInit(CKM_SHA256_HMAC, prkHandle)` +
   `C_Sign(info || 0x01)` → Header-Key als 32-Byte-Klartext im
   Server-RAM. `defer zeroize(headerKey)` setzt **sofort** nach
   `C_Sign` ein. **Wichtig:** `info || 0x01` ist die einzelne
   HKDF-Expand-Iteration für L=32 (ein Output-Block); Bytes
   sind die UTF-8-`HeaderHMACInfo`-Bytes gefolgt vom Counter-
   Byte `0x01`.
4. **Header-Key-Re-Import** = `C_CreateObject(CKK_GENERIC_SECRET,
   CKA_VALUE=headerKey, …)` → `headerKeyHandle`. PRK-Handle
   kann jetzt zerstört werden (`C_DestroyObject(prkHandle)`).
5. **Header-HMAC** = `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)`
   + `C_Sign(headerBytes)` → 32-Byte-Tag.
6. **Cleanup:** `C_DestroyObject(headerKeyHandle)`.

Beide Klartext-Werte (PRK + Header-Key) leben mikrosekunden im
Server-RAM. Spec-Anforderung „weder PRK noch Header-Key verlässt
das HSM" ist nur erfüllt, wenn **beide** Zeroize-Loops greifen
(Doppel-Pflicht).

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
3. **Zeroize-Ownership (Owner = Extract/Expand-Funktion,
   Pattern = `defer`):** Sowohl `Extract(ctx, session, masterKey,
   salt)` als auch `Expand(ctx, session, prkHandle, info)` setzen
   `defer zeroize(buf)` **unmittelbar** nach `C_Sign` und **vor**
   dem `return`. Der Aufrufer sieht den Klartext nie über die
   Funktionsgrenze hinaus. Spike-Test injiziert eine Mock-Funktion,
   die einen Klartext-Snapshot zwischen `C_Sign` und Zeroize
   abgreift; der Snapshot muss nach `return` Null-Bytes zeigen.
   Kein Logging, kein Trace, kein temp-File des PRK- oder
   Header-Key-Werts (Code-Review- + `gosec`-Gate).
4. **Header-HMAC:** `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)`
   + `C_Sign(headerBytes)` liefert einen 32-Byte-HMAC. Roundtrip:
   zweimal mit identischem Input liefert byteweise identischen
   Output (Determinismus).
5. **Pure-Go-Vergleich (profilspezifisch):** Der HSM-`C_Sign`-
   Output stimmt byteweise mit
   `ExpectedHeaderMACProfileB(FixtureIKM, salt, info, headerBytes)`
   aus einer **eigenen** `verify_b.go` im `profilbspike`-Paket
   überein. Die Funktion implementiert **die Spec-Konstruktion**
   `HMAC(HMAC(HMAC(IKM, salt), info||0x01), headerBytes)` —
   das ist **nicht** identisch mit
   `hkdfspike.ExpectedHeaderMAC` (welches RFC-5869-HKDF nutzt).
   **Cross-Profil-Vergleich:** Profil-A-Output (RFC-5869-HKDF)
   und Profil-B-Output (HMAC-Konstruktion gemäß Spec) liefern
   **unterschiedliche** Header-Keys; ein Container, der mit
   Profil A erzeugt wurde, kann nicht mit Profil B verifiziert
   werden (siehe ADR 0007 §2.1 Korrektur, folgt im selben PR
   wie diese Spike-Korrektur).
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
│        verify_b.go, hsm_test.go)
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

**Pure-Go-Referenz profilspezifisch:** Der Profil-B-Pure-Go-Vergleich
lebt in einer eigenen `verify_b.go` im `profilbspike`-Paket. Sie
implementiert die Spec-Konstruktion
`HMAC(HMAC(HMAC(IKM, salt), info||0x01), headerBytes)` — eigene
Reference, **kein Re-Use** der RFC-5869-`ExpectedHeaderMAC` aus
`hkdfspike`. Profil A und Profil B liefern unterschiedliche
Header-Keys, weil die Spec-Konstruktion `HMAC(IKM, salt)` mit
vertauschten Argumenten gegenüber RFC-5869-HKDF-Extract arbeitet
(siehe §3 Punkt 5 + ADR 0007 §2.1 Korrektur). Damit entfällt auch
ein Cross-Spike-Import — die `verify.go`-Funktion aus
`hkdfspike` wird im Profil-B-Spike **nicht** verwendet.

---

## 5. Vorgehen

1. **Pure-Go-Referenz `verify_b.go`:** Eigene Funktion
   `ExpectedHeaderMACProfileB(ikm, salt, info, headerBytes []byte)
   []byte` mit drei nested `hmac.Sum` aus `crypto/hmac`. Test
   gegen einen festen Hex-Dump (mit dem Fixture-IKM, einem festen
   Salt und festen headerBytes — Snapshot beim ersten Lauf
   eingefroren). **Kein** Import aus `hkdfspike`.
2. **Extract gegen beide Module testen:** CGO-Helper
   `Extract(ctx, session, masterKey, salt []byte) ([]byte, error)`,
   der `C_SignInit(CKM_SHA256_HMAC, masterKey)` + `C_Sign(salt)`
   ausführt. **Zeroize-Owner:** der Helper setzt `defer
   zeroize(buf)` **unmittelbar** nach `C_Sign` und gibt eine
   Kopie des PRK an den Aufrufer zurück, die er selbst auch
   wieder zeroizen muss. Erst der `Reimport`-Helper macht den
   PRK „endgültig flüchtig" durch sofortiges `C_CreateObject`.
3. **Expand gegen beide Module testen:** CGO-Helper
   `Expand(ctx, session, prkHandle, info []byte) ([]byte, error)`,
   der `C_SignInit(CKM_SHA256_HMAC, prkHandle)` +
   `C_Sign(info || 0x01)` ausführt. Wieder
   `defer zeroize(buf)`-Pattern. Aufrufer übergibt das Ergebnis
   an `ReimportHeaderKey`, der `C_CreateObject` ruft und das
   ursprüngliche `[]byte` zeroizen lässt.
4. **Re-Import gegen beide Module:**
   `ReimportPRK(ctx, session, prk []byte) (handle, error)` und
   `ReimportHeaderKey(ctx, session, hk []byte) (handle, error)`
   rufen `C_CreateObject` mit dem PKCS#11-Template. Pfad (a)
   zuerst; bei `CKR_TEMPLATE_INCONSISTENT` automatisch (b)
   probieren und pro Modul protokollieren. **Direkt nach
   Rückkehr** aus diesen Funktionen wird der Klartext-Buffer
   vom Aufrufer zeroized (oder besser: die `Extract`/`Expand`-
   Funktion gibt einen Buffer zurück, dessen `defer`-Zeroize
   am Stack-Frame-Ende greift, also nachdem der Re-Import
   abgeschlossen ist).
5. **Header-HMAC + Vergleich:**
   `SignHeader(ctx, session, headerKeyHandle, headerBytes []byte)`
   ruft `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)` +
   `C_Sign(headerBytes)`. Vergleich gegen
   `ExpectedHeaderMACProfileB(FixtureIKM, salt,
   []byte(HeaderHMACInfo), headerBytes)` aus `verify_b.go`.
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
