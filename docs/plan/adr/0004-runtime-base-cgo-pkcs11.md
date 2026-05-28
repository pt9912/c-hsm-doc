# ADR 0004 — Runtime-Base für CGO/PKCS#11

**Status:** Accepted
**Datum:** 2026-05-27
**Bezug:** [Lastenheft](../../../spec/lastenheft.md) (`HSM-FA-HSM-001..003`,
`HSM-API-P11-001..002`, `HSM-NFA-SEC-007..008`, `HSM-TECH-006`),
[Spezifikation](../../../spec/spezifikation.md)
(`HSM-API-P11-002`, `HSM-FMT-006`, `HSM-CC-002`),
[ADR 0001](0001-documentation-and-planning-structure.md),
[ADR 0002](0002-docker-only-build-pipeline.md)
(geschärft durch diese ADR — §2.7),
[Slice 002a](../planning/done/002a-cgo-build-pipeline.md)

---

## 1. Kontext

Slice 002a führt den Übergang von pure-Go zum dynamisch gelinkten
PKCS#11-Pfad ein. Damit kollidiert die in ADR 0002 §2.7 festgelegte
Runtime-Base mit den neuen Anforderungen aus Slice 002b und der
PKCS#11-Anbindung:

- `HSM-FA-HSM-001..003` verlangt PKCS#11-Anbindung über Vendor-`.so`-
  Module — `softhsm2.so` in Dev und ein zweites herstellerfremdes
  Modul (Default OpenCryptoki) im CI; produktive HSM-Vendor-Module in
  M3.
- Vendor-`.so`-Module sind dynamisch gegen `libc` und weitere Shared-
  Libraries (z. B. `libsoftokn3`, `libnss3`, `libsqlite3` bei SoftHSM)
  gelinkt. Das Runtime-Image aus ADR 0002 §2.7
  (`gcr.io/distroless/static-debian12:nonroot`) bietet keine
  Glibc-Toolchain — `dlopen()` schlägt mit
  „library not found" oder Loader-Fehler beim ersten Pod-Start fehl.
- `HSM-API-P11-003` schreibt das Go-Binding `github.com/miekg/pkcs11`
  vor; das Binding nutzt Cgo (`-lc`, `-ldl`). Damit ist
  `CGO_ENABLED=1` für die Server-Build-Stage nicht optional.
- `HSM-FMT-006` (Header-HMAC, HKDF-Profil A) verlangt `CKM_HKDF_DERIVE`
  als nicht-extrahierbare Schlüsselableitung im HSM. Slice 002b setzt
  Profil A als M1-Pflicht; die CI-Module müssen den Mechanismus
  bedienen.
- `HSM-NFA-SEC-007` (distroless oder vergleichbares Base-Image) und
  `HSM-NFA-SEC-008` (Pod-Härtung mit `runAsNonRoot`,
  `readOnlyRootFilesystem`) bleiben verbindlich — ein Wechsel auf
  ein Debian-Slim- oder Alpine-Image wäre Rückschritt.
- BuildKit-Caching ist scharf: Eine reine Verifikations-Stage ohne
  konsumierten Output wird übersprungen, und einmal grüne Stages
  werden aus dem Cache befriedigt. Eine PKCS#11-Closure-Verifikation
  muss diese Falle umgehen, sonst hätte sie nur Build-Time-Aussage.

