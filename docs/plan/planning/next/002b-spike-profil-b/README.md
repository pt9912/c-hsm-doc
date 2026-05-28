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

Konkrete Konstruktion:

1. **Extract** = `C_SignInit(CKM_SHA256_HMAC, master)` +
   `C_Sign(salt)` → PRK als 32-Byte-Klartext im Server-RAM.
2. **Re-Import** = `C_CreateObject(CKK_GENERIC_SECRET,
   CKA_VALUE=PRK, CKA_SIGN=true, CKA_TOKEN=false,
   CKA_EXTRACTABLE=false, CKA_SENSITIVE=true)` → Header-Key-Handle.
3. **Zeroize** = PRK-`[]byte` wird **unmittelbar** nach
   `C_CreateObject` mit Null-Bytes überschrieben
   ([ADR 0007 §4](../../../adr/0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md)
   Pflicht-Invariante).
4. **Header-HMAC** = `C_SignInit(CKM_SHA256_HMAC, headerKey)` +
   `C_Sign(headerBytes)` → 32-Byte-Tag.

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

Der Spike gilt als **grün**, wenn alle sieben Punkte gegen
**beide CI-Module** (SoftHSM v2 und Bouncy HSM 2.x) nachweisbar sind:

1. **Extract:** `C_SignInit(CKM_SHA256_HMAC, master)` +
   `C_Sign(salt)` liefert genau 32 Byte (HMAC-SHA-256-Tag-Länge);
   master ist der mit `softhsm.sh`/`bouncyhsm.sh` importierte
   nicht-extrahierbare `CKK_GENERIC_SECRET`-Key.
2. **Re-Import:** `C_CreateObject(CKK_GENERIC_SECRET,
   CKA_VALUE=PRK, CKA_SIGN=true, CKA_TOKEN=false,
   CKA_EXTRACTABLE=false, CKA_SENSITIVE=true)` liefert ein
   gültiges Object-Handle. Die zugesicherten Bits werden über
   `C_GetAttributeValue` verifiziert.
3. **Zeroize:** Der PRK-`[]byte`-Slice ist nach dem
   `C_CreateObject` ausschließlich Null-Bytes. Spike-Test belegt das
   per `bytes.Equal(prk, make([]byte, 32))`. Code-Review-Akzeptanz:
   Zeroize-Loop steht **unmittelbar** nach `C_CreateObject`, vor
   jeder weiteren Anweisung.
4. **Header-HMAC:** `C_SignInit(CKM_SHA256_HMAC, headerKey)` +
   `C_Sign(headerBytes)` liefert einen 32-Byte-HMAC. Roundtrip:
   zweimal mit identischem Input liefert byteweise identischen
   Output (Determinismus).
5. **Pure-Go-Vergleich:** Der HSM-`C_Sign`-Output stimmt byteweise
   mit `ExpectedHeaderMAC(FixtureIKM, salt, info, headerBytes)` aus
   [`../002b-spike-hkdf/spike/verify.go`](../002b-spike-hkdf/spike/verify.go)
   überein. **Profil-A- und Profil-B-Pfade liefern denselben Tag**,
   weil beide HKDF-Extract+Expand über identisches Master-IKM +
   Salt + Info ausführen.
6. **Sensitive-Durchsetzung:** `C_GetAttributeValue(headerKey,
   CKA_VALUE)` liefert leeren Wert oder
   `CKR_ATTRIBUTE_SENSITIVE` (analog
   [Profil-A-Spike-Trace Schritt 7](../002b-spike-hkdf/trace/README.md)).
7. **Modul-spezifische Quirks:** Der Spike protokolliert pro
   Modul, welche Re-Import-Variante (a/b) verwendet wird, ob
   `CKR_BUFFER_TOO_SMALL`-/`CKR_ATTRIBUTE_VALUE_INVALID`-Pfade
   getriggert wurden, und welche `CKK_*`/`CKM_*`-Werte aktiv sind.
   Ergebnis wandert in §6 „Ergebnis" und in den Slice-002b-Plan
   §HeaderMAC-Port-Profil-B-Block.

---

## 4. Layout

```
docs/plan/planning/next/002b-spike-profil-b/
├── README.md             (dieser Plan; nach Lauf um §6 „Ergebnis" ergänzt)
├── spike/
│   ├── README.md         (Konventionen + Datei-Inventar)
│   ├── doc.go            (Paket-Doc, Build-Tag-Klammer)
│   ├── fixture.go        (FixtureIKM + HeaderHMACInfo, synchron zu 002b-spike-hkdf)
│   └── (geplant: extract.go, reimport.go, sign_b.go, hsm_test.go)
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

**Pure-Go-Referenz wiederverwendet:** `ExpectedHeaderMAC` aus
[`../002b-spike-hkdf/spike/verify.go`](../002b-spike-hkdf/spike/verify.go)
liefert den Vergleichswert. Profil A und Profil B berechnen
dasselbe HKDF-Ergebnis; der Spike importiert die Funktion direkt
aus dem `hkdfspike`-Paket (Cross-Spike-Import unter demselben
Build-Tag-Set ist zulässig, beide Pakete tragen `//go:build spike
&& cgo && (amd64 || arm64)`).

---

## 5. Vorgehen

1. **Pure-Go-Referenz nutzen:** `ExpectedHeaderMAC` aus dem
   Profil-A-Spike-Paket importieren; keine eigene HKDF-
   Implementation in `profilbspike`.
2. **Extract gegen beide Module testen:** kleiner CGO-Helper
   `Extract(ctx, session, masterKey, salt) (prk [32]byte, error)`,
   der `C_SignInit`/`C_Sign` auf dem Master-Key ausführt.
3. **Re-Import gegen beide Module:**
   `ReimportPRK(ctx, session, prk) (handle, error)` ruft
   `C_CreateObject` mit dem PKCS#11-Template. Pfad (a) zuerst;
   bei `CKR_TEMPLATE_INCONSISTENT` automatisch (b) probieren und
   pro Modul protokollieren.
4. **Zeroize-Pfad:** Spike-Code zerstört den PRK-Buffer
   **unmittelbar** nach `C_CreateObject` (vor jedem Logging oder
   sonstigen Code). Spike-Test liest den Buffer-Speicher nach
   dem Adapter-Aufruf und prüft, dass alle Bytes null sind.
5. **Header-HMAC + Vergleich:** `C_Sign(headerBytes)` auf dem
   Re-Importierten Header-Key, Vergleich gegen
   `ExpectedHeaderMAC(FixtureIKM, salt, info, headerBytes)`.
6. **Sensitive-Negativtest + Cleanup:** identisch zum
   Profil-A-Spike-Pattern.
7. **Make-Targets** (`make spike-profil-b-test`,
   `spike-profil-b-bouncyhsm`, `spike-profil-b-softhsm`) folgen
   im Probe-Code-Inkrement; analog
   `make spike-hkdf-bouncyhsm`.

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
