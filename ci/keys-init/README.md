# CI-Spike-Schlüsselinitialisierung

**Status:** `softhsm.sh` + `bouncyhsm.sh` landed (Bouncy-HSM-Lauf gegen
Pfad (a) Shim ist Spike-Erfolg, siehe Spike-README §6.2).
SoftHSM bleibt blockiert wegen fehlendem `CKM_HKDF_DERIVE`.
**Bezug:** [Slice 002b §Akzeptanz](../../docs/plan/planning/next/002b-pkcs11-encrypt-hexagon.md),
[Spike-README](../../docs/plan/planning/next/002b-spike-hkdf/README.md),
[Slice-002b-Plan §3 Punkt 5](../../docs/plan/planning/next/002b-spike-hkdf/README.md)

---

## Zweck

Modul-spezifische Setup-Skripte für den HKDF-Profil-A-Spike (Vorbedingung 3
von Slice 002b). Jedes Skript initialisiert ein PKCS#11-Modul für den
Spike-Lauf:

1. Slot/Token mit definierter Label, User-PIN und SO-PIN.
2. **Master-HMAC-Key** mit dem 32-Byte-`FixtureIKM` aus
   [`spike/fixture.go`](../../docs/plan/planning/next/002b-spike-hkdf/spike/fixture.go).
   Alle CKA-Attribute werden in **einem** `C_CreateObject`-Aufruf gesetzt:
   `CKA_VALUE`, `CKA_DERIVE=true`, `CKA_SENSITIVE=true`,
   `CKA_EXTRACTABLE=false`, `CKA_MODIFIABLE=false`. Nachträgliches Umschalten
   dieser Attribute ist gemäß
   [Spike-README §3 Punkt 5](../../docs/plan/planning/next/002b-spike-hkdf/README.md)
   **kein zulässiger Pfad**.

Vendor-Sniffing im Adapter-Code wird vermieden — jedes Modul bekommt sein
eigenes Init-Skript (siehe Slice 002b §Akzeptanz „Modul-spezifisches
Key-Setup").

## Abgrenzung zu `dev/softhsm/`

`dev/softhsm/` initialisiert nur den Token für die lokale Dev-Loop
(M1-Service-Bootstrap); es importiert **keinen** Master-HMAC-Key, weil der
M1-Service den noch nicht braucht. Spike-spezifischer Key-Import lebt
deshalb hier in `ci/keys-init/`.

## Tool-Anforderungen

- `softhsm2-util` (für SoftHSM-Token-Init)
- `python3` + `PyKCS11` (für den `C_CreateObject`-Aufruf mit vollem
  CKA-Template — `pkcs11-tool` aus OpenSC ≤ 0.23 setzt
  `CKA_EXTRACTABLE` nicht atomar)
- Modul-spezifische Tools für OpenCryptoki (folgt mit `opencryptoki.sh`)

## Skripte

- [`softhsm.sh`](softhsm.sh) — SoftHSM v2 Spike-Init. Skript ist
  technisch grün, **aber das Modul implementiert `CKM_HKDF_DERIVE`
  weder in 2.6.1 noch in 2.7.0**; der Spike-HSM-Test skippt
  entsprechend.
- [`bouncyhsm.sh`](bouncyhsm.sh) — Bouncy HSM 2.x Spike-Init. Slot via
  REST-API anlegen, FixtureIKM via PyKCS11 + `BouncyHsm.Pkcs11Lib.so`
  importieren. Spike-Erfolg (HKDF + HMAC-SHA-256 byteweise gegen
  Pure-Go-Referenz verifiziert).
- `opencryptoki.sh` — **nicht geplant** (HKDF nur in EP11-Hardware-
  Backend, Software-Token unterstützt es nicht — siehe Spike-README
  §6.1).
