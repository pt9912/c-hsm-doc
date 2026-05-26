# Dev-only SoftHSM v2 Init-Container.
#
# Initialisiert einen SoftHSM-Token in einem Compose-Volume, das der
# c-hsm-doc-server in einer künftigen Dev-Variante via PKCS#11
# attached. Erfüllt HSM-ENV-003 (lokale Entwicklung gegen SoftHSM v2).
#
# NICHT für Produktion. Produktive HSMs werden über HSM-TECH-006
# (Utimaco CryptoServer, Thales Luna) angesprochen.

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        softhsm2 \
        opensc \
    && rm -rf /var/lib/apt/lists/*

COPY init.sh /usr/local/bin/init.sh
RUN chmod +x /usr/local/bin/init.sh

ENV TOKEN_LABEL=c-hsm-doc-dev
ENV USER_PIN=1234
ENV SO_PIN=5678
ENV SOFTHSM2_CONF=/etc/softhsm/softhsm2.conf

VOLUME ["/var/lib/softhsm/tokens"]

ENTRYPOINT ["/usr/local/bin/init.sh"]
