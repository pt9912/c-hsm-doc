# Spike-Trace-Output (Platzhalter)

**Status:** leer (Trace-Logs folgen mit dem Spike-Lauf)
**Bezug:** [Spike-README](../README.md)

---

Hier landen die PKCS#11-Aufrufprotokolle pro Modul und Spike-Pfad
als reproduzierbarer Beleg, dass `CKM_HKDF_DERIVE` + `CKM_SHA256_HMAC`
in der erwarteten Aufruffolge laufen.

## Dateibenennung

`<modul>-<pfad>.log`, jeweils klein geschrieben:

- `softhsm-a.log` — SoftHSM v2, Pfad (a) Shim
- `softhsm-b.log` — SoftHSM v2, Pfad (b) Fork (nur falls Pfad a fehlschlägt)
- `opencryptoki-a.log` — OpenCryptoki, Pfad (a) Shim
- `opencryptoki-b.log` — OpenCryptoki, Pfad (b) Fork

## Capture-Mechanik

- **Primärquelle:** `pkcs11-spy` als `LD_PRELOAD`-Wrapper. Spy-Output
  wird per `PKCS11SPY_OUTPUT` in die Trace-Datei umgeleitet. Setup
  läuft im Spike-Docker-Container (kein Host-Eingriff).
- **Sekundärquelle (Diagnose):** Modul-eigene Logs
  (`/var/log/opencryptoki/`, SoftHSM `softhsm2.conf` `log.level=DEBUG`)
  werden bei Bedarf separat als `<modul>-<pfad>.modlog` abgelegt; sie
  sind nicht das primäre Akzeptanz-Artefakt.
- **Reproduzierbarkeit:** Jedes Trace-File startet mit einem
  Header-Kommentar (`# c-hsm-doc spike 002b-hkdf`, Modulpfad, Pfad,
  Datum, Container-Image-Digest). Damit ist die Aufzeichnung gegen
  eine konkrete CI-Umgebung verankert.

## Kanonische Aufruffolge (single source of truth)

Diese Sequenz ist der verbindliche Erfolgsmaßstab für den Spike;
[`../README.md` §3](../README.md) verweist hierhin statt eine
zweite, abweichende Liste zu führen. Pro Lauf in genau dieser
Reihenfolge:

1. `C_Initialize`
2. `C_OpenSession`
3. `C_Login`
4. `C_FindObjectsInit` + `C_FindObjects` + `C_FindObjectsFinal`
   (Master-HMAC-Lookup)
5. `C_DeriveKey` mit `CKM_HKDF_DERIVE` — **exakt 1 Aufruf** (HKDF
   hat keine Output-Length-Probe; `CKA_VALUE_LEN=32` steht im Template).
6. `C_GetAttributeValue` (Verifikation `CKA_EXTRACTABLE=false`,
   `CKA_SIGN=true`, `CKA_SENSITIVE=true`, `CKA_VALUE_LEN=32`) —
   deckt [`../README.md` §3 Punkt 1](../README.md) ab. **1 oder 2
   Aufrufe** zulässig: `github.com/miekg/pkcs11.Ctx.GetAttributeValue`
   ruft `C_GetAttributeValue` typischerweise zweimal auf (erst mit
   `pValue=NULL` zur Längenabfrage, dann mit allokiertem Buffer);
   dieser Two-Call-Wrapper-Pfad ist akzeptiert. Eine Low-Level-/
   Fixed-Buffer-Variante mit nur einem Aufruf ist ebenfalls
   zulässig, weil die Attributgrößen hier deterministisch sind
   (Bool=1, ULong=8).
7. `C_GetAttributeValue` mit `CKA_VALUE` auf dem Header-Key-Handle
   → erwartete Antwort `CKR_ATTRIBUTE_SENSITIVE` (deckt
   [`../README.md` §3 Punkt 3](../README.md) ab; **erwarteter Fehler**,
   nicht ein Spike-Bug). Auch hier **1 oder 2 Aufrufe** zulässig
   (Wrapper-Pfad).
8. `C_SignInit` mit `CKM_SHA256_HMAC`
9. `C_Sign` (32-Byte-Output) — deckt
   [`../README.md` §3 Punkt 2 + 5](../README.md) ab. **1 oder 2
   Aufrufe** zulässig: `github.com/miekg/pkcs11.Ctx.Sign` ruft
   `C_Sign` ebenfalls zweimal auf (Längen-Probe, dann Sign);
   eine Fixed-Buffer-Variante mit einem Aufruf ist akzeptiert,
   weil `CKM_SHA256_HMAC` deterministisch 32 Byte liefert.
   **Wichtig:** Diese Zwei-Call-Toleranz gilt nicht für
   `C_Encrypt` im produktiven Adapter — dort fordert
   `HSM-FA-ENC-006` genau einen Aufruf pro Chunk
   (siehe Slice-002b §PKCS#11-Adapter, „Binding-Falle").
10. `C_DestroyObject` (Header-Key-Handle) — **exakt 1 Aufruf**.
11. `C_SignInit` mit demselben Handle → erwartete Antwort
    `CKR_OBJECT_HANDLE_INVALID` (deckt [`../README.md` §3 Punkt 4](../README.md)
    ab; **erwarteter Fehler**).
12. `C_Logout` + `C_CloseSession` + `C_Finalize`

Abweichungen (z. B. doppelte `C_DeriveKey`-Aufrufe, fehlender
`C_DestroyObject`, unerwartete `C_GenerateKey`-Pfade, oder ein
`C_GetAttributeValue(CKA_VALUE)`-Erfolg statt
`CKR_ATTRIBUTE_SENSITIVE`) sind Spike-Befunde und gehören in §6
„Ergebnis" der Spike-README.
