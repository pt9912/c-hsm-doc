# 002a — CGO-Build-Pipeline für PKCS#11

**Meilenstein:** M1 (siehe [`roadmap.md`](../in-progress/roadmap.md))
**Status:** `next` (Scope skizziert, noch nicht aktiv)
**Datum:** 2026-05-27

## Ziel

Aus der heutigen `distroless/static`-Pipeline (pure-Go, `CGO_ENABLED=0`)
wird die CGO-fähige Pipeline, die der nachfolgende Slice 002b für den
PKCS#11-Adapter braucht. Slice 002a löst **ausschließlich** den
Build-Pfad ein — kein Adapter-Code, keine Hexagon-Schicht, kein
Encrypt-Stream. Damit wird Open-Trigger 002
([`002-distroless-base-fuer-cgo.md`](../open/002-distroless-base-fuer-cgo.md))
abgeschlossen und ADR 0004 als Schärfung von ADR 0002 angelegt
([ADR 0001 §2.3](../../adr/0001-documentation-and-planning-structure.md)).

Der Split aus dem ursprünglichen Slice 002 (Build-Pipeline + Encrypt-
Hexagon in einem Slice) ist eine bewusste Scope-Reduktion: 002a liefert
die Voraussetzung, 002b den fachlichen Kern. Beide Slices sind in M1,
die Folge-Slices (003 Decrypt, 004 Audit-Hash-Chain, 005 Helm-Chart)
hängen an 002b, nicht an 002a.

## Scope

### Build-Pipeline (Open-Trigger 002 einlösen)

- `Dockerfile`: `RUNTIME_BASE_IMAGE`-Default von
  `gcr.io/distroless/static-debian12:nonroot` auf
  `gcr.io/distroless/base-debian12:nonroot` umstellen.
- `build`-Stage: `CGO_ENABLED=1`, dynamisches Linken aktivieren.
- Vendor-`.so`-Pfad (`softhsm2.so` im Dev-Bild, beliebiges Vendor-
  Modul in Produktion) wird über separate `COPY --from=...`-Stage in
  das Runtime-Image gebracht. Lokale CI nutzt SoftHSM aus dem
  offiziellen Paket.
