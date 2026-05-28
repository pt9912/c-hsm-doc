#!/usr/bin/env bash
# ci/keys-init/bouncyhsm.sh — Spike-002b Bouncy-HSM-Init für HKDF-Profil-A.
#
# Bouncy HSM ist ein .NET-basierter Software-HSM-Simulator mit
# nativem CKM_HKDF_DERIVE (im Gegensatz zu SoftHSM 2.6.1/2.7.0 und
# OpenCryptoki-Software-Token, siehe Spike-README §6.1).
#
# Workflow:
#  1) Slot/Token über die REST-API anlegen (idempotent),
#  2) Fixture-IKM per PyKCS11 + BouncyHsm.Pkcs11Lib.so importieren
#     — alle CKA-Attribute in einem C_CreateObject-Aufruf, analog
#     softhsm.sh (Slice-002b §3 Punkt 5).
#
# Anforderungen im Container: curl, python3 mit PyKCS11,
# BouncyHsm.Pkcs11Lib.so (typisch aus dem Bouncy-HSM-Release
# extrahiert). Über BOUNCY_HSM_CFG_STRING wird die Connection
# String zur Server-Instanz konfiguriert.
#
# Env-Variablen (alle mit Spike-Default; KEINE produktiven PINs):
#   SPIKE_BOUNCYHSM_REST_BASE  (default: http://localhost:8080)
#   SPIKE_BOUNCYHSM_TCP_HOST   (default: localhost)
#   SPIKE_BOUNCYHSM_TCP_PORT   (default: 8765)
#   SPIKE_PKCS11_MODULE        (default: /opt/bouncyhsm/BouncyHsm.Pkcs11Lib.so)
#   SPIKE_PKCS11_TOKEN         (default: c-hsm-doc-spike)
#   SPIKE_PKCS11_PIN           (default: 1234)
#   SPIKE_PKCS11_SO_PIN        (default: 5678)
#   SPIKE_MASTER_HMAC_LABEL    (default: spike-master-hmac)

set -euo pipefail

REST_BASE="${SPIKE_BOUNCYHSM_REST_BASE:-http://localhost:8080}"
TCP_HOST="${SPIKE_BOUNCYHSM_TCP_HOST:-localhost}"
TCP_PORT="${SPIKE_BOUNCYHSM_TCP_PORT:-8765}"
MODULE="${SPIKE_PKCS11_MODULE:-/opt/bouncyhsm/BouncyHsm.Pkcs11Lib.so}"
TOKEN_LABEL="${SPIKE_PKCS11_TOKEN:-c-hsm-doc-spike}"
USER_PIN="${SPIKE_PKCS11_PIN:-1234}"
SO_PIN="${SPIKE_PKCS11_SO_PIN:-5678}"
KEY_LABEL="${SPIKE_MASTER_HMAC_LABEL:-spike-master-hmac}"

# Fixture-IKM = hkdfspike.FixtureIKM aus spike/fixture.go (0x00..0x1f).
# Synchron mit softhsm.sh halten, damit beide Module mit identischem
# Master-Material laufen.
FIXTURE_IKM_HEX="000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

# PKCS#11-Library findet den Server über diese Env-Variable.
export BOUNCY_HSM_CFG_STRING="Server=${TCP_HOST}; Port=${TCP_PORT};"

echo "[bouncyhsm-init] REST=$REST_BASE TCP=$TCP_HOST:$TCP_PORT module=$MODULE"
echo "[bouncyhsm-init] token=$TOKEN_LABEL key=$KEY_LABEL"

# 1) Slot/Token über REST anlegen (idempotent).
EXISTING=$(curl -s "$REST_BASE/Slot" | python3 -c "
import json, sys
slots = json.load(sys.stdin)
for s in slots:
    if s.get('Token', {}).get('Label') == '$TOKEN_LABEL':
        print(s['SlotId'])
        break
") || true

if [[ -n "$EXISTING" ]]; then
    SLOT_ID="$EXISTING"
    echo "[bouncyhsm-init] slot '$TOKEN_LABEL' already present (slotId=$SLOT_ID), skipping POST /Slot"
else
    PAYLOAD=$(cat <<JSON
{
  "IsHwDevice": true,
  "IsRemovableDevice": false,
  "Description": "c-hsm-doc spike (HKDF Profil A)",
  "Token": {
    "Label": "$TOKEN_LABEL",
    "SerialNumber": "spike01",
    "UserPin": "$USER_PIN",
    "SoPin": "$SO_PIN",
    "SimulateHwRng": false,
    "SimulateHwMechanism": false,
    "SimulateQualifiedArea": false
  }
}
JSON
)
    RESP=$(curl -s -X POST "$REST_BASE/Slot" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD")
    SLOT_ID=$(echo "$RESP" | python3 -c 'import json,sys; print(json.load(sys.stdin)["SlotId"])')
    echo "[bouncyhsm-init] slot created (slotId=$SLOT_ID)"
fi

# 2) Fixture-IKM-Import + Attribut-Verifikation via PyKCS11.
#    Logik wie ci/keys-init/softhsm.sh — Modul-Pfad und Connection-
#    String unterscheiden sich, alles andere ist identisch.
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
    sys.exit("[bouncyhsm-init] ERROR: PyKCS11 not installed (apt: python3-pykcs11)")

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
    sys.exit(f"[bouncyhsm-init] ERROR: token '{TOKEN_LABEL}' not found via PKCS#11")

session = p11.openSession(slot, PyKCS11.CKF_SERIAL_SESSION | PyKCS11.CKF_RW_SESSION)
session.login(USER_PIN)

try:
    existing = session.findObjects([
        (CKA_CLASS, CKO_SECRET_KEY),
        (CKA_LABEL, KEY_LABEL),
    ])
    if existing:
        print(f"[bouncyhsm-init] key '{KEY_LABEL}' already imported — verifying attributes")
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
        print(f"[bouncyhsm-init] imported key '{KEY_LABEL}' (32 bytes)")

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
            print(f"[bouncyhsm-init] ERROR: {e}", file=sys.stderr)
        sys.exit(1)

    try:
        val = session.getAttributeValue(obj, [CKA_VALUE])
        leaked = val[0] if val else None
        leaked_len = len(leaked) if leaked else 0
        if leaked_len == 0:
            print("[bouncyhsm-init] sensitive-check ok: CKA_VALUE returned empty")
        else:
            sys.exit(f"[bouncyhsm-init] ERROR: CKA_VALUE leak — got {leaked_len} bytes")
    except PyKCS11.PyKCS11Error as e:
        if "CKR_ATTRIBUTE_SENSITIVE" in str(e):
            print("[bouncyhsm-init] sensitive-check ok: CKA_VALUE → CKR_ATTRIBUTE_SENSITIVE")
        else:
            sys.exit(f"[bouncyhsm-init] ERROR: unexpected error reading CKA_VALUE: {e}")

    print("[bouncyhsm-init] attributes verified: sensitive=True, extractable=False, derive=True")
finally:
    session.logout()
    session.closeSession()
PYEOF

echo "[bouncyhsm-init] ready"
