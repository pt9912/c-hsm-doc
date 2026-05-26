# Architektur – `c-hsm-doc`

| Dokument         | Architektur                                                                |
| ---------------- | -------------------------------------------------------------------------- |
| Projektname      | `c-hsm-doc`                                                                |
| Kurzbeschreibung | Strukturelle und visuelle Sicht auf den HSM-Service und den Java-Client    |
| Version          | 0.1                                                                        |
| Status           | Entwurf                                                                    |
| Datum            | 2026-05-26                                                                 |
| Begleitdokumente | [spec/lastenheft.md](lastenheft.md), [spec/spezifikation.md](spezifikation.md) |

---

## 0. Lesehinweise

Dieses Dokument liefert **strukturelle und visuelle Sicht** auf die
Architektur: Komponenten, Deployment-Topologie, Vertrauensgrenzen und
Sequenzen für die wichtigsten Abläufe.

Es ist **bewusst dünn**:

- Es DARF NICHT eigene Anforderungen einführen. Anforderungen leben im
  Lastenheft (vertraglich) bzw. in der Spezifikation (technisch).
- Es verweist mit `HSM-…`-IDs auf die Quelle der jeweiligen Anforderung.
- Bei Konflikt zwischen Diagramm und Spezifikation hat die Spezifikation
  Vorrang; das Diagramm ist dann zu korrigieren.

ASCII-Diagramme werden bevorzugt, damit das Dokument unabhängig von
Mermaid-Renderern lesbar bleibt.

---

## 1. Systemkontext

```text
+---------------------+        gRPC/TLS 1.3 (mTLS opt.)       +-------------------------+
| Aufrufendes Backend |  <----------------------------------> |   c-hsm-doc-server      |
| (Java 21)           |   bidirektionales Streaming           |   (Go)                  |
| + c-hsm-doc-client  |                                       |                         |
+---------------------+                                       +-----------+-------------+
                                                                          |
                                                  +-----------------------+-----------------------+
                                                  |                       |                       |
                                                  v                       v                       v
                                          +---------------+      +---------------+      +-----------------+
                                          | Secret-Store  |      |  HSM          |      | Audit-Sink      |
                                          | (K8s / Vault) |      |  via PKCS#11  |      | (FS / S3 / SIEM)|
                                          +---------------+      +---------------+      +-----------------+
                                                                                                  |
                                                                                                  v
                                                                                          +-----------------+
                                                                                          | Verankerung     |
                                                                                          | (TSA / Rekor /  |
                                                                                          |  zweiter Log)   |
                                                                                          +-----------------+
```

Bezug: `HSM-PUE-001`, `HSM-PUE-003` (Lastenheft).

---

## 2. Komponentensicht (Ports und Adapter)

`c-hsm-doc-server` ist hexagonal aufgebaut (`HSM-ARCH-001`, Lastenheft):
der Domain-Kern (Stream-Orchestrierung, Chunking, Container-Codec)
hängt nicht von Infrastruktur ab. Adapter erfüllen die Ports.

```text
                          +---------------------------+
                          |     Domain-Kern           |
                          |  - Stream-Orchestrierung  |
                          |  - Chunk-State-Machine    |
                          |  - Container-Codec        |
                          |  - Reorder-Buffer         |
                          |  - Backpressure           |
                          +---+----------+--------+---+
                              |          |        |
                  Inbound:    |          |        |   Outbound:
                  RPC-Port    |          |        |   Crypto-Port, Audit-Port, Key-Port, Metrics-Port
                              v          v        v
            +-----------------+   +------+----+   +------------------+   +---------------+
            | gRPC-Adapter    |   | Worker-   |   | PKCS#11-Adapter  |   | Audit-Adapter |
            | (proto3, mTLS)  |   | Pool      |   | (miekg/pkcs11)   |   | FS/S3/SIEM    |
            +-----------------+   +-----------+   +------------------+   +---------------+
                                                          |                      |
                                                          v                      v
                                                  +---------------+      +---------------+
                                                  | Session-Pool  |      | Verankerung   |
                                                  | + Re-Login    |      | (TSA/Rekor)   |
                                                  | + CircuitBrk. |      +---------------+
                                                  +---------------+
                                                          |
                                                          v
                                                  +---------------+
                                                  | HSM-Vendor-   |
                                                  | Modul (.so)   |
                                                  +---------------+

   Telemetry:
   - OTel-Adapter (Logs, Traces, Metrics)
   - Prometheus-Adapter (/metrics)
   - Health-/Ready-Adapter (/healthz, /readyz)
```

Bezug: `HSM-ARCH-001..002` (Lastenheft), `HSM-ARCH-003..005`, `HSM-FA-FAIL-005`,
`HSM-FA-HSM-004` (Spezifikation).

`c-hsm-doc-client` (Java 21) spiegelt nur die RPC-Seite:

```text
+----------------------+    +----------------------+    +--------------+       gRPC/TLS 1.3
|  Application Code    |--->|  Public Client API   |--->|  gRPC-Stub   |-----> Network
+----------------------+    +----------+-----------+    +--------------+
                                      |
                                      v
                            +----------------------+
                            | Retry/Backpressure   |
                            | Exception-Mapping    |
                            | TLS/mTLS-Config      |
                            +----------------------+
```