- **Transitive Shared-Library-Closure:** Distroless-base bringt nur
  `libc`/`libdl`/`libpthread` mit. Vendor-`.so`-Module ziehen
  typischerweise weitere Libraries nach (z. B. SoftHSM v2 →
  `libsoftokn3`, `libnss3`, `libsqlite3`; OpenCryptoki →
  `libica`, `libcrypto`). Eine zusätzliche Build-Stage ermittelt
  die Closure **deterministisch über `lddtree`** aus
  `pax-utils` (alternativ `scanelf -nB` + `readelf -d` für die
  NEEDED-/RPATH-/RUNPATH-Auflösung), nicht über rohes `ldd`-
  Parsing — `ldd` ist nicht für Stücklisten gedacht und bringt
  „not found"/RPATH-/`$ORIGIN`-Fallstricke. Der Build:
  1. Ruft `lddtree --root $RUNTIME_FS --list --skip-non-elfs
     $MODULE` in der `deps-closure`-Stage explizit gegen das
     Distroless-base-Sysroot auf. `$MODULE` ist dabei der Pfad des
     Moduls innerhalb dieses Sysroots, nicht der zufällig gleiche Pfad
     der Debian-Build-Stage.
  2. Schreibt die deduplizierte Pfadliste in
     `/build/pkcs11-libs.list` (Stückliste, als Build-Artefakt und im
     Runtime-Image prüfbar) **und** kopiert in derselben Stage jeden
     gelisteten Pfad nach `/staging/pkcs11-rootfs/` unter Erhalt der
     Verzeichnisstruktur (`install -D` pro Eintrag). Hintergrund:
     Dockerfile-`COPY` kann nicht über eine zur Build-Zeit erzeugte
     Liste iterieren — deshalb wird die Iteration in der
     Build-Stage gemacht und das fertige Staging-Verzeichnis als
     statischer Pfad übernommen.
  3. Im Runtime-Image: ein einziger
     `COPY --from=deps-closure /staging/pkcs11-rootfs/ /` bringt
     das PKCS#11-Modul plus alle transitiven Libraries ins Image.
     Die Stückliste wird parallel als
     `COPY --from=deps-closure /build/pkcs11-libs.list /etc/hsmdoc/pkcs11-libs.txt`
     ausgeliefert.
  4. Verifiziert die Closure **zweistufig**:
     - **Build-Time:** Eine separate `closure-check`-Stage auf
       Debian-Slim (mit `lddtree`/`ldd`) wird mit
       `lddtree --root $RUNTIME_FS` (pax-utils-Flag, nicht
       `--rootfs`) gegen den Runtime-Rootfs aufgerufen und prüft,
       dass `lddtree` keine „not found"-Einträge meldet.
       Distroless selbst hat keine Shell und kein `ldd`, deshalb
       passiert die Prüfung außerhalb des Runtime-Images, aber
       gegen denselben Dateibaum.
     - **Runtime-Smoke:** Ein winziges Go-Helper-Binary
       `cmd/pkcs11-dlopen-smoke/` (in derselben Build-Stage
       erzeugt, ins Runtime-Image kopiert) ruft
       `dlopen($MODULE, RTLD_NOW)` auf; Fehlschlag → Exit-Code
       ≠ 0 mit `dlerror()`-Output. Das deckt RPATH-/`$ORIGIN`-
       Auflösung **echt zur Laufzeit** ab und braucht keine
       Shell im Distroless.
       **Aufrufpunkte in 002a:**
       - `make smoke-dlopen` ruft das Binary außerhalb des
         Service-Starts auf (CI-Pfad, Forensik, manuelle Diagnose).
       - Der Startup-Synchron-Hook im `hsmdoc`-Hauptprozess
         (Smoke vor `C_Initialize`/Pool-Aufbau, Exit-Code ≠ 0 →
         `STARTUP_PKCS11_DLOPEN_FAILED`) wird in 002b verdrahtet,
         sobald der Service real `C_Initialize` ausführt. In 002a
         existiert das Binary, aber der Server hat noch keinen
         PKCS#11-Initialisierungspfad — der Smoke-Aufruf ist
         deshalb in 002a CI-only.
  Die Stückliste wird im ADR 0004 dokumentiert und im Image als
  `/etc/hsmdoc/pkcs11-libs.txt` ausgeliefert (Forensik-Hilfe).
- **Folge-ADR** (geplant: `ADR 0004 — Runtime-Base für CGO/PKCS#11`)
  begründet den Wechsel auf `distroless/base` und das Aktivieren von
  CGO. ADR 0002 ist `Accepted` und bleibt nach
  [ADR 0001 §2.3](../../adr/0001-documentation-and-planning-structure.md)
  inhaltlich unverändert; der ADR-Index trägt 0004 als „Schärfung von
  0002" ein
  ([`HSM-NFA-SEC-007`](../../../../spec/lastenheft.md) bleibt erfüllt:
  keine Shell, kein Paketmanager).
- Image-Größe + Trivy-Scan gegenprüfen, Werte in ADR 0004 aufnehmen.
  002a führt dafür ein Docker-only Make-Target `make image-scan` ein:
  `TRIVY_VERSION` und `TRIVY_BASE_IMAGE` werden im `Makefile` gepinnt,
  der Scan läuft über das Trivy-Container-Image gegen das gebaute
  Runtime-Image und schreibt ein reproduzierbares Artefakt unter
  `out/security/trivy-runtime.json`. Die Größenmessung wird ebenfalls
  als Make-Target/Script ausgeführt und schreibt
  `out/security/image-size.txt` mit alter Referenz (`distroless/static`-
  Stand aus Slice 001) und neuer Größe (`distroless/base` + Closure).
- Open-Trigger 002 nach `done/` migrieren (analog zu Open-Trigger 001
  durch Slice 001).

### CI-Vorbereitung für 002b

