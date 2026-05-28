# Spike-Trace-Output (Profil B, Platzhalter)

**Status:** leer (Trace-Logs folgen mit dem Spike-Lauf)
**Bezug:** [Spike-README](../README.md),
[ADR 0007 §4](../../../../adr/0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md)

---

Hier landen die PKCS#11-Aufrufprotokolle pro Modul für den
Profil-B-Spike. Format und Capture-Mechanik analog zum
Profil-A-Spike ([`../../002b-spike-hkdf/trace/README.md`](../../002b-spike-hkdf/trace/README.md));
nur die kanonische Aufruffolge unterscheidet sich (siehe unten).

## Dateibenennung

`<modul>-profil-b.log`, jeweils klein geschrieben:

- `softhsm-profil-b.log` — SoftHSM v2.x, Profil-B-Konstruktion
- `bouncyhsm-profil-b.log` — Bouncy HSM 2.x, Profil-B-Konstruktion

## Capture-Mechanik

Wie im Profil-A-Spike-Trace: `pkcs11-spy` als `LD_PRELOAD`-Wrapper
im Spike-Docker-Container; Output über `PKCS11SPY_OUTPUT` in die
Trace-Datei umgeleitet. Header-Kommentar pro Datei
(`# c-hsm-doc spike 002b-profil-b`, Modulpfad, Datum,
Container-Image-Digest).

## Kanonische Aufruffolge (single source of truth)

Diese Sequenz ist der verbindliche Erfolgsmaßstab für den Profil-B-
Spike; [`../README.md` §3](../README.md) verweist hierhin statt eine
zweite, abweichende Liste zu führen. Pro Lauf in genau dieser
Reihenfolge:

1. `C_Initialize`
2. `C_OpenSession`
3. `C_Login`
4. `C_FindObjectsInit` + `C_FindObjects` + `C_FindObjectsFinal`
   (Master-HMAC-Lookup) — identisch zu Profil-A-Spike Schritt 4.
5. **`ExtractAndReimportPRK`-Aufrufgruppe (modulabhängig,
   [ADR 0010 §2.1](../../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)).**
   - **Pfad H (Bouncy HSM):** **genau ein** PKCS#11-Call:
     `C_DeriveKey(CKM_HKDF_DERIVE, extractParams, masterKey,
     prkTemplate)` → `prkHandle` direkt. Kein Klartext-PRK im
     Server-RAM. **Schritte 6 + 7 entfallen** (kein
     `C_CreateObject`, kein Zeroize).
   - **Pfad K (Spike-Erkundung pro Modul):** mindestens ein
     HSM-Aufruf, der `HMAC(salt, IKM)` realisiert, ohne das
     nicht-extrahierbare Master-Material zu exportieren —
     konkrete Aufruffolge ist Spike-Erkundungs-Material und
     wird im §6 Ergebnis pro Modul protokolliert. Output:
     32-Byte-PRK als Klartext **innerhalb** des
     `ExtractAndReimportPRK`-Helpers. Schritte 6 + 7 folgen.
   Findet der Spike für ein Modul keinen Pfad H und keinen
   vendor-konformen Pfad K → Modul ist für Profil B **nicht
   freigegeben**.
6. **(Nur Pfad K)** `C_CreateObject` mit Template
   `(CKA_CLASS=CKO_SECRET_KEY, CKA_KEY_TYPE=CKK_GENERIC_SECRET,
   CKA_VALUE=PRK, CKA_SIGN=true, CKA_TOKEN=false,
   CKA_EXTRACTABLE=false, CKA_SENSITIVE=true,
   CKA_MODIFIABLE=false)` → `prkHandle`. **PRK-Re-Import,
   weiterhin innerhalb des `ExtractAndReimportPRK`-Helpers.**
