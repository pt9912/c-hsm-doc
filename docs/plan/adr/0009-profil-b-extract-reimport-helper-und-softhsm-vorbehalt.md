# ADR 0009 — Profil-B-Extract/Expand-Reimport-Helper und SoftHSM-Vorbehalt

**Status:** Accepted
**Datum:** 2026-05-28
**Bezug:** [ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
(ADR-Immutabilität),
[ADR 0007](0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md)
(geschärft durch §2.2 dieser ADR — §2.2 SoftHSM-Pauschalaussage),
[ADR 0008](0008-profil-b-spec-konstruktion-zeroize-owner.md)
(geschärft durch §2.1 dieser ADR — §2.3 Zeroize-Owner-Vertrag),
[Spezifikation HSM-FMT-006](../../../spec/spezifikation.md),
[Slice 002b](../planning/next/002b-pkcs11-encrypt-hexagon.md),
[Spike 002b-Profil-B](../planning/next/002b-spike-profil-b/README.md)

---

## 1. Kontext

Zweites Review zu ADR 0007/0008 (2026-05-28) hat zwei Architektur-
Lücken und einen ADR-Drift gefunden:

### 1.1 Zeroize-Owner-Vertrag aus ADR 0008 §2.3 ist in Go nicht erfüllbar

ADR 0008 §2.3 sagt:

> Helper allokiert den Klartext-Buffer, ruft C_Sign, setzt
> **vor jedem weiteren Statement** ein `defer zeroize(buf)` und gibt
> eine **Kopie** an den Aufrufer zurück.

In Go ist beides gleichzeitig nicht machbar:

- Wenn `return buf` denselben Slice zurückgibt, läuft `defer
  zeroize(buf)` nach `return` — der Aufrufer bekommt ein
  Null-Slice.
- Wenn `return append([]byte{}, buf...)` eine Kopie zurückgibt,
  zeroized das Helper-`defer` nur das Original; die Kopie beim
  Aufrufer bleibt unbehandelt und hat keinen Owner-Vertrag.

Der einzige sichere Pfad ist, den Klartext **nie über die
Helper-Grenze** zu führen — der Helper macht `C_Sign` **und**
`C_CreateObject` selbst, gibt nur das HSM-Objekt-Handle zurück.

### 1.2 ADR 0007 §2.2 widerspricht ADR 0008 §2.2

ADR 0007 §2.2 sagt:

> Profil B nutzt ausschließlich `CKM_SHA256_HMAC` (universell in
> jedem ernsthaften PKCS#11-Modul). SoftHSM startet erfolgreich,
> Bouncy HSM startet erfolgreich, jedes M3-Vendor-HSM mit
> HMAC-SHA-256 startet erfolgreich.

ADR 0008 §2.2 sagt dagegen, dass die Realisierung von
`HMAC(salt, IKM)` mit nicht-extrahierbarem IKM nicht über
`CKM_SHA256_HMAC` alleine geht; SoftHSM ohne geeigneten
Vendor-/Derive-Pfad ist für Profil B **nicht freigegeben** —
Akzeptanz pro Modul ist Spike-Befund.

Beide ADRs sind `Accepted` und damit textlich unveränderlich.
Diese ADR 0009 zieht den Drift sichtbar und macht die ADR-0008-
Lesart zur maßgeblichen Fassung der SoftHSM-Frage.

### 1.3 Spike-Plan hatte alte Aufruffolge behalten

Der Profil-B-Spike-Plan beschrieb an mehreren Stellen weiterhin
`C_SignInit(masterKey) + C_Sign(salt)` als Extract — das ist
`HMAC(IKM, salt)`, nicht das spec-konforme `HMAC(salt, IKM)` aus
ADR 0008 §2.1. Die Plan-Pflege folgt dieser ADR (siehe §3
Konsequenzen).

---

## 2. Entscheidung

### 2.1 Helper-Schnitt: `ExtractAndReimportPRK` / `ExpandAndReimportHeaderKey`

Die in ADR 0008 §2.3 vorgesehenen Helper `Extract` + `ReimportPRK`
und `Expand` + `ReimportHeaderKey` werden durch **zwei einzelne
kombinierte Helper** ersetzt, die jeweils HSM-Aufruf + Re-Import
in **einer** Funktion bündeln und den Klartext **nie** über die
Funktionsgrenze führen:

```go
// ExtractAndReimportPRK realisiert HMAC(salt, IKM) auf dem
// nicht-extrahierbaren Master-Key und importiert den PRK direkt
// als nicht-extrahierbares HSM-Objekt zurück. Der Klartext-PRK
// lebt nur in dieser Funktion (defer zeroize läuft am
// Funktions-Stack-Frame-Ende, NACH dem C_CreateObject).
//
// Die genaue PKCS#11-Aufruffolge der HMAC(salt, IKM)-Stufe ist
// pro Modul Spike-Erkundungs-Material (ADR 0008 §2.2):
// Vendor-HKDF-Mechanismus, Salt-as-Key-Pattern via C_DeriveKey,
// oder Modul-Disqualifikation für Profil B.
func ExtractAndReimportPRK(
    ctx *pkcs11.Ctx,
    session pkcs11.SessionHandle,
    masterKey pkcs11.ObjectHandle,
    salt []byte,
) (prkHandle pkcs11.ObjectHandle, err error)

// ExpandAndReimportHeaderKey realisiert HMAC(PRK, info || 0x01)
// und importiert den Header-Key direkt als nicht-extrahierbares
// HSM-Objekt zurück. Selbes Zeroize-Pattern wie
// ExtractAndReimportPRK; Klartext lebt nur in dieser Funktion.
func ExpandAndReimportHeaderKey(
    ctx *pkcs11.Ctx,
    session pkcs11.SessionHandle,
    prkHandle pkcs11.ObjectHandle,
    info []byte,
) (headerKeyHandle pkcs11.ObjectHandle, err error)
```

**Owner-Vertrag (verbindlich):**

- Helper allokiert den Klartext-Buffer in einem lokalen `[]byte`.
- **Unmittelbar** nach dem HSM-Aufruf, der den Klartext zurückgibt,
  steht `defer zeroize(buf)` als nächstes Statement.
- Helper ruft `C_CreateObject(CKA_VALUE=buf, …)` und gibt **nur**
  das resultierende Object-Handle an den Aufrufer zurück.
- Der `defer`-Loop läuft am Helper-Stack-Frame-Ende, also **nach**
  dem `C_CreateObject` — der HSM hat die Bytes bereits in das
  importierte Objekt übernommen, der Klartext-Buffer wird sicher
  überschrieben, bevor irgend ein anderer Code-Pfad ihn sehen kann.
- Aufrufer hat **niemals** Zugriff auf den Klartext und damit
  auch keinen Zeroize-Pfad zu verantworten.

Damit ist der ADR-0008-§2.3-Vertrag in Go umsetzbar.

### 2.2 SoftHSM-Pauschalaussage aus ADR 0007 §2.2 zurückgenommen

ADR 0007 §2.2 behauptete pauschal: „SoftHSM startet erfolgreich".
Diese Aussage ist nach ADR 0008 §2.2 nicht mehr haltbar — Spec-
konforme Realisation von `HMAC(salt, IKM)` mit nicht-extrahierbarem
IKM braucht einen Vendor- oder `C_DeriveKey`-Pfad, den SoftHSM
2.6.1/2.7.0 **nicht** im Standard-Repertoire hat.

Maßgebliche Fassung der SoftHSM-Tragfähigkeit für Profil B ist
ab dieser ADR die **Spike-Befund-Position**:

- SoftHSM ist für Profil B freigegeben, **wenn und nur wenn** der
  Profil-B-Spike (Slice 002b §Vorbedingung 4) eine spec-konforme
  Realisation von `HMAC(salt, IKM)` gegen SoftHSM findet
  (Vendor-Mechanismus oder Salt-as-Key-Pattern).
- Findet der Spike keinen solchen Pfad, ist SoftHSM für Profil B
  **nicht** freigegeben — Service-Start verlangt dann ein
  alternatives Modul mit einem Profil-B-tauglichen Mechanismus,
  oder Profil A (Bouncy HSM).
- `HSM-FA-HSM-001` ist damit **nicht** durch ADR 0007 §2.2
  geschlossen; die Akzeptanz hängt am Spike-Erkundungs-Befund.

ADR 0007 §2.2-Text bleibt nach [ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
unverändert. Der ADR-Index trägt ADR 0009 als Schärfung von
ADR 0007 in der „Schärfungen"-Spalte ein.

### 2.3 ADR 0008 §2.3 bleibt textlich, Helper-Schnitt aus §2.1 dieser ADR ist maßgeblich

ADR 0008 §2.3 bleibt nach [ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
textlich erhalten. Die in §2.3 von ADR 0008 beschriebene
„Helper gibt eine Kopie zurück + defer zeroize im Helper"-
Konstruktion ist in Go nicht erfüllbar; maßgeblich ist ab
sofort der kombinierte Helper-Schnitt aus §2.1 dieser ADR.
Der ADR-Index trägt ADR 0009 als Schärfung von ADR 0008 ein.

---

## 3. Konsequenzen

- **Spike-Plan + Slice-Plan korrigieren:** Die Helper-Signaturen
  `Extract` / `Reimport*` / `Expand` aus den vorherigen Plan-
  Fassungen werden durch `ExtractAndReimportPRK` /
  `ExpandAndReimportHeaderKey` ersetzt. Die separate
  `reimport.go`-Datei aus dem Spike-Layout entfällt — Re-Import
  passiert in den Extract/Expand-Helpern.
- **Spike-Aufrufsequenz (PKCS#11-Trace):** §Erfolgs-Kriterien und
  trace-README dürfen die `HMAC(salt, IKM)`-Realisation nicht
  mehr als `C_SignInit(masterKey) + C_Sign(salt)` festschreiben.
  Die Realisation wird pro Modul erkundet (ADR 0008 §2.2); der
  Trace-README zeigt die nominelle Aufruf-Familie + Hinweis
  „pro Modul zu konkretisieren".
- **SoftHSM-Status klargestellt:** Spike-Plan §1 + Slice-Plan
  §HeaderMAC-Port-Profil-B-Block markieren SoftHSM-Akzeptanz als
  Spike-Befund-abhängig statt pauschal grün. `HSM-FA-HSM-001`
  bleibt offener M1-Akzeptanzpunkt, gemeinsam mit dem Profil-A-
  Pfad aus ADR 0006.
- **Code-Review-Akzeptanz:** Adapter-Code unter
  `internal/adapter/driven/pkcs11/headermac/profile_b.go` MUSS
  die Helper-Signaturen aus §2.1 exakt verwenden. Ein Adapter,
  der separate `Extract` + `ReimportPRK`-Funktionen exportiert,
  ist Akzeptanz-Verletzung.
- **ADR-Index trägt zwei Schärfungs-Beziehungen:** ADR 0009 →
  ADR 0007 (SoftHSM-Pauschalaussage) und ADR 0009 → ADR 0008
  (Zeroize-Owner-Vertrag).

---

## 4. Pflege-Regeln

- Eine spätere Erweiterung der Helper-Signatur (z. B. zusätzlicher
  Parameter für Vendor-Mechanismus-Wahl) ist additiv zulässig und
  braucht keine Folge-ADR, solange der Klartext-nie-zurückgeben-
  Vertrag erhalten bleibt. Eine Aufspaltung in
  Extract + ReimportPRK separat ist **nicht** zulässig — eigene
  Folge-ADR nötig.
- Wenn der Spike-Befund eine Salt-as-Key-Realisation findet, die
  ein temporäres salt-Objekt im HSM importiert, ist die
  Lifecycle-Pflicht des salt-Objekts (`C_DestroyObject` nach
  Extract) im Adapter-Code zu garantieren — `defer
  ctx.DestroyObject(session, saltHandle)` analog zum
  Zeroize-Pattern.
- SoftHSM bleibt unter Beobachtung; sollte eine künftige SoftHSM-
  Version `CKM_HKDF_DERIVE` oder einen geeigneten Derive-Pfad
  bekommen, wird das im Modul-Inventar (ADR 0004 + ADR 0006)
  gepflegt und Profil B auf SoftHSM nachträglich freigegeben.

---

## 5. Nicht Gegenstand dieser ADR

- **Spike-Probe-Code** (`extract_reimport.go`,
  `expand_reimport.go`, `sign_b.go`, `hsm_test.go`) — folgt im
  nächsten Inkrement; diese ADR fixiert nur die Helper-Schnitt-
  Architektur und den SoftHSM-Vorbehalt.
- **Bouncy-HSM-Status für Profil B:** Bouncy HSM hat
  `CKM_HKDF_DERIVE` (siehe Spike 002b-HKDF §6.1) und damit per
  Definition einen geeigneten Pfad für `HMAC(salt, IKM)`. Profil B
  auf Bouncy HSM ist deshalb nicht spike-abhängig und freigegeben.
- **Profil A** ist von dieser ADR nicht berührt. Bouncy HSM bleibt
  das Profil-A-CI-Modul gemäß ADR 0006.
