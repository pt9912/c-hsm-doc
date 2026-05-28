# ADR 0010 — Profil-B-Helper-Zwei-Pfade und `HSM-FA-HSM-001`-Status

**Status:** Accepted
**Datum:** 2026-05-28
**Bezug:** [Lastenheft `HSM-FA-HSM-001`, `HSM-LESE-004`](../../../spec/lastenheft.md),
[Spezifikation HSM-FMT-006](../../../spec/spezifikation.md),
[ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
(ADR-Immutabilität),
[ADR 0006](0006-hkdf-profil-a-binding-und-bouncy-hsm.md),
[ADR 0007](0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md),
[ADR 0008](0008-profil-b-spec-konstruktion-zeroize-owner.md),
[ADR 0009](0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md)
(geschärft durch §2.1 + §3 dieser ADR — §2.1 Helper-Pfade
+ §3 `HSM-FA-HSM-001`-Status)

---

## 1. Kontext

Drittes Review (2026-05-28) zu Profil B hat zwei strukturelle
Lücken aufgedeckt:

### 1.1 ADR 0009 mischt Handle- und Klartext-Pfade

ADR 0009 §2.1 beschreibt `ExtractAndReimportPRK` als Helper, der
**immer** einen Klartext-PRK in einem lokalen `[]byte` allokiert,
`defer zeroize(buf)` setzt und `C_CreateObject(CKA_VALUE=PRK)`
aufruft.

ADR 0009 §2.2 nennt aber als Bouncy-HSM-Realisation den
`CKM_HKDF_DERIVE`-Vendor-Pfad. `C_DeriveKey(CKM_HKDF_DERIVE, …)`
liefert **kein** Klartext-PRK, sondern direkt ein
Object-Handle. Damit gilt:

- Entweder der Helper macht `C_DeriveKey` → Handle, dann gibt es
  **nichts zu zeroizen** und **nichts zu reimportieren**.
- Oder der Helper exportiert Klartext-PRK aus dem nicht-
  extrahierbaren Master-HMAC-Key, was die Nicht-Export-Garantie
  aus [HSM-FMT-006](../../../spec/spezifikation.md) berührt und
  pro Modul zu prüfen ist.

ADR 0009 mischt beide Pfade in einer Helper-Beschreibung. Diese
ADR 0010 trennt sie sauber.

### 1.2 `HSM-FA-HSM-001`-Status weiterhin unklar formuliert

Slice-Plan §HSM-FA-HSM-001 Vendor-Smoke sagt nach ADR 0009 noch
„bedingt erfüllt" und nennt „Bouncy-HSM-Doppel-Profil als
Akzeptanz-Ersatz mit Modul-Vorbehalt". Lastenheft
[`HSM-FA-HSM-001`](../../../spec/lastenheft.md) verlangt aber
explizit Start gegen **SoftHSM v2** **und** ein zweites
herstellerfremdes Modul ohne Codeänderung. Zwei verschiedene
Profile auf demselben Bouncy-HSM-Modul sind kein Ersatz für den
SoftHSM-Anteil — Lastenheft-Akzeptanz lässt das nicht zu.

`HSM-FA-HSM-001` ist damit **nicht erfüllt** und kann nur durch
einen der folgenden Pfade aufgelöst werden:

1. Spike-Befund findet eine spec-konforme Profil-B-Realisation
   gegen SoftHSM.
2. Modul-Tausch in der Lastenheft-Akzeptanz (HSM-LESE-004-Frage).

Die Sprachregelung „bedingt erfüllt" ist damit zu weich und wird
durch §3 dieser ADR zurückgenommen.

---

## 2. Entscheidung

### 2.1 Helper-Zwei-Pfade explizit getrennt

`ExtractAndReimportPRK` aus
[ADR 0009 §2.1](0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md)
bleibt Funktionsname und Helper-Signatur unverändert
(`(prkHandle pkcs11.ObjectHandle, err error)`), aber die interne
Realisierung wählt pro Modul **einen** von zwei Pfaden:

#### Pfad H — Native-Derive (Handle direkt)

Für Module mit einem HKDF-Derive-Mechanismus
(`CKM_HKDF_DERIVE`, `CKM_NSS_HKDF`,
`CKM_SP800_108_COUNTER_KDF` mit HKDF-Parametern, oder
modul-äquivalent):

```go
// Pfad H — keine Klartext-PRK-Allokation, kein C_CreateObject,
// kein defer zeroize. Helper ruft direkt C_DeriveKey und gibt
// das HSM-Handle zurück.
mech := []*pkcs11.Mechanism{
    pkcs11.NewMechanism(uint(CKM_HKDF_DERIVE), extractParams),
}
prkHandle, err := ctx.DeriveKey(session, mech, masterKey, prkTemplate)
return prkHandle, err
```

**Zwei getrennte Templates** für die zwei Pfad-H-Schritte
(`ExtractAndReimportPRK` → PRK; `ExpandAndReimportHeaderKey` →
Header-Key):

- **`prkTemplate` (Extract-Output, Base-Key für Expand):**
  `CKA_CLASS=CKO_SECRET_KEY`,
  `CKA_KEY_TYPE=CKK_GENERIC_SECRET`,
  **`CKA_DERIVE=true`** (PRK wird im nächsten `C_DeriveKey` als
  Base-Key verwendet),
  `CKA_SIGN=false`,
  `CKA_TOKEN=false`,
  `CKA_EXTRACTABLE=false`,
  `CKA_SENSITIVE=true`.
- **`headerKeyTemplate` (Expand-Output, finaler Header-HMAC-Key):**
  `CKA_CLASS=CKO_SECRET_KEY`,
  `CKA_KEY_TYPE=CKK_GENERIC_SECRET`,
  `CKA_DERIVE=false`,
  **`CKA_SIGN=true`** (Header-Key wird per
  `C_Sign(CKM_SHA256_HMAC)` über `headerBytes` aufgerufen),
  `CKA_TOKEN=false`,
  `CKA_EXTRACTABLE=false`,
  `CKA_SENSITIVE=true`.

Der PRK lebt ausschließlich im HSM — keine Spec-Berührung.

**Mechanismus-Liste Pfad H in M1:** `CKM_HKDF_DERIVE`. Bouncy HSM
2.x ist über diesen Pfad realisiert. Weitere HKDF-Derive-
Mechanismen aus der breiteren Liste (`CKM_NSS_HKDF`,
`CKM_SP800_108_COUNTER_KDF`, vendor-äquivalent) sind in M3 möglich
und werden dann pro Vendor-HSM in einer eigenen Folge-ADR-
Schärfung des Mechanism-Checks aktiviert.

#### Pfad K — Klartext-Reimport (nur unter Bedingungen zulässig)

Für Module ohne HKDF-Derive-Mechanismus ist der einzige
verbleibende Pfad eine vendor-konforme Realisation, die einen
Klartext-PRK in einem lokalen `[]byte` allokiert und per
`C_CreateObject(CKA_VALUE=PRK)` reimportiert:

```go
// Pfad K — nur zulässig, wenn der Klartext-PRK auf einem Pfad
// entsteht, der die HSM-FMT-006-Nicht-Export-Garantie nicht
// verletzt. Pro Modul Spike-Erkundungs- und Compliance-Befund.
prk := make([]byte, 32)
defer zeroize(prk)
// modulabhängige HMAC(salt, IKM)-Aufruffolge füllt prk
prkHandle, err := ctx.CreateObject(session, prkReimportTemplate(prk))
return prkHandle, err
```

**Pfad K ist nur dann zulässig, wenn der Klartext-PRK aus einem
modulinternen Mechanismus stammt, der das nicht-extrahierbare
Master-Material schützt** (z. B. ein Vendor-Mechanismus, der den
Master als Base-Key in `C_DeriveKey` nimmt und einen Klartext-
PRK in einem definierten Wrap-Schritt zurückgibt). Eine reine
Software-`HMAC(salt, IKM)`-Konstruktion, die das Master-Material
über `C_Sign` exportiert, ist **nicht** zulässig — Master ist
nicht-extrahierbar.

Findet der Spike für ein Modul keinen vendor-konformen Pfad K,
ist das Modul für Profil B **nicht freigegeben**. Die Adapter-
Funktion verweigert dann den Start mit
`STARTUP_HSM_MECHANISM_MISSING` (siehe Slice-Plan
§Startup-Fehlerklassen) und einer Profil-spezifischen Meldung.

#### Code-Layout-Konsequenz

Pfad H und Pfad K leben im selben `profile_b.go`. Die Auswahl
erfolgt beim Helper-Aufbau aus der Mechanismus-Liste des Moduls
(`C_GetMechanismList`); kein Vendor-String im Adapter-Code.

`profile_a.go` und der H-Pfad in `profile_b.go` führen denselben
PKCS#11-Aufruf (`C_DeriveKey(CKM_HKDF_DERIVE)`); das Refactoring
auf eine gemeinsame interne Funktion ist Slice-002b-
Implementierungs-Detail (siehe §4 Pflege).

### 2.2 `defer zeroize` ist Pfad-spezifisch

Die in
[ADR 0009 §2.1](0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md)
beschriebene Zeroize-Invariante gilt **nur** für Pfad K — Pfad H
allokiert keinen Klartext-Buffer und braucht kein `defer zeroize`.

Spike-Test ist analog gestaffelt:

- Pfad H: Test prüft, dass `C_DeriveKey` aufgerufen wird, dass
  das zurückgegebene Handle die geforderten CKA-Bits trägt, und
  dass kein lokaler Buffer existiert, der zeroized werden müsste
  (Adapter-Code-Review-Akzeptanz: keine `make([]byte, 32)` /
  `C_Sign` / `C_CreateObject`-Sequenz im H-Pfad).
- Pfad K: Test prüft Zeroize-Invariante wie in ADR 0009 §2.1
  beschrieben — Mock-Hook zwischen Klartext-Erzeugung und
  `C_CreateObject` greift den Buffer ab; nach Helper-Rückkehr
  ist der Buffer null.

### 2.3 `HSM-FA-HSM-001` ist nicht erfüllt — Sprachregelung verbindlich

Slice-Plan §Akzeptanz und alle abhängigen Dokumente verwenden ab
dieser ADR die folgende Sprachregelung:

> `HSM-FA-HSM-001` ist mit Slice 002b **nicht erfüllt**, solange
> kein vendor-konformer Pfad K (oder Pfad H) gegen SoftHSM v2
> nachgewiesen ist. Bouncy HSM allein trägt den
> Lastenheft-Anteil „zweites herstellerfremdes Modul", **ersetzt
> aber nicht** den SoftHSM-Anteil. Die Akzeptanz ist offen bis
> entweder der Profil-B-Spike gegen SoftHSM grün läuft, oder
> eine Lastenheft-Änderung über `HSM-LESE-004` die Akzeptanz
> umformuliert (z. B. „zwei beliebige PKCS#11-Module ohne
> Codeänderung").

Sprachregeln wie „bedingt erfüllt" oder „durch Bouncy-HSM-
Doppel-Profil mit Modul-Vorbehalt" entfallen — sie sind
sachlich unklar und werden im Slice-Plan-Akzeptanzblock
zurückgenommen.

### 2.4 ADR-Vorgänger bleiben textlich

ADR 0006 / ADR 0007 / ADR 0008 / ADR 0009 bleiben nach
[ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
textlich unverändert. Diese ADR 0010:

- schärft ADR 0009 §2.1 (Helper-Zwei-Pfade) und §3 (Status
  HSM-FA-HSM-001),
- zieht die ADR 0007 §2.2-Pauschalaussage zur SoftHSM-
  Tragfähigkeit endgültig in den Spike-Befund (ADR 0009 §2.2
  hatte sie bereits eröffnet),
- ergänzt den ADR-Index bei ADR 0006 + ADR 0009 mit
  Verweis auf 0010.

---

## 3. Konsequenzen

- **Spike-Plan korrigieren:** ExtractAndReimportPRK-Beschreibung
  splittet in Pfad H / Pfad K; trace-README §5-7 markiert
  modulabhängig „native Derive" vs. „Klartext-Reimport".
- **Slice-Plan §Akzeptanz HSM-FA-HSM-001:** „bedingt erfüllt"
  durch „nicht erfüllt — Profil-B-Spike + SoftHSM-Befund offen"
  ersetzen.
- **Slice-Plan §HeaderMAC-Port-Profil-B-Block:** Pfad-H- und
  Pfad-K-Verzweigung explizit.
- **ADR-Index:** ADR 0006 + ADR 0009 erhalten zusätzliche
  Schärfungs-Hinweise auf ADR 0010.
- **Code-Review-Akzeptanz** für Slice 002b: `profile_b.go`
  enthält klar getrennten H- und K-Pfad; die Auswahl folgt der
  Mechanismus-Liste, nicht einem Vendor-String. Pfad K darf
  nur dann gewählt werden, wenn der Modul-Befund einen
  vendor-konformen Klartext-PRK-Erzeugungspfad belegt.

---

## 4. Pflege-Regeln

- **`profile_a.go` und Pfad H aus `profile_b.go`** nutzen denselben
  PKCS#11-Mechanismus (`CKM_HKDF_DERIVE`), aber mit **pfad-
  spezifischen Parametern** — die Aufrufe sind nicht eins-zu-eins
  identisch:
  - **Profil A**: ein `C_DeriveKey` mit `bExtract=true,
    bExpand=true, masterKey` → finaler `headerKeyHandle` direkt
    (RFC-5869-HKDF in einem Schritt, L=32).
  - **Profil B Pfad H**: zwei separate `C_DeriveKey`-Aufrufe —
    Extract (`bExtract=true, bExpand=false, masterKey`) → `prkHandle`,
    dann Expand (`bExtract=false, bExpand=true, prkHandle`) →
    `headerKeyHandle`. Output ist über RFC-5869 identisch mit
    Profil A, die PKCS#11-Aufruffolge ist es nicht.

  Template-Attribute (`CKA_SIGN`, `CKA_EXTRACTABLE`,
  `CKA_SENSITIVE`, `CKA_TOKEN`) sind für das finale
  `headerKeyHandle` in beiden Profilen identisch.
  Eine interne Refactoring-Funktion (z. B. `deriveHKDFStep(extract,
  expand bool, baseKey pkcs11.ObjectHandle, template
  []*pkcs11.Attribute)`) ist additiv zulässig und braucht keine
  Folge-ADR, solange die pfad-spezifischen Parameter sauber
  durchgereicht werden. Die Helper-Signaturen aus
  [ADR 0009 §2.1](0009-profil-b-extract-reimport-helper-und-softhsm-vorbehalt.md)
  bleiben als die maßgebliche Adapter-API.
- Eine neue Modul-Akzeptanz für Pfad K erfordert eine eigene
  Folge-ADR, die den Compliance-Befund (Klartext-PRK-Erzeugung
  ohne Master-Export) explizit dokumentiert.
- Wenn `HSM-FA-HSM-001` über einen Lastenheft-Change geschlossen
  wird (HSM-LESE-004-Pfad), trägt eine eigene Folge-ADR das
  geänderte Akzeptanz-Mapping ein. Diese ADR 0010 selbst
  bleibt unverändert.

---

## 5. Nicht Gegenstand dieser ADR

- **Spike-Probe-Code** (extract_reimport.go, expand_reimport.go,
  hsm_test.go) — folgt im nächsten Inkrement; diese ADR fixiert
  nur die Pfad-Trennung und den HSM-FA-HSM-001-Status.
- **Pfad-C-Vendor-KDF** aus HSM-FMT-006 §1 bleibt M3-Scope.
- **Wire-Format-Änderungen** (Container-Header trägt Profil)
  sind nicht nötig — Pfad H und Pfad K liefern denselben
  `header_key` über RFC-5869-HKDF (siehe ADR 0008 §2.1).
- **Lastenheft-Änderung** über HSM-LESE-004 ist außerhalb dieser
  ADR; sie ist eine der zwei Optionen, um HSM-FA-HSM-001 zu
  schließen, aber kein ADR-Eingriff.
