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
5. `C_SignInit` mit `CKM_SHA256_HMAC` + masterKey-Handle.
   **HKDF-Extract-Phase Schritt 1 von 2.**
6. `C_Sign` mit `salt` als Daten → liefert 32-Byte-PRK als
   Klartext im Server-RAM. **1 oder 2 Aufrufe** zulässig
   (miekg/pkcs11.Ctx.Sign macht Two-Call-Wrapper; identisch
   zu Profil-A-Spike Schritt 9).
7. `C_CreateObject` mit Template
   `(CKA_CLASS=CKO_SECRET_KEY, CKA_KEY_TYPE=CKK_GENERIC_SECRET,
   CKA_VALUE=PRK, CKA_SIGN=true, CKA_TOKEN=false,
   CKA_EXTRACTABLE=false, CKA_SENSITIVE=true, CKA_MODIFIABLE=false)`
   → Header-Key-Handle. **HKDF-Expand + Re-Import-Phase.**
8. **(Adapter-internal, kein PKCS#11-Call):** PRK-`[]byte` wird
   unmittelbar nach Schritt 7 zeroized (`for i := range prk { prk[i]
   = 0 }`). Der Trace zeigt dafür nichts — die Invariante wird im
   Adapter-Unit-Test gegen ein Mock-PKCS#11-Modul belegt
   ([Spike-README §3 Punkt 3](../README.md)).
9. `C_GetAttributeValue` (Verifikation `CKA_EXTRACTABLE=false`,
   `CKA_SIGN=true`, `CKA_SENSITIVE=true`, `CKA_KEY_TYPE=
   CKK_GENERIC_SECRET`) — analog Profil-A-Spike Schritt 6.
   **1 oder 2 Aufrufe** zulässig (Two-Call-Wrapper-Pfad).
10. `C_GetAttributeValue` mit `CKA_VALUE` auf dem Header-Key-Handle
    → erwartete Antwort `CKR_ATTRIBUTE_SENSITIVE` (analog
    Profil-A-Spike Schritt 7; **erwarteter Fehler**). **1 oder 2
    Aufrufe** zulässig.
11. `C_SignInit` mit `CKM_SHA256_HMAC` + Header-Key-Handle.
    **Header-HMAC-Phase Schritt 1 von 2.**
12. `C_Sign` mit `headerBytes` als Daten → liefert 32-Byte-Header-
    HMAC-Tag. **1 oder 2 Aufrufe** zulässig (Two-Call-Wrapper).
    Der Wert muss byteweise mit `hkdfspike.ExpectedHeaderMAC(
    FixtureIKM, salt, info, headerBytes)` übereinstimmen
    (deckt [`../README.md` §3 Punkt 5](../README.md)).
13. `C_DestroyObject` (Header-Key-Handle) — **exakt 1 Aufruf**.
14. `C_SignInit` mit demselben Handle → erwartete Antwort
    `CKR_OBJECT_HANDLE_INVALID` (deckt [`../README.md` §3 Punkt
    6](../README.md); **erwarteter Fehler**).
15. `C_Logout` + `C_CloseSession` + `C_Finalize`.

Abweichungen (z. B. zusätzliche `C_FindObjects`-Aufrufe nach
Schritt 7, fehlender Zeroize-Check im Unit-Test, ein
`C_GetAttributeValue(CKA_VALUE)`-Erfolg statt
`CKR_ATTRIBUTE_SENSITIVE`, oder ein zweiter `C_CreateObject`-Aufruf
nach `CKR_TEMPLATE_INCONSISTENT`) sind Spike-Befunde und gehören
in §6 „Ergebnis" der Spike-README — Pfad (b) Vendor-Variante wird
genau dort dokumentiert.

## Profil-A-vs-Profil-B-Diff (Trace-Ebene)

Zwischen Profil-A-Spike und Profil-B-Spike unterscheiden sich:

- **Schritt 5/6 (Profil B) ↔ Schritt 5 (Profil A):**
  `C_DeriveKey(CKM_HKDF_DERIVE, …)` (1 Aufruf) wird in Profil B durch
  `C_SignInit(CKM_SHA256_HMAC, …)` + `C_Sign(salt)` (2 Aufrufe;
  Two-Call-Wrapper-Tolerant) + `C_CreateObject(…)` (Schritt 7)
  ersetzt.
- **Zwischen Schritt 7 und 9 (Profil B):** Adapter-internes Zeroize
  ohne PKCS#11-Sichtbarkeit. Im Profil A entfällt der Zeroize-Schritt
  vollständig (kein PRK im Server-RAM).
- **Sonst identisch:** Find/Login/Logout/Init/Finalize sind
  spiegelbar; Attribut- und Sensitive-Checks gleichen sich nach
  Schritt 7 (Profil B) bzw. Schritt 5 (Profil A) wieder an.
