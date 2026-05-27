# ADR 0003 — Plattform- und Service-Mesh-Neutralität

**Status:** Accepted
**Datum:** 2026-05-27
**Bezug:** [Lastenheft](../../../spec/lastenheft.md)
(`HSM-ENV-001..004`, `HSM-API-GRPC-002..003`, `HSM-PE-003`,
`HSM-MVP-005`, `HSM-ACCEPT-003`, `HSM-ACCEPT-005`, `HSM-FA-FAIL-002`,
`HSM-FA-TENANT-001..002`, `HSM-OPS-CFG-002`, `HSM-THREAT-002`),
[ADR 0001](0001-documentation-and-planning-structure.md),
[ADR 0002](0002-docker-only-build-pipeline.md)

---

## 1. Kontext

Das Lastenheft hatte Kubernetes (`HSM-ENV-002`) zunächst als
zentralen Betriebsmodus geführt und mTLS (`HSM-API-GRPC-003`) am
Server terminiert. Sobald ein Service Mesh wie Istio (Sidecar oder
Ambient), Linkerd oder ein vergleichbarer Layer (z. B. Cilium mit
Envoy) zwischen Caller und Server steht, wechselt der mTLS-
Terminationspunkt vom Server zum Sidecar/Knoten-Proxy.

Daraus folgen drei harte Konsequenzen:

1. Der Server sieht in dem Fall **nicht mehr** das originale
   Client-Zertifikat. Die in `HSM-FA-TENANT-001..002` und
   `HSM-FA-AUDIT-001` geforderte Ableitung des Audit-Felds `caller`
   und der `tenant_id` aus dem mTLS-Subject bricht ohne weitere
   Maßnahme.
2. Ein Mesh ist kein zwingender Bestandteil der Zielumgebung.
   Betreiber führen den Service auch als reinen Bare-Container
   (Docker/Podman) oder in Kubernetes ohne Mesh; der Service darf
   weder das Vorhandensein eines Orchestrators noch eines Meshs als
   Voraussetzung treffen.
3. `HSM-THREAT-002` benennt das Injizieren eines Sidecars durch
   einen Cluster-Admin explizit als Bedrohung — eine Identitäts-
   ableitung aus Headern, die jeder mit Pod-Zugriff setzen kann,
   wäre eine direkte Eskalation dieser Bedrohung.

Offene Frage: Wie wird Plattform-Neutralität operationalisiert, ohne
die mTLS-Identität in Mesh-Umgebungen zu verlieren oder durch
spoofbare Header zu ersetzen?

---

## 2. Entscheidung

### 2.1 Vier kanonische Betriebsmodi

Der Service MUSS in den vier in `HSM-ENV-004` festgeschriebenen
Modi lauffähig sein:

| # | Modus                                                   | mTLS-Terminierung     | Identitätsquelle (Default) |
| - | ------------------------------------------------------- | --------------------- | -------------------------- |
| 1 | Bare-Container (Docker/Podman/containerd)               | Server                | `mtls-subject`             |
| 2 | Kubernetes ohne Service Mesh                            | Server                | `mtls-subject`             |
| 3 | Kubernetes mit L4-Passthrough-Mesh (Istio Ambient ztunnel-only, Linkerd ohne mTLS-Termination) | Server | `mtls-subject`             |
| 4 | Kubernetes mit L7-/mTLS-terminierendem Mesh (Istio Sidecar mit `PeerAuthentication STRICT`, Linkerd-Proxy mit Mesh-mTLS) | Mesh-Sidecar | `header`                   |

Modi 1–3 sind aus Server-Sicht identisch: das originale Client-
Zertifikat ist am Server-Endpunkt sichtbar. Nur Modus 4 trennt
Transport-Identität (Mesh) von Anwendungs-Identität (Header).

### 2.2 Zwei konfigurierbare Identitätsquellen

Der Server bietet eine Konfigurationseinstellung
`identity.source` mit den Werten:

- `mtls-subject` (Default): Subject-Name oder SAN aus dem am Server
  terminierten Client-Zertifikat.
- `header`: ein konfigurierbarer Header trägt die Identität (z. B.
  `x-forwarded-client-cert`, ein SPIFFE-ID-Header oder ein
  JWT-Claim).

Es gibt **keine** Auto-Detection. Der Betreiber wählt die Quelle
passend zum gewählten Betriebsmodus; ein Konfigurationsfehler MUSS
gemäß `HSM-OPS-CFG-002` einen harten Start-Abbruch auslösen, kein
Silent-Fallback.

### 2.3 Peer-Allowlist als Sicherheitsanker für `header`

Wird `identity.source=header` gesetzt, MUSS eine nicht-leere
Allowlist vertrauenswürdiger Peers konfiguriert sein. Vertrauen
wird über mindestens eines der folgenden Kriterien hergestellt:

- die Peer-Adresse (z. B. Loopback `127.0.0.1`/`::1` aus demselben
  Pod bei Sidecar-Termination),
- ein vom Mesh am Transport-mTLS präsentiertes Sidecar-Zertifikat
  mit definierter SAN (SPIFFE-ID, DNS-Name),
- eine Kombination beider.

