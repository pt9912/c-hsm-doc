# internal/

Nicht-exportierbare Go-Pakete für `c-hsm-doc-server`. Strukturiert
nach dem hexagonalen Architektur-Pattern aus
[`HSM-ARCH-001`](../spec/lastenheft.md) und
[`spec/architecture.md`](../spec/architecture.md).

## Geplantes Layout

```text
internal/
├── hexagon/
│   ├── domain/              # reine Datentypen (Chunk, Container, KeyID), keine I/O
│   ├── application/         # Use-Cases (encrypt-stream, decrypt-stream, key-rotate); ruft nur Ports auf
│   └── port/
│       ├── driving/         # Interfaces, die der gRPC-Adapter konsumiert
│       └── driven/          # Interfaces nach außen (PKCS#11, Audit, Metrics, Secrets)
└── adapter/
    ├── driving/             # gRPC-Server-Adapter, Health-Adapter
    └── driven/              # pkcs11/, audit/, otel/, secrets/, …
```

## Status

Aktiver Stand (Slice 002a):

- `internal/config/` — 12-Factor-Konfiguration mit Start-Abbruch bei
  Validierungsfehlern (Slice 001).
- `internal/adapter/driving/grpc/` — gRPC-Server-Stub, alle
  RPC-Methoden liefern `codes.Unimplemented` (Slice 001).
- `internal/adapter/driving/health/` — HTTP-Health-/Ready-Adapter
  (Slice 001).
- `internal/gen/chsmdocv1/` — generierter Protobuf-Code aus
  `spec/proto/` (Slice 001).

Das Coverage-Gate ist **nicht mehr im Bootstrap-Modus**: Slice 001
hat es per M1-DoD-05 abgehakt, aktueller Stand liegt ≥ 80 % gemäß
ADR 0002 §2.5.

Das oben skizzierte `internal/hexagon/`-Layout (`domain/`,
`application/`, `port/{driving,driven}/`) wird mit Slice 002b
(PKCS#11-Adapter + Encrypt-Hexagon) eingezogen; siehe
[`docs/plan/planning/next/002b-pkcs11-encrypt-hexagon.md`](../docs/plan/planning/next/002b-pkcs11-encrypt-hexagon.md).
