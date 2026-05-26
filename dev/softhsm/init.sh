#!/usr/bin/env bash
# Initialisiert einen SoftHSM-Token im Compose-Volume.
#
# Idempotent: ein erneuter Lauf läuft nicht in einen Fehler, wenn der
# Token mit demselben Label bereits existiert.
#
# USER_PIN und SO_PIN MUESSEN von aussen gesetzt werden (typischerweise
# durch docker-compose.dev.yml). Damit landet kein PIN-Default im Image-
# Layer, der bei versehentlichem Re-Tagging in einer Prod-Registry
# stehen koennte.

set -euo pipefail

TOKEN_LABEL="${TOKEN_LABEL:-c-hsm-doc-dev}"

if [[ -z "${USER_PIN:-}" ]] || [[ -z "${SO_PIN:-}" ]]; then
    echo "softhsm-init: ERROR — USER_PIN und SO_PIN MUESSEN als Env-Variablen gesetzt sein." >&2
    echo "softhsm-init: docker-compose.dev.yml setzt sie mit Dev-Defaults; fuer manuellen" >&2
    echo "softhsm-init: Lauf: docker run -e USER_PIN=... -e SO_PIN=... ..." >&2
    exit 2
fi

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
