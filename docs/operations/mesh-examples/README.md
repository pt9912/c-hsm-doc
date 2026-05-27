# Mesh-Beispiel-Manifeste — `c-hsm-doc`

Diese Beispiele sind **referenzielle Konfigurationen**, kein Bestandteil
der späteren Helm-Chart-Defaults. Sie zeigen, wie ein Betreiber den
Service in den vier in `HSM-ENV-004` definierten Betriebsmodi an die
jeweilige Plattform andockt, ohne dass der Service selbst Plattform-
oder Mesh-Annahmen treffen muss.

Bezug: [ADR 0003 §2.5](../../plan/adr/0003-plattform-und-mesh-neutralitaet.md),
[Lastenheft `HSM-ENV-004`, `HSM-API-GRPC-003`](../../../spec/lastenheft.md),
[Spezifikation `HSM-API-GRPC-006..008`](../../../spec/spezifikation.md).

---

## Mode-zu-Manifest-Mapping

| Modus aus `HSM-ENV-004` | Plattform                                         | mTLS-Terminierung | `identity.source` | Beispiel-Manifest                                          |
| ----------------------- | ------------------------------------------------- | ----------------- | ----------------- | ---------------------------------------------------------- |
| **1** Bare-Container     | Docker/Podman/containerd ohne Orchestrator        | Server            | `mtls-subject`    | _kein Manifest_ — `docker run`-Aufruf in `HSM-MVP-005`     |
| **2** K8s ohne Mesh      | Vanilla Kubernetes                                | Server            | `mtls-subject`    | [`mode-2-networkpolicy.yaml`](mode-2-networkpolicy.yaml)   |
| **3** K8s + L4-Mesh      | Istio Ambient (ztunnel-only, kein Waypoint)       | Server (HBONE-passthrough) | `mtls-subject`    | [`mode-3-istio-ambient.yaml`](mode-3-istio-ambient.yaml)   |
| **4** K8s + L7-Mesh      | Istio Sidecar mit `PeerAuthentication STRICT`     | Mesh-Sidecar      | `header`          | [`mode-4-istio-sidecar.yaml`](mode-4-istio-sidecar.yaml)   |
| **4** K8s + L7-Mesh      | Linkerd-Proxy (Default-Konfiguration)             | Mesh-Sidecar      | `header`          | [`mode-4-linkerd.yaml`](mode-4-linkerd.yaml)               |

---

## Server-Konfiguration pro Modus (`identity.*`-ENV-Variablen)

Die Werte unten sind die Empfehlung; sie greifen `HSM-API-GRPC-006`
(Konfigurationsschema) und `HSM-API-GRPC-007` (Peer-Allowlist) auf.

### Modus 1 + 2 + 3 — `mtls-subject` am Server

```env
HSMDOC_IDENTITY_SOURCE=mtls-subject
HSMDOC_IDENTITY_MTLS_SUBJECT_ATTRIBUTE=subject_dn   # oder san_uri für SPIFFE-IDs
# identity.peer.allowlist bleibt leer (nur für source=header relevant)
```

### Modus 4 — Istio Sidecar (Header `x-forwarded-client-cert`)

```env
HSMDOC_IDENTITY_SOURCE=header
HSMDOC_IDENTITY_HEADER_NAME=x-forwarded-client-cert
HSMDOC_IDENTITY_HEADER_FORMAT=xfcc
HSMDOC_IDENTITY_PEER_ALLOWLIST=ip:127.0.0.1,ip:::1
```

Begründung der Allowlist: Der Istio-Sidecar (`istio-proxy`) läuft im
selben Pod wie der Service-Container und reicht Traffic über
Loopback weiter. Damit ist Loopback ein hinreichender Peer-Anker
für `HSM-API-GRPC-007`. Der gRPC-Listener MUSS in diesem Modus auf
`127.0.0.1`/`::1` gebunden sein (`server.listen=127.0.0.1:9443`),
nicht auf `0.0.0.0`.

### Modus 4 — Linkerd-Proxy (SPIFFE-ID-Header)

```env
HSMDOC_IDENTITY_SOURCE=header
HSMDOC_IDENTITY_HEADER_NAME=l5d-client-id
HSMDOC_IDENTITY_HEADER_FORMAT=raw
HSMDOC_IDENTITY_PEER_ALLOWLIST=ip:127.0.0.1,ip:::1
```

Linkerd setzt `l5d-client-id` mit dem ServiceAccount-Identifier des
Callers (Format: `<serviceaccount>.<namespace>.serviceaccount.identity.linkerd.cluster.local`).
`format=raw` reicht die Identität unverändert in das Audit-Feld
`caller`; eine `tenant_id`-Ableitung ist Betreiber-Konfiguration.

---

## Was diese Beispiele NICHT abdecken

- **SPIFFE/SPIRE als zweite Vertrauenswurzel** — ADR 0003 §2.6 hat das
  als Folge-ADR ausgewiesen.
- **Wegfall von `mtls-subject`** (Pflicht-mTLS am Server abschalten) —
  ist konfigurativ möglich, aber gegen das in `HSM-API-GRPC-003`
  formulierte Standard-Härtungsniveau. Beispiele zeigen den
  empfohlenen, nicht den minimal-zulässigen Pfad.
- **NetworkPolicies für Modi 3 und 4** — die Mesh-Implementierung
  liefert eigene Cluster-Network-Garantien (Istio Ambient ztunnel,
  Linkerd-Proxy). NetworkPolicy ist primär für Modus 2 relevant
  und wird dort gezeigt.
- **Ingress-/Gateway-Konfiguration** — externer Traffic-Eintritt zum
  Service liegt außerhalb dieser ADR; das Beispiel-Set fokussiert
  auf Pod-zu-Pod-Verkehr im Cluster.

---

## Test-Hinweis

Slice 006 (`docs/plan/planning/next/006-identity-source-und-peer-allowlist.md`)
sieht für die Modus-4-Integrationstests einen In-Process-Fake-Sidecar
vor. Die Beispiele hier sind dagegen für reale Betreiber gedacht —
sie ergänzen, ersetzen aber nicht die Test-Pipeline.
