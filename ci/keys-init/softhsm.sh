#!/usr/bin/env bash
# ci/keys-init/softhsm.sh — Spike-002b SoftHSM-Init für HKDF-Profil-A.
#
# Initialisiert einen SoftHSM v2 Slot/Token und importiert den
# Spike-Fixture-IKM (32 Byte, hkdfspike.FixtureIKM aus
# docs/plan/planning/next/002b-spike-hkdf/spike/fixture.go) als
# Master-HMAC-Key. Alle CKA-Attribute werden in EINEM C_CreateObject-
# Aufruf gesetzt (Slice-002b §3 Punkt 5): CKA_VALUE, CKA_DERIVE=true,
# CKA_SENSITIVE=true, CKA_EXTRACTABLE=false. Nachträgliches Umschalten
# ist kein zulässiger Spike-Pfad.
#
# Anforderungen im Container: softhsm2-util, python3 mit PyKCS11.
# Env-Variablen (alle mit Spike-Default; KEINE produktiven PINs):
#   SPIKE_PKCS11_MODULE       (default: /usr/lib/softhsm/libsofthsm2.so)
#   SPIKE_PKCS11_TOKEN        (default: c-hsm-doc-spike)
#   SPIKE_PKCS11_PIN          (default: 1234)
#   SPIKE_PKCS11_SO_PIN       (default: 5678)
#   SPIKE_MASTER_HMAC_LABEL   (default: spike-master-hmac)

set -euo pipefail

MODULE="${SPIKE_PKCS11_MODULE:-/usr/lib/softhsm/libsofthsm2.so}"
TOKEN_LABEL="${SPIKE_PKCS11_TOKEN:-c-hsm-doc-spike}"
USER_PIN="${SPIKE_PKCS11_PIN:-1234}"
SO_PIN="${SPIKE_PKCS11_SO_PIN:-5678}"
KEY_LABEL="${SPIKE_MASTER_HMAC_LABEL:-spike-master-hmac}"

# Fixture-IKM = hkdfspike.FixtureIKM aus spike/fixture.go (0x00..0x1f,
# 32 Byte). Wenn dieser Wert geändert wird, MUSS spike/fixture.go
# synchron gepflegt werden — die Pure-Go-HKDF-Referenz vergleicht
# gegen beide Werte.
FIXTURE_IKM_HEX="000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

echo "[softhsm-init] module=$MODULE token=$TOKEN_LABEL key=$KEY_LABEL"

# 1) Token-Init (idempotent — bei wiederholtem Lauf nichts tun).
#    softhsm2-util zeigt das Label mit Trailing-Spaces; awk normalisiert.
if softhsm2-util --show-slots 2>/dev/null | awk -v want="$TOKEN_LABEL" '
        /^[[:space:]]*Label:/ {
            sub(/^[[:space:]]*Label:[[:space:]]+/, "")
            sub(/[[:space:]]+$/, "")
            if ($0 == want) { found=1; exit }
        }
        END { exit !found }
    '; then
    echo "[softhsm-init] token '$TOKEN_LABEL' already present, skipping --init-token"
else
    softhsm2-util --init-token --free \
        --label "$TOKEN_LABEL" \
        --pin "$USER_PIN" \
        --so-pin "$SO_PIN"
    echo "[softhsm-init] token initialized"
fi

# 2) Master-HMAC-Key-Import + Attribut-Verifikation via PyKCS11.
#    Hex und Labels werden über Env an Python gereicht, damit die
#    Bash-Substitution nicht in den Python-Quelltext leakt.
export SPIKE_INIT_MODULE="$MODULE"
export SPIKE_INIT_TOKEN="$TOKEN_LABEL"
export SPIKE_INIT_PIN="$USER_PIN"
export SPIKE_INIT_LABEL="$KEY_LABEL"
export SPIKE_INIT_IKM_HEX="$FIXTURE_IKM_HEX"

python3 - <<'PYEOF'
import binascii
import os
import sys

try:
    import PyKCS11
except ImportError:
    sys.exit("[softhsm-init] ERROR: PyKCS11 not installed (apt: python3-pykcs11, pip: PyKCS11)")

# Modul-Konstanten direkt verwenden — Debian-PyKCS11 (1.5.x)
# exportiert keine CKA.CLASS-Klassen-API, nur Modul-Symbole.
CKA_CLASS = PyKCS11.CKA_CLASS
CKA_KEY_TYPE = PyKCS11.CKA_KEY_TYPE
CKA_LABEL = PyKCS11.CKA_LABEL
CKA_TOKEN = PyKCS11.CKA_TOKEN
CKA_PRIVATE = PyKCS11.CKA_PRIVATE
CKA_VALUE = PyKCS11.CKA_VALUE
CKA_DERIVE = PyKCS11.CKA_DERIVE
CKA_SENSITIVE = PyKCS11.CKA_SENSITIVE
CKA_EXTRACTABLE = PyKCS11.CKA_EXTRACTABLE
CKA_MODIFIABLE = PyKCS11.CKA_MODIFIABLE
CKO_SECRET_KEY = PyKCS11.CKO_SECRET_KEY
CKK_GENERIC_SECRET = PyKCS11.CKK_GENERIC_SECRET

