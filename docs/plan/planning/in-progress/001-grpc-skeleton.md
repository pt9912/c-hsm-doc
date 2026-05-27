# 001 — gRPC-Skeleton für `c-hsm-doc-server`

**Meilenstein:** M1 (siehe [`roadmap.md`](roadmap.md))
**Status:** `in-progress` (aktiv ab 2026-05-27)
**Datum:** 2026-05-27

## Ziel

Den gRPC-Endpoint des Go-Servers als ausführbares Skeleton bereitstellen.
Alle Service-Methoden liefern `codes.Unimplemented` zurück. Damit ist
die Vertrauenskette Client → TLS → gRPC → Server-Stub end-to-end
verkabelt, bevor HSM-Logik dazu kommt.

Dieser Slice ist der erste echte M1-Schritt nach dem Bootstrap-Stand.

## Scope

- `spec/proto/c_hsm_doc.proto`: Proto3-Definition mit Service
  `HsmDocService` und Methoden `Encrypt` (bidi-stream), `Decrypt`
  (bidi-stream), `ListKeys`, `Health` gemäß
  [`HSM-API-GRPC-004`](../../../../spec/spezifikation.md) und
  [`HSM-API-GRPC-001`](../../../../spec/lastenheft.md).
- `internal/adapter/driving/grpc/`: gRPC-Server-Stub mit allen
  Methoden, die `status.Error(codes.Unimplemented, …)` zurückliefern.
- `cmd/hsmdoc/main.go`: ersetzt den Bootstrap-Placeholder, bindet
  gRPC auf konfigurierbaren Port (Default `9443`), behandelt `SIGTERM`
  für Graceful Shutdown (`HSM-NFA-OPS-002`).
- TLS 1.3 mit lokal generiertem Self-Signed-Cert für Dev (Compose
  mountet TLS-Material), kein mTLS in diesem Slice
  (`HSM-API-GRPC-002`).
- `/healthz` und `/readyz` als separate HTTP-Endpoints; beide liefern
  im Skeleton immer `200` (`HSM-API-CFG-001`).
- 12-Factor-Konfiguration aus Env-Variablen (`HSM-NFA-OPS-001`):
  `HSMDOC_GRPC_PORT`, `HSMDOC_HEALTH_PORT`, `HSMDOC_TLS_CERT`,
  `HSMDOC_TLS_KEY`.
- Generierter Protobuf-Code wird im Repo eingecheckt (kein `protoc`-
  Stage im Dockerfile — Entscheidung in Vorbedingungen).
- Unit-Test: in-process gRPC-Server akzeptiert Verbindung, Encrypt-
  Aufruf liefert `UNIMPLEMENTED`.

## Vorbedingungen für die Aktivierung

1. **ADR oder Slice-interne Entscheidung:** Generated Protobuf-Code
   eingecheckt vs. via Dockerfile-Stage generiert. Empfehlung:
   einchecken, kein neuer Build-Stage; spart Toolchain-Dep.
2. **Open-Trigger 001** (`go.sum` Strict-Mode) wird mit diesem Slice
   miterledigt — er triggert durch das erste echte `import`.
3. **Open-Trigger 002** (CGO-Base-Switch) bleibt offen; dieser Slice
   nutzt noch keinen PKCS#11-Adapter und kommt mit `CGO_ENABLED=0` aus.
4. **Coverage-Gate** wird mit diesem Slice aus dem Bootstrap-Modus
   gehoben: erster `make coverage-gate THRESHOLD=80` muss grün sein.

## Akzeptanzkriterien

- `make ci` läuft grün gegen den Skeleton-Code (lint, test, coverage
  ≥ 80 % auf `./internal/...`, docs-check, govulncheck).
- `make run` startet den Server; ein gRPC-Client (z. B. `grpcurl`)
  bekommt für `Encrypt` eine `UNIMPLEMENTED`-Antwort und für `Health`
  eine `SERVING`-Antwort.
- `HSM-MVP-006`-Vorbedingung erfüllt: kein JNI im Server, gRPC-Proto
  liegt vor und ist auch für den späteren Java-Client verwendbar.
- Open-Trigger 001 wird im selben PR nach `done/` migriert (oder
  explizit re-bewertet, falls sich der Scope geändert hat).
- Roadmap-M1-Status wird aktualisiert: Slice-Bestand zeigt diesen
  Slice und seine Folgeplanung.

## Abgrenzung — NICHT in diesem Slice

- Keine PKCS#11-Anbindung (separater Slice; siehe Open-Trigger 002).
- Kein Container-Codec, kein Chunk-State-Machine (Folge-Slices in M1).
- Keine echte AES-GCM-Logik (Folge-Slice).
- Keine mTLS-Pflicht (Folge-Slice; mTLS bleibt aber konfigurierbar
  vorbereitet).
- Keine Audit-Hash-Chain (M2).
- Keine Mandantenisolation jenseits eines hartkodierten
  `tenant_id=default` (volle Auflösung folgt in einem späteren M1-
  Slice oder M4).

## Geplante Slice-Folge in M1

| Nr.   | Slice                                | Aktiviert                                        |
| ----- | ------------------------------------ | ------------------------------------------------ |
| `001` | gRPC-Skeleton (dieser Slice)         | Open-Trigger 001 (`go.sum`)                      |
| `002` | PKCS#11-Adapter + Encrypt            | Open-Trigger 002 (CGO-Base)                      |
| `003` | Container-Codec + Decrypt            | `HSM-FMT-001..006`, `HSM-FA-CHUNK-004..007`      |
| `004` | Basis-Audit-Log mit Hash-Chain       | `HSM-FA-AUDIT-001..005`                          |
| `005` | Helm-Chart + Kind-Smoke              | `HSM-MVP-005`, `HSM-ACCEPT-005`                  |

## Bezug

- [Lastenheft `HSM-API-GRPC-001..003`, `HSM-MVP-001..006`, `HSM-NFA-OPS-001..003`](../../../../spec/lastenheft.md)
- [Spezifikation `HSM-API-GRPC-004..005`, `HSM-API-CFG-001..002`](../../../../spec/spezifikation.md)
- [Architektur Kapitel 2 (Komponenten), 5.1 (Encrypt-Stream-Sequenz)](../../../../spec/architecture.md)
- [Open-Trigger 001 — `go.sum` Strict-Mode](../open/001-gosum-strict-mode.md)
- [Open-Trigger 002 — CGO-Base-Switch](../open/002-distroless-base-fuer-cgo.md)
- [Roadmap M1](../in-progress/roadmap.md)
