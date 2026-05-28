# Bouncy HSM Server für den 002b-HKDF-Spike.
#
# Lädt das offizielle BouncyHsm-Release (.NET-Server + native
# PKCS#11-Library) und startet das Web-/REST-Frontend auf 8080
# und den PKCS#11-RPC-Endpoint auf 8765. SPIKE-/TEST-ONLY —
# Bouncy HSM schützt Schlüsselmaterial weder im Storage noch im
# Netzwerk (laut Repo-README: "not intended for production data").
#
# Bezug: Slice 002b §Vorbedingung 3, ADR 0004 (Zweitmodul-Wahl
# wird in Folge-ADR 0006 auf Bouncy HSM präzisiert; SoftHSM hat
# kein CKM_HKDF_DERIVE, siehe Spike-README §6.1).

ARG BOUNCYHSM_VERSION=2.1.0
# SHA-256 of the BouncyHsm.zip release artifact for the pinned version.
# Pinning is required by ADR 0002 (supply-chain posture); ADR 0006 §2.2
# documents this for the spike-only Bouncy HSM container.
ARG BOUNCYHSM_SHA256=04c4600348212baf1d69e036cfaea182e1cdc9f65691e0100c0eeb3e125ad593
ARG ALPINE_BASE=alpine:3.23
ARG ASPNET_BASE=mcr.microsoft.com/dotnet/aspnet:10.0

FROM ${ALPINE_BASE} AS download
ARG BOUNCYHSM_VERSION
ARG BOUNCYHSM_SHA256
WORKDIR /unzip
ADD --checksum=sha256:${BOUNCYHSM_SHA256} \
    https://github.com/harrison314/BouncyHsm/releases/download/v${BOUNCYHSM_VERSION}/BouncyHsm.zip .
RUN apk --update add unzip && \
    rm -rf /var/cache/apk/* && \
    unzip -q BouncyHsm.zip && \
    rm BouncyHsm.zip

FROM ${ASPNET_BASE} AS runtime
WORKDIR /App
COPY --from=download /unzip /App

EXPOSE 8080
EXPOSE 8765
VOLUME /var/BouncyHsm/

ENV ASPNETCORE_ENVIRONMENT=Docker

LABEL org.opencontainers.image.title="c-hsm-doc-bouncyhsm" \
      org.opencontainers.image.description="Bouncy HSM server image for the 002b HKDF spike; SPIKE/TEST-only." \
      org.opencontainers.image.vendor="pt9912" \
      purpose="spike-only" \
      warning="Contains Bouncy HSM with dev defaults; not for production use."

CMD ["dotnet", "BouncyHsm.dll"]
