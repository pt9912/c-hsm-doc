# 006 — Identitätsquelle und Peer-Allowlist (`identity.*`-Konfiguration)

**Meilenstein:** M2 (siehe [`roadmap.md`](../in-progress/roadmap.md))
**Status:** `next` (Scope skizziert, noch nicht aktiv)
**Datum:** 2026-05-27

## Ziel

Setzt `HSM-API-GRPC-006..008` aus
[`spezifikation.md`](../../../../spec/spezifikation.md) und damit die in
[ADR 0003](../../adr/0003-plattform-und-mesh-neutralitaet.md) §2.2–2.4
fixierte Plattform-Neutralität konkret um:

- Der Server kennt zwei Identitätsquellen (`mtls-subject`, `header`).
- Die `header`-Quelle ist nur in Verbindung mit einer Peer-Allowlist
  zulässig; ohne sie scheitert der Start hart.
- Das Audit-Feld `caller` wird deterministisch aus der gewählten
  Quelle abgeleitet.
- Die Reject-Pfade sind metrisch und im Audit sichtbar.

Damit schließt der Slice gleichzeitig `M2-DoD-06`
(`roadmap.md`-Zeile zur zweigleisigen mTLS-Abnahme) und erfüllt
`HSM-ACCEPT-003` (Security-Abnahme in den Modi 1+2+4 aus
`HSM-ENV-004`).

## Scope

### Konfiguration (`HSM-API-GRPC-006`)

- Neue Konfig-Sektion `identity.*` (12-Factor: ENV-Variablen
  `HSMDOC_IDENTITY_SOURCE`, `HSMDOC_IDENTITY_MTLS_SUBJECT_ATTRIBUTE`,
  `HSMDOC_IDENTITY_HEADER_NAME`, `HSMDOC_IDENTITY_HEADER_FORMAT`,
  `HSMDOC_IDENTITY_PEER_ALLOWLIST` — letzteres als
  Komma-/Newline-getrennte Liste).
- Defaults: `source=mtls-subject`, `subject_attribute=subject_dn`,
  `header.name=x-spiffe-id`, `header.format=spiffe`, leere Allowlist.
- Validierung beim Start gemäß `HSM-API-GRPC-006`:
  - `source=mtls-subject` ohne Client-CA → harter Start-Abbruch
    (`HSM-OPS-CFG-002`),
  - `source=header` mit leerer Allowlist → harter Start-Abbruch,
  - unbekannter Enum-Wert in `source` oder `header.format` →
    harter Start-Abbruch.

### gRPC-Interceptor (`HSM-API-GRPC-007`)

- Pflicht-Server-Interceptor `identityInterceptor` läuft als erster
  Unary- und Stream-Interceptor in `internal/adapter/driving/grpc/`.
- Quelle `mtls-subject`: extrahiert Identität aus
  `peer.AuthInfo` (TLS-Peer-Zertifikat), Attribut je
  `subject_attribute`-Config.
- Quelle `header`:
  1. liest Peer-Adresse aus `peer.FromContext`,
  2. matched gegen Allowlist (Eintragsformate `ip:`, `cidr:`,
     `spiffe:`, `san-uri:`, `san-dns:`),
  3. **erst danach** wird der Identitäts-Header gelesen und je
     `header.format` (`spiffe` / `xfcc` / `raw`) geparst,
  4. Allowlist-Miss → `codes.Unauthenticated`, Audit-Eintrag mit
     `caller=anonymous@<peer-addr>`, `error_class=UNAUTHENTICATED`,
     `result=error`.
- Sentinel-Audit nutzt denselben Audit-Adapter wie reguläre Aufrufe;
  kein Side-Channel.

### Audit-Anbindung (`HSM-API-GRPC-008`)

- `caller`-Ableitung aus der `IdentityContext`-Struktur, die der
  Interceptor in den Request-Kontext einträgt. Tabelle aus
  `HSM-API-GRPC-008` 1:1 abgebildet.
- Bei akzeptiertem Aufruf mit leerer Identität (sollte nicht
  vorkommen): `error_class=IDENTITY_MISSING`, gRPC-Status `INTERNAL`,
  gemäß Statuscode-Tabelle (`HSM-API-GRPC-005`).

### Observability

- Neue Prometheus-Metrik
  `hsmdoc_identity_peer_rejected_total{reason}` mit den vier
  reasons aus `HSM-NFA-OBS-003` (`not_in_allowlist`,
  `tls_handshake_failed`, `header_missing`, `header_malformed`).
- Strukturierte Logs gemäß `HSM-NFA-OBS-002` (`caller` und
  `tenant_id` schon Pflichtfelder; jetzt mit echten Werten).

### Tests (M2-DoD-06 zweigleisig)

