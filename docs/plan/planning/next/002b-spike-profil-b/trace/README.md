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
5. `C_SignInit` mit `CKM_SHA256_HMAC` + `masterKey`. **HKDF-Extract,
   Phase 1.**
6. `C_Sign` mit `salt` als Daten → liefert 32-Byte-`PRK` als
   Klartext im Server-RAM. **1 oder 2 Aufrufe** zulässig
   (miekg-Two-Call-Wrapper).
7. `C_CreateObject` mit Template `(CKA_CLASS=CKO_SECRET_KEY,
   CKA_KEY_TYPE=CKK_GENERIC_SECRET, CKA_VALUE=PRK, CKA_SIGN=true,
   CKA_TOKEN=false, CKA_EXTRACTABLE=false, CKA_SENSITIVE=true,
   CKA_MODIFIABLE=false)` → `prkHandle`. **PRK-Re-Import.**
8. **(Adapter-internal, kein PKCS#11-Call):** Der `Extract`-Helper
   gibt zurück, und sein `defer zeroize(prkBuf)` löscht den PRK-
   Buffer aus dem Go-Heap. Der Trace zeigt dafür nichts — Invariante
   wird im Adapter-Unit-Test mit Mock-Hook geprüft (siehe
   [Spike-README §3 Punkt 3](../README.md)).
9. `C_GetAttributeValue` auf `prkHandle` (Verifikation
   `CKA_EXTRACTABLE=false`, `CKA_SIGN=true`, `CKA_SENSITIVE=true`).
   **1 oder 2 Aufrufe** zulässig.
10. `C_GetAttributeValue` mit `CKA_VALUE` auf `prkHandle` →
    erwartete Antwort `CKR_ATTRIBUTE_SENSITIVE` (Spike-Erfolgs-
    Kriterium §3 Punkt 6, erste Hälfte).
11. `C_SignInit` mit `CKM_SHA256_HMAC` + `prkHandle`. **HKDF-Expand,
    Phase 1.**
12. `C_Sign` mit `info || 0x01` als Daten → liefert 32-Byte-
    `headerKey` als Klartext im Server-RAM. **1 oder 2 Aufrufe**.
13. `C_CreateObject` mit demselben Template wie Schritt 7, aber
    `CKA_VALUE=headerKey` → `headerKeyHandle`. **Header-Key-Re-Import.**
14. **(Adapter-internal):** `defer zeroize(headerKeyBuf)` löscht
    den Header-Key-Buffer aus dem Go-Heap (zweite Hälfte der
    Zeroize-Pflicht-Invariante).
15. `C_DestroyObject(prkHandle)` — PRK-Handle wird nicht mehr
    gebraucht. **Exakt 1 Aufruf.**
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

Abweichungen (z. B. fehlendes Zeroize zwischen Schritt 7 und 9,
fehlendes Zweit-Zeroize zwischen Schritt 13 und 16, ein
`C_GetAttributeValue(CKA_VALUE)`-Erfolg statt
`CKR_ATTRIBUTE_SENSITIVE`, oder eine `C_CreateObject`-Wiederholung
nach `CKR_TEMPLATE_INCONSISTENT`) sind Spike-Befunde und gehören
in §6 „Ergebnis" der Spike-README — Pfad (b) Vendor-Variante wird
genau dort dokumentiert.

## Profil-A-vs-Profil-B-Diff (Trace-Ebene)

Zwischen Profil-A-Spike und Profil-B-Spike unterscheiden sich:

- **Schritte 5–14 (Profil B) ↔ Schritt 5 (Profil A):**
  Ein `C_DeriveKey(CKM_HKDF_DERIVE, …)`-Aufruf in Profil A wird in
  Profil B durch zwei `C_Sign`-Aufrufe (Extract, Expand) plus zwei
  `C_CreateObject`-Aufrufe (PRK-Re-Import, Header-Key-Re-Import)
  ersetzt — sechs PKCS#11-Calls statt einem. Zwei Klartext-Buffer
  (`PRK`, `headerKey`) leben mikrosekunden im Server-Heap.
- **Cross-Profil-Identität bleibt erhalten.** Spec-konforme
  Profil-B-Konstruktion (`HMAC(salt, IKM)` für Extract, RFC-5869-
  HKDF mit L=32 ein Expand-Block) liefert denselben Header-Key
  wie Profil A. Container sind cross-profil-verifizierbar — ein
  Profil-A-Container kann mit Profil B gelesen werden und
  umgekehrt ([ADR 0008 §2.1](../../../../adr/0008-profil-b-spec-konstruktion-zeroize-owner.md)).
- **Sonst identisch:** Find/Login/Logout/Init/Finalize sind
  spiegelbar.
