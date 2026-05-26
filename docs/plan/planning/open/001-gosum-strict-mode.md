# 001 — `go.sum` Strict-Mode aktivieren

**Trigger:** Befund M4 aus dem Build-Pipeline-Security-Review
(2026-05-26).

## Beobachtung

Im Bootstrap-Stand existiert noch keine `go.sum`-Datei (keine
produktiven Imports). Der [`Dockerfile`](../../../../Dockerfile) nutzt
deshalb `COPY go.su[m] ./` mit Wildcard, damit der Build auch ohne
`go.sum` durchläuft. Damit sind die Modul-Integritäts-Hashes derzeit
nicht erzwungen.

Sobald produktive Imports (z. B. `google.golang.org/grpc`,
`github.com/miekg/pkcs11`, `go.opentelemetry.io/otel/...`) im Code
landen, MUSS:

1. `COPY go.su[m] ./` durch `COPY go.sum ./` ersetzt werden (strict),
2. eine zusätzliche `RUN go mod verify`-Zeile im `deps`-Stage des
   Dockerfile die Hashes prüfen,
3. ein Slice-Plan in `next/` die Migration sauber begleiten (Lockdown
   geht nur einmal, jedes spätere Bypass wäre ein CR).

## Aktivierungsbedingung

Erster Commit, der echte Imports in `go.mod` oder `internal/`
einbringt. Spätestens beim ersten Slice von Roadmap-Meilenstein M1.

## Ergebnis

Slice-Plan unter `docs/plan/planning/next/` mit Titel `00X-gosum-strict-mode.md`,
der die Migration in einen einzelnen Commit packt (Dockerfile-Change +
erstem `go.sum`-Inhalt + ggf. Anpassung der Pin-Politik in ADR 0002
§2.4 falls erforderlich).

## Bezug

- [Befund-Dokument: Security-Review-Output, Commit `c00a2c9`-Vorlauf](../../../../docs/plan/adr/0002-docker-only-build-pipeline.md)
- [Dockerfile `deps`-Stage](../../../../Dockerfile)
- ADR 0002 §2.4 (Pin-Politik)
