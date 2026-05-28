#!/usr/bin/env bash
# scripts/spike-hkdf-bouncyhsm.sh — Reproducible end-to-end run for
# the 002b HKDF spike against Bouncy HSM.
#
# Steps:
#  1) build the spike-only Bouncy HSM server image (cached after the
#     first run; idempotent),
#  2) create a dedicated Docker network and start the server,
#  3) extract BouncyHsm.Pkcs11Lib.so from the server image,
#  4) run ci/keys-init/bouncyhsm.sh to initialize slot + fixture IKM,
#  5) run the Go end-to-end test (TestHKDFEndToEndAgainstHSM, build tag
#     spike) against the running server,
#  6) tear down server + network on exit (trap; runs even on error).
#
# All side effects live inside Docker — host stays clean (ADR 0002).

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
RUN_ID="$$-$(date +%s)"
NET="spike-hkdf-net-${RUN_ID}"
SRV="bouncyhsm-srv-${RUN_ID}"
LIB_TMP="$(mktemp -d -t spike-hkdf-lib-XXXXXX)"
BOUNCY_IMAGE="${SPIKE_BOUNCYHSM_IMAGE:-c-hsm-doc-bouncyhsm:spike}"

cleanup() {
    local rc=$?
    set +e
    if [[ ${rc} -ne 0 ]] && docker ps -a --format '{{.Names}}' | grep -qx "${SRV}"; then
        echo "[spike-hkdf] cleanup after error rc=${rc}; server logs (last 30):" >&2
        docker logs --tail 30 "${SRV}" 2>&1 >&2 || true
    fi
    docker stop "${SRV}" >/dev/null 2>&1 || true
    docker network rm "${NET}" >/dev/null 2>&1 || true
    rm -rf "${LIB_TMP}" 2>/dev/null || true
    exit ${rc}
}
trap cleanup EXIT

echo "[spike-hkdf] step 1: build Bouncy HSM image (${BOUNCY_IMAGE})"
docker build -t "${BOUNCY_IMAGE}" "${REPO_ROOT}/ci/bouncyhsm/" >/dev/null

echo "[spike-hkdf] step 2: create network (${NET})"
docker network create "${NET}" >/dev/null

echo "[spike-hkdf] step 3: start server (${SRV})"
docker run -d --rm --name "${SRV}" --network "${NET}" "${BOUNCY_IMAGE}" >/dev/null

echo "[spike-hkdf] step 4: wait for server ready"
# aspnet:10.0 image has neither curl nor wget, so we probe from a
# throwaway helper container on the same Docker network.
if ! docker run --rm --network "${NET}" curlimages/curl:8.10.1 \
        sh -c "for i in \$(seq 1 30); do curl -fs -o /dev/null http://${SRV}:8080/Slot && exit 0; sleep 1; done; exit 1" \
        >/dev/null 2>&1; then
    echo "[spike-hkdf] server did not become ready within 30s" >&2
    exit 1
fi
echo "[spike-hkdf] server ready"

echo "[spike-hkdf] step 5: extract PKCS#11 library"
docker cp "${SRV}":/App/native/Linux-x64/BouncyHsm.Pkcs11Lib.so "${LIB_TMP}/" >/dev/null

echo "[spike-hkdf] step 6: run init script"
docker run --rm --network "${NET}" \
    -e SPIKE_BOUNCYHSM_REST_BASE="http://${SRV}:8080" \
    -e SPIKE_BOUNCYHSM_TCP_HOST="${SRV}" \
    -e SPIKE_PKCS11_MODULE=/opt/bouncyhsm/BouncyHsm.Pkcs11Lib.so \
    -v "${LIB_TMP}/BouncyHsm.Pkcs11Lib.so":/opt/bouncyhsm/BouncyHsm.Pkcs11Lib.so:ro \
    -v "${REPO_ROOT}/ci/keys-init":/scripts:ro \
    debian:bookworm-slim \
    bash -c '
        export DEBIAN_FRONTEND=noninteractive
        apt-get update -qq >/dev/null 2>&1
        apt-get install -qq -y --no-install-recommends curl python3 python3-pykcs11 >/dev/null 2>&1
        bash /scripts/bouncyhsm.sh
    '

echo "[spike-hkdf] step 7: run Go end-to-end test"
docker run --rm --network "${NET}" \
    -e BOUNCY_HSM_CFG_STRING="Server=${SRV}; Port=8765;" \
    -e SPIKE_PKCS11_MODULE=/opt/bouncyhsm/BouncyHsm.Pkcs11Lib.so \
    -e SPIKE_PKCS11_TOKEN=c-hsm-doc-spike \
    -e SPIKE_PKCS11_PIN=1234 \
    -e SPIKE_MASTER_HMAC_LABEL=spike-master-hmac \
    -e GOFLAGS="-mod=readonly -buildvcs=false" \
    -v "${LIB_TMP}/BouncyHsm.Pkcs11Lib.so":/opt/bouncyhsm/BouncyHsm.Pkcs11Lib.so:ro \
    -v "${REPO_ROOT}":/src -w /src \
    golang:1.26.3 \
    go test -tags=spike -v -run TestHKDFEndToEndAgainstHSM ./docs/plan/planning/next/002b-spike-hkdf/spike/...

echo "[spike-hkdf] all steps green"