ADR 0002 ist `Accepted` und nach [ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
inhaltlich unveränderlich. Die Schärfung von §2.7 und die zugehörigen
neuen Build-Stages werden deshalb in dieser Folge-ADR dokumentiert.

---

## 2. Entscheidung

### 2.1 Runtime-Base auf `gcr.io/distroless/base-debian12:nonroot`

Die Runtime-Stage des Servers wechselt von
`gcr.io/distroless/static-debian12:nonroot`
(ADR 0002 §2.7) auf `gcr.io/distroless/base-debian12:nonroot`.

Begründung:

- `distroless/base-debian12` bringt eine minimale Debian-Glibc-
  Userland (`libc`, `libdl`, `libpthread`, `libm`, `libresolv`, …)
  mit, ausreichend zum dynamischen Laden eines PKCS#11-Moduls.
- Keine Shell, kein Paketmanager, kein `apt`, kein `dpkg` — die
  Pod-Härtung aus `HSM-NFA-SEC-007` und `HSM-NFA-SEC-008` bleibt
  unverändert erfüllt.
- `nonroot`-Tag bedeutet UID 65532 (entspricht Distroless-Konvention),
  damit `runAsNonRoot` ohne Zusatzkonfiguration weiterhin greift.

`RUNTIME_BASE_IMAGE`-Default im Dockerfile wechselt entsprechend; die
Digest-Pin-Politik aus ADR 0002 §2.4 bleibt unverändert.

### 2.2 `CGO_ENABLED=1` für die `build`-Stage

Die `build`-Stage des Servers schaltet auf `CGO_ENABLED=1` um.

Begründung:

- `github.com/miekg/pkcs11` nutzt Cgo (`#include <dlfcn.h>`,
  `-lc -ldl`); ohne CGO wäre der PKCS#11-Adapter-Pfad aus Slice 002b
  nicht baubar.
- Das Server-Binary wird damit dynamisch gegen `libc` gelinkt; das
  ist mit der neuen Runtime-Base aus §2.1 kompatibel.
- Build-Stage bleibt im selben Debian-Builder-Image (siehe
  `GO_VERSION` aus ADR 0002 §2.4), damit die libc-Version zwischen
  Builder und Runtime ABI-kompatibel ist.

### 2.3 `deps-closure`-Stage mit `lddtree` für transitive Closure

Vendor-`.so`-Module bringen transitive Shared-Library-Abhängigkeiten
mit (SoftHSM v2 → `libsoftokn3`, `libnss3`, `libsqlite3`;
OpenCryptoki → `libica`, `libcrypto`). Diese müssen ins Runtime-Image
kopiert werden, sonst schlägt `dlopen()` zur Laufzeit fehl.

Mechanik:

- Eine neue `deps-closure`-Stage (Debian-Slim-Builder mit
  `pax-utils`) ruft
  `lddtree --root $RUNTIME_FS --list --skip-non-elfs $MODULE` gegen
  das Distroless-base-Sysroot auf.
- Ergebnis wird als Stückliste (`/build/pkcs11-libs.list`)
  geschrieben **und** parallel in ein Staging-Verzeichnis
  (`/staging/pkcs11-rootfs/`) kopiert (`install -D` pro Eintrag,
  unter Erhalt der Verzeichnisstruktur).
- Das Runtime-Image zieht das Staging-Verzeichnis mit einem einzigen
  statischen `COPY --from=deps-closure /staging/pkcs11-rootfs/ /`.
- `ldd` wird **nicht** für Stücklisten verwendet; es ist für
  interaktiven Gebrauch gedacht und scheitert an „not found"-,
  RPATH-/`$ORIGIN`-Fallstricken.

### 2.4 `closure-check`-Stage mit Markerdatei + Sentinel-`COPY`

Eine separate `closure-check`-Stage verifiziert, dass das fertige
Runtime-Rootfs alle Library-Abhängigkeiten erfüllt. Drei Mechanismen
sind kombiniert, damit die Stage tatsächlich pro Build läuft:

1. **`touch /closure-check.ok` als letzte Stage-Anweisung** —
   atomar erst bei erfolgreicher Verifikation.
2. **Sentinel-`COPY` im Runtime-Image:**
   `COPY --from=closure-check /closure-check.ok /etc/hsmdoc/closure-check.ok`
   — macht die Stage zur Build-Voraussetzung für `make fullbuild`.
   Die Stage selbst hat `COPY --from=runtime / /rootfs` als Input,
   damit jeder Runtime-Wechsel ihren Cache invalidiert.
3. **`make closure-check`** mit `--no-cache-filter closure-check`
   analog zum `NO_CACHE_FILTER_*`-Pattern in Slice 001
   (`NO_CACHE_FILTER_TEST/LINT/COVERAGE`) — erzwingt explizite
   Re-Verifikation jederzeit.

### 2.5 `pkcs11-dlopen-smoke`-Helper-Binary

Distroless hat keine Shell und kein `ldd`. Eine echte
Runtime-Verifikation der Closure läuft über ein winziges Helper-
Binary:

- Pfad im Repo: `cmd/pkcs11-dlopen-smoke/`.
- Bau-Stage: dieselbe `build`-Stage wie der Server (`CGO_ENABLED=1`),
  zweiter `RUN go build ./cmd/pkcs11-dlopen-smoke`.
- Implementierung: `dlopen($MODULE, RTLD_NOW)` via Cgo
  (`#include <dlfcn.h>`). Pure-Go-Stub mit `//go:build !cgo` deckt
  den `coverage`-Stage (`CGO_ENABLED=0 go test ./...`) ab.
- Installation im Runtime-Image: fester Pfad
  `/usr/local/bin/pkcs11-dlopen-smoke`. Optionaler Override über
  Env-Variable `HSMDOC_PKCS11_DLOPEN_SMOKE_BIN`.
- Aufrufpunkte: `make smoke-dlopen` (CI-Pfad, manuelle Diagnose);
  in Slice 002b ein synchron-blockierender Startup-Hook im
  `hsmdoc`-Hauptprozess (vor `C_Initialize`/Pool-Aufbau).

Begründung der Cgo-Wahl gegenüber `purego`:

- Cgo ist konsistent mit dem PKCS#11-Adapter, der `miekg/pkcs11`
  (cgo-basiert) verwendet — eine zweite Library-Lade-Disziplin
  wäre ein zusätzlicher Drift-Vektor.
- `purego` würde eine neue Go-Modul-Dependency einziehen und den
  `go.sum`-Strict-Mode (Slice 001 / Open-Trigger 001) unnötig
  belasten.
- Cgo erlaubt direkten Zugriff auf `dlerror()` für aussagekräftige
  Fehlermeldungen, was die Forensik bei Library-Closure-Fehlern
  vereinfacht.

### 2.6 Zweites herstellerfremdes OSS-PKCS#11-Modul im CI

`HSM-FA-HSM-001` Akzeptanz verlangt Start gegen SoftHSM v2 **und**
ein zweites herstellerfremdes Modul ohne Codeänderung. Slice 002a
legt die Modulwahl **vor** dem HKDF-Spike (Slice 002b Vorbedingung
3) fest, damit das CI-Image deterministisch reproduzierbar ist.

- **Default-Zweitmodul:** OpenCryptoki (`opencryptoki` Debian-Paket,
  ICA-/Software-Token-Modus). Voraussetzung sind `CKM_AES_GCM` und
  `CKM_HKDF_DERIVE`; die genaue OpenCryptoki-Konfiguration
  (Soft-Token, AES-Mechanismen aktiviert) ist im CI-Setup-Skript
  unter `ci/keys-init/opencryptoki.sh` (entsteht in Slice 002a-
  Implementierung) festgehalten.
- **Alternative:** Falls OpenCryptoki im CI nicht stabil bedient
  (z. B. `CKM_HKDF_DERIVE` nicht verfügbar in der gepinnten
  Distribution), wird auf Mozilla-NSS-Softoken (`libsoftokn3` als
  eigenständiges PKCS#11-Modul via PKCS#11-Wrapper) gewechselt —
  in einer Folge-ADR (geplant: `ADR 0006 — Zweitmodul-Korrektur`)
  dokumentiert. Diese ADR 0004 selbst bleibt nach
  [ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
  inhaltlich unverändert.
- **Keine zwei SoftHSM-Instanzen als Ersatz:** Ein zweites SoftHSM
  mit divergenter Token-Konfiguration ist kein „herstellerfremdes
  Modul" im Sinne von HSM-FA-HSM-001 und zählt nicht als
  Akzeptanznachweis.

### 2.7 Image-Größe und Trivy-Scan

`distroless/base-debian12:nonroot` ist deutlich größer als
`distroless/static-debian12:nonroot` (Base + ELF-Loader + glibc +
SoftHSM-/OpenCryptoki-Closure). Erste Slice-002a-Messung (`make image-size`
und `make image-scan`, Build-Datum 2026-05-27):

| Messung                       | Slice-002a-Stand (distroless/base + Closure)         |
| ----------------------------- | ---------------------------------------------------- |
| Runtime-Image-Größe           | 46 001 254 Byte (≈ 43,9 MiB)                         |
| Trivy HIGH-Findings (Debian)  | 0                                                    |
| Trivy CRITICAL-Findings (Debian) | 0                                                  |
| Trivy HIGH/CRITICAL (Go-Binaries) | 0 (Server + `pkcs11-dlopen-smoke`)               |

Vergleich zum Slice-001-Stand (`distroless/static-debian12:nonroot` +
einzelnes Go-Binary, statisch gelinkt) wird beim ersten Build im
selben CI-Lauf erhoben und in einer Folge-Notiz nachgetragen;
ungefähre Größenordnung typisch ~22 MiB Slice-001-Stand → ~43,9 MiB
Slice-002a-Stand (Differenz dominiert durch glibc-Userland +
NSS/OpenSSL aus der PKCS#11-Closure).

Akzeptanz für Slice 002a: keine **neuen** HIGH/CRITICAL-Findings
gegenüber dem Vorzustand — erfüllt (0 Findings nach Stage-Init).
Eine Verschlechterung ist Release-Blocker.

Akzeptanz für Slice 002a: keine **neuen** HIGH/CRITICAL-Findings
gegenüber dem Vorzustand. Eine Verschlechterung ist Release-Blocker.

`make image-scan` ist Docker-only mit gepinntem Trivy-Image
(`TRIVY_VERSION`, `TRIVY_BASE_IMAGE` als `ARG`). Größenmessung
läuft ebenfalls über ein reproduzierbares Make-Target.

### 2.8 Pin-Politik und Stückliste

- `RUNTIME_BASE_IMAGE` bleibt eine `ARG`-Variable im Dockerfile
  (Default jetzt `gcr.io/distroless/base-debian12:nonroot`,
  Digest-Pin in CI/Release-Builds via `--build-arg`).
- `PKCS11_VENDOR_IMAGE` als neue `ARG`-Variable für die Build-Stage,
  aus der die `.so`-Module gezogen werden (Default: Debian
  `softhsm2`-Paket aus dem Builder-Image; OpenCryptoki über ein
  separates Test-Image-Pin).
- Die Stückliste (`/etc/hsmdoc/pkcs11-libs.txt`) ist im Image
  präsent als Forensik-Hilfe und im Repo per Diff prüfbar.
- Updates dieser Pins folgen der Routine aus ADR 0002 §2.4:
  Begründung im Commit-Body, kein ADR-Pflicht für
  Patch-/Minor-Hebungen. Major-Wechsel (z. B. SoftHSM 2 → 3,
  Wechsel der Distroless-Variante) lösen eine Folge-ADR aus.

---

## 3. Konsequenzen

- Runtime-Image ist signifikant größer (Distroless-base + Vendor-`.so`-
  Closure). Image-Pull-Zeit und Knoten-Disk-Bedarf steigen entsprechend.
- Build-Zeit pro Image steigt durch die zusätzlichen Stages
  (`deps-closure`, `closure-check`) und das CGO-Linken. BuildKit-
  Caching mildert das auf inkrementelle PRs.
- Der Server hat eine Glibc-Laufzeitabhängigkeit und ist nicht mehr
  statisch — ein Wechsel auf eine andere Glibc-Variante (z. B. Musl
  oder neuere Debian-Version) ist ohne Closure-Neumessung nicht
  sicher.
- PKCS#11-Closure-Verifikation läuft bei jedem Pod-Start (Slice 002b
  §Startup-Hook), nicht nur zur Build-Zeit — Library-Closure-Fehler
  werden zur Laufzeit erkannt, nicht erst beim ersten
  `C_Initialize`.
- Zweitmodul-Akzeptanz erweitert den CI-Pfad um einen zusätzlichen
  Integrationstest-Lauf gegen OpenCryptoki. CI-Laufzeit steigt
  entsprechend; der 10-GiB-Heap-Cap-Test (Slice 002b) läuft ggf. als
  Nightly.

---

## 4. Pflege-Regeln

- Library-Closure-Diffs (Änderungen in `/etc/hsmdoc/pkcs11-libs.txt`)
  werden bei jedem PR sichtbar — nicht-triviale Diffs brauchen eine
  Begründung im Commit-Body.
- Modulwahl-Änderungen (Default-Modul oder Alternative) erzeugen
  eine Folge-ADR (z. B. ADR 0006); ADR 0004 bleibt inhaltlich
  unverändert.
- Hebungen der Distroless-Major-Version (z. B. `debian12 → debian13`)
  brauchen einen Closure-Neulauf gegen den neuen Rootfs und ggf.
  eine Folge-ADR.
- `pkcs11-dlopen-smoke` ist Teil des Runtime-Images-Inventars; ein
  Entfernen oder Pfad-Wechsel braucht Slice-Plan-Anpassung in 002b
  (Startup-Hook).

---

## 5. Nicht Gegenstand dieser ADR

- PKCS#11-Adapter-Implementierung (Domain, Application, Driven-
  Adapter, Audit-Sink, Key-Registry) — Slice 002b.
- `CK_HKDF_PARAMS`-Shim oder Binding-Wechsel — Slice 002b
  Vorbedingung 3 (HKDF-Spike), ggf. Folge-ADR.
- Vault- oder K8s-Secret-CSI-Adapter für PIN-Bezug — bleibt eigene
  ADR, sobald ein Betreiber den nativen Adapter nachfragt
  (siehe Slice 002b §Abgrenzung).
- SBOM (CycloneDX/SPDX) und Image-Signierung (cosign) — eigene ADR
  in M2 (Slice 010 laut Tracker §3).
- Confidential-Compute-Pfad für `HSM-THREAT-008` — eigene ADR.
- Produktive HSM-Profile (Utimaco, Thales) — `HSM-TECH-006` /
  M3-Scope, eigene ADR pro Profil.
