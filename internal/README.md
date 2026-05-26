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

Bootstrap. Noch keine produktiven Pakete. Das Coverage-Gate läuft im
Bootstrap-Modus (ADR 0002 §2.5); ein echter Schwellwert wird mit dem
Einzug echter Logik aktiviert (separater Slice in der Roadmap).
