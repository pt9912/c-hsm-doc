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

## Was die Logs belegen müssen

Aufruffolge (in dieser Reihenfolge) pro Lauf:

1. `C_Initialize`
2. `C_OpenSession`
3. `C_Login`
4. `C_FindObjectsInit` + `C_FindObjects` + `C_FindObjectsFinal`
   (Master-HMAC-Lookup)
5. `C_DeriveKey` mit `CKM_HKDF_DERIVE`
6. `C_GetAttributeValue` (Verifikation `CKA_EXTRACTABLE=false`,
   `CKA_SIGN=true`)
7. `C_SignInit` mit `CKM_SHA256_HMAC`
8. `C_Sign` (32-Byte-Output)
9. `C_DestroyObject` (Header-Key-Handle)
10. `C_Logout` + `C_CloseSession` + `C_Finalize`

Abweichungen (z. B. doppelte `C_DeriveKey`-Aufrufe, fehlender
`C_DestroyObject`, unerwartete `C_GenerateKey`-Pfade) sind Spike-
Befunde und gehören in §6 „Ergebnis" der Spike-README.