Liegt diese Allowlist nicht vor oder ist leer, MUSS der Service
nicht starten. Anfragen von nicht-allowlisteten Peers werden mit
`UNAUTHENTICATED` abgewiesen, bevor der Header überhaupt gelesen
wird. Das schließt die Eskalation aus `HSM-THREAT-002` (Sidecar-
Injektion durch Cluster-Admin) auf das Niveau des Bedrohungsmodells,
das das Mesh selbst liefert.

### 2.4 Audit-`caller` ist quellenneutral

Das Audit-Feld `caller` (`HSM-FA-AUDIT-001`, `spec/spezifikation.md`
§Audit-Schema) trägt unabhängig von der Identitätsquelle den
abgeleiteten Identitäts-String. Eine Konvention für das Format
(z. B. SPIFFE-ID-URI in Modus 4, X.509-Subject-DN in Modi 1–3)
gehört in die Technische Spezifikation, nicht in diese ADR.

### 2.5 Auslieferungsartefakte

- **Helm-Chart** ist Pflicht für die Kubernetes-Modi (2–4) und
  bleibt der primäre Auslieferungspfad für Cluster-Betreiber.
- **`docker run`/`podman run`-Beispielaufruf** und das bestehende
  `docker-compose.dev.yml` (aus ADR 0002 §2.6) decken Modus 1 ab.
- Mesh-spezifische Konfigurationsbeispiele (Istio `PeerAuthentication`,
  Linkerd-Annotationen, Ambient-Mesh-`namespace`-Labels) liegen
  als Beispiel-Manifeste neben dem Helm-Chart, sind aber nicht
  Bestandteil des Chart-Defaults.

### 2.6 Was nicht Bestandteil dieser ADR ist

- **Wahl konkreter Header-Namen** und ihrer Parsing-Regeln (z. B.
  `x-forwarded-client-cert` vs. `x-spiffe-id`) — gehört in die
  Technische Spezifikation, abhängig vom konkreten Mesh-Profil.
- **SPIFFE/SPIRE-Integration** als zweite Vertrauenswurzel (siehe
  `HSM-THREAT-002` Restrisiko-Erwägung) — eigene Folge-ADR, wenn
  und falls ein Betreiber diese Härtungsstufe nachfragt.
- **NetworkPolicy-Vorgaben** für die Kubernetes-Modi — gehört zum
  Helm-Chart-Härtungs-Slice, nicht in diese ADR.
- **Token-Issuer-Pfad** (OAuth/OIDC-basierte Identität jenseits von
  mTLS-Headern) — das Lastenheft erwähnt diesen Pfad in
  `HSM-NONGOAL-005` und `lastenheft.md` §15-Annahme; eine
  Implementierung wäre eine eigene Identitätsquelle und eine
  eigene ADR.

---

## 3. Konsequenzen

- Der Service trägt eine Pflicht-Konfiguration `identity.source`
  mit Default `mtls-subject`; eine missing-by-default Allowlist
  bei `header` bedeutet bewussten Start-Fehlschlag, kein
  Silent-Allow.
- M2-Test-Suite muss zweigleisig sein: `M2-DoD-06` deckt das
  bereits ab; die konkrete Mesh-Test-Infrastruktur (z. B. ein
  Kind-Cluster mit Istio Ambient bzw. mit `PeerAuthentication`) ist
  Slice-Arbeit innerhalb von M2.
- Betriebs-Akzeptanz (`HSM-ACCEPT-005`) verlangt jetzt zwei Pfade:
  `docker run` für Modus 1 und `helm install` für Modus 2. Modi 3+4
  sind durch das Helm-Chart + Mesh-Beispielmanifeste erreichbar,
  aber nicht eigenständig abnahmepflichtig in M2.
- Restrisiko `HSM-THREAT-002` (Sidecar-Injektion) bleibt; die
  Peer-Allowlist verhindert nur die Spoofbarkeit innerhalb der
  bestehenden Cluster-Bedrohung, nicht eine Cluster-Admin-Übernahme
  als solche. Weitergehende Härtung liegt bei einer optionalen
  SPIFFE/SPIRE-Folge-ADR.
- Audit-Schema bleibt rückwärtskompatibel: `caller` ist ein
  String-Feld; nur seine Quelle wechselt je nach Modus.

---

## 4. Pflege-Regeln

- Neue Mesh-Profile (z. B. ein neues L7-Mesh-Produkt) führen nicht
  zu einer neuen ADR, solange sie in eines der vier Modi-Schubladen
  fallen. Sie werden als Beispiel-Manifest neben dem Helm-Chart
  ergänzt.
- Neue Identitätsquellen jenseits von `mtls-subject` und `header`
  (z. B. ein OIDC-Token-Pfad) lösen eine Folge-ADR aus.
- Änderungen an der Peer-Allowlist-Pflicht (etwa eine Auto-
  Discovery-Variante) lösen eine Folge-ADR aus, weil sie das
  Sicherheits-Argument aus §2.3 verschieben.

---

## 5. Nicht Gegenstand dieser ADR

- Konkrete Helm-Chart-Werte (`values.yaml`-Layout), Default-Resource-
  Limits, NetworkPolicy-Templates — Slice-Plan im Querschnitt von
  M1/M2.
- Konkretes Mapping von Header-Werten auf `tenant_id` (Header-zu-
  Tenant-Regelwerk) — Technische Spezifikation.
- Performance-Implikation der Mesh-Modi (zusätzliche TCP-Hops über
  ztunnel/Sidecar) — Messung in M3 gegen das Produktionsprofil.
