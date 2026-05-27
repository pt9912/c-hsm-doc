# c-hsm-doc

**HSM-backed document encryption service — Go server, Java client.**

`c-hsm-doc` is a hardware-backed cryptographic service that encrypts
and decrypts documents of arbitrary size using AES-256-GCM. All AES
operations run inside an HSM via PKCS#11; key material never leaves
the HSM. The Go-based server speaks gRPC; a Java 21 client library
streams documents to and from it.

> **Sprachversion:** Die deutsche Variante dieses README liegt unter
> [`README.de.md`](README.de.md). Lastenheft, Spezifikation und
> Architektur sind auf Deutsch verfasst (siehe `spec/`).

## Status

**Bootstrap — no production code yet.** Specification, architecture
and planning are in place; the build pipeline runs against a
placeholder `main.go`. Implementation starts with milestone M1
(see [`docs/plan/planning/in-progress/roadmap.md`](docs/plan/planning/in-progress/roadmap.md)).

| Phase                        | Status         | Source                                                                                  |
| ---------------------------- | -------------- | --------------------------------------------------------------------------------------- |
| Lastenheft (contractual)     | Draft 0.2      | [`spec/lastenheft.md`](spec/lastenheft.md)                                              |
| Spezifikation (technical)    | Draft 0.2      | [`spec/spezifikation.md`](spec/spezifikation.md)                                        |
| Architecture                 | Draft 0.1      | [`spec/architecture.md`](spec/architecture.md)                                          |
| Architecture decisions       | 2 ADRs         | [`docs/plan/adr/`](docs/plan/adr/)                                                      |
| Roadmap                      | M1..M4 defined | [`docs/plan/planning/in-progress/roadmap.md`](docs/plan/planning/in-progress/roadmap.md)|

## Quickstart

The build is **Docker-only** (ADR 0002): no Go toolchain on the host
is required. Only Docker and `make` need to be installed.

```bash
make help            # list all targets
make build           # build the runtime image (distroless static, nonroot)
make run             # smoke test: docker run c-hsm-doc-server --version
```

Inner-loop quality gates:

```bash
make lint            # golangci-lint
make test            # go test ./...
make coverage-gate   # coverage gate (bootstrap-aware, ADR 0002 §2.5)
make gates           # lint + test + coverage-gate
make ci              # gates + govulncheck
make fullbuild       # ci + build (full closure)
```

Local SoftHSM token for development (HSM-ENV-003):

```bash
make dev-softhsm     # initialize the SoftHSM token in the compose volume
make dev-down        # tear down the compose environment (volume preserved)
```

## Repository layout

```text
.
├── cmd/hsmdoc/             # Go server entry point (bootstrap placeholder)
├── internal/               # hexagonal layout (see spec/architecture.md)
├── scripts/                # build helpers (coverage-gate.sh)
├── dev/softhsm/            # dev-only SoftHSM init container
├── spec/                   # Lastenheft, Spezifikation, Architektur
├── docs/                   # ADRs and planning (Kanban buckets)
├── Dockerfile              # multi-stage Go build (ADR 0002)
├── Makefile                # docker-only workflow
├── .dockerignore           # build context filter
├── docker-compose.dev.yml  # local SoftHSM dev environment (HSM-ENV-003)
└── go.mod
```

## Documentation

- **Lastenheft** (contractual specification, German):
  [`spec/lastenheft.md`](spec/lastenheft.md).
- **Technische Spezifikation** (technical bindings, freely amendable
  per `HSM-LESE-004`):
  [`spec/spezifikation.md`](spec/spezifikation.md).
- **Architecture overview** (components, deployment, trust
  boundaries, sequences):
  [`spec/architecture.md`](spec/architecture.md).
- **Architecture Decision Records:**
  [`docs/plan/adr/`](docs/plan/adr/).
- **Planning artefacts (slices, roadmap):**
  [`docs/plan/planning/{open,next,in-progress,done}/`](docs/plan/planning/).
- **Quality gates:**
  [`docs/user/quality.md`](docs/user/quality.md).

## Prerequisites

For building from source:

- Docker Engine
- GNU `make`

There is intentionally no host Go toolchain requirement (ADR 0002).

## License

MIT — see [`LICENSE`](LICENSE).
