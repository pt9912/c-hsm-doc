# 002 — Runtime-Base auf `distroless/base` umstellen, sobald CGO/PKCS#11 dazukommt

**Status:** `done` (eingelöst durch [Slice 002a](002a-cgo-build-pipeline.md)
am 2026-05-28).
**Trigger:** Befund L1 aus dem Build-Pipeline-Security-Review
(2026-05-26).
**Umsetzung:** Dockerfile-Runtime-Base wechselt auf
`gcr.io/distroless/base-debian12:nonroot`; `build`-Stage schaltet
`CGO_ENABLED=1`; neue `deps-closure`-Stage ermittelt die transitive
Shared-Library-Closure deterministisch über `lddtree` aus `pax-utils`
und stagt sie in das Runtime-Image; `closure-check`-Stage verifiziert
Build-Time, `pkcs11-dlopen-smoke`-Binary verifiziert Runtime. ADR 0004
(Runtime-Base CGO/PKCS#11) trägt die Begründung und Messwerte
(Runtime-Image 43,9 MiB, Trivy 0 HIGH/CRITICAL). Lieferung im
Slice-002a-Implementations-Commit `ec77196` (2026-05-27).

## Beobachtung

Die Runtime-Stage im [`Dockerfile`](../../../../Dockerfile) basiert
heute auf `gcr.io/distroless/static-debian12:nonroot`. Das ist korrekt
für pure-Go-Binaries mit `CGO_ENABLED=0` (HSM-NFA-SEC-007).

Sobald der Server real auf einen PKCS#11-Adapter zugreift
(`HSM-FA-HSM-001..004`, M1-Roadmap), wird das Go-Binary `libsofthsm2.so`
bzw. ein Vendor-`.so`-Modul dynamisch laden. Dafür braucht es eine
funktionierende libc (glibc oder musl). Das `static`-Distroless-Image
hat sie nicht.

Falsche Default-Pfad bei M1: Build schlägt entweder beim Linken oder
beim ersten `dlopen()` zur Laufzeit fehl — beides spät und schwer
diagnostizierbar.

## Aktivierungsbedingung

Erster Slice von M1, der `github.com/miekg/pkcs11` (oder ein
Vendor-Modul) importiert und damit CGO erzwingt.

## Ergebnis

Slice-Plan unter `docs/plan/planning/next/` mit folgenden Schritten:

1. `Dockerfile`: `RUNTIME_BASE_IMAGE`-Default auf
   `gcr.io/distroless/base-debian12:nonroot` wechseln.
2. `build`-Stage: `CGO_ENABLED=1` setzen, dynamisches Linken erlauben.
3. Vendor-`.so`-Pfade ins Runtime-Image kopieren (separate Stage oder
   `COPY --from=…`).
4. ADR 0002 §2.7 ergänzen oder Folge-ADR aufmachen, die den Wechsel
   begründet (HSM-NFA-SEC-007 wird durch `distroless/base` ebenso
   erfüllt; keine Shell, kein Paketmanager).
5. Image-Größe und Härtung gegenprüfen (`docker history`, Trivy-Scan).

## Bezug

- [Dockerfile `runtime`-Stage](../../../../Dockerfile)
- ADR 0002 §2.7 (Distroless-nonroot als Runtime-Base)
- HSM-NFA-SEC-007 (`spec/lastenheft.md`)
- HSM-FA-HSM-001..004 (`spec/lastenheft.md`)
- Roadmap M1 (`docs/plan/planning/in-progress/roadmap.md`)