7. **(Nur Pfad K, Adapter-internal, kein PKCS#11-Call):** Der
   `defer zeroize(prkBuf)` läuft am Stack-Frame-Ende **nach**
   Schritt 6 und löscht den lokalen PRK-Buffer aus dem Go-Heap,
   **bevor** der Aufrufer das `prkHandle` zurückbekommt. Der
   Trace zeigt dafür nichts — Invariante wird im Adapter-
   Unit-Test mit Mock-Hook geprüft (siehe
   [Spike-README §3 Punkt 3](../README.md) +
   [ADR 0010 §2.2](../../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)).
9. `C_GetAttributeValue` auf `prkHandle` (Verifikation
   `CKA_EXTRACTABLE=false`, `CKA_SIGN=true`, `CKA_SENSITIVE=true`).
   **1 oder 2 Aufrufe** zulässig.
10. `C_GetAttributeValue` mit `CKA_VALUE` auf `prkHandle` →
    erwartete Antwort `CKR_ATTRIBUTE_SENSITIVE` (Spike-Erfolgs-
    Kriterium §3 Punkt 6, erste Hälfte).
11. **`ExpandAndReimportHeaderKey`-Aufrufgruppe (modulabhängig,
    [ADR 0010 §2.1](../../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)).**
    - **Pfad H (Bouncy HSM):** **genau ein** PKCS#11-Call:
      `C_DeriveKey(CKM_HKDF_DERIVE, expandParams, prkHandle,
      headerKeyTemplate)` mit `bExtract=false, bExpand=true`,
      `prkHandle` als Base-Key → `headerKeyHandle` direkt. Kein
      Klartext-Header-Key im Server-RAM. **Schritte 12 + 13 + 14
      entfallen** (kein `C_Sign`, kein `C_CreateObject`, kein
      Zeroize).
    - **Pfad K (Spike-Erkundung pro Modul):**
      `C_SignInit(CKM_SHA256_HMAC, prkHandle)` + `C_Sign(info || 0x01)`
      + internes `C_CreateObject` — die volle Klartext-Reimport-
      Sequenz (Schritte 12 + 13 + 14).
12. **(Nur Pfad K)** `C_Sign` mit `info || 0x01` als Daten →
    liefert 32-Byte-`headerKey` als Klartext im Server-RAM.
    **1 oder 2 Aufrufe**.
13. **(Nur Pfad K)** `C_CreateObject` mit demselben Template wie
    Schritt 6, aber `CKA_VALUE=headerKey` → `headerKeyHandle`.
    **Header-Key-Re-Import, weiterhin innerhalb des
    `ExpandAndReimportHeaderKey`-Helpers.**
14. **(Nur Pfad K, Adapter-internal):** `defer zeroize(headerKeyBuf)`
    aus dem Helper läuft am Stack-Frame-Ende nach Schritt 13 und
    löscht den Header-Key-Buffer aus dem Go-Heap, bevor der
    Aufrufer das `headerKeyHandle` zurückbekommt.
15. `C_DestroyObject(prkHandle)` — PRK-Handle wird nicht mehr
    gebraucht. **Exakt 1 Aufruf** (vom Aufrufer der Helper-
    Funktionen ausgelöst).
16. `C_GetAttributeValue` mit `CKA_VALUE` auf `headerKeyHandle` →
    erwartete Antwort `CKR_ATTRIBUTE_SENSITIVE` (zweite Hälfte
    Spike-Erfolgs-Kriterium §3 Punkt 6).
17. `C_SignInit` mit `CKM_SHA256_HMAC` + `headerKeyHandle`.
    **Header-HMAC, Phase 1.**
18. `C_Sign` mit `headerBytes` als Daten → liefert 32-Byte-
    Header-HMAC-Tag. Wert muss byteweise mit
    `hkdfspike.ExpectedHeaderMAC(FixtureIKM, salt,
    HeaderHMACInfo-Bytes, headerBytes)` aus dem Profil-A-Spike-
    Paket übereinstimmen — Spec-konforme Profil-B-Konstruktion
    liefert denselben HKDF-Output wie Profil A
    ([ADR 0008 §2.1](../../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)).
    Deckt [`../README.md` §3 Punkt 5](../README.md). **1 oder
    2 Aufrufe.**
19. `C_DestroyObject(headerKeyHandle)` — **exakt 1 Aufruf.**
20. `C_SignInit` mit `headerKeyHandle` → erwartete Antwort
    `CKR_OBJECT_HANDLE_INVALID` (analog Profil-A-Spike-Trace
    Schritt 11).
21. `C_Logout` + `C_CloseSession` + `C_Finalize`.

Abweichungen sind pfad-spezifisch zu lesen:

- **Auf Pfad H** sind Schritte 6–7 (PRK-Re-Import + Zeroize) und
  12–14 (Header-Key-Re-Import + Zeroize) **nicht zu erwarten** —
  ihr Auftauchen wäre Befund, dass der Helper fälschlich Pfad K
  fährt (Adapter-Bug, Code-Review-Akzeptanzverletzung). Erwartet
  wird stattdessen genau ein `C_DeriveKey` pro Stufe (Extract,
  Expand) ohne dazwischenliegendes `C_CreateObject`.
- **Auf Pfad K** ist fehlendes Zeroize zwischen Schritt 7 und 9
  bzw. zwischen 13 und 16 ein Bug (Zeroize-Owner-Vertrag aus
  [ADR 0010 §2.2](../../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)
  verletzt); ein `C_GetAttributeValue(CKA_VALUE)`-Erfolg statt
  `CKR_ATTRIBUTE_SENSITIVE` ist Spec-Verletzung; eine
  `C_CreateObject`-Wiederholung nach `CKR_TEMPLATE_INCONSISTENT`
  ist erwartete Pfad-K-Quirk-Erkundung.

Alle pfad-spezifischen Befunde gehören in §6 „Ergebnis" der
Spike-README; die pro Modul gewählte Variante (Pfad H, Pfad K
mit konkretem Vendor-Mechanismus, oder „nicht freigegeben")
wird genau dort dokumentiert.

## Profil-A-vs-Profil-B-Diff (Trace-Ebene)

Zwischen Profil-A-Spike und Profil-B-Spike unterscheiden sich
die Aufruffolgen pfad-spezifisch
([ADR 0010 §2.1](../../../../adr/0010-profil-b-helper-zwei-pfade-und-fa-hsm-001-status.md)):

- **Profil A (Pfad-a-Shim aus ADR 0006):**
  ein `C_DeriveKey(CKM_HKDF_DERIVE, params={bExtract=true,
  bExpand=true}, masterKey, headerKeyTemplate)` → finaler
  `headerKeyHandle` in einem Schritt. Kein Klartext im
  Server-RAM.
- **Profil B Pfad H (Bouncy HSM):** zwei `C_DeriveKey`-Aufrufe —
  `bExtract=true, bExpand=false, masterKey` → `prkHandle`, dann
  `bExtract=false, bExpand=true, prkHandle` → `headerKeyHandle`.
  Output ist identisch zu Profil A (RFC-5869-HKDF mit L=32),
  PKCS#11-Aufrufe sind zwei statt eins. Kein Klartext im
  Server-RAM. Templates wie in ADR 0010 §2.1 spezifiziert
  (PRK: `CKA_DERIVE=true`/`CKA_SIGN=false`; Header-Key:
  `CKA_DERIVE=false`/`CKA_SIGN=true`).
- **Profil B Pfad K (vendor-konformer Klartext-Reimport):**
  vendor-konformer PKCS#11-Pfad für Extract liefert
  Klartext-PRK → `C_CreateObject(CKA_VALUE=PRK)` →
  `C_SignInit/C_Sign` für Expand → `C_CreateObject(CKA_VALUE=
  headerKey)`. Vier PKCS#11-Calls plus zwei Klartext-Buffer
  (PRK, header_key), die durch `defer zeroize` in den Helpern
  geschützt werden.
- **Cross-Profil-Identität bleibt erhalten.** Spec-konforme
  Profil-B-Konstruktion (Extract = `HMAC(salt, IKM)`, Expand =
  `HMAC(PRK, info||0x01)`, L=32) liefert denselben Header-Key
  wie Profil A — über Pfad H und Pfad K gleichermaßen
  ([ADR 0008 §2.1](../../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)).
  Container sind cross-profil-verifizierbar.
- **Sonst identisch:** Find/Login/Logout/Init/Finalize sind
  spiegelbar.
