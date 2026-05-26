#!/usr/bin/env bash
# Initialisiert einen SoftHSM-Token im Compose-Volume.
#
# Idempotent: ein erneuter Lauf läuft nicht in einen Fehler, wenn der
# Token mit demselben Label bereits existiert.

set -euo pipefail

TOKEN_LABEL="${TOKEN_LABEL:-c-hsm-doc-dev}"
USER_PIN="${USER_PIN:-1234}"
SO_PIN="${SO_PIN:-5678}"

echo "softhsm-init: Token-Label='${TOKEN_LABEL}'"

if softhsm2-util --show-slots 2>/dev/null | grep -qE "^[[:space:]]*Label:[[:space:]]+${TOKEN_LABEL}$"; then
    echo "softhsm-init: Token mit Label '${TOKEN_LABEL}' existiert bereits — nichts zu tun"
    exit 0
fi

echo "softhsm-init: initialisiere neuen Token"
softhsm2-util --init-token --free \
    --label "${TOKEN_LABEL}" \
    --pin "${USER_PIN}" \
    --so-pin "${SO_PIN}"

echo "softhsm-init: fertig"
echo "  Modul:    /usr/lib/softhsm/libsofthsm2.so"
echo "  Token:    ${TOKEN_LABEL}"
echo "  USER-PIN: ${USER_PIN}   (NUR für Dev!)"