MODULE = os.environ["SPIKE_INIT_MODULE"]
TOKEN_LABEL = os.environ["SPIKE_INIT_TOKEN"]
USER_PIN = os.environ["SPIKE_INIT_PIN"]
KEY_LABEL = os.environ["SPIKE_INIT_LABEL"]
IKM_HEX = os.environ["SPIKE_INIT_IKM_HEX"]

IKM = binascii.unhexlify(IKM_HEX)
assert len(IKM) == 32, f"fixture IKM must be 32 bytes, got {len(IKM)}"

p11 = PyKCS11.PyKCS11Lib()
p11.load(MODULE)

slots = p11.getSlotList(tokenPresent=True)
slot = next(
    (s for s in slots if p11.getTokenInfo(s).label.strip() == TOKEN_LABEL),
    None,
)
if slot is None:
    sys.exit(f"[softhsm-init] ERROR: token '{TOKEN_LABEL}' not found")

session = p11.openSession(slot, PyKCS11.CKF_SERIAL_SESSION | PyKCS11.CKF_RW_SESSION)
session.login(USER_PIN)

try:
    existing = session.findObjects([
        (CKA_CLASS, CKO_SECRET_KEY),
        (CKA_LABEL, KEY_LABEL),
    ])
    if existing:
        print(f"[softhsm-init] key '{KEY_LABEL}' already imported — verifying attributes")
        obj = existing[0]
    else:
        template = [
            (CKA_CLASS, CKO_SECRET_KEY),
            (CKA_KEY_TYPE, CKK_GENERIC_SECRET),
            (CKA_LABEL, KEY_LABEL),
            (CKA_TOKEN, True),
            (CKA_PRIVATE, True),
            (CKA_VALUE, IKM),
            (CKA_DERIVE, True),
            (CKA_SENSITIVE, True),
            (CKA_EXTRACTABLE, False),
            (CKA_MODIFIABLE, False),
        ]
        obj = session.createObject(template)
        print(f"[softhsm-init] imported key '{KEY_LABEL}' (32 bytes)")

    # CKA_SENSITIVE / CKA_EXTRACTABLE / CKA_DERIVE durchsetzen — alle
    # drei müssen genau das gewünschte Bit tragen.
    attrs = session.getAttributeValue(obj, [CKA_SENSITIVE, CKA_EXTRACTABLE, CKA_DERIVE])
    sensitive, extractable, derive = attrs
    errs = []
    if not bool(sensitive):
        errs.append(f"CKA_SENSITIVE = {sensitive}, expected True")
    if bool(extractable):
        errs.append(f"CKA_EXTRACTABLE = {extractable}, expected False")
    if not bool(derive):
        errs.append(f"CKA_DERIVE = {derive}, expected True")
    if errs:
        for e in errs:
            print(f"[softhsm-init] ERROR: {e}", file=sys.stderr)
        sys.exit(1)

    # Sensitive-Negativtest (Spike-README §3 Punkt 3): CKA_VALUE darf
    # nicht enthüllt werden. Spec-konforme Module geben dafür
    # CKR_ATTRIBUTE_SENSITIVE zurück — PyKCS11 1.5 verschluckt diesen
    # Fehler aber bei aggregierten Calls und liefert stattdessen 0 Byte
    # oder None für das betroffene Attribut. Beide Pfade gelten als
    # „durchgesetzt"; nur ein nicht-leerer CKA_VALUE-Return ist ein
    # Leak.
    try:
        val = session.getAttributeValue(obj, [CKA_VALUE])
        leaked = val[0] if val else None
        leaked_len = len(leaked) if leaked else 0
        if leaked_len == 0:
            print("[softhsm-init] sensitive-check ok: CKA_VALUE returned empty (PyKCS11-aggregated CKR_ATTRIBUTE_SENSITIVE)")
        else:
            sys.exit(f"[softhsm-init] ERROR: CKA_VALUE leak — getAttributeValue returned {leaked_len} bytes")
    except PyKCS11.PyKCS11Error as e:
        if "CKR_ATTRIBUTE_SENSITIVE" in str(e):
            print("[softhsm-init] sensitive-check ok: CKA_VALUE → CKR_ATTRIBUTE_SENSITIVE")
        else:
            sys.exit(f"[softhsm-init] ERROR: unexpected error reading CKA_VALUE: {e}")

    print("[softhsm-init] attributes verified: sensitive=True, extractable=False, derive=True")
finally:
    session.logout()
    session.closeSession()
PYEOF

echo "[softhsm-init] ready"