- Unit-Tests für den Interceptor:
  - happy-path je `source` × `subject_attribute` × `header.format`,
  - Reject-Pfade je Allowlist-Eintragsformat (ip, cidr, spiffe, san),
  - Start-Validierung scheitert deterministisch bei leerer
    Allowlist + `source=header`.
- Integrationstest **Modus 1+2** (Bare/K8s ohne Mesh): gRPC-Client
  ohne Zertifikat → `UNAUTHENTICATED`; mit gültigem Zertifikat
  erscheint Subject im Audit-`caller`.
- Integrationstest **Modus 4** (Mesh-Termination): zweiter Listener-
  Pfad simuliert Sidecar (Loopback + Identitäts-Header). Anfragen
  von außerhalb der Allowlist → `UNAUTHENTICATED`; von innerhalb →
  Header-Identität als `caller`.
  Test verwendet einen In-Process-Fake-Sidecar, **kein** echter
  Istio-/Linkerd-Stack — ein Demonstrationspfad gegen einen echten
  Stack ist optionaler Folge-Slice.

## Vorbedingungen für die Aktivierung

1. **Slice 001 (gRPC-Skeleton)** in `done/`. Dieser Slice setzt auf
   einer bestehenden gRPC-Server-Struktur auf und kann den
   Skeleton-`UNIMPLEMENTED`-Pfad als happy-path-Träger benutzen.
2. **Slice 004 (Basis-Audit-Log)** in `done/`. Reject-Pfade
   schreiben Audit-Einträge; der Audit-Adapter muss existieren.
3. **ADR 0003** ist `Accepted` (gegeben — siehe ADR-Index).
4. **Konfigurations-Loader** muss `HSM-OPS-CFG-002` (Fail-Fast bei
   Konfigurationsfehler) bereits umsetzen. Wenn nicht aus 001 oder
   einem Zwischen-Slice vorhanden: parallel ziehen.

## Akzeptanzkriterien

- `make ci` grün; Unit-Test-Coverage auf
  `internal/adapter/driving/grpc/` mindestens auf
  Projekt-Schwellwert.
- Integrationstest **Modus 1+2** (`mtls-subject`): Reject und
  Accept beide grün — entspricht
  [`HSM-ACCEPT-003`](../../../../spec/lastenheft.md) Teil (a).
- Integrationstest **Modus 4** (`header` + Fake-Sidecar): Reject
  und Accept beide grün — entspricht `HSM-ACCEPT-003` Teil (b).
- Start-Abbruch-Tests: Server startet nicht bei
  `source=header` ∧ leerer Allowlist; Fehler-Message zeigt auf
  `HSM-API-GRPC-006`.
- Metrik `hsmdoc_identity_peer_rejected_total` wird in den Reject-
  Tests inkrementiert (Smoke via `/metrics`-Endpoint).
- `M2-DoD-06` in `roadmap.md` wird mit diesem Slice von `[ ]` auf
  `[x]` gehoben.
- Sicherheits-Review: Reject-Audit enthält genug Forensik
  (`peer-addr`, `reason`), aber kein verleakter Header-Inhalt
  (insbesondere kein Cookie / kein Authorization-Header — der
  konfigurierte Identitäts-Header darf protokolliert werden, andere
  Request-Header nicht).

## Abgrenzung — NICHT in diesem Slice

- Keine echte Istio-/Linkerd-Test-Pipeline (Fake-Sidecar reicht für
  das funktionale DoD; ein realer Mesh-Smoke-Test ist optional und
  würde einen eigenen Slice nach M2 lohnen).
- Kein SPIFFE/SPIRE-Trust-Bundle-Pfad — bleibt eine offene
  Folge-ADR (ADR 0003 §2.6).
- Keine Tenant-Mapping-DSL (`caller` → `tenant_id`). Bleibt
  Betreiber-Konfiguration; ein generisches Mapping-Modul wäre
  eigener Slice in M4.
- Keine NetworkPolicy- oder Istio-`PeerAuthentication`-Manifeste im
  Helm-Chart-Default (eigene Slice-Empfehlung im Helm-Chart-Slice).
- Keine Token-Issuer-Variante (OAuth/OIDC) — `HSM-NONGOAL-005`
  bleibt gültig.

## Bezug

- [Lastenheft `HSM-API-GRPC-003`, `HSM-ENV-004`, `HSM-ACCEPT-003`, `HSM-FA-TENANT-001..002`, `HSM-OPS-CFG-002`](../../../../spec/lastenheft.md)
- [Spezifikation `HSM-API-GRPC-005..008`, `HSM-NFA-OBS-002..003`, `HSM-DATA-001`](../../../../spec/spezifikation.md)
- [Architektur Kapitel 6 (Querschnitts-Themen)](../../../../spec/architecture.md)
- [ADR 0003 — Plattform- und Service-Mesh-Neutralität](../../adr/0003-plattform-und-mesh-neutralitaet.md)
- [Roadmap M2-DoD-06](../in-progress/roadmap.md)