Bezug: `HSM-API-JAVA-001` (Lastenheft), `HSM-API-JAVA-002..005` (Spezifikation),
`HSM-ARCH-002` (Java ohne JNI/PKCS#11).

---

## 3. Deployment-Topologie

```text
Kubernetes Namespace: hsm-doc
+---------------------------------------------------------------------------+
|                                                                           |
|   +-----------------------------+        +----------------------------+   |
|   |  Deployment: c-hsm-doc      |        |  Deployment: callers        |  |
|   |  - replicas: N (>=2)        |        |  (other backends, mTLS)     |  |
|   |  - readOnlyRootFilesystem   |        +-------------+--------------+   |
|   |  - runAsNonRoot             |                      |                  |
|   |  - seccompProfile: Default  |                      | gRPC/mTLS        |
|   |  - distroless image         |<---------------------+                  |
|   |                             |                                         |
|   |  Probes:                    |        ConfigMap: c-hsm-doc-config      |
|   |   /healthz (liveness)       |   <--- (HSM module path, pool sizes,    |
|   |   /readyz  (readiness)      |        chunk size, retention, ...)      |
|   |                             |                                         |
|   |  Volumes:                   |        Secret: hsm-pin                  |
|   |   /audit (NVMe-PVC)         |   <--- (HSM-PIN)                        |
|   |   /tls   (mTLS material)    |        Secret: tls-server, tls-client   |
|   +-----------+----------+------+                                         |
|               |          |                                                |
|       PKCS#11 |          | OTel/Prometheus                                |
|               v          v                                                |
|   +-----------------+   +----------------------+                          |
|   | HSM (HW oder    |   | OTel-Collector       |                          |
|   | Netzwerk-HSM)   |   | Prometheus Scraper   |                          |
|   +-----------------+   +----------------------+                          |
|                                                                           |
+---------------------------------------------------------------------------+

Externe Senken:
- Audit-Sink (S3 Object Lock / dediziertes Append-only-Volume / SIEM)
- Verankerungssenke (RFC-3161 TSA / Sigstore Rekor / zweiter Log-Service)
```

Bezug: `HSM-ENV-001..003`, `HSM-NFA-SEC-007..008`, `HSM-NFA-HA-001..002`
(Lastenheft); `HSM-FA-AUDIT-010..011` (Spezifikation).

---

## 4. Vertrauensgrenzen

```text
TRUSTED                                  | UNTRUSTED
                                         |
+------------+      mTLS, Audit-Bind     |    Internet / andere Namespaces
| HSM        |    +-------------+        |
+-----+------+    | Service-Pod |--------+----- aufrufende Backends
      | PKCS#11   +------+------+        |
      | (PIN aus         |               |
      |  Secret)         | fsync         |
      v                  v               |
+-------------+   +----------------+     |    Cluster-Admin (Restrisiko,
| Secret-Store|   |  Audit-Volume  |-----+--- siehe HSM-THREAT-002, -008)
+-------------+   +----------------+     |
                          |              |
                          | Sync         |
                          v              |
                  +----------------+     |
                  | Verankerungs-  |-----+--- organisatorisch getrennt
                  | Senke (extern) |     |    (HSM-FA-AUDIT-007/011)
                  +----------------+     |
```

Bezug: `HSM-PUE-003`, `HSM-THREAT-001..010` (Lastenheft).

---

## 5. Sequenzdiagramme

### 5.1 Encrypt-Stream (Happy Path, paralleles Chunking)

```text
Caller          Client          Server (gRPC)     Worker(s)     HSM        Audit-Sink
  |                |                  |                |          |             |
  |--encrypt()---->|                  |                |          |             |
  |                |--Open Encrypt--->|                |          |             |
  |                |   (stream)       |                |          |             |
  |                |                  |--read chunk--->|          |             |
  |                |<--ACK header-----|                |          |             |
  |                |                  |--read chunk--->|          |             |
  |                |--chunk[1]------->|--enqueue------>|          |             |
  |                |--chunk[2]------->|--enqueue------>|          |             |
  |                |--chunk[3]------->|--enqueue------>|          |             |
  |                |                  |                |--encrypt->|             |
  |                |                  |                |<--ct[1]--|             |
  |                |                  |                |---------->|             |
  |                |                  |                |           |--audit ok-->|
  |                |                  |<--SEALED[1]----|                         |
  |                |                  |--reorder-->emit(1)                       |
  |                |<--ct[1]----------|                                          |
  |                |                  |<--SEALED[3]----|  (out of order)         |
  |                |                  |  (buffer 3)                              |
  |                |                  |<--SEALED[2]----|                         |
  |                |                  |--emit(2), emit(3)                        |
  |                |<--ct[2], ct[3]---|                                          |
  |                |--trailer-------->|                                          |
  |                |<--final ACK------|                                          |
  |<---OK----------|                                                             |
```

Bezug: `HSM-FA-CHUNK-004..007` (Spezifikation: Chunk-State-Machine,
Reorder-Buffer, Commit-Punkte), `HSM-FA-ENC-006` (Per-Chunk-AEAD).

### 5.2 Decrypt-Stream mit Tag-Mismatch

```text
Caller       Client       Server        Worker        HSM
  |             |             |             |           |
  |--decrypt--->|             |             |           |
  |             |--Open------>|             |           |
  |             |--header---->|             |           |
  |             |             |--lookup key |           |
  |             |             |  (HSM-FA-DEC-003)       |
  |             |             |--ct[1]----->|---decrypt-+
  |             |             |             |<--ok------+
  |             |<--pt[1]-----|             |           |
  |             |--ct[2]----->|---decrypt-->|---decrypt-+
  |             |             |             |<--TAG_MISMATCH (CKR_DATA_INVALID)
  |             |             |<--abort-----|           |
  |             |<--DATA_LOSS-|                         |
  |<--Integrity-|                                       |
  |  Exception  |                                       |
```

Bezug: `HSM-FA-DEC-002` (Lastenheft: Stream-Abbruch bei Mismatch),
`HSM-FA-FAIL-003` (Spezifikation: `CKR_DATA_INVALID` → `TAG_MISMATCH`).

### 5.3 Key-Rotation ohne Stream-Abbruch

```text
Crypto-Officer    Server        Key-Adapter      HSM           Running Stream
      |              |               |             |                 |
      |--rotate ---->|               |             |                 |
      |   keyId=K    |               |             |                 |
      |              |--create-new-->|             |                 |
      |              |               |--C_GenKey-->|                 |
      |              |               |<--handle----|                 |
      |              |--mark current |             |                 |
      |              |  deprecated   |             |                 |
      |              |--HKDF-derive  |             |                 |
      |              |  new headerKey|             |                 |
      |              |               |             |                 |
      |              |  Stream weiterhin mit deprecated key,         |
      |              |  HSM-FA-KEY-003 verbietet Abbruch             |
      |              |  -------------------------------------------->|
      |              |                                               |
      |              |  Neue Streams nutzen neue keyVersion          |
      |<--OK---------|                                               |
```

Bezug: `HSM-FA-KEY-001..003` (Lastenheft), `HSM-FMT-006` (HKDF im HSM,
Spezifikation), `HSM-FA-KEY-006` (Usage-Limits, Spezifikation).

### 5.4 HSM-Failure-Recovery (Token-Removal)

```text
Server           Session-Pool        Circuit-Breaker    HSM-Connection-Loop
  |                  |                      |                   |
  |                  |--C_Encrypt---------->|                   |
  |                  |<--CKR_DEVICE_REMOVED-|                   |
  |                  |--drain all sessions  |                   |
  |--readiness rot-->|                      |                   |
  |                  |--breaker.open()----->|                   |
  |                  |                      |--start backoff--->|
  |                  |                      |   1s, 2s, 4s, ..., cap 60s
  |                  |                      |                   |
  |                  |                      |   token re-inserted
  |                  |                      |<--C_Initialize OK |
  |                  |                      |<--C_Login OK      |
  |                  |                      |<--mechanism check |
  |                  |<--refill pool--------|                   |
  |                  |--breaker.halfOpen()->|                   |
  |                  |--probe encrypt------>|                   |
  |                  |<--ok-----------------|                   |
  |--readiness gruen<|--breaker.close()---->|                   |
```

Bezug: `HSM-FA-FAIL-001..002` (Lastenheft), `HSM-FA-FAIL-003..009`
(Spezifikation).

---

## 6. Querschnitts-Themen

| Thema             | Verantwortliche Adapter            | Spezifikations-IDs                |
| ----------------- | ---------------------------------- | --------------------------------- |
| AuthN/AuthZ       | gRPC-Adapter (mTLS), Tenant-Filter | `HSM-API-GRPC-003`, `HSM-FA-TENANT-001..002` |
| Tracing           | OTel-Adapter                       | `HSM-NFA-OBS-001`, `HSM-NFA-OBS-004`           |
| Metriken          | Prometheus-Adapter                 | `HSM-NFA-OBS-003`                              |
| Strukturierte Logs| Logging-Adapter                    | `HSM-NFA-OBS-002`                              |
| Konfiguration     | Config-Loader                      | `HSM-NFA-OPS-001`, `HSM-OPS-CFG-001..002`     |
| Geheimnisbezug    | Secret-Adapter                     | `HSM-FA-HSM-002`, `HSM-NFA-SEC-003`            |
| Health/Ready      | Health-Adapter                     | `HSM-API-CFG-001`, `HSM-FA-FAIL-009`           |

---

## 7. Verweise

- Vertragliche Anforderungen: [spec/lastenheft.md](lastenheft.md)
- Technische Verfahren und Detail-Codes: [spec/spezifikation.md](spezifikation.md)
- Bedrohungsmodell: Lastenheft Kapitel 13
- Architekturentscheidungen: [docs/plan/adr/](../docs/plan/adr/)
