# Dev-only SoftHSM v2 Init-Container.
#
# Initialisiert einen SoftHSM-Token in einem Compose-Volume, das der
# c-hsm-doc-server in einer künftigen Dev-Variante via PKCS#11
# attached. Erfüllt HSM-ENV-003 (lokale Entwicklung gegen SoftHSM v2).
#
# NICHT für Produktion. Produktive HSMs werden über HSM-TECH-006
# (Utimaco CryptoServer, Thales Luna) angesprochen.

FROM debian:bookworm-slim

LABEL org.opencontainers.image.title="c-hsm-doc-softhsm-init" \
      org.opencontainers.image.description="Dev-only SoftHSM v2 init container for c-hsm-doc local dev environment." \
      org.opencontainers.image.vendor="pt9912" \
      purpose="dev-only" \
      warning="Contains SoftHSM and dev defaults; not for production use."

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        softhsm2 \
        opensc \
    && rm -rf /var/lib/apt/lists/*

COPY init.sh /usr/local/bin/init.sh
RUN chmod +x /usr/local/bin/init.sh

# TOKEN_LABEL ist nicht sensibel und bleibt im Image. USER_PIN und SO_PIN
# werden bewusst NICHT als ENV gesetzt, damit sie nicht im Image-Layer
# (`docker history`) auftauchen — die Compose-Datei reicht sie zur
# Laufzeit weiter (mit Dev-Defaults), init.sh prueft sie strict.
ENV TOKEN_LABEL=c-hsm-doc-dev
ENV SOFTHSM2_CONF=/etc/softhsm/softhsm2.conf

VOLUME ["/var/lib/softhsm/tokens"]

ENTRYPOINT ["/usr/local/bin/init.sh"]