- **SoftHSM v2 im CI-Build-Image** verfügbar machen (`apt-get install
  softhsm2` oder ein vorgefertigtes Test-Image). Das Bild basiert
  explizit auf Debian 12/Bookworm, passend zur Runtime
  `distroless/*-debian12`; die Closure wird nicht aus einem
  andersartigen Debian-/Ubuntu-/Alpine-Rootfs übernommen. 002a
  aktiviert den Mechanismus noch nicht im Service, stellt aber sicher,
  dass das CI-Image SoftHSM v2 ausführen kann — Voraussetzung für 002b.
- **Zweites herstellerfremdes OSS-PKCS#11-Modul** für den Vendor-
  Smoke aus 002b im CI-Image bereitstellen. Default: OpenCryptoki
  (ICA-/Software-Token-Modus). Falls OpenCryptoki im CI nicht stabil
  bedienbar ist, wird die Alternative (z. B. Mozilla-NSS-Softoken)
  in ADR 0004 festgelegt. Die Auswahl ist Teil von 002a, die
  funktionale Validierung gegen `CKM_AES_GCM` + `CKM_HKDF_DERIVE`
  läuft erst in 002b (Vorbedingung 3 dort).

## Vorbedingungen für die Aktivierung

1. **Slice 001** ist nach `done/` migriert (Akzeptanzkriterien laut
   [`001-grpc-skeleton.md`](../in-progress/001-grpc-skeleton.md) §
   „Akzeptanzkriterien" erfüllt). Ohne Skeleton kein Anschluss-Point.
2. **Open-Trigger 002** ist noch in `open/` (gegeben — siehe
   [`002-distroless-base-fuer-cgo.md`](../open/002-distroless-base-fuer-cgo.md)).
   Dieser Slice löst ihn ein.
3. **Coverage-Schwellwert ≥ 80 %** bleibt erhalten — der CGO-Switch
   darf keinen Coverage-Regress erzeugen. In 002a fällt noch kein
   PKCS#11-Adapter-Code an, deshalb gibt es auch keinen
   Integrationstest-Pfad. **Mechanik in 002a unverändert gegenüber
   Slice 001:** Coverage läuft weiter in der bestehenden Docker-
   `coverage`-Stage mit `CGO_ENABLED=0 go test ... ./...` (siehe
   [`Dockerfile`](../../../../Dockerfile) §coverage); nur der
   `build`-Stage schaltet auf `CGO_ENABLED=1`. Die Aggregations-
   Mechanik (`gocovmerge` aus Unit- + Integrationsprofilen, mit
   Build-Tag-Trennung) wird erst in 002b eingeführt, weil dort der
   Adapter-Code dazu kommt, der nur unter CGO und nur mit
   SoftHSM-Integration kompilier-/testbar ist.

## Akzeptanzkriterien

- `make ci` läuft grün gegen den Slice-002a-Code (Lint, Unit-Tests,
  Coverage ≥ 80 % auf `./internal/...`, docs-check, govulncheck) —
  ohne PKCS#11-Adapter-Code, also dieselbe Aggregations-Mechanik wie
  in Slice 001. Zusätzlich läuft `make fullbuild` grün, weil der
  kritische 002a-Pfad erst im Runtime-Image entsteht: `build`-Stage mit
  `CGO_ENABLED=1`, Distroless-base-Runtime, `COPY` der Closure aus
  `deps-closure` und `closure-check`-Stage. Da 002a bewusst noch keinen
  PKCS#11-Adapter-Code in den Server zieht, ist ein dynamisch gelinktes
  `hsmdoc`-Server-Binary **kein** 002a-Akzeptanzkriterium; die echte
  Server-Linkage gegen PKCS#11 wird in 002b geprüft, sobald der
  produktive Adapter importiert wird. Ein grünes `make ci` ohne grünes
  Runtime-Image ist für 002a nicht abnahmefähig.
- **Shared-Library-Closure** (Build-Pipeline):
  Zweistufige Verifikation grün — (a) `lddtree --root
  $RUNTIME_FS` aus der `closure-check`-Stage meldet keine
  „not found"-Einträge gegen den Distroless-Runtime-Rootfs mit
  SoftHSM v2 als Beispielmodul, (b) `pkcs11-dlopen-smoke`-Binary
  im Runtime-Image öffnet `$HSMDOC_PKCS11_MODULE` mit Exit 0.
  Stückliste `/etc/hsmdoc/pkcs11-libs.txt` ist im Image vorhanden
  und identisch zur `closure-check`-Stage-Ausgabe.
- **Image-Größe + Trivy-Scan**: `make image-scan` läuft Docker-only
  mit gepinntem Trivy-Image und schreibt
  `out/security/trivy-runtime.json`; die Image-Größe wird über das
  ebenfalls reproduzierbare Größen-Target in
  `out/security/image-size.txt` dokumentiert (Vergleichsmessung vs.
  `distroless/static`-Stand aus Slice 001). ADR 0004 übernimmt genau
  diese Artefaktwerte. Trivy-Scan-Ergebnis ohne neue HIGH/CRITICAL-
  Findings gegenüber dem Vorzustand.
- **`make smoke-dlopen`** ist als Make-Target verfügbar und grün
  gegen das CI-SoftHSM-Modul; das Binary `cmd/pkcs11-dlopen-smoke/`
  ist im Runtime-Image vorhanden.
- **CI-Bild kann SoftHSM v2 ausführen** (`softhsm2-util --version`
  läuft im CI-Build-Image; Slot-Init-Skript für 002b liegt im
  Repo). Das zweite OSS-Modul (OpenCryptoki o. Ä.) ist im
  CI-Build-Image installiert oder per Test-Image-Pin
  reproduzierbar. Build-/Closure-Stages für SoftHSM/OpenCryptoki sind
  auf Debian 12/Bookworm gepinnt, damit die ermittelte
  Shared-Library-Closure ABI-kompatibel zur
  `distroless/base-debian12`-Runtime bleibt.
- **ADR 0004 angelegt + ADR-Index aktualisiert.** Status
  `Accepted`, Verweis auf ADR 0002 als geschärfte Vorgängerin
  gemäß [ADR 0001 §2.3](../../adr/0001-documentation-and-planning-structure.md).
  Image-Größe + Trivy-Scan-Ergebnis sind im ADR dokumentiert. Die
  Wahl des zweiten OSS-Moduls (OpenCryptoki vs. Alternative)
  inklusive Begründung ist im ADR festgehalten. Ein Slice 002a
  ohne ADR-Spur ist nicht abnahmefähig.
- **Folge-ADR zur Planstruktur angelegt + ADR-Index aktualisiert**
  (im selben PR oder als abgespaltener Mini-PR vor Slice-002a-
  Closure): `ADR 0001` bleibt als `Accepted`-ADR inhaltlich
  unverändert. Die folgenden zwei additiven Planstruktur-Patterns
  werden stattdessen in einer neuen Folge-ADR zu `ADR 0001`
  dokumentiert und im ADR-Index als Schärfung eingetragen, weil beide
  Patterns mit 002a/002b produktiv etabliert werden:
  - **Open-Trigger-Lifecycle** (`open/ → done/`-Migration aus dem
    einlösenden Slice heraus) — Slice 001 hatte ihn zum ersten Mal
    angewandt (Open-Trigger 001 → `done/`), 002a wendet ihn zum
    zweiten Mal an (Open-Trigger 002 → `done/`).
  - **`next/<slice>/`-Sub-Verzeichnis-Pattern** — Slice 002b wird
    es mit `next/002b-spike-hkdf/` produktiv nutzen; die Folge-ADR
    jetzt anzulegen verhindert, dass 002b sie nachträglich einziehen
    muss.
- **Open-Trigger 002** wird im selben PR nach `done/` migriert (oder
  re-bewertet, falls Scope sich geändert hat).
- **Roadmap-Lifecycle** wird in zwei Schritten aktualisiert:
  - **Bei Slice-Aktivierung** (Migration `next/` → `in-progress/`):
    Slice-Tabelle in [`roadmap.md`](../in-progress/roadmap.md)
    führt 002a als `in-progress`; Open-Trigger-Block streicht 002
    (gleicher Lifecycle wie 001 beim Aktivieren).
  - **Bei Slice-Abschluss** (Merge des Schluss-PR, alle Akzeptanz-
    kriterien grün): Slice-Tabelle führt 002a als `done`,
    Slice-Datei wandert nach `done/`; M1-DoD-Tabelle hakt
    `M1-DoD-07` ab (Open-Trigger 002 nach `done/`).
- **MaxRecvMsgSize-TODO** in `cmd/hsmdoc/main.go` bleibt stehen —
  das ist Scope von 002b (Encrypt-Stream landet erst dort);
  Item §2.1 aus
  [`offene-arbeitsfaeden.md`](../in-progress/offene-arbeitsfaeden.md)
  bleibt offen bis 002b.

## Abgrenzung — NICHT in diesem Slice

- **Kein PKCS#11-Adapter-Code.** Weder Session-Pool noch
  `KeyBinding`/`ChunkSealer`/`HeaderMAC`-Implementierung. Das
  ganze `internal/adapter/driven/pkcs11/`-Paket entsteht erst in
  002b.
- **Keine Hexagon-Schicht** (`internal/hexagon/`). Domain, Application
  und Ports kommen mit 002b.
- **Kein Encrypt-Stream.** `Encrypt`-RPC bleibt `UNIMPLEMENTED` aus
  Slice 001.
- **Kein Audit-Adapter.** JSONL-Sink + Durability-Vertrag kommen mit
  002b.
- **Keine Key-Registry.** YAML/JSON-Datei + Schema-Validierung
  kommen mit 002b.
- **Kein HKDF-Spike.** Der `CKM_HKDF_DERIVE`-Spike (gegen SoftHSM v2
  und das zweite OSS-Modul) ist Vorbedingung von 002b und läuft als
  Sub-Artefakt unter `next/002b-spike-hkdf/`.
- **Keine PIN-Quelle.** `HSMDOC_PKCS11_PIN_FILE`/`PIN_DEV`-Validierung
  + Modus-Whitelist `{0400, 0440}` kommen mit 002b.
- **Keine Proto-Erweiterung.** `EncryptResponse.container_header` /
  `container_trailer` und der `REVOKED → DESTROYED`-Enum-Rename
  kommen mit 002b.

## Geplante Slice-Folge danach

| Nr.    | Slice                                       | Aktiviert durch 002a                                  |
| ------ | ------------------------------------------- | ----------------------------------------------------- |
| `002b` | PKCS#11-Adapter + Encrypt-Hexagon + Audit   | CGO-Pipeline + Closure-Verifikation + dlopen-Smoke + CI-SoftHSM + zweites OSS-Modul stehen |

Die weiteren Folge-Slices (003 Decrypt, 004 Audit-Hash-Chain, 005
Helm-Chart + Kind-Smoke) hängen am Output von 002b und sind dort
gelistet.

## Bezug

- Direkter Implementierungs-/Akzeptanzbezug im Slice 002a:
  [Lastenheft `HSM-NFA-SEC-007`](../../../../spec/lastenheft.md)
  (Distroless-nonroot bleibt erfüllt)
- Direkter Implementierungs-/Akzeptanzbezug im Slice 002a
  (Teil-Erfüllung — Voll-Erfüllung in 002b):
  [Spezifikation `HSM-API-P11-002`](../../../../spec/spezifikation.md).
  002a deckt nur den `dlopen`-Aspekt (Closure-Smoke, Existenz +
  RTLD-Auflösung) ab; den vollen Existenz/ELF-Header/`C_GetInfo`-
  Check liefert 002b im PKCS#11-Adapter (§Modul-Validierung).
- [Open-Trigger 002 — CGO-Base-Switch](../open/002-distroless-base-fuer-cgo.md)
- [ADR 0001 §2.3 — Accepted-ADRs sind immutable, Schärfung über neuen ADR](../../adr/0001-documentation-and-planning-structure.md)
- [ADR 0002 — Docker-only Build-Pipeline](../../adr/0002-docker-only-build-pipeline.md)
- [Folge-Slice 002b — PKCS#11-Adapter + Encrypt-Hexagon](002b-pkcs11-encrypt-hexagon.md)
- [Roadmap M1](../in-progress/roadmap.md)
