# 002b — PKCS#11-Adapter + Encrypt-Hexagon + Audit

**Meilenstein:** M1 (siehe [`roadmap.md`](../in-progress/roadmap.md))
**Status:** `next` (Scope skizziert, noch nicht aktiv)
**Datum:** 2026-05-27

## Ziel

Aus dem `UNIMPLEMENTED`-Skeleton (Slice 001) und der CGO-fähigen
Build-Pipeline (Slice 002a) wird der erste produktive Encrypt-Pfad:
gRPC-Server nimmt einen bidirektionalen Encrypt-Stream entgegen, der
Domain-Layer chunked den Klartext nach Chunk-State-Machine
([`HSM-FA-CHUNK-004`](../../../../spec/spezifikation.md)), ein
PKCS#11-Adapter (driven) führt je Chunk genau eine AES-256-GCM-
Operation gegen das HSM aus
([`HSM-FA-ENC-006`](../../../../spec/spezifikation.md)), der Server
emittiert einen formatkonformen Container
([`HSM-FMT-001..006`](../../../../spec/spezifikation.md)). Validiert
wird gegen SoftHSM v2 **und** ein zweites herstellerfremdes OSS-
PKCS#11-Modul im CI (siehe ADR 0004 aus 002a).

Dieser Slice ist der fachliche Kern von M1: er etabliert die
hexagonale Domain-Schicht, den ersten driven Adapter, den durablen
Audit-Sink und den ersten Encrypt-End-to-End-Pfad. Die CGO-Build-
Pipeline ist Vorbedingung und wird durch Slice 002a geliefert.

## Scope

### Hexagon-Layout — Domain und Application
([`HSM-ARCH-001`](../../../../spec/lastenheft.md),
[`internal/README.md`](../../../../internal/README.md))

Der Slice folgt dem im Repo dokumentierten Ziel-Layout
`internal/hexagon/{domain,application,port/{driving,driven}}`. Der
Domain-Kern hängt nicht an PKCS#11, gRPC oder Storage — Ports werden
von der Application konsumiert, Adapter implementieren sie.

- **Neu (domain):** `internal/hexagon/domain/chunk/` — Chunk-State-
  Machine ([`HSM-FA-CHUNK-004`](../../../../spec/spezifikation.md))
  als Enum + Übergangstabelle, Unit-Tests decken alle Übergänge ab.
  Reine Typen, keine I/O.
- **Neu (domain):** `internal/hexagon/domain/nonce/` — Nonce-Generator
  ([`HSM-FA-ENC-004`](../../../../spec/spezifikation.md)) mit
  32-Bit-Random-Prefix + 64-Bit-Counter je `(key_id, stream_id)`.
- **Neu (domain):** `internal/hexagon/domain/aad/` — AAD-Konstruktion
  je Chunk ([`HSM-FA-ENC-005`](../../../../spec/spezifikation.md)):
  der finale Container-Header gemäß HSM-FMT-001 **inklusive**
  `header_hmac` + `key_id` + `key_version` + `seq` + `stream_id`.
  Reihenfolge: Header-Bytes ohne HMAC encodieren, `HeaderMAC.Sign`
  über genau diesen Prefix berechnen, finalen Header mit HMAC
  zusammensetzen und erst danach Chunks mit diesem finalen Header als
  AAD verschlüsseln. Damit folgt der Slice direkt der Spec und braucht
  keine abweichende `header_without_hmac`-Sonderregel.
- **Neu (application):** `internal/hexagon/application/encrypt/` —
  Use-Case `EncryptStream`, der einen Klartext-Stream entgegennimmt
  und einen Container-Bytestream zurückgibt. Streaming-Interface
  (kein Vollpuffer; erfüllt
  [`HSM-FA-ENC-003`](../../../../spec/lastenheft.md) und
  [`HSM-NFA-MEM-001`](../../../../spec/lastenheft.md)). Ruft
  ausschließlich Ports auf:
  `KeyRegistry.LookupActive` (key_version-Auflösung),
  `KeyBinding.Bind` (opaker logischer Key-Snapshot aus Registry-
  Metadaten und Labels),
  `ChunkSealer.Seal` (AES-GCM je Chunk),
  `HeaderMAC.Sign` (Container-Header-HMAC),
  `AuditSink.Write` (audit-attempt-Commit). Kein direkter
  PKCS#11-Import im Application-Paket — Lint-Regel
  (`depguard`-Constraint) hält das durch.
- **Neu (application):** Worker-Pool-Orchestrierung
  ([`HSM-FA-CHUNK-005`](../../../../spec/spezifikation.md),
  [`HSM-ARCH-003`](../../../../spec/spezifikation.md)) als
  Application-Detail im Encrypt-Use-Case; Reorder-Buffer für die
  Commit- und Emit-Strecke in strikter `seq`-Reihenfolge. Chunks dürfen
  out-of-order `SEALED` werden, aber der Use-Case führt danach pro
  Stream ein gemeinsames Ordering-Gate: `audit-attempt` wird erst für
  den nächsten erwarteten `seq` durabel geschrieben, danach darf genau
  dieser Chunk `SEALED → EMITTED` wechseln. Damit bleiben sowohl
  `HSM-FA-CHUNK-005` (paralleles Seal + geordnetes Emit) als auch
  `HSM-FA-AUDIT-010` (append-only in `seq`-Folge je Stream) erfüllt.

### Container-Codec (`internal/hexagon/domain/container/`, schreibender Pfad)

Reiner Wire-Layout-Code; keine HSM- oder PKCS#11-Abhängigkeit.

- **Neu:** Encoder für Header
  ([`HSM-FMT-001`](../../../../spec/spezifikation.md)), Chunk-Frame
  ([`HSM-FMT-002`](../../../../spec/spezifikation.md)) und Trailer
  ([`HSM-FMT-003`](../../../../spec/spezifikation.md)).
  Big-Endian-Encoding strikt
  ([`HSM-FMT-005`](../../../../spec/spezifikation.md)). Version `0x01`,
  Cipher `0x01` (AES-256-GCM).
- Der Header-HMAC ist im Wire-Layout reserviert (32 Byte am
  Header-Ende, siehe HSM-FMT-001) — der Domain-Encoder akzeptiert
  ihn als opaquen 32-Byte-Wert. Wie er erzeugt wird, gehört in den
  PKCS#11-Adapter (siehe HeaderMAC-Port unten).
- **Abgrenzung:** Decoder (für Decrypt) ist Scope von Slice 003;
  Encoder muss aber so geschnitten sein, dass Slice 003 ihn ohne
  Refactor wiederverwenden kann (gemeinsame Konstanten + Wire-Layout).

### ChunkSealer-Port (`internal/hexagon/port/driven/chunksealer/`)
([`HSM-FA-ENC-001..002`](../../../../spec/lastenheft.md),
[`HSM-FA-ENC-006`](../../../../spec/spezifikation.md),
[`HSM-ARCH-001`](../../../../spec/lastenheft.md))

Application darf nur Ports aufrufen, nicht den PKCS#11-Adapter direkt
— deshalb sitzt die AES-GCM-Operation hinter einem eigenen
Driven-Port:

- **Neu (port):**
  `ChunkSealer.Seal(ctx context.Context, keyRef KeyRef, nonce [12]byte, aad []byte, plaintext []byte) (ciphertextWithTag []byte, error)`.
  `keyRef` ist der opake logische Key-Snapshot aus
  `KeyBinding.Bind`; der Adapter löst raw PKCS#11-Handles
  session-affin auf. Der Port nimmt nur Bytes/IDs und gibt
  Bytes/`error` zurück. `error` deckt die Fehlerklassen aus
  [`HSM-FA-FAIL-003`](../../../../spec/spezifikation.md) ab (gleicher
  Vertrag wie `HeaderMAC`).
- Der PKCS#11-Adapter implementiert den Port mit
  `C_EncryptInit` + `C_Encrypt` (siehe PKCS#11-Adapter-Sektion). Ein
  Mock-Adapter (`internal/testfixtures/chunksealer/`, siehe §Lint-Regeln) für
  Unit-Tests der Application-Schicht verwendet Go-`crypto/aes` +
  `crypto/cipher`. Der Mock liegt entweder in `_test.go`-Dateien im
  Testpackage oder in einem Test-Helper-Paket, das nur von Tests
  importiert wird; kein impliziter Build-Tag `testing` wird
  vorausgesetzt, weil `go test` diesen Tag nicht automatisch setzt.

### HeaderMAC-Port (`internal/hexagon/port/driven/headermac/`)
([`HSM-FMT-006`](../../../../spec/spezifikation.md))

Application-Layer ruft beim Header-Encoding einen Port; die konkrete
Profilwahl ist HSM-Sache und wohnt im PKCS#11-Adapter.

- **`key_version`-Auflösung serverseitig:** Der Proto-`EncryptHeader`
  ([`spec/proto/chsmdocv1/c_hsm_doc.proto`](../../../../spec/proto/chsmdocv1/c_hsm_doc.proto))
  trägt nur `key_id`, **keinen** `key_version`. Slice 002b löst die
  aktive `key_version` ausschließlich serverseitig aus der
  Key-Registry auf — sobald der Encrypt-Header eintrifft, wählt der
  Server den eindeutig als `active` markierten Eintrag für die
  übergebene `key_id`. Pro `key_id` darf es genau einen `active`-
  Eintrag geben; mehrere `active`-Versionen sind ein Schema-Fehler
  beim Start. Die so ermittelte `key_version` wird in den
  Container-Header
  ([`HSM-FMT-001`](../../../../spec/spezifikation.md)), in AAD je
  Chunk ([`HSM-FA-ENC-005`](../../../../spec/spezifikation.md)),
  in HKDF-Salt
  ([`HSM-FMT-006`](../../../../spec/spezifikation.md)) und in das
  Audit-Feld geschrieben. Decrypt (Slice 003) liest `key_version`
  zurück aus dem Container-Header — keine Proto-Erweiterung nötig.
- **Neu (port):**
  `HeaderMAC.Sign(ctx context.Context, keyRef KeyRef, headerBytesWithoutHMAC) ([32]byte, error)`.
  Der Port nimmt **denselben** `KeyRef`, der vorher aus
  `KeyBinding.Bind` kam und auch an `ChunkSealer.Seal`
  weitergereicht wird — kein zweiter Registry-Lookup und keine
  Änderung des logischen Key-Snapshots; raw HSM-Handles werden nur
  session-affin aus diesem Snapshot aufgelöst oder session-lokal
  gecached. Damit entsteht kein Driftrisiko durch zwischenzeitlichen
  Reload. `ctx` trägt Cancellation und Request-Tracing in den Adapter;
  der zurückgegebene `error` deckt die Fehlerklassen aus
  [`HSM-FA-FAIL-003`](../../../../spec/spezifikation.md) ab
  (`SESSION_INVALID`, `HSM_DEVICE_ERROR`, `HSM_NOT_LOGGED_IN`,
  `MECHANISM_MISSING`, …). Keine PKCS#11-Typen leaken über die
  Port-Grenze. Profil-Lookup-Methode `Profile() string` exportiert
  den aktiven Modus für Logs/Metriken (in M1 immer `"A"`).
- **Stream-Snapshot-Invariante:** Encrypt-Use-Case bindet
  `KeyRecord` über die Registry und daraus genau einmal `KeyRef`
  über `KeyBinding.Bind` am Stream-Anfang (vor Container-Header-Emit).
  Danach reicht er `KeyRef` unverändert an HeaderMAC und an alle
  ChunkSealer-Aufrufe. Damit operieren Header-HMAC und Chunk-
  Verschlüsselung garantiert auf identischem logischem `(key_id,
  key_version, pkcs11_label, master_hmac_pkcs11_label)`-Tupel — auch
  wenn die Registry-Datei zwischen Header und letztem Chunk reloaded
  wird. Bei der nächsten Encrypt-Anfrage wird der Snapshot frisch
  aufgelöst.
- **KeyRef-Aufbau:** `KeyRef` ist die opake Struktur, die der
  PKCS#11-Adapter über den `KeyBinding`-Port zurückgibt — enthält den
  logischen Stream-Snapshot (`key_id`, `key_version`, beide Labels)
  und einen adapterinternen Resolver, aber **keine roh
  sessionsübergreifend wiederverwendeten PKCS#11-Handles**. Object-
  Handles sind session-affin: der Adapter löst sie beim Ausleihen
  einer Session aus den Labels auf oder nutzt nur einen Cache, der
  explizit an genau diese Session gebunden ist. Bei
  `CKR_KEY_HANDLE_INVALID` wird der session-lokale Cache verworfen und
  der Handle aus dem unveränderten Snapshot neu aufgelöst. Der
  File-Registry-Adapter kennt nur Metadaten und Labels; er importiert
  kein PKCS#11-Paket.

### PKCS#11-Adapter (`internal/adapter/driven/pkcs11/`)

- **Neu:** Driven Adapter, der `github.com/miekg/pkcs11` einbindet
  ([`HSM-API-P11-003`](../../../../spec/spezifikation.md)).
- **CGO-Build-Tag-Schnitt:** Dateien, die `github.com/miekg/pkcs11`
  oder Cgo importieren, tragen `//go:build cgo`. Das Paket enthält
  zusätzlich einen kleinen `!cgo`-Stub ohne PKCS#11-Import, der nur
  Konstruktoren/Interfaces kompilierbar hält und bei echter
  Initialisierung deterministisch mit `STARTUP_PKCS11_CGO_DISABLED`
  scheitert. Damit bleiben Pure-Go-Läufe (`CGO_ENABLED=0 go test
  ./...`) für Domain/Application/Config möglich, während der reale
  Adapter ausschließlich in `CGO_ENABLED=1`-Builds und
  Integrationstests verwendet wird. Der Runtime-`build`-Stage aus
  002a baut weiterhin mit `CGO_ENABLED=1`; ein Produktionsimage mit
  Stub ist nicht zulässig.
- **Neu:** Adapter-nahe CKR-Übersetzung: raw PKCS#11-Returncodes
  (`CKR_*`) und `github.com/miekg/pkcs11`-Konstanten bleiben auf
  `internal/adapter/driven/pkcs11/` beschränkt. Der Adapter mappt sie
  auf interne Fehlerklassen aus dem Application-/Port-Vertrag; Domain
  und Application sehen keine PKCS#11-Konstanten und importieren kein
  PKCS#11-Paket.
- **Neu:** Session-Pool
  ([`HSM-FA-HSM-003`](../../../../spec/lastenheft.md),
  [`HSM-FA-HSM-004`](../../../../spec/spezifikation.md)) mit
  konfigurierbarer `pool.size` (Default 8), `pool.maxIdle` (4),
  `pool.maxLifetime` (1 h), `pool.acquireTimeout` (5 s),
  `pool.loginRetry` (3). Lifecycle: Login bei Acquire, Re-Login bei
  `CKR_USER_NOT_LOGGED_IN`, Recycling nach `maxLifetime`. Re-Login ist
  gemäß [`HSM-FA-FAIL-008`](../../../../spec/spezifikation.md)
  gedrosselt: Default maximal ein Re-Login pro Session pro 60 s;
  zusätzliche `CKR_USER_NOT_LOGGED_IN` innerhalb des Fensters führen
  zum Session-Recycling statt zu weiteren Login-Versuchen. Metrik
  `hsmdoc_hsm_relogin_total` zählt erfolgreiche und fehlgeschlagene
  Re-Login-Versuche pro Slot.
- **Neu:** Modul-Validierung beim Start
  ([`HSM-API-P11-002`](../../../../spec/spezifikation.md)):
  Existenz-Check, ELF-Header-Check, `C_GetInfo`-Aufruf.
- **Startup-Synchron-Hook für `pkcs11-dlopen-smoke`** (Binary aus
  Slice 002a): der `hsmdoc`-Hauptprozess ruft das Smoke-Binary
  **synchron als ersten Schritt** im Startup auf (über `os/exec`,
  vor `C_Initialize`/Pool-Aufbau); Exit-Code ≠ 0 → Service-Start
  bricht mit `STARTUP_PKCS11_DLOPEN_FAILED` ab. Damit ist die
  Closure-Garantie keine Build-Time-Annahme, sondern pro Pod-Start
  neu validiert. (002a hat das Binary geliefert; 002b verdrahtet
  den Aufruf, weil hier erst der echte `C_Initialize`-Pfad
  entsteht.)
  **Binary-Pfad-Vertrag:** Default-Pfad im Runtime-Image ist
  `/usr/local/bin/pkcs11-dlopen-smoke` (von 002a fest auf diesen
  Pfad ausgeliefert). Optionaler Override über
  `HSMDOC_PKCS11_DLOPEN_SMOKE_BIN`. Fehlt das Binary unter dem
  konfigurierten Pfad → Start-Abbruch mit
  `STARTUP_PKCS11_DLOPEN_FAILED`.
- **Neu:** `KeyBinding`-Port-Implementierung: nimmt einen
  Registry-`KeyRecord` mit `pkcs11_label` und
  `master_hmac_pkcs11_label`, validiert beide Labels gegen das HSM und
  liefert den opaken `KeyRef`-Snapshot für HeaderMAC und ChunkSealer.
  Die zurückgegebene Struktur trägt keine raw Handles über
  Session-Grenzen; konkrete Handles werden je ausgeliehener Session
  aufgelöst oder session-lokal gecached. Ein nicht auflösbarer AES-
  oder HMAC-Handle führt beim Stream-Start zu `FAILED_PRECONDITION` +
  `KEY_NOT_FOUND`; der File-Registry-Adapter bleibt frei von
  PKCS#11-Abhängigkeiten.
- **Neu:** Mechanismus-Check
  ([`HSM-FA-HSM-005`](../../../../spec/spezifikation.md)):
  `CKM_AES_GCM` Pflicht; **`CKM_HKDF_DERIVE` Pflicht** (Profil A
  ist M1-only). Fehlen eines der beiden → harter Start-Abbruch
  mit Hinweis auf das fehlende HSM-Profil im Sinne von HSM-FMT-006.
- **Neu:** HeaderMAC-Port-Implementierung
  ([`HSM-FMT-006`](../../../../spec/spezifikation.md)) mit **Profil A
  (natives HKDF)** als einzigem M1-Pfad: `C_DeriveKey` mit
  `CKM_HKDF_DERIVE` und `CK_HKDF_PARAMS` (Salt `key_id || key_version`,
  Info `"c-hsm-doc/header-hmac/v1"`, L=32) leitet den Header-Key
  als session-ephemeres HSM-Handle aus dem Master-HMAC-Key ab.
  Template-Pflicht: `CKA_CLASS=CKO_SECRET_KEY`,
  `CKA_KEY_TYPE=CKK_GENERIC_SECRET` (oder moduläquivalentes Secret-
  Key-Attribut für HMAC), `CKA_SIGN=true`, `CKA_TOKEN=false`,
  `CKA_EXTRACTABLE=false`, `CKA_SENSITIVE=true`. Anschließend
  `C_SignInit(CKM_SHA256_HMAC, headerKeyHandle)` +
  `C_Sign(headerBytes)` für den 32-Byte-HMAC. Nach dem Signieren wird
  der abgeleitete Header-Key mit `C_DestroyObject` zerstört; Cleanup-
  Fehler recyceln die Session und erhöhen eine Diagnosemetrik, damit
  kein Header-Key-Objekt pro Stream im Token liegen bleibt. Das aktive
  Profil wird im Start-Log und in der Metrik
  `hsmdoc_header_hmac_profile{profile="A"}` ausgewiesen.
- **Profil B in M1 ausgeschlossen:** Spec
  ([`HSM-FMT-006`](../../../../spec/spezifikation.md) §1 Profil B)
  beschreibt eine spec-konforme HMAC-Konstruktion (Extract via
  `CKM_SHA256_HMAC` auf dem nicht-extrahierbaren Master-Key,
  Expand über PRK-Re-Import in ein nicht-extrahierbares Secret-
  Key-Objekt oder eine zweite HMAC-Operation direkt auf dem
  PRK-Handle, sodass weder PRK noch Header-Key das HSM verlassen).
  Der Pfad ist technisch erreichbar, aber **pro HSM-Modul
  separat zu validieren**: der konkrete Re-Import-Mechanismus
  (`CKM_GENERIC_SECRET_KEY_GEN` mit `CKA_VALUE`-Befüllung vs.
  Vendor-Konstruktion), das Attribut-Template und die
  Mechanismus-Liste unterscheiden sich von Vendor zu Vendor.
  Slice 002b hat dafür keinen Validierungsbudget — ein einziger
  M1-Pfad (Profil A) reduziert das Risiko. Profil B bleibt
  deshalb M3-Scope: Pro produktivem HSM-Profil
  ([`HSM-TECH-006`](../../../../spec/lastenheft.md)) wird der
  Profil-B-Pfad als Vendor-Detail (z. B. via `CKM_*_DERIVE_DATA`
  oder Vendor-KDF) validiert und freigegeben. Für M1 ist `CKM_HKDF_DERIVE`
  Pflicht-Mechanismus; HSMs ohne natives HKDF sind nicht freigegeben.
  Profil C (Vendor-Mechanismus) bleibt ebenfalls M3.
- **Neu:** PIN-Bezug aus externem Secret-Store
  ([`HSM-FA-HSM-002`](../../../../spec/lastenheft.md)). Slice 002b
  unterstützt genau eine produktiv zulässige Quelle:
  **Datei mit restriktivem Mode**, gemountet aus einem K8s-Secret-
  Volume oder von einem Vault-Agent gerendert. Validierung beim
  Start — modus-abhängig, weil der Kubernetes-Secret-Mount mit
  `fsGroup` nicht die UID, sondern nur die GID an den Prozess
  bindet:
  - **Mode `0400`** (Owner-only-Lesepfad, z. B. Vault-Agent-Render):
    Datei-`UID` MUSS `getuid()` entsprechen.
  - **Mode `0440`** (Group-Read-Pfad, z. B. K8s-Secret-Volume mit
    `fsGroup`): Datei-`UID` darf abweichen (typisch `root`), aber
    Datei-`GID` MUSS entweder der primären GID des Prozesses
    (`getgid()`) oder einer Supplementary Group aus `getgroups()`
    entsprechen.
  - **Generelle Bits:** Welt-Bits (`o-rwx`), Group-Write und
    Group-Execute (`g-wx`) MÜSSEN ausgeschlossen sein. Andere Modi
    außerhalb der Whitelist `{0400, 0440}` → harter Start-Abbruch.
  - **Größen-/Inhalt-Check:** Datei > 0 Byte, ≤ 4 KiB, kein NUL-Byte
    außer am Ende.

  Eine rohe Prozess-Env als allgemeine Produktionsquelle ist nicht
  zulässig (widerspricht HSM-FA-HSM-002). Für lokale Dev-Loops
  ist `HSMDOC_PKCS11_PIN_DEV` als **alternative** Quelle (nicht
  Ergänzung) zulässig — genau eine PIN-Quelle ist aktiv, siehe
  §Konfiguration. Bedingungen für PIN_DEV: Start-Warn-Log,
  Verweigerung sobald `HSMDOC_ENV=prod` gesetzt ist oder wenn
  das Binary aus dem produktiven Container-Image startet
  (Detektion über Build-Marker im Image oder Env-Whitelist-Datei).

  Native Vault- oder K8s-Secret-CSI-Adapter bleiben Folge-Slice
  (eigener Open-Trigger oder Helm-Chart-Scope) — das
  Volume-Mount-Pattern reicht für M1.
- **Neu:** ChunkSealer-Port-Implementierung — AES-256-GCM-Operation
  je Chunk ([`HSM-FA-ENC-001..002`](../../../../spec/lastenheft.md),
  [`HSM-FA-ENC-006`](../../../../spec/spezifikation.md)):
  `C_EncryptInit(CKM_AES_GCM, gcmParams)` + `C_Encrypt(plaintext)`
  (kein `C_EncryptUpdate`-Streaming; Codepfad und PKCS#11-Trace-Test
  belegen die Granularität). `gcmParams` bindet die Pro-Chunk-AAD;
  Tag-Länge 128 Bit. Schlüsselattribut `CKA_EXTRACTABLE=false` ist
  Pflicht-Erwartung beim Key-Lookup.
  **Binding-Falle:** Der bequeme `github.com/miekg/pkcs11`-Helper
  `Ctx.Encrypt` DARF hier nicht verwendet werden, weil er intern zuerst
  einen Längen-Probe-Call `C_Encrypt(..., NULL, ...)` und danach den
  eigentlichen `C_Encrypt` ausführt. Das verletzt die
  HSM-FA-ENC-006-Akzeptanz „genau ein `C_Encrypt` pro Chunk". Der
  Adapter implementiert deshalb einen kleinen adapter-lokalen CGO-Shim
  (oder den durch den HKDF-Spike ohnehin dokumentierten Binding-Fork),
  der den Output-Buffer deterministisch als `len(plaintext)+16`
  vorallokiert und exakt einen `C_Encrypt` mit nicht-NULL Output-Puffer
  aufruft. Ein `CKR_BUFFER_TOO_SMALL` aus diesem Pfad bleibt
  `ADAPTER_BUFFER_UNDERSIZED` und ist Adapter-Bug.

### Key-Registry und minimaler Lifecycle
([`HSM-FA-KEY-001`](../../../../spec/lastenheft.md),
[`HSM-FA-KEY-002`](../../../../spec/lastenheft.md),
[`HSM-FA-KEY-004`](../../../../spec/lastenheft.md),
[`HSM-FA-DEC-003`](../../../../spec/spezifikation.md))

- **Neu (port):** `internal/hexagon/port/driven/keyregistry/` mit
  zwei Methoden:
  - `LookupActive(ctx, keyID) (KeyRecord, error)`
    — wird vom Encrypt-Use-Case aufgerufen (Proto-Header trägt
    keine `key_version`, siehe HeaderMAC-Port). Liefert den
    genau einen eindeutigen `active`-Eintrag. Fehler:
    - kein `active`-Eintrag für `keyID` → `FAILED_PRECONDITION` +
      `KEY_NOT_FOUND`.
    - mehrere `active`-Einträge → Schema-Fehler beim Start
      (Validierung); zur Laufzeit Fehlerklasse `INTERNAL` mit
      Detailcode `KEY_REGISTRY_AMBIGUOUS` (defensiv).
  - `Lookup(ctx, keyID, keyVersion) (KeyRecord, KeyStatus, error)`
    — wird vom Decrypt-Use-Case (Slice 003) aufgerufen.
    `KeyStatus` ∈ {`active`, `deprecated`, `destroyed`}.
  `KeyRecord` enthält nur logische Metadaten und Lookup-Labels
  (`key_id`, `key_version`, `status`, `algorithm`, `pkcs11_label`,
  `master_hmac_pkcs11_label`, Zeitfelder). Kein PKCS#11-Typ leakt
  über den Registry-Port.

  Encrypt-Verhalten je `KeyStatus` (gilt für Lookup-Aufrufe aus
  Encrypt — über `LookupActive` direkt nur `active` zu sehen, aber
  Defense-in-Depth):
  - `active` → Encrypt fährt durch.
  - `deprecated` → `FAILED_PRECONDITION` + `KEY_NOT_FOUND`
    (Detailcode `KEY_STATE_INVALID`; Encrypt darf deprecated Keys
    nicht nutzen).
  - `destroyed` → `FAILED_PRECONDITION` + `KEY_NOT_FOUND` (gemäß
    [`HSM-FA-DEC-003`](../../../../spec/spezifikation.md)).
- **Neu (adapter):** `internal/adapter/driven/keyregistry/file/`
  liest eine YAML-/JSON-Datei (Pfad
  `HSMDOC_KEYREGISTRY_PATH`, Default `/etc/hsmdoc/keys.yaml`). Jeder
  Eintrag trägt `key_id` (UUID), `key_version` (Integer), `status`,
  `algorithm`, `pkcs11_label` (Lookup-Label des AES-Encrypt-Keys),
  **`master_hmac_pkcs11_label`** (Lookup-Label des HMAC-IKM-Keys für
  Profil A HKDF-Ableitung gemäß
  [`HSM-FMT-006`](../../../../spec/spezifikation.md)), `created_at`,
  `rotated_at`. Schlüsselmaterial wird **nicht** dupliziert
  ([`HSM-FA-KEY-004`](../../../../spec/lastenheft.md)) — nur
  Metadaten und Lookup-Labels. Schema-Validierung beim Start
  (beide Labels Pflicht), harter Abbruch bei Schema-Fehler.
  Inkonsistenz auf Registry-Ebene führt zum Start-Abbruch; nicht
  auflösbare HSM-Labels werden erst vom `KeyBinding`-Port erkannt und
  führen beim Stream-Start zu `FAILED_PRECONDITION` + `KEY_NOT_FOUND`,
  nicht zum Start-Abbruch (operativ: Key kann erst nach Service-Start
  im HSM registriert worden sein).
- **Neu (port):** `internal/hexagon/port/driven/keybinding/` mit
  `Bind(ctx, KeyRecord) (KeyRef, error)`. `KeyRef` ist eine opake
  Struktur mit logischem Key-Snapshot und adapterinternem,
  session-affinem Handle-Resolver (siehe PKCS#11-Adapter); kein
  PKCS#11-Typ leakt über den Port.
- **Abgrenzung:** Keine Rotations-Trigger, kein Statuswechsel
  zur Laufzeit (das ist [`HSM-FA-KEY-003`](../../../../spec/lastenheft.md),
  M2-Scope). Eine externe Verwaltung (CLI, K8s-Operator) ändert die
  Datei, der Service liest beim nächsten Lookup neu (oder watcht
  per `fsnotify`, falls trivial).
- **Neu:** HMAC-SHA-256-Operation für Header-HMAC (auf dem in
  Profil A abgeleiteten Header-Key-Handle, siehe HeaderMAC-Port
  oben).

### gRPC-Adapter (`internal/adapter/driving/grpc/`)

- Encrypt-RPC nicht mehr `UNIMPLEMENTED`: Stream-Annahme, Stream-ID
  als UUIDv4 generieren
  ([`HSM-DATA-004`](../../../../spec/spezifikation.md)), Klartext-
  Frames an Application-Use-Case weiterreichen, Container-Bytes als
  Response-Frames emittieren (siehe Wire-Mapping unten).
- **mTLS-/Caller-Minimalpfad für M1:** Damit `HSM-FA-AUDIT-001`
  bereits in M1 semantisch erfüllt wird, zieht 002b den kleinsten
  produktiven Identitätspfad aus `HSM-API-GRPC-003` ein:
  `mtls-subject` ist in 002b die einzige zulässige Identitätsquelle.
  Der gRPC-Server lädt `HSMDOC_GRPC_CLIENT_CA`, setzt
  `tls.Config.ClientAuth = tls.RequireAndVerifyClientCert` und lehnt
  Encrypt-Streams ohne gültiges Client-Zertifikat mit
  `UNAUTHENTICATED` ab. Der `caller` wird aus dem Peer-Zertifikat
  abgeleitet (`HSMDOC_IDENTITY_MTLS_SUBJECT_ATTRIBUTE`, Default
  `subject_dn`, alternativ `san_uri`) und über den Stream-Kontext an
  Audit, Logs und Metriken gereicht. Die Header-Identitätsquelle,
  Peer-Allowlist, Mesh-Termination und die vollständige
  `identity.*`-Konfigurationsmatrix bleiben Slice 006; 002b etabliert
  nur den M1-Pfad, damit akzeptierte Streams nie mit
  Platzhalter-Caller laufen.
- **Single-Tenant-Normalisierung in M1:** Der Proto-Header trägt zwar
  `tenant_id`, Slice 002b unterstützt aber nur den
  Single-Tenant-Bootstrap. Für akzeptierte Streams verwendet der Server
  deshalb ausschließlich den kanonischen Tenant `default` in
  Container-Header, AAD, Audit, Logs und Metriken. Eingehende
  `EncryptHeader.tenant_id`-Werte sind entweder leer oder exakt
  `default`; jeder andere Wert wird vor Key-Lookup mit
  `INVALID_ARGUMENT` + Detailcode `TENANT_UNSUPPORTED_IN_M1`
  abgelehnt. So können Wire-Daten und Audit nicht auseinanderlaufen.
  Die echte Ableitung aus mTLS/Header-Identität bleibt Slice 006.
- **Proto-Erweiterung — Container-Wire-Mapping:** Das aktuelle
  `EncryptResponse`
  ([`spec/proto/chsmdocv1/c_hsm_doc.proto`](../../../../spec/proto/chsmdocv1/c_hsm_doc.proto))
  hat nur `EncryptAck`, `DataChunk`, `EncryptFinal` im `oneof body`
  und keine semantischen Felder für Container-Header (HSM-FMT-001)
  und Trailer (HSM-FMT-003). Slice 002b erweitert `EncryptResponse`
  um zwei explizite Oneof-Felder:
  ```proto
  message EncryptResponse {
    oneof body {
      EncryptAck ack = 1;
      DataChunk chunk = 2;
      EncryptFinal final = 3;
      bytes container_header = 4;   // HSM-FMT-001 Bytes
      bytes container_trailer = 5;  // HSM-FMT-003 Bytes
    }
  }
  ```
  Slice 001 hat nur `UNIMPLEMENTED`-Stubs ausgeliefert; eine
  additive Oneof-Erweiterung ist ohne Konsumenten ungefährlich,
  bricht aber das Schema-Diff. Der Wire-Ablauf eines Encrypt-
  Streams ist damit:
  1. Server → `EncryptAck{stream_id}`.
  2. Server → `container_header` mit den Header-Bytes (inkl.
     Header-HMAC).
  3. Pro Chunk: Server → `DataChunk{seq=k, payload=<frame-bytes>}`,
     wobei `seq` = Container-Frame-`seq` aus HSM-FMT-002
     (1-basiert, monoton, gilt strikt; out-of-order ist Bug).
     Die Doppelung (`DataChunk.seq` und der `seq`-Wert im
     HSM-FMT-002-Frame in `payload`) ist bewusst: der
     gRPC-Layer kann Backpressure/Reorder-Buffer-Logik
     fahren, ohne den Frame zu decodieren. Slice 003 spiegelt
     die Doppelung im Decrypt-Pfad.
     **Typ-Mismatch DataChunk.seq vs. HSM-FMT-002-`seq`:**
     `DataChunk.seq` ist heute `uint32` (Proto), HSM-FMT-002 spezifiziert
     `seq` als 8 Byte BE (uint64). Slice 002b zieht **`DataChunk.seq`
     auf `uint64` hoch** (additive Proto-Erweiterung im selben Diff
     wie `container_header`/`container_trailer`); ohne Konsumenten
     außerhalb der `UNIMPLEMENTED`-Stubs aus Slice 001 ist das
     wire-kompatibel (Protobuf-`uint32`/`uint64` sind beide Varint
     auf demselben Field-Number; Source-Break im generierten
     Go-Code ist genauso akzeptabel wie beim `REVOKED →
     DESTROYED`-Rename). Damit haben Wire-`seq` und Container-
     Frame-`seq` denselben Wertebereich; `EncryptTrailer.total_chunks`
     und `EncryptFinal.chunk_count` werden im selben Diff
     ebenfalls auf `uint64` gehoben.
  4. Server → `container_trailer` mit den Trailer-Bytes.
  5. Server → `EncryptFinal{stream_id, chunk_count, total_bytes}`.

  Der Reorder-Buffer der Application sorgt für die `seq`-Monotonie
  vor Emit. Decrypt-Wire (Slice 003) spiegelt den Mapping in der
  Request-Richtung: `DecryptRequest`-Oneof bekommt analog
  `container_header` und `container_trailer`. Slice 002b bereitet
  diese Proto-Felder additiv vor, lässt den Decrypt-Serverpfad aber
  weiter `UNIMPLEMENTED`; Slice 003 nutzt die Felder fachlich.
- **MaxRecvMsgSize** + Server-Keepalive setzen — schließt
  `TODO(slice-002)` aus [`cmd/hsmdoc/main.go:109-111`](../../../../cmd/hsmdoc/main.go)
  und Item §2.1 aus
  [`offene-arbeitsfaeden.md`](../in-progress/offene-arbeitsfaeden.md).
  Default: `max_recv` = Chunk-Size + 1 MiB Spielraum für Wire-Overhead.
- Cancellation-Pfad
  ([`HSM-FA-STREAM-004`](../../../../spec/spezifikation.md)):
  ≤ 100 ms keine neuen HSM-Ops für gecancelten Stream, laufende
  HSM-Ops zu Ende laufen lassen, Result verwerfen, Session bei
  undefiniertem Zustand aus Pool entfernen.
- Backpressure
  ([`HSM-FA-STREAM-003`](../../../../spec/spezifikation.md)): HTTP/2-
  Flow-Control durchsetzen, interne Queue-Tiefe bounded.

### Queue (`internal/hexagon/domain/queue/`)

- **Neu:** Begrenzte In-Memory-Job-Queue
  ([`HSM-FA-QUEUE-001`](../../../../spec/lastenheft.md),
  [`HSM-FA-QUEUE-002`](../../../../spec/spezifikation.md)) mit
  konfigurierbarer Kapazität (Default 256) **und** separater
  In-flight-Byte-Grenze. Stream-Pfad nutzt sie als Übergabepunkt
  zwischen Reader-Goroutine und Worker-Pool; Queue-Einträge tragen
  Plaintext-Chunks und dürfen deshalb nicht nur nach Item-Anzahl
  begrenzt werden.
- **In-flight-Byte-Semaphore (Pflicht in 002b):** Vor dem Einlesen eines
  Chunks reserviert der Reader `len(plaintext)` gegen
  `HSMDOC_INFLIGHT_PLAINTEXT_BYTES`. Die Reservierung wird erst
  freigegeben, wenn der Chunk entweder emittiert, endgültig fehlgeschlagen
  oder wegen Cancellation verworfen ist. Default: `268435456`
  (256 MiB) — deutlich unter dem `GOMEMLIMIT`-Default aus
  [`HSM-CC-002`](../../../../spec/spezifikation.md) (1 GiB), damit
  Encrypt-Pipeline + Reorder-Buffer + Audit-Buffering gemeinsam im
  Gesamt-Heap-Cap bleiben. Mindestwert `2 * HSMDOC_CHUNK_SIZE_BYTES`,
  Maximalwert `2147483648` (2 GiB). Start-Abbruch, wenn der konfigurierte
  Wert die Untergrenze unterschreitet oder die Obergrenze überschreitet.
  Damit bleiben Queue, Worker-Pool und Reorder-Buffer gemeinsam durch
  eine Byte-Grenze kontrolliert; eine Queue-Kapazität von 256 bedeutet
  dann nur Scheduling-Kapazität, nicht 256 volle Chunks im Heap.
  **Abgrenzung zu `GOMEMLIMIT`:** `HSMDOC_INFLIGHT_PLAINTEXT_BYTES`
  ist die anwendungsseitige In-flight-Plaintext-Cap (verhindert, dass
  die Encrypt-Pipeline den Heap füllt); `GOMEMLIMIT` aus
  [`HSM-CC-002`](../../../../spec/spezifikation.md) ist die Go-Runtime-
  Gesamt-Heap-Cap (Default 1 GiB, Range 256 MiB..8 GiB). Die zwei
  Größen sind semantisch verschieden: die In-flight-Cap muss
  ausreichend kleiner als `GOMEMLIMIT` bleiben, sonst löst die
  Go-Runtime GC-Pressure aus, bevor die Semaphore greift.
- **Wartezeit-Strategie**
  ([`HSM-FA-QUEUE-003`](../../../../spec/spezifikation.md)):
  Wartezeit vor Ablehnung ist konfigurierbar
  (`HSMDOC_QUEUE_WAIT_MS`, Default 0 ms = sofortige Ablehnung).
  Bei Überschreitung wird `RESOURCE_EXHAUSTED` mit Hinweis-
  Metadatum (empfohlene Wartezeit) emittiert
  ([`HSM-FA-QUEUE-002`](../../../../spec/spezifikation.md) Akzeptanz).

### Retry-Klassifikation (`internal/hexagon/domain/retry/`)

- **Neu:** Klassifikator für interne HSM-Fehlerklassen in `transient`,
  `permanent` und `client`
  ([`HSM-FA-RETRY-001`](../../../../spec/lastenheft.md)). Die Domain
  klassifiziert ausschließlich interne Fehlerklassen wie
  `SESSION_INVALID`, `HSM_DEVICE_ERROR`, `KEY_HANDLE_STALE` oder
  `INTERNAL`; sie kennt keine raw `CKR_*`-Codes. Die
  **CKR-Mapping-Tabelle** aus
  [`HSM-FA-FAIL-003`](../../../../spec/spezifikation.md) liegt
  adapter-nah unter `internal/adapter/driven/pkcs11/` als
  Code-Konstante + Unit-Test-Fixture und wird dort vollständig
  abgebildet — alle 14 Returncode-Klassen
  (`CKR_SESSION_HANDLE_INVALID`, `CKR_SESSION_CLOSED`,
  `CKR_DEVICE_ERROR`, `CKR_DEVICE_REMOVED`, `CKR_TOKEN_NOT_PRESENT`,
  `CKR_FUNCTION_FAILED`, `CKR_GENERAL_ERROR`,
  `CKR_USER_NOT_LOGGED_IN`, `CKR_PIN_INCORRECT`,
  `CKR_KEY_HANDLE_INVALID`, `CKR_MECHANISM_INVALID`,
  `CKR_BUFFER_TOO_SMALL`, `CKR_DATA_INVALID` /
  `CKR_ENCRYPTED_DATA_INVALID`, sonstige `CKR_*`) werden über ein
  Mock-PKCS#11-Modul exerziert. Nach dieser Übersetzung entscheidet
  der Domain-Retry-Klassifikator nur noch auf Basis der internen
  Fehlerklasse. **`CKR_BUFFER_TOO_SMALL` wird adapterseitig auf
  `INTERNAL` gemappt und zählt damit nicht als `transient`** — der
  Adapter muss den Output-Buffer für AES-GCM deterministisch als
  `len(plaintext)+16` Byte (128-Bit-Tag) vorallokieren, damit der
  Codepfad pro Chunk bei genau einem `C_Encrypt` bleibt. Ein realer
  `CKR_BUFFER_TOO_SMALL` ist Adapter-Bug, kein HSM-Zustand und auch
  keine Client-Verschuldung.
  Kein Retry, harter Stream-Fehler mit Fehlerklasse `INTERNAL` und
  Detailcode `ADAPTER_BUFFER_UNDERSIZED` (deckt sich mit
  HSM-FA-FAIL-003-Mapping `INTERNAL`). Damit ist ein
  Endlos-Retry-Loop auf einen Implementierungsfehler ausgeschlossen.
  **Wichtig:** Slice 002b implementiert die _Mapping-Tabelle_ und
  die _Reaktionen_, die der Encrypt-Pfad direkt braucht
  (Session-Recycling, Retry, partieller Sicherheits-Smoke auf
  DEVICE_REMOVED/TOKEN_NOT_PRESENT — siehe unten). Die vollständige
  Reaktionssemantik aus FAIL-003/FAIL-006 (insbesondere
  Token-Removal-Recovery und Reconnect) fällt unter
  [`HSM-FA-FAIL-001`](../../../../spec/lastenheft.md), das laut
  [`roadmap.md`](../in-progress/roadmap.md) §M2 explizit M2-Scope ist
  (Slice 009). Slice 002b erfüllt damit das _Mapping_ aus FAIL-003,
  nicht die vollständige FAIL-003-Reaktionskette.
- Session-Lifecycle-Reaktion folgt
  [`HSM-FA-FAIL-004`](../../../../spec/spezifikation.md):
  Sessions mit Fehlerklasse `SESSION_INVALID`, `HSM_DEVICE_ERROR`,
  `HSM_FUNCTION_FAILED`, `HSM_GENERAL_ERROR` oder `KEY_HANDLE_STALE`
  werden aus dem Pool entfernt und ersetzt; Metrik
  `hsmdoc_sessions_recycled_total` wird inkrementiert.
- Re-Login bei `HSM_NOT_LOGGED_IN` gemäß `pool.loginRetry` (Default
  3) — eingebettet in den Session-Pool-Adapter, aber durch
  [`HSM-FA-FAIL-008`](../../../../spec/spezifikation.md) gedrosselt
  (Default maximal ein Re-Login pro Session pro 60 s). Metrik
  `hsmdoc_hsm_relogin_total` zählt Re-Login-Versuche pro Slot.
- **Exponential Backoff mit Jitter**
  ([`HSM-FA-RETRY-003`](../../../../spec/spezifikation.md)):
  Retries warten zwischen den Versuchen exponentiell wachsend.
  Defaults: Basis 50 ms, Faktor 2, max. 5 Versuche; alle drei
  konfigurierbar (`HSMDOC_RETRY_BASE_MS`, `HSMDOC_RETRY_FACTOR`,
  `HSMDOC_RETRY_MAX_ATTEMPTS`). Jitter ist `±25 %` (uniform
  randomisiert) um den jeweiligen Backoff-Wert herum, fest verdrahtet
  (Spec verlangt Jitter, ohne konkretes Verfahren). Nach
  `MAX_ATTEMPTS` wird der Chunk als `FAILED_PERMANENT` markiert und
  der Stream abgebrochen.
- Retry pro Chunk hält `seq` und Klartext stabil, generiert neue
  Nonce je Retry
  ([`HSM-FA-CHUNK-006`](../../../../spec/spezifikation.md)).
- **Token-/Device-Removal (partieller M1-Smoke in 002b):** Bei
  `CKR_DEVICE_REMOVED` und `CKR_TOKEN_NOT_PRESENT` muss der Adapter
  laut [`HSM-FA-FAIL-003`](../../../../spec/spezifikation.md)
  „Pool drainen, Readiness rot, Reconnect-Schleife" auslösen.
  Slice 002b setzt als defensiven M1-Smoke nur die ersten zwei Schritte
  um; das ist **keine vollständige Erfüllung von HSM-FA-FAIL-003**:
  - **Pool-Drain:** Alle Sessions zur betroffenen HSM-Quelle werden
    aus dem Pool entfernt; offene Encrypt-Streams brechen mit
    `HSM_REMOVED` bzw. `HSM_TOKEN_GONE` ab; neue Streams werden mit
    `UNAVAILABLE` abgelehnt.
  - **Readiness rot:** Der Health-Adapter
    ([`HSM-DATA-003`](../../../../spec/spezifikation.md)) wechselt
    `hsmStatus` auf `DOWN`; `/readyz` liefert HTTP `503`.
  Damit wird in M1 verhindert, dass Ciphertext auf einer kaputten
  Quelle emittiert wird. Die vollständige HSM-FA-FAIL-003-/006-
  Reaktionskette bleibt Slice 009.
- **Abgrenzung — bleibt M2:**
  - **Reconnect-Schleife** mit Exponential Backoff
    ([`HSM-FA-FAIL-006`](../../../../spec/spezifikation.md) Schritte
    3–4: Pool neu auffüllen nach erfolgreichem
    `C_Initialize`/`C_OpenSession`/`C_Login`).
  - **Circuit Breaker** mit Fehlerraten-Fenster
    ([`HSM-FA-FAIL-005`](../../../../spec/spezifikation.md)).
  - **Netzwerk-HSM-Heartbeat**
    ([`HSM-FA-FAIL-007`](../../../../spec/spezifikation.md)).

  Diese drei vervollständigen die Spec-Reaktion und sind in
  Slice 009 zusammengeführt (siehe
  [`offene-arbeitsfaeden.md`](../in-progress/offene-arbeitsfaeden.md) §3).
  Bis Slice 009 abgeschlossen ist, muss ein Operator nach
  Token-Removal den Service neu starten — das ist dokumentierte
  M1-Einschränkung.

### Audit-Adapter (minimal durabel)
([`HSM-FA-AUDIT-005`](../../../../spec/lastenheft.md),
[`HSM-FA-AUDIT-010`](../../../../spec/spezifikation.md),
[`HSM-FA-RETRY-002`](../../../../spec/lastenheft.md),
[`HSM-FA-CHUNK-007`](../../../../spec/spezifikation.md))

Ein No-Op-Adapter erfüllt die Audit-/Commit-Pflicht **nicht**:
`audit-attempt` darf erst dann committed sein, wenn der Eintrag
durabel persistiert ist; ohne Persistenz darf kein Ciphertext-Chunk
emittiert werden ([`HSM-FA-AUDIT-010`](../../../../spec/spezifikation.md)
ist explizit). Slice 002b zieht deshalb einen minimal durablen
Audit-Sink ein:

- **Neu (port):** `internal/hexagon/port/driven/audit/` — Interface
  `AuditSink.Write(ctx, entry) error`. **Vertragsgarantie:** Der
  Aufruf kehrt erst zurück, wenn der Eintrag durabel persistiert ist
  (`fsync` oder Backend-Äquivalent abgeschlossen). Das gilt
  unabhängig von der Sync-Strategie — bei `batched-fsync` blockiert
  `Write` so lange, bis der enthaltende Batch synchronisiert wurde
  (Implementierung: interne Queue + Sync-Signal je Eintrag; Caller
  wartet auf das Signal). Damit ist HSM-FA-AUDIT-010 unabhängig vom
  Strategie-Modus erfüllt.
- **Neu (adapter):** `internal/adapter/driven/audit/file/` —
  JSONL-Append-Sink. Schreibreihenfolge append-only in `seq`-Folge je
  Stream. Der Adapter validiert diese Reihenfolge defensiv und lehnt
  einen Eintrag ab, dessen `seq` **kleiner als der zuletzt
  erfolgreich persistierte `seq` ist** (Rückwärtssprung) **oder
  größer als der aktuell offene `seq+1`** (Vorwärtssprung über den
  nächsten erwarteten Wert hinaus). Mehrere Einträge zum **selben
  offenen `seq=k`** sind dagegen ausdrücklich zulässig — Retries
  produzieren je Versuch einen eigenen `result=error`-Eintrag
  (siehe Commit-Idempotenz-Akzeptanz), und erst der finale
  `result=ok` mit demselben `seq=k` schließt den Eintrag ab; der
  nächste zulässige `seq` ist danach `k+1`. Das primäre Ordering
  passiert im Encrypt-Use-Case vor `AuditSink.Write`, nicht im
  Dateiadapter. Sync-Strategie
  konfigurierbar
  ([`HSM-FA-AUDIT-010`](../../../../spec/spezifikation.md)):
  - `batched-fsync` (Default; ≤ 100 ms oder ≤ 1000 Einträge):
    `Write` puffert intern, wartet auf den Batch-Sync und kehrt
    erst danach zurück. Liefert besseren Durchsatz, ohne die
    Durability-Garantie aufzuweichen.
  - `per-entry-fsync` (für regulierte Umgebungen): `Write` synct
    nach jedem Eintrag separat.
- **Sync-Fehler-Pfad:** `Write` gibt Fehler zurück → Encrypt-Use-Case
  bricht den Stream mit Fehlerklasse `AUDIT_DURABILITY_FAILED` ab
  ([`HSM-FA-AUDIT-010`](../../../../spec/spezifikation.md)).
  Der Adapter versucht zusätzlich einen Fehler-Audit-Eintrag mit
  `operation=error` und `error_class=AUDIT_DURABILITY_FAILED` durabel
  zu schreiben; schlägt auch dieser Schreibversuch fehl, ist der
  Prozessfehler über Log/Metrik sichtbar und es wird kein weiterer
  Ciphertext emittiert.
- **Klartext-Verbot:** Eintrag enthält nur Pflichtfelder gemäß
  [`HSM-FA-AUDIT-001`](../../../../spec/lastenheft.md) /
  [`HSM-DATA-001`](../../../../spec/spezifikation.md);
  Klartext-/Schlüssel-/Cipher-Inhalt ist verboten
  ([`HSM-FA-AUDIT-003`](../../../../spec/lastenheft.md)).
- **Pflichtfeld-Befüllung (Slice-002b-Festlegung):** HSM-DATA-001
  listet 15 Pflichtfelder. Slice 002b liefert für `caller` den
  minimalen `mtls-subject`-Pfad und verwendet für Tracing noch einen
  deterministischen Platzhalter, solange der vollständige OTel-Stack
  erst in M2 kommt. Damit das Schema deterministisch validierbar ist,
  legt Slice 002b für jedes Pflichtfeld eine Quelle fest:

  | Feld           | Quelle in Slice 002b                                                                                                                           |
  | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
  | `timestamp`    | Aus `AuditClock` in UTC, RFC 3339 mit Nanosekunden, monoton steigend pro Stream (Re-Read vom Audit-Sink prüft Monotonie je Stream).             |
  | `operation`    | `encrypt` für jeden `audit-attempt` mit `result=ok`/`result=error`; `error` für `STREAM_ABORTED`- und `AUDIT_DURABILITY_FAILED`-Sonderfälle.    |
  | `key_id`       | Aus `EncryptHeader.key_id` (Proto, Stream-Annahme).                                                                                            |
  | `key_version`  | Serverseitig aus `KeyRegistry.LookupActive` aufgelöst (siehe §HeaderMAC-Port). Im Stream invariant durch den `KeyRef`-Snapshot.                |
  | `doc_id`       | Aus `EncryptHeader.doc_id` (Proto, Stream-Annahme). Leer ist Schema-Fehler → Stream-Reject mit `INVALID_ARGUMENT`; Slice 002b schreibt also nie einen `audit-attempt` mit leerem `doc_id`. |
  | `caller`       | Aus dem verifizierten mTLS-Peer-Zertifikat (`subject_dn` oder `san_uri`, siehe gRPC-Adapter §mTLS-/Caller-Minimalpfad). Leer/fehlend wird vor Use-Case-Aufruf mit `UNAUTHENTICATED` abgelehnt. |
  | `tenant_id`    | Kanonischer Server-Tenant `default` in Slice 002b; abweichende Header-Werte werden vor Key-Lookup abgelehnt. Multi-Tenant ab M4.                |
  | `result`       | `ok` (Versuch lieferte Ciphertext) oder `error` (Versuch scheiterte).                                                                          |
  | `error_class`  | Bei `result=ok` der konstante String `"none"` (nicht leer, nicht `null`, damit Schema-Validierung deterministisch greift); bei `result=error` die Klasse aus der CKR-Mapping-Tabelle (§Retry-Klassifikation). |
  | `attempt`      | 1-basierter Versuchszähler pro `(stream_id, seq)`. Erfolgreicher Versuch erhöht den Zähler nicht — er trägt den Wert seines Versuchs (z. B. `attempt=4` nach drei transienten Fehlern). |
  | `chunk_count`  | Pro `audit-attempt` der **bis zu diesem Versuch verarbeitete Klartext-Stand**: bei `result=ok` gleich `seq`; bei `result=error` der zum Versuch gehörige `seq`. Niemals der bereits emittierte Stand. |
  | `total_bytes`  | Summe der Klartext-Bytes bis einschließlich des aktuellen `seq` (analog `chunk_count`).                                                        |
  | `request_id`   | gRPC-Metadata-Header `x-request-id` falls vorhanden, sonst serverseitig generierte UUIDv4 je Stream-Annahme. Server spiegelt den Wert in `trailing-metadata` zurück (Korrelations-Hilfe). |
  | `trace_id`    | gRPC-Metadata-Header `traceparent` (W3C Trace-Context) falls vorhanden — extrahiert den 16-Byte-Trace-Id-Teil. Sonst hex-encodierte 16-Byte-UUIDv4 als Platzhalter. Slice 002b reicht `traceparent` nur durch, baut keinen aktiven Tracer; der vollständige OTel-Anschluss folgt im Observability-Slice (M2). |
  | `stream_id`    | UUIDv4 vom Server bei Stream-Annahme generiert ([`HSM-DATA-004`](../../../../spec/spezifikation.md)).                                          |

  Der letzte erfolgreiche `audit-attempt` des Streams trägt damit
  automatisch die Endwerte; ein zusätzlicher Schluss-Eintrag ist
  nicht nötig und wäre auch nicht spec-konform — HSM-DATA-001 legt
  die zulässigen `operation`-Werte abschließend fest
  (`encrypt`/`decrypt`/`key-lookup`/`key-rotate`/`error`). Die
  Stream-Aggregate sind zusätzlich über das gRPC-`EncryptFinal`
  transportiert und im Trace nachvollziehbar.

- **Trusted-Time-Vertrag für HSM-FA-AUDIT-005:** Slice 002b führt einen
  kleinen Clock-Port `AuditClock.Now(ctx) (time.Time, ClockStatus, error)`
  ein. Der File-Audit-Adapter nimmt Zeitstempel ausschließlich aus diesem
  Port entgegen; direkte `time.Now()`-Aufrufe im Audit-Pfad sind per
  Code-Review- und Unit-Test-Check verboten. Der Produktionsadapter nutzt
  die Systemuhr nur, wenn `HSMDOC_TIME_SOURCE=system-ntp` gesetzt ist und
  beim Start eine explizite Trust-Assertion vorliegt
  (`HSMDOC_TIME_TRUSTED=true`, gesetzt durch Deployment/Node-Baseline).
  Ohne diese Assertion startet der Service mit
  `STARTUP_TIME_SOURCE_UNTRUSTED` nicht. Der CI-/Unit-Test-Pfad nutzt
  einen Fake-Clock-Adapter, um Monotonie und Fehlerpfade deterministisch
  zu prüfen. Die spätere Helm-/Kind-Integration aus Slice 005 belegt die
  Node-/Cluster-Seite der NTP-Synchronisation; 002b stellt sicher, dass
  der Service keine Audit-Zeitstempel ohne explizit vertrauenswürdige
  Zeitquelle akzeptiert.

- Sonderfall `STREAM_ABORTED`: bei Client-Cancel, Netzwerkabbruch
  oder lokalem Stream-Abbruch vor `stream-final-commit` wird ein
  zusätzlicher Audit-Eintrag mit `operation=error`, `result=error`
  und `error_class=STREAM_ABORTED` durabel persistiert
  ([`HSM-FA-CHUNK-007`](../../../../spec/spezifikation.md)).
  `attempt` trägt dabei den letzten begonnenen Versuchswert,
  `chunk_count`/`total_bytes` den zuletzt erreichten Stand.
- **Caller-Quelle:** In Slice 002b gibt es keinen
  `caller`-Platzhalter für akzeptierte Streams. Der Wert kommt aus dem
  verifizierten mTLS-Peer-Zertifikat (`subject_dn` oder `san_uri`).
  Fehlt ein gültiges Client-Zertifikat oder ist das konfigurierte
  Attribut leer, wird der Stream vor Key-Lookup mit
  `UNAUTHENTICATED` bzw. `IDENTITY_MISSING` abgelehnt. Die Header-
  Quelle und Peer-Allowlist aus
  [`HSM-API-GRPC-006..008`](../../../../spec/spezifikation.md) bleiben
  Slice 006; `tenant_id` bleibt in 002b der kanonische Wert `default`.
- **Abgrenzung:** **Keine Hash-Chain, keine Signatur, keine externe
  Verankerung, kein Verify-Tool** in Slice 002b. Die Basis-Hash-Chain
  ([`HSM-FA-AUDIT-002`](../../../../spec/lastenheft.md)) ist
  M1-Scope und kommt in Slice 004 vor M1-Closure. Slice 002b schreibt
  nur die Durability- und Reihenfolge-Pflicht aus HSM-FA-AUDIT-005/010,
  plus Pflichtfelder + Klartext-Verbot; Slice 004 ergänzt
  Manipulationsschutz ohne Port-Bruch. Die regulierten Detail-
  Verfahren ([`HSM-FA-AUDIT-006..008`](../../../../spec/spezifikation.md))
  sind M2-Scope.
- **Audit-Reihenfolge** `audit-attempt` vor `emit-commit` ist im
  Encrypt-Use-Case verdrahtet und durch Out-of-order-/Cancel-/Sync-
  Fehler-Tests abgedeckt.

### Konfiguration (`internal/config/`)

Neue Env-Variablen, alle in `Load()` validiert
([`HSM-OPS-CFG-001..002`](../../../../spec/lastenheft.md)).

**Startup-Fehlerklassen (neu in Slice 002b):** HSM-FA-FAIL-003
normiert Laufzeit-Fehlerklassen aus PKCS#11-Returncodes, sagt
aber nichts über Start-Validierungsfehler. Slice 002b etabliert
deshalb eine `STARTUP_*`-Sammelklasse für deterministische
Start-Abbrüche, exit-code ≠ 0 mit eindeutigem Log-String. Die in
diesem Slice eingeführten Codes:
`STARTUP_PKCS11_DLOPEN_FAILED` (Closure-Smoke schlägt fehl),
`STARTUP_PKCS11_PIN_AMBIGUOUS` (mehr als eine PIN-Quelle gesetzt),
`STARTUP_PKCS11_PIN_MISSING` (keine PIN-Quelle gesetzt),
`STARTUP_PKCS11_PIN_DEV_FORBIDDEN` (`PIN_DEV` im Produktionsmodus
oder Produktionsimage) und `STARTUP_PKCS11_CGO_DISABLED`
(PKCS#11-Adapter ohne CGO initialisiert) sowie
`STARTUP_IDENTITY_CLIENT_CA_MISSING` (`mtls-subject`-M1-Pfad ohne
Client-CA konfiguriert) und
`STARTUP_TIME_SOURCE_UNTRUSTED` (Audit-Zeitquelle nicht explizit als
NTP-synchronisiert vertrauenswürdig freigegeben). Weil diese Codes
observable Test- und Betriebsoberfläche sind, ergänzt Slice 002b die
Spezifikation im selben PR um einen neuen Abschnitt
`HSM-FA-FAIL-010 — Startup-Validierungsfehler` und dokumentiert dort
die eingeführten `STARTUP_*`-Codes samt gRPC-unabhängiger Exit-Semantik.
Folge-Slices erweitern denselben Abschnitt additiv.

- `HSMDOC_GRPC_CLIENT_CA` — PEM-Bundle der Client-CA für mTLS.
  Pflicht in 002b, weil akzeptierte Encrypt-Streams einen echten
  Audit-`caller` aus dem Client-Zertifikat brauchen. Fehlt der Wert
  oder ist das Bundle unlesbar/ungültig, startet der Service mit
  `STARTUP_IDENTITY_CLIENT_CA_MISSING` nicht. Slice 006 erweitert
  diesen Minimalpfad später um `identity.source=header` und
  Peer-Allowlist.
- `HSMDOC_IDENTITY_MTLS_SUBJECT_ATTRIBUTE` — `subject_dn` (Default)
  oder `san_uri`. Bestimmt, welcher Wert aus dem verifizierten
  Client-Zertifikat als Audit-`caller` verwendet wird. Andere Werte
  → Start-Abbruch; leerer extrahierter Wert → Anfrage-Abbruch mit
  `IDENTITY_MISSING`.
- `HSMDOC_PKCS11_MODULE` — Pfad zum Modul (`.so`/`.dll`). Pflicht.
- `HSMDOC_PKCS11_SLOT` oder `HSMDOC_PKCS11_TOKEN_LABEL` — Slot-/Token-
  Auswahl. Genau eine Quelle Pflicht.
- **PIN-Quelle — genau eine aktiv (Pflicht):** entweder
  `HSMDOC_PKCS11_PIN_FILE` (produktiv) **oder**
  `HSMDOC_PKCS11_PIN_DEV` (Dev-Pfad mit Whitelist-Check).
  Beide gleichzeitig gesetzt → Start-Abbruch
  (`STARTUP_PKCS11_PIN_AMBIGUOUS`); keine von beiden gesetzt →
  Start-Abbruch (`STARTUP_PKCS11_PIN_MISSING`).
  - `HSMDOC_PKCS11_PIN_FILE` — Pfad zur PIN-Datei. Mode aus
    Whitelist `{0400, 0440}` mit modus-abhängiger UID-/GID-
    Bindung (siehe PKCS#11-Adapter §PIN); Mode-Verstoß, fremder
    Owner bei `0400`, fremde GID bei `0440` oder Whitelist-Miss
    → Start-Abbruch.
  - `HSMDOC_PKCS11_PIN_DEV` — **nur Dev**. Wird nur akzeptiert,
    wenn `HSMDOC_ENV≠prod` und nicht im Container-Build aktiv
    (siehe PKCS#11-Adapter-Abschnitt). Das Produktionsimage trägt
    einen Build-Marker; ist dieser Marker vorhanden, wird
    `PIN_DEV` auch dann verweigert, wenn `HSMDOC_ENV` fehlt oder
    falsch gesetzt ist. Jeder akzeptierte Dev-Override emittiert
    eine Start-Warn-Log-Zeile mit Hinweis auf HSM-FA-HSM-002.
- `HSMDOC_PKCS11_POOL_SIZE`, `_MAX_IDLE`, `_MAX_LIFETIME`,
  `_ACQUIRE_TIMEOUT`, `_LOGIN_RETRY` — Pool-Tuning, alle mit
  Defaults aus
  [`HSM-FA-HSM-004`](../../../../spec/spezifikation.md).
- `HSMDOC_CHUNK_SIZE_BYTES` — Default 4 MiB, Bereich 64 KiB..64 MiB
  ([`HSM-FA-CHUNK-008`](../../../../spec/spezifikation.md)),
  außerhalb des Bereichs → Start-Abbruch.
- `HSMDOC_WORKERS` — Worker-Pool-Größe, Default `runtime.NumCPU() * 2`,
  Bereich 1..512. HSM-ARCH-003 fordert einen Worker-Pool, gibt aber
  keine Größe vor; der Default ist Slice-Entscheidung (CPU-skalierend
  mit Faktor 2, um HSM-IO-Wartezeit zu überdecken) und wird in
  M3 (Performance-Profile, `HSM-NFA-PERF-001..004`) nachgeschärft,
  sobald Messdaten vorliegen
  ([`HSM-ARCH-003`](../../../../spec/spezifikation.md)).
- `HSMDOC_QUEUE_DEPTH` — Job-Queue-Kapazität, Default 256
  ([`HSM-FA-QUEUE-002`](../../../../spec/spezifikation.md)).
- `HSMDOC_QUEUE_WAIT_MS` — Wartezeit vor Ablehnung, Default 0
  ([`HSM-FA-QUEUE-003`](../../../../spec/spezifikation.md)).
- `HSMDOC_INFLIGHT_PLAINTEXT_BYTES` — harte Byte-Grenze für
  In-flight-Plaintext (Reader-Buffer + Queue + Worker + Reorder).
  Default 256 MiB, Range `2 * HSMDOC_CHUNK_SIZE_BYTES` bis 2 GiB.
  Abgrenzung zu `GOMEMLIMIT` (HSM-CC-002): siehe §Queue.
- `HSMDOC_RETRY_BASE_MS` / `_FACTOR` / `_MAX_ATTEMPTS` — Exponential-
  Backoff-Parameter, Defaults 50 / 2 / 5
  ([`HSM-FA-RETRY-003`](../../../../spec/spezifikation.md)).
- `HSMDOC_KEYREGISTRY_PATH` — Pfad zur Key-Registry-Datei
  (Default `/etc/hsmdoc/keys.yaml`,
  [`HSM-FA-KEY-002`](../../../../spec/lastenheft.md),
  [`HSM-FA-KEY-004`](../../../../spec/lastenheft.md)).
- **`HSMDOC_MAX_HEAP_BYTES` entfällt.** Die Go-Runtime-Gesamt-Heap-Cap
  ist `GOMEMLIMIT` aus
  [`HSM-CC-002`](../../../../spec/spezifikation.md) (Default 1 GiB,
  Range 256 MiB..8 GiB); die anwendungsseitige In-flight-Cap ist
  `HSMDOC_INFLIGHT_PLAINTEXT_BYTES` (siehe oben).
- `HSMDOC_AUDIT_SINK_PATH` — Pfad zur JSONL-Audit-Datei. Pflicht.
- `HSMDOC_AUDIT_SYNC_MODE` — `batched-fsync` (Default) oder
  `per-entry-fsync`
  ([`HSM-FA-AUDIT-010`](../../../../spec/spezifikation.md)).
  M1 läuft im Klassifikations-Modus „Standardumgebung" gemäß
  HSM-FA-AUDIT-010; regulierte Profile setzen
  `HSMDOC_AUDIT_SYNC_MODE=per-entry-fsync` und werden in
  M2-Compliance-Slices abgenommen. Die `Write`-Durability-Garantie
  ist in **beiden** Modi gleich (siehe §Audit-Adapter:
  `Write` blockt bis fsync).
- `HSMDOC_TIME_SOURCE` — in 002b genau `system-ntp`. Andere Werte
  sind reserviert und führen zu Start-Abbruch.
- `HSMDOC_TIME_TRUSTED` — muss im Produktionspfad exakt `true` sein;
  fehlt der Wert oder ist er anders gesetzt, startet der Service mit
  `STARTUP_TIME_SOURCE_UNTRUSTED` nicht. CI-Tests dürfen den
  Fake-Clock-Adapter verwenden und setzen diese Variable nicht.

### Lint-Regeln (`.golangci.yml`)

- **Application-Schicht-Sperre:** Die bestehende `depguard`-Regel
  für `internal/hexagon/application/**` wird so erweitert, dass sie
  Imports aus `internal/adapter/driven/pkcs11/`, `internal/adapter/driven/audit/`
  und `internal/adapter/driven/keyregistry/` explizit verbietet —
  Application darf diese Adapter ausschließlich über die Ports
  (`KeyRegistry`, `KeyBinding`, `ChunkSealer`, `HeaderMAC`, `AuditSink`)
  konsumieren. **Test-Mock-Ablage außerhalb `adapter/driven/`:**
  Damit die Sperre nicht versehentlich Application-Unit-Tests blockt,
  wohnt der In-Memory-Mock-Adapter unter
  `internal/testfixtures/chunksealer/` (und analog für andere Ports),
  **nicht** unter `internal/adapter/driven/chunksealer/inmemory/`.
  `internal/testfixtures/**` darf von Application-Tests importiert
  werden; produktiver Code (inkl. Adapter) hat keinen Grund, dorthin
  zu greifen — die `depguard`-Matrix sperrt entsprechend.
  Ein zusätzliches Make-Target `make check-hexagon-imports`
  (Docker-only) führt einen statischen `grep`-Check über
  `internal/hexagon/application/**.go` (ohne `_test.go`) durch und
  scheitert deterministisch, sobald ein PKCS#11- oder
  `adapter/driven`-Importpfad auftaucht. Das Target ist in
  `make ci` eingehängt.
- Re-Bewertung der Cross-Adapter-Sibling-Regel
  ([`offene-arbeitsfaeden.md` §1.1](../in-progress/offene-arbeitsfaeden.md)):
  Mit `driven/pkcs11/` zieht ein zweites Adapter-Sibling ein.
  Slice 002b entscheidet dokumentiert, ob die Sibling-Filter-Regel
  jetzt sinnvoll wird (z. B. selektiv „driving darf driven nicht
  importieren, nur via Port") oder weiter aufgeschoben bleibt.

## Vorbedingungen für die Aktivierung

**Reihenfolge der Vorbedingungen ist strikt seriell:** 1 (Slice 002a
in `done/`, inkl. Folge-ADR zu `ADR 0001`, die das
`next/<slice>/`-Sub-Pattern legitimiert) → 3 (HKDF-Spike unter
`next/002b-spike-hkdf/` läuft + ist grün) → Slice 002b nach
`in-progress/`. Vorbedingung 2 (Coverage) ist eine fortlaufende
Anforderung, kein Reihenfolge-Gate. Damit ist die Spike-Ablage nicht
zirkulär: die Sub-Verzeichnis-Legitimation kommt aus 002a, der Spike
darf erst danach starten.

1. **Slice 002a** ist nach `done/` migriert (CGO-Build-Pipeline,
   `pkcs11-dlopen-smoke`-Binary, Closure-Verifikation, ADR 0004,
   Folge-ADR zur Planstruktur — alles laut Akzeptanz von 002a).
   Ohne 002a kein CGO-Pfad, kein dlopen-Smoke, keine Vendor-Modul-
   Vorbereitung im CI und keine ADR-Grundlage für den
   `next/002b-spike-hkdf/`-Sub-Pfad.
2. **Coverage-Schwellwert ≥ 80 %** bleibt erhalten — der neue
   Adapter + Domain-Layer wird per Unit-Tests + Integrationstests
   abgedeckt; Integrationstests laufen mit Build-Tag und werden in
   die Coverage-Aggregation einbezogen (siehe Akzeptanz §1).
3. **`CKM_HKDF_DERIVE`-Spike** muss vor Slice-Aktivierung grün sein
   (zeitlich nach Vorbedingung 1, weil das Sub-Verzeichnis-Pattern
   erst durch die Folge-ADR aus 002a explizit zulässig ist).
   Das vorgeschriebene Go-Binding
   ([`github.com/miekg/pkcs11`](https://pkg.go.dev/github.com/miekg/pkcs11),
   `HSM-API-P11-003`) hat keine native Unterstützung für
   `CK_HKDF_PARAMS` in der öffentlichen API. Der Spike validiert
   **gegen beide CI-Module** (SoftHSM v2 **und** das in ADR 0004
   gewählte zweite herstellerfremde OSS-Modul — Default
   OpenCryptoki), nicht nur SoftHSM. Dadurch wird verhindert, dass
   `CKM_AES_GCM` + `CKM_HKDF_DERIVE` erst im späten Akzeptanztest
   am Zweitmodul scheitern. Drei Pfade — Ergebnis in einer neuen
   Folge-ADR zu ADR 0004 (geplant: `ADR 0005 —
   HKDF-Profil-A-Binding`) protokolliert:
   - (a) **Shim:** `CK_HKDF_PARAMS` wird als `[]byte` korrekt
     serialisiert (C-Struct-Layout aus PKCS#11 v3.0 §6.31) und über
     `pkcs11.NewMechanism(CKM_HKDF_DERIVE, paramBytes)` an
     `C_DeriveKey` übergeben; **beide CI-Module** akzeptieren den
     Aufruf und liefern einen Header-Key-Handle mit
     `CKA_EXTRACTABLE=false`.
   - (b) **Forked Binding:** Ein gepflegter Fork von
     `github.com/miekg/pkcs11` mit nativer `CK_HKDF_PARAMS`-
     Unterstützung wird ausgewählt, die `replace`-Direktive in
     `go.mod` dokumentiert, und der Spike bestätigt
     funktionierende Ableitung **auf beiden Modulen**.
   - (c) **Fallback-Eskalation:** Beide oben schlagen auf einem
     der Module fehl. Slice 002b wird zurück zur Planung
     geschoben — entweder mit Profil B als M1-Pfad (vendor-
     spezifische Non-Export-Konstruktion) oder mit einer
     Binding-Wechsel-Entscheidung als eigener Open-Trigger oder
     mit einem anderen Zweitmodul.

   Die Wahl des Zweitmoduls (Modulpfad, Token-Konfiguration,
   erwartete Mechanismen) ist bereits in Slice 002a (ADR 0004)
   festgehalten; der Spike validiert, dass `CKM_HKDF_DERIVE`
   gegen genau dieses Modul funktioniert. Das Validierungsergebnis,
   der gewählte Binding-Pfad (Shim oder Fork) und ggf. die
   Fallback-Entscheidung werden nicht nachträglich in ADR 0004
   geschrieben, weil ADR 0004 nach 002a `Accepted` ist.

   **Spike-Ablage:** Der Spike läuft komplett in `next/` als
   eigenes Sub-Artefakt unter `next/002b-spike-hkdf/`. Das
   Sub-Verzeichnis-Pattern unter `next/` ist durch die Folge-ADR zu
   `ADR 0001` aus Slice 002a jetzt explizit zulässig. Layout:
   - `README.md` — Vorgehen, geprüfte Pfade (a/b/c), Ergebnis,
     Verweis auf ADR 0004 (Modulwahl) und ADR 0005
     (Binding-/HKDF-Entscheidung).
   - `spike/` — minimaler Go-Code, der `C_DeriveKey` mit
     `CKM_HKDF_DERIVE` gegen beide CI-Module aufruft (kein
     Application-Code, isoliert vom restlichen Repo per Build-Tag
     `spike`).
   - `trace/` — PKCS#11-Aufrufprotokoll (SoftHSM-Log bzw.
     `pkcs11-spy`-Output) pro Modul als reproduzierbarer Beleg.

   Sobald der Spike grün ist, wandert das `next/002b-spike-hkdf/`-
   Verzeichnis mit dem Slice nach `in-progress/` (und später nach
   `done/`) als historischer Spike-Nachweis. Slice 002b wird
   **nicht** ohne diesen Spike-Output aktiviert. Der Spike erzeugt
   außerdem die Folge-ADR zu ADR 0004 (geplant: ADR 0005) und den
   ADR-Index-Eintrag; ohne diese ADR-Spur wird 002b nicht nach
   `in-progress/` migriert.

## Akzeptanzkriterien

- `make ci` läuft grün gegen den Slice-002b-Code (Lint inkl.
  `depguard`-Regeln, Pure-Go-Unit-Tests, CGO-Integrationstests,
  Coverage ≥ 80 % auf `./internal/...`, docs-check, `proto-check`,
  `proto-breaking`, govulncheck). Slice 002b erweitert den
  Makefile-Aggregator so, dass `ci` nicht nur `gates + govulncheck`
  ausführt, sondern auch die neuen Proto- und Integration-Gates.
  `make test` bleibt der schnelle `CGO_ENABLED=0`-Pure-Go-Lauf; der
  PKCS#11-Adapter kompiliert dort über den `!cgo`-Stub. Der neue
  `make integration`-Pfad läuft mit `CGO_ENABLED=1` und lädt den
  realen Adapter gegen SoftHSM v2 und das zweite OSS-Modul.
  - **Neuer Test-Pfad `test/integration/`:** Slice 002b legt das
    Verzeichnis `test/integration/` an (existiert heute nicht).
    Dort leben Go-Tests mit Build-Tag `integration`, die gegen
    SoftHSM v2 und das zweite OSS-Modul laufen und den PKCS#11-
    Adapter im Testprozess end-to-end exerzieren. Unit-Tests
    bleiben unter `./internal/...` mit dem Standard-Build-Tag.
  - **Coverage-Mechanik-Umbau:** Die bestehende Docker-`coverage`-
    Stage ([`Dockerfile`](../../../../Dockerfile) §coverage, aktuell
    `CGO_ENABLED=0 go test -coverprofile ... ./...`) wird auf zwei
    separate Läufe + `gocovmerge` umgestellt. Der bestehende Ausschluss
    von generiertem Code bleibt erhalten: `COVERPKG` wird wie heute
    über `go list ./internal/...` gebildet und filtert
    `github.com/pt9912/c-hsm-doc/internal/gen/` heraus. Beide Läufe
    verwenden exakt denselben gefilterten `COVERPKG`-Wert;
    `internal/gen/**` darf nicht in das 80-%-Gate einfließen.
    1. `CGO_ENABLED=0 go test -coverprofile=unit.out
       -coverpkg="$COVERPKG" ./internal/...` (Unit-Lauf; PKCS#11-
       Paket über `!cgo`-Stub kompilierbar, kein
       `github.com/miekg/pkcs11`-Import).
    2. `CGO_ENABLED=1 go test -tags=integration
       -coverprofile=integration.out -coverpkg="$COVERPKG"
       ./test/integration/...` (CGO + SoftHSM-Pfad, Server und
       Adapter in-process, damit `-coverpkg` den ausgeführten
       Produktivcode erfasst, ohne generierte Stubs zu bewerten).
    3. `gocovmerge unit.out integration.out > merged.out`.
    4. `scripts/coverage-gate.sh` (aktuell auf
       `coverage-func.txt` aus einem einzelnen Profil) wird in
       002b so erweitert, dass es `merged.out` als Eingabe
       akzeptiert. Das 80-%-Gate wertet ab dann ausschließlich
       `merged.out`. Damit fällt der PKCS#11-Adapter, der nur
       unter Integrationstest geladen wird, nicht aus dem
       Aggregat. `gocovmerge` wird neu als Go-Tool-Dependency
       eingezogen (über die `tool`-Direktive in `go.mod`; das
       Repo läuft bereits auf Go 1.26).
- **SoftHSM-Integrationstest** (neuer `make ci` Sub-Target oder
  Build-Tag-Test; in-process für Coverage, separate Smoke-Targets
  dürfen zusätzlich den Runtime-Container als externen Prozess
  starten):
  - SoftHSM v2 wird im CI initialisiert (Slot, Token-Label,
    AES-256-Schlüssel mit `CKA_EXTRACTABLE=false` **und**
    Master-HMAC-Key vom Typ `CKO_SECRET_KEY` /
    `CKK_GENERIC_SECRET` mit `CKA_EXTRACTABLE=false`,
    `CKA_SENSITIVE=true`, `CKA_DERIVE=true` — Voraussetzung für
    den HKDF-Profil-A-Pfad gemäß HSM-FMT-006). Beide Labels werden
    in einer Test-Key-Registry-Datei eingetragen.
  - **Modul-spezifisches Key-Setup:** Das zweite OSS-Modul
    (Default OpenCryptoki, siehe ADR 0004) wird mit dem für das
    Modul passenden Master-HMAC-Keytyp initialisiert — z. B.
    `CKK_GENERIC_SECRET` (OpenCryptoki ICA), `CKK_SHA256_HMAC`
    oder ein modul-spezifisches Äquivalent. Die genaue Wahl je
    Modul ist Output des HKDF-Spike (Vorbedingung 3) und in ADR 0004
    bzw. der Folge-ADR 0005 dokumentiert. Setup-Skripte liegen
    pro Modul als separates Init-Script im Repo
    (`ci/keys-init/{softhsm,opencryptoki,…}.sh`), damit kein
    Vendor-Sniffing im Adapter-Code nötig wird.
  - Encrypt-Stream über einen in-process Test-Client mit 100 MiB
    Klartext; ein zusätzlicher `grpcurl`-Smoke gegen den Runtime-
    Container ist zulässig, aber nicht die Coverage-Quelle.
  - Container wird vollständig empfangen, Header + Frames + Trailer
    spec-konform encodiert (binär-vergleichbar gegen Referenzlayout).
    Test-Client liest `container_header`, alle `DataChunk` in
    `seq`-Reihenfolge und `container_trailer`, konkateniert die
    Bytes und vergleicht gegen ein vorberechnetes Referenzlayout.
- **Pro-Chunk-AEAD-Belegung** ([`HSM-FA-ENC-006`](../../../../spec/spezifikation.md)):
  PKCS#11-Trace-Test (oder Codepfad-Inspektion durch dedizierten Unit-
  Test) zeigt: ein `C_EncryptInit` + ein `C_Encrypt` pro Chunk; kein
  `C_EncryptUpdate` über Chunk-Grenzen.
- **Heap-Cap** ([`HSM-FA-ENC-003`](../../../../spec/lastenheft.md),
  [`HSM-NFA-MEM-001`](../../../../spec/lastenheft.md),
  [`HSM-CC-002`](../../../../spec/spezifikation.md)):
  Memory-Probe-Test für **10-GiB-Eingabe** zeigt Heap-Cap unter
  `GOMEMLIMIT` aus HSM-CC-002 (Test-Lauf nutzt
  `GOMEMLIMIT=1GiB`, Default-Wert; In-flight-Cap
  `HSMDOC_INFLIGHT_PLAINTEXT_BYTES` auf 256 MiB Default). Der
  10-GiB-Test ist hartes Abnahmekriterium aus HSM-FA-ENC-003 —
  keine Skalierungsannahme. Ein zusätzlicher 1-GiB-Smoke darf als
  schneller CI-Pfad laufen, ersetzt den 10-GiB-Gate aber nicht.
  Wenn der 10-GiB-Test die PR-CI-Laufzeit sprengt, läuft er als
  Nightly-Job mit Release-Block-Charakter (kein grüner Release ohne
  grünen 10-GiB-Job).
- **In-flight-Byte-Grenze:** Testmatrix für
  `HSMDOC_CHUNK_SIZE_BYTES`, `HSMDOC_WORKERS`,
  `HSMDOC_QUEUE_DEPTH` und
  `HSMDOC_INFLIGHT_PLAINTEXT_BYTES` belegt: Der Start bricht ab,
  wenn `HSMDOC_INFLIGHT_PLAINTEXT_BYTES` außerhalb der Range
  (`2 * CHUNK_SIZE`..2 GiB) liegt;
  unter Last blockiert der Reader an der Byte-Semaphore, statt weitere
  Chunks in die Queue zu legen. Der 10-GiB-Memory-Probe zeigt zusätzlich,
  dass Queue, Worker und Reorder-Buffer gemeinsam unter der konfigurierten
  Grenze bleiben.
- **Cancellation** ([`HSM-FA-STREAM-004`](../../../../spec/spezifikation.md)):
  100-paralleler-Stream-Cancel-Test zeigt ≤ 100 ms zwischen Cancel
  und letztem `C_EncryptInit` für diesen Stream (PKCS#11-Trace).
  Der gleiche Test prüft die Commit-Semantik aus HSM-FA-CHUNK-007:
  kein `stream-final-commit`, bereits erreichte `audit-attempt`-
  Einträge bleiben durabel, und ein zusätzlicher Audit-Eintrag
  `operation=error`, `error_class=STREAM_ABORTED` ist vorhanden.
- **Backpressure** ([`HSM-FA-STREAM-003`](../../../../spec/spezifikation.md)):
  Lasttest mit gedrosseltem Receiver zeigt stabile Service-Speicherwerte.
- **Mechanismus-Check** ([`HSM-FA-HSM-005`](../../../../spec/spezifikation.md)):
  Start gegen ein modifiziertes SoftHSM-Setup ohne `CKM_AES_GCM`
  scheitert deterministisch mit Hinweis auf fehlenden Mechanismus.
- **mTLS-Caller-Pfad** ([`HSM-API-GRPC-003`](../../../../spec/lastenheft.md),
  [`HSM-API-GRPC-008`](../../../../spec/spezifikation.md)):
  Integrationstest mit gültigem Client-Zertifikat zeigt, dass
  `caller` im Audit-Eintrag aus `subject_dn` bzw. `san_uri` stammt.
  Client ohne Zertifikat wird mit `UNAUTHENTICATED` abgelehnt, bevor
  Key-Lookup oder HSM-Operation starten. Start ohne lesbares
  `HSMDOC_GRPC_CLIENT_CA` scheitert mit
  `STARTUP_IDENTITY_CLIENT_CA_MISSING`. Akzeptierte Streams enthalten
  keinen Platzhalter-Caller.
- **PIN-Hygiene** ([`HSM-FA-HSM-002`](../../../../spec/lastenheft.md)):
  Image-Scan und Log-Scan finden keine PIN. Datei-Mode-Tests:
  - `0400` mit Datei-UID = Prozess-UID → akzeptiert.
  - `0400` mit fremder UID → Start-Abbruch.
  - `0440` mit Datei-GID ∈ `{primary-gid, supplementary-groups}` →
    akzeptiert, auch wenn Datei-UID ≠ Prozess-UID (deckt K8s-Secret-
    Volume mit `fsGroup` ab).
  - `0440` mit Datei-GID außerhalb der Prozess-Groups →
    Start-Abbruch.
  - Modi außerhalb der Whitelist (`0444`, `0460`, `0660`, `0644`,
    `0777`, …) → Start-Abbruch.
  `HSMDOC_PKCS11_PIN_DEV` mit `HSMDOC_ENV=prod` führt zu
  Start-Abbruch. Derselbe `PIN_DEV`-Wert im produktiven Runtime-Image
  führt auch bei fehlendem oder falsch gesetztem `HSMDOC_ENV` zu
  `STARTUP_PKCS11_PIN_DEV_FORBIDDEN`; der Test prüft den
  Build-Marker-Pfad explizit. Akzeptierter Dev-Override loggt
  unmittelbar nach dem Start eine Warnung mit Verweis auf
  HSM-FA-HSM-002.
- **Exponential Backoff + Jitter** ([`HSM-FA-RETRY-003`](../../../../spec/spezifikation.md)):
  Retry-Test mit gemockten transienten PKCS#11-Fehlern zeigt:
  Wartezeiten zwischen Versuchen folgen `base * factor^attempt`
  ± Jitter (Default 50 ms · 2^attempt); nach `MAX_ATTEMPTS = 5`
  wird der Chunk als `FAILED_PERMANENT` markiert und der Stream
  beendet.
- **Commit-Idempotenz pro Chunk** ([`HSM-FA-RETRY-002`](../../../../spec/lastenheft.md),
  [`HSM-FA-AUDIT-010`](../../../../spec/spezifikation.md),
  [`HSM-FA-CHUNK-007`](../../../../spec/spezifikation.md)):
  Fehlerinjektions-Test „3 transient failures, dann Erfolg" für
  einen Chunk mit fester `seq=k` zeigt:
  - Audit-Log enthält **genau einen** Eintrag mit
    `(seq=k, result=ok)` und drei Einträge mit
    `(seq=k, result=error, attempt=1..3, error_class=...)`.
  - gRPC-Response-Stream emittiert **genau einen** Ciphertext-Frame
    für `seq=k` (den des erfolgreichen Versuchs); die drei
    fehlgeschlagenen Versuche erzeugen keinen Frame.
  - Jeder der vier Versuche verwendet eine andere Nonce
    ([`HSM-FA-CHUNK-006`](../../../../spec/spezifikation.md)
    Akzeptanz).
- **Queue-Wartezeit** ([`HSM-FA-QUEUE-003`](../../../../spec/spezifikation.md)):
  Test mit `HSMDOC_QUEUE_WAIT_MS=200` und voller Queue zeigt:
  Anfrage wartet ≈ 200 ms, dann `RESOURCE_EXHAUSTED`. Mit
  Default 0 ms erfolgt sofortige Ablehnung.
- **CKR-Mapping vollständig** ([`HSM-FA-FAIL-003`](../../../../spec/spezifikation.md),
  [`HSM-FA-FAIL-004`](../../../../spec/spezifikation.md)):
  Code-Konstante + Unit-Test-Fixture decken alle 14 in HSM-FA-FAIL-003
  genannten Returncode-Klassen + Sammelklasse `sonstige` ab; jeder
  Eintrag wird über ein Mock-PKCS#11-Modul exerziert. Metrik
  `hsmdoc_sessions_recycled_total` steigt in den fünf
  Session-Lifecycle-relevanten Fehlerklassen.
- **Token-/Device-Removal-Teilsmoke** ([`HSM-FA-FAIL-003`](../../../../spec/spezifikation.md)):
  Fehlerinjektions-Test (Mock-Modul liefert `CKR_DEVICE_REMOVED`
  bzw. `CKR_TOKEN_NOT_PRESENT`) belegt: Pool-Drain ist sichtbar
  (Metrik `hsmdoc_pkcs11_sessions_active` sinkt auf 0); `/readyz`
  liefert HTTP `503` mit `hsmStatus=DOWN`; offene Streams brechen
  mit `HSM_REMOVED`/`HSM_TOKEN_GONE` ab; neue Encrypt-Anfragen
  werden mit `UNAVAILABLE` abgelehnt. Das ist ein partieller
  Sicherheits-Smoke, **keine vollständige HSM-FA-FAIL-003-Erfüllung**.
  Reconnect und Circuit-Breaker sind nicht Bestandteil der 002b-
  Akzeptanz — der Service bleibt rot, bis er neu gestartet wird; das
  ist dokumentierte M1-Einschränkung (Slice 009 schließt sie).
- **Audit-Durability** ([`HSM-FA-AUDIT-010`](../../../../spec/spezifikation.md),
  [`HSM-FA-RETRY-002`](../../../../spec/lastenheft.md)):
  Fehlerinjektions-Test zeigt: gemockter `fsync`-Fehler bricht den
  Stream mit `AUDIT_DURABILITY_FAILED` ab; **kein** Ciphertext-Chunk
  wird emittiert, dessen Audit-Eintrag nicht durabel ist. Reihenfolge
  `audit-attempt → emit-commit` ist im Test sichtbar (Audit-Datei
  enthält den Eintrag vor der Stream-Antwort). Ein separater
  Out-of-order-Test erzwingt, dass Worker `seq=2` vor `seq=1` in den
  Zustand `SEALED` bringen: Audit-Datei und gRPC-Response bleiben
  trotzdem strikt in `seq=1,2,...`-Reihenfolge; der Dateiadapter würde
  einen out-of-order `AuditSink.Write` defensiv ablehnen.
- **Trusted-Time-Gate** ([`HSM-FA-AUDIT-005`](../../../../spec/lastenheft.md)):
  Start ohne `HSMDOC_TIME_SOURCE=system-ntp` und
  `HSMDOC_TIME_TRUSTED=true` bricht mit
  `STARTUP_TIME_SOURCE_UNTRUSTED` ab; akzeptierter Start schreibt eine
  strukturierte Startup-Log-Zeile mit Zeitquellenmodus. Audit-Unit-Tests
  nutzen einen Fake-Clock-Adapter und belegen monotone Timestamps pro
  Stream sowie Fehlerpropagation, wenn der Clock-Port einen untrusted
  Status liefert. Code-Review-Akzeptanz: im Audit-Pfad gibt es keinen
  direkten `time.Now()`-Aufruf außerhalb des Clock-Adapters.
- **Audit-Pflichtfelder** (Erfüllung für den 002b-Encrypt-Pfad von
  [`HSM-FA-AUDIT-001`](../../../../spec/lastenheft.md),
  [`HSM-FA-AUDIT-003`](../../../../spec/lastenheft.md),
  [`HSM-DATA-001`](../../../../spec/spezifikation.md)):
  Jeder JSONL-Eintrag enthält strukturell alle in HSM-DATA-001
  geforderten Pflichtfelder; Schema-Validierungs-Test scheitert
  deterministisch bei fehlendem Feld. `caller` stammt aus dem
  verifizierten mTLS-Peer-Zertifikat; leere oder fehlende Identität
  erzeugt keinen akzeptierten Encrypt-Stream. Klartext-/Schlüssel-/PIN-
  Scan über die Audit-Datei findet keine Treffer.
- **Proto- und Spec-Update** ([`spec/proto/chsmdocv1/c_hsm_doc.proto`](../../../../spec/proto/chsmdocv1/c_hsm_doc.proto)
  und [`spec/spezifikation.md`](../../../../spec/spezifikation.md)):
  - `EncryptResponse.oneof body` ist um `bytes container_header = 4`
    und `bytes container_trailer = 5` ergänzt; generierte Stubs
    (`internal/gen/chsmdocv1/`) sind regeneriert und im Commit.
  - `DataChunk.seq` ist von `uint32` auf `uint64` gehoben — entspricht
    HSM-FMT-002 (`seq` 8 Byte BE). Im selben Diff: `EncryptTrailer.total_chunks`
    und `EncryptFinal.chunk_count` auf `uint64`, weil HSM-FMT-003
    `total_chunks` ebenfalls als 8 Byte BE festschreibt.
  - `DecryptRequest.oneof body` ist additiv um spiegelbildliche
    `bytes container_header` und `bytes container_trailer`-Felder
    vorbereitet — Slice 003 nutzt sie, Slice 002b lässt sie auf
    Server-Seite leer (`UNIMPLEMENTED`-Stub bleibt) bis Decrypt
    landet. Die Vorbelegung jetzt vermeidet ein zweites
    Schema-Diff für Slice 003.
  - `KeyInfo.KeyStatus` wird auf die Lastenheft-Zustände
    `ACTIVE`, `DEPRECATED`, `DESTROYED` harmonisiert; das bisherige
    `REVOKED`-Enum wird vor produktiver Nutzung ersetzt, damit Proto
    und HSM-FA-KEY-001 dieselbe Zustandsmenge verwenden.
  - Spec-Text in `spezifikation.md` §HSM-API-GRPC-005 dokumentiert,
    dass Slice-002b-Detailcodes (`KEY_REGISTRY_AMBIGUOUS`,
    `KEY_STATE_INVALID`, `ADAPTER_BUFFER_UNDERSIZED`) keine neuen
    Fehlerklassen sind: sie mappen auf bestehende Fehlerklassen
    `INTERNAL` bzw. `KEY_NOT_FOUND` und damit auf die vorhandenen
    gRPC-Statuscodes.
  - Spec-Text in `spezifikation.md` §HSM-API-GRPC-* dokumentiert
    den Wire-Ablauf (Ack → container_header → DataChunk* →
    container_trailer → Final) als verbindliche Reihenfolge.
  - Spec-Text in `spezifikation.md` §HSM-DATA-001 ergänzt: Bei
    `result=ok` ist `error_class` der konstante String `"none"`
    (nicht leer, nicht `null`, damit JSONL-Schema-Validierung
    deterministisch greift). Damit ist die Slice-002b-Konvention
    spec-bindend für alle Folge-Slices (insbesondere Slice 003
    Decrypt, der dieselbe Audit-Semantik schreibt).
  - Spec-Text in `spezifikation.md` §HSM-CC-002 wird **nicht**
    geändert — die Go-Runtime-Gesamt-Heap-Cap bleibt
    `GOMEMLIMIT` Default 1 GiB, Range 256 MiB..8 GiB. Slice 002b
    führt zusätzlich `HSMDOC_INFLIGHT_PLAINTEXT_BYTES` (Default
    256 MiB) als anwendungsseitige In-flight-Plaintext-Cap ein —
    semantisch andere Größe, semaphore-basiert auf den Reader-
    Pfad gegrenzt, deutlich kleiner als `GOMEMLIMIT`. Das
    10-GiB-Heap-Cap-Gate (HSM-FA-ENC-003) misst Heap-Wachstum
    gegen `GOMEMLIMIT`, nicht gegen die In-flight-Cap.
  - Spec-Text in `spezifikation.md` §HSM-FMT-002 ergänzt:
    Container-Frame-`seq` ist **1-basiert** und monoton aufsteigend
    je Stream (erster Chunk hat `seq=1`, nicht `seq=0`). Damit ist
    die Audit-Konvention `chunk_count = max(seq)` spec-bindend.
  - Spec-Text in `spezifikation.md` §HSM-FA-CHUNK-007 ergänzt:
    `audit-attempt` ist pro Versuch idempotent über
    `(stream_id, seq, attempt)`. Mehrere Audit-Einträge zum selben
    `seq` (`attempt=1..N` mit `result=error` plus genau ein
    finaler `result=ok`) sind ausdrücklich zulässig; der nächste
    `seq+1` darf erst nach `result=ok` von `seq` geschrieben werden.
    Damit hat die Audit-Adapter-Defense-Regel und die
    Commit-Idempotenz-Akzeptanz einen Spec-Anker.
  - Spec-Text in `spezifikation.md` ergänzt
    `HSM-FA-FAIL-010 — Startup-Validierungsfehler` mit den in 002b
    eingeführten `STARTUP_*`-Codes:
    `STARTUP_PKCS11_DLOPEN_FAILED`,
    `STARTUP_PKCS11_PIN_AMBIGUOUS`,
    `STARTUP_PKCS11_PIN_MISSING`,
    `STARTUP_PKCS11_PIN_DEV_FORBIDDEN`,
    `STARTUP_PKCS11_CGO_DISABLED`,
    `STARTUP_IDENTITY_CLIENT_CA_MISSING` und
    `STARTUP_TIME_SOURCE_UNTRUSTED`.
  - **Spec-Update-Charakter:** Die neuen Oneof-Felder, Detailcodes
    und der Wire-Ablauf sind additive technische Klärungen. Der
    Enum-Name-Wechsel `REVOKED → DESTROYED` ist dagegen ein bewusst
    akzeptierter Proto-Source/API-Break vor produktiver Nutzung, kein
    additives Detail. Damit liegen die Änderungen außerhalb des
    ADR-0001-Immutability-Regimes (ADR 0001 §2.3 regelt
    ADR-Immutability, nicht Spec-Versionierung); die Klärung läuft im
    selben PR wie der Code-Diff und ist als „Spec-Klarstellung Slice
    002b" in der Commit-Message gekennzeichnet. Ein eigener Spec-
    Versionssprung ist nicht erforderlich.
  - Backwards-Compat-Check: Slice 001 hat nur `UNIMPLEMENTED`-
    Stubs ausgeliefert, deshalb gibt es keine Konsumenten — die
    additive Oneof-Erweiterung ist wire-kompatibel; der Enum-Value-
    Rename `REVOKED → DESTROYED` hält zwar den numerischen Wert `3`
    stabil, ist aber für generierten Go-Code ein Source/API-Break.
    Er ist nur deshalb akzeptabel, weil Slice 001 ausschließlich
    `UNIMPLEMENTED`-Stubs geliefert hat und keine produktiven
    Konsumenten existieren. Slice 002b führt dazu ein neues
    Docker-only-Make-Target `proto-breaking` ein und hängt
    `proto-check` + `proto-breaking` in `make ci` ein. Das Target ist
    trotz `.dockerignore` reproduzierbar: Host-`git` ermittelt den
    Merge-Base von Slice-002b-Branch und `origin/main`, exportiert
    ausschließlich `spec/proto/` dieses Commits per `git archive` in
    ein temporäres Baseline-Verzeichnis unter `out/proto-breaking/`,
    und startet danach das gepinnte `bufbuild/buf`-Container-Image mit
    zwei read-only Mounts (`baseline` und aktueller Workspace). Im
    Container läuft `buf breaking` gegen diese Baseline; es wird kein
    lokales Go-/Buf-Tool und kein `.git` im Docker-Build-Kontext
    benötigt. Der Check läuft mit zwei expliziten, dokumentierten
    Allowlist-Einträgen:
    - **Oneof-Felder additiv** (`ONEOF_NO_DELETE`-konform; neue
      Tags 4, 5 in `EncryptResponse` und gespiegelt in
      `DecryptRequest`).
    - **Enum-Value-Rename** (`ENUM_VALUE_NO_DELETE`-Ausnahme für
      `KeyInfo.KeyStatus.REVOKED=3 → DESTROYED=3`); der Tag-Wert
      bleibt stabil, nur der Name ändert sich. Die Allowlist-
      Begründung wird im Commit dokumentiert und mit dem
      Slice-001-Konsumenten-Nullstand belegt.
- **HKDF-Profil-A-Non-Export** ([`HSM-FMT-006`](../../../../spec/spezifikation.md)):
  Tests gegen SoftHSM v2 und das zweite OSS-Modul belegen die
  Nicht-Extrahierbarkeit:
  - `C_GetAttributeValue` für `CKA_EXTRACTABLE` liefert `false` auf
    Master-HMAC-Key und auf dem via `CKM_HKDF_DERIVE` erzeugten
    Header-Key-Handle; `CKA_SENSITIVE=true` ebenso.
  - Ein expliziter `C_WrapKey`-Versuch auf beide Handles schlägt
    mit `CKR_KEY_UNEXTRACTABLE` fehl.
  - Code-Inspektion (statischer Check + Code-Review-Akzeptanz):
    Adapter-Code enthält keine HMAC-/HKDF-Software-Implementierung;
    `grep -E "hmac\.New|hkdf\."` im PKCS#11-Adapter ergibt keine
    Treffer. Header-Key wird ausschließlich via `C_DeriveKey`
    erzeugt und via `C_SignInit`/`C_Sign` benutzt.
  - PKCS#11-Trace-Test zeigt, dass der Header-Key nie als Klartext
    in einem `C_GetAttributeValue`- oder `C_WrapKey`-Returnwert
    auftaucht.
- **Header-Key-Lifecycle:** PKCS#11-Trace- und Objektzählungs-Test
  zeigen pro HeaderMAC-Aufruf: `C_DeriveKey` erzeugt einen
  session-ephemeren Header-Key mit `CKA_CLASS=CKO_SECRET_KEY`,
  `CKA_SIGN=true`, `CKA_TOKEN=false`, `CKA_EXTRACTABLE=false`,
  `CKA_SENSITIVE=true`; nach `C_Sign` folgt `C_DestroyObject`. Nach
  1000 Encrypt-Streams steigt die Anzahl persistenter Token-Objekte
  nicht.
- **Session-affine KeyRef-Auflösung:** Test mit Pool-Größe ≥ 2 und
  erzwungenem Session-Wechsel zwischen HeaderMAC und ChunkSealer
  zeigt: `KeyRef` transportiert nur den logischen Snapshot, raw
  Object-Handles werden je Session aus den Labels aufgelöst oder
  session-lokal gecached. Ein injiziertes `CKR_KEY_HANDLE_INVALID`
  invalidiert nur den betroffenen Session-Cache und löst einen
  Handle-Refresh aus dem unveränderten Snapshot aus.
- **Re-Login-Throttle** ([`HSM-FA-FAIL-008`](../../../../spec/spezifikation.md)):
  Test mit erzwungenem Logout zeigt: Innerhalb des Default-Fensters
  von 60 s wird pro Session höchstens ein Re-Login versucht; weitere
  `CKR_USER_NOT_LOGGED_IN`-Fehler recyceln die Session. Metrik
  `hsmdoc_hsm_relogin_total` steigt pro Slot erwartungsgemäß.
- **Stream-Snapshot-Konsistenz:** Test mit Registry-Reload zwischen
  Container-Header und erstem Chunk-Frame:
  - Stream startet, Server löst `KeyRecord` und daraus den logischen
    `KeyRef`-Snapshot auf, schreibt Container-Header mit Header-HMAC.
  - Test ändert die Key-Registry-Datei (rotiert die `active`-
    Version: alt `v1 → deprecated`, neu `v2 → active`) und löst
    Reload aus.
  - Stream sendet weitere Chunks. Alle bis zum Stream-Ende
    verarbeiteten Chunks MÜSSEN denselben logischen Snapshot (`v1`,
    Labels aus `v1`) benutzen wie der Header — keine Mischung mit
    `v2`. Session-affine raw Handles dürfen zwischen Pool-Sessions
    verschieden sein.
  - Audit-Einträge des Streams zeigen einheitlich `key_version=1`;
    Container-Header und alle Frames sind konsistent mit `v1`-
    Material.
  - Die folgende Encrypt-Anfrage benutzt `v2`.
- **Single-Tenant-Konsistenz:** Tests belegen, dass in Slice 002b
  nur der kanonische Tenant `default` akzeptiert wird:
  - fehlendes oder leeres `EncryptHeader.tenant_id` wird zu `default`
    normalisiert;
  - `tenant_id=default` wird akzeptiert;
  - jeder andere Wert wird vor Key-Lookup mit `INVALID_ARGUMENT` und
    Detailcode `TENANT_UNSUPPORTED_IN_M1` abgelehnt;
  - Container-Header (`tenant_id_hash`), Pro-Chunk-AAD, Audit,
    strukturierte Logs und Tenant-Metriken verwenden für akzeptierte
    Streams denselben kanonischen Wert `default`.
- **Key-Lifecycle** ([`HSM-FA-KEY-001`](../../../../spec/lastenheft.md),
  [`HSM-FA-KEY-002`](../../../../spec/lastenheft.md),
  [`HSM-FA-KEY-004`](../../../../spec/lastenheft.md)):
  - Key-Registry-Schema-Validierung scheitert deterministisch bei
    fehlenden Pflichtfeldern: `key_id`, `key_version`, `status`,
    `pkcs11_label`, `master_hmac_pkcs11_label`. Tests decken jedes
    fehlende Pflichtfeld einzeln ab.
  - Encrypt-Test mit `status=active` (genau ein Eintrag) → grün.
  - Encrypt-Test mit **kein `active`-Eintrag** für `key_id` →
    `FAILED_PRECONDITION` + `KEY_NOT_FOUND`.
  - Encrypt-Test mit **mehreren `active`-Einträgen** für dieselbe
    `key_id` → Start-Abbruch (Schema-Validierung) bzw.
    Defense-in-Depth-Lookup-Fehler mit Fehlerklasse `INTERNAL` und
    Detailcode `KEY_REGISTRY_AMBIGUOUS`.
  - Encrypt-Test mit `status=deprecated` → `FAILED_PRECONDITION` +
    Fehlerklasse `KEY_NOT_FOUND` und Detailcode `KEY_STATE_INVALID`.
  - Encrypt-Test mit `status=destroyed` → `FAILED_PRECONDITION` +
    `KEY_NOT_FOUND`.
  - Encrypt-Test mit gültigem Registry-Eintrag, aber im HSM nicht
    auflösbarem `master_hmac_pkcs11_label` → `KeyBinding.Bind`-
    Fehler am Stream-Start
    `FAILED_PRECONDITION` + `KEY_NOT_FOUND`.
  - Proto-Enum `KeyInfo.KeyStatus` enthält `DESTROYED` statt
    `REVOKED`; generierte Stubs und Tests verwenden durchgehend die
    Lastenheft-Zustände `active`/`deprecated`/`destroyed`.
  - Repo-Audit: Key-Registry-Datei enthält weder Klartext-Schlüssel
    noch Wrap-Keys (HSM-FA-KEY-004 Akzeptanz).
- **HSM-FA-HSM-001 Vendor-Smoke** ([`HSM-FA-HSM-001`](../../../../spec/lastenheft.md)):
  CI führt den gleichen Integrations-Test-Pfad zweimal aus —
  einmal gegen SoftHSM v2, einmal gegen OpenCryptoki (oder das in
  ADR 0004 dokumentierte Alternativmodul) — **ohne Codeänderung**,
  nur durch Umschalten von `HSMDOC_PKCS11_MODULE` und
  `HSMDOC_PKCS11_TOKEN_LABEL`. Beide Läufe sind grüner Release-
  Block.
- **HKDF-Profil-A-Pflicht** ([`HSM-FMT-006`](../../../../spec/spezifikation.md) §1):
  Beide CI-Module (SoftHSM v2 und das zweite OSS-Modul) müssen
  `CKM_HKDF_DERIVE` unterstützen — Start gegen ein Modul ohne
  diesen Mechanismus scheitert deterministisch mit Hinweis auf
  HSM-FMT-006. Metrik `hsmdoc_header_hmac_profile{profile="A"}`
  wird in beiden Läufen gesetzt; Roundtrip-Test (Encrypt-Container
  → Header-HMAC neu berechnen aus identischen Inputs → byteweiser
  Vergleich) ist grün.
- **`CK_HKDF_PARAMS`-Shim verifiziert** (Spike-Output, siehe
  Vorbedingung 3): Der eingesetzte Pfad — Shim, Fork oder Fallback —
  ist in einer neuen Folge-ADR zu ADR 0004 (geplant: ADR 0005)
  dokumentiert und der ADR-Index ist aktualisiert. Ein dedizierter
  Adapter-Unit-Test
  ruft `C_DeriveKey` mit `CKM_HKDF_DERIVE` und prüft, dass der
  zurückgegebene Handle `CKA_SIGN=true`, `CKA_TOKEN=false`,
  `CKA_EXTRACTABLE=false` / `CKA_SENSITIVE=true` trägt, gegen
  `C_WrapKey` mit `CKR_KEY_UNEXTRACTABLE` antwortet und per
  `C_DestroyObject` zerstörbar ist.
- **HSM-FA-HSM-001 — Vendor-Portabilität (Pflicht in Slice 002b):**
  Modulpfad und Slot/Token-Label sind konfigurierbar; der Adapter-
  Codepfad enthält keine Vendor-Strings — Mechanismus-Wahl läuft
  strikt über `C_GetMechanismList`. Code-Review-Akzeptanz:
  `grep -iE "softhsm|opencryptoki|utimaco|thales"` im Adapter-Code
  findet keine Vendor-Verzweigung. **Zweiter herstellerfremder
  PKCS#11-Modul-Smoke** läuft im CI ohne Codeänderung gegen
  SoftHSM v2 **und** das in ADR 0004 (aus 002a) festgelegte
  zweite Modul. Ein zweites SoftHSM-Image mit divergenter
  Token-Konfiguration ist **kein** Ersatz-Nachweis und zählt
  höchstens als zusätzlicher Smoke. Damit ist
  [`HSM-FA-HSM-001`](../../../../spec/lastenheft.md)
  Akzeptanz („Start gegen SoftHSM v2 und ein zweites
  herstellerfremdes Modul ohne Codeänderung") mit Slice 002b
  erfüllt.
- **CGO/Pure-Go-Gates:** `make test` läuft mit `CGO_ENABLED=0`
  erfolgreich und kompiliert das PKCS#11-Paket nur über den `!cgo`-
  Stub; ein Test belegt, dass der Stub bei Initialisierung
  `STARTUP_PKCS11_CGO_DISABLED` liefert. `make integration` läuft mit
  `CGO_ENABLED=1` und belegt, dass der reale PKCS#11-Adapter gebaut
  und verwendet wird. Das Runtime-Image wird ausschließlich aus dem
  `CGO_ENABLED=1`-Build-Stage erzeugt.
- **`MaxRecvMsgSize`-TODO** in `cmd/hsmdoc/main.go` ist entfernt;
  Item §2.1 aus
  [`offene-arbeitsfaeden.md`](../in-progress/offene-arbeitsfaeden.md)
  ist gestrichen.
- **`pkcs11-dlopen-smoke`-Startup-Hook** ist verdrahtet: Service-
  Start ruft das Binary synchron vor `C_Initialize` auf;
  Exit-Code ≠ 0 → Service bricht mit `STARTUP_PKCS11_DLOPEN_FAILED`
  ab. Test mit absichtlich kaputtem `HSMDOC_PKCS11_MODULE`-Pfad
  belegt das Verhalten.
- **Cross-Adapter-Lint-Regel** (offene-arbeitsfaeden §1.1) ist
  entweder umgesetzt oder mit Begründung weiter aufgeschoben — beides
  ist OK, aber dokumentiert.
- **Roadmap-Lifecycle** wird in zwei Schritten aktualisiert:
  - **Bei Slice-Aktivierung** (Migration `next/` → `in-progress/`):
    Slice-Tabelle in [`roadmap.md`](../in-progress/roadmap.md)
    führt 002b als `in-progress`.
  - **Bei Slice-Abschluss** (Merge des Schluss-PR, alle Akzeptanz-
    kriterien grün): Slice-Tabelle führt 002b als `done`,
    Slice-Datei wandert nach `done/`; M1-DoD-Tabelle hakt nur die von
    002b vollständig erfüllten DoD-Items ab. `M1-DoD-01`
    (`HSM-ACCEPT-001`, funktionale Abnahme gegen SoftHSM) und
    `M1-DoD-03` (1-GiB-Demo Encrypt-Decrypt mit identischer SHA-256)
    bleiben offen, weil beide Encrypt **und** Decrypt verlangen. Slice
    002b liefert dafür nur die Encrypt-Hälfte; hakbar werden sie erst
    mit Slice 003.

## Abgrenzung — NICHT in diesem Slice

- **Kein Decrypt-Pfad.** Container-Decoder + Tag-Verifikation kommen
  in Slice 003.
- **Keine Audit-Hash-Chain, keine Signatur, keine externe Verankerung,
  kein Verify-Tool in 002b.** Slice 002b schreibt einen durablen
  JSONL-Sink (Pflichtfelder + Klartext-Verbot + Sync-Garantie), aber
  **keinen** Manipulationsschutz nach
  [`HSM-FA-AUDIT-002`](../../../../spec/lastenheft.md). Die Basis-
  Hash-Chain (HSM-FA-AUDIT-002) ist **M1-Scope** und kommt in
  Slice 004 vor M1-Closure
  ([`roadmap.md`](../in-progress/roadmap.md) §M1). Die Detail-
  Verfahren Segment-Signatur, externe Verankerung und Chain-Rotation
  ([`HSM-FA-AUDIT-006..008`](../../../../spec/spezifikation.md)) sind
  M2-Scope (M2-DoD-02..04).
- **Keine Key-Rotation, keine Usage-Limits.** Sowohl
  [`HSM-FA-KEY-003`](../../../../spec/lastenheft.md)
  (Schlüsselrotation) als auch
  [`HSM-FA-KEY-005`](../../../../spec/lastenheft.md) /
  [`HSM-FA-KEY-006`](../../../../spec/spezifikation.md) (Hard/Soft-
  Usage-Limits + Auto-Rotation) sind laut
  [`roadmap.md`](../in-progress/roadmap.md) §M1 ausdrücklich aus M1
  ausgeschlossen und M2-Scope (Slice 011). Slice 002b nimmt eine
  einzelne `active` Key-ID an und führt keinen Operationszähler.
- **Kein Tenant-Mapping über `default` hinaus.** `tenant_id=default`
  wie in Slice 001; Multi-Tenant kommt in M4-Slices.
- **Keine vollständige Identity-Source-Matrix.** 002b liefert nur den
  M1-Pflichtpfad `mtls-subject` für echte Audit-`caller`. Header-
  Quelle, Peer-Allowlist, Mesh-Termination und die vollständige
  [`HSM-API-GRPC-006..008`](../../../../spec/spezifikation.md)-
  Konfigurationsmatrix bleiben Slice 006.
- **Nur HKDF-Profil A in M1.** Slice 002b implementiert ausschließlich
  Profil A (natives `CKM_HKDF_DERIVE`). Profile B
  (HMAC-Konstruktion mit non-export PRK) und C (Vendor-KDF) werden
  je Produktionsprofil in M3 freigegeben
  ([`HSM-FMT-006`](../../../../spec/spezifikation.md) §1,
  [`HSM-TECH-006`](../../../../spec/lastenheft.md)). HSMs ohne
  natives HKDF sind für M1 nicht freigegeben.
- **Kein produktives HSM in 002b.** Slice 002b schließt
  [`HSM-FA-HSM-001`](../../../../spec/lastenheft.md) mit
  SoftHSM v2 + einem zweiten OSS-Modul ab; das Profil-Smoke gegen
  ein produktives HSM (Utimaco/Thales) bleibt M3-Scope
  ([`HSM-TECH-006`](../../../../spec/lastenheft.md)).
- **Keine Vault-/K8s-CSI-Secret-Backends als eigener Adapter.**
  Slice 002b erwartet die PIN als Datei mit Mode aus der Whitelist
  `{0400, 0440}` (siehe PKCS#11-Adapter §PIN) — das deckt
  Kubernetes-Secret-Volumes (`defaultMode: 0440` + `fsGroup`) und
  Vault-Agent-Renders (`0400`) bereits ab. Ein nativer Vault- oder
  CSI-Adapter ist M2/M3-Scope und braucht einen eigenen Slice
  (Open-Trigger noch nicht angelegt).
- **Kein TLS-Material-Reload.** Bleibt `TODO(slice-006)` und ist
  Scope von Slice 006.
- **Kein Build-Pipeline-Switch.** CGO-Umstellung, Distroless-base-
  Wechsel, `lddtree`-Closure, `pkcs11-dlopen-smoke`-Binary und
  ADR 0004 sind alle Scope von Slice 002a; 002b setzt darauf auf
  und verdrahtet nur den Startup-Hook.

## Geplante Slice-Folge danach

| Nr.   | Slice                                       | Aktiviert durch 002b                                  |
| ----- | ------------------------------------------- | ----------------------------------------------------- |
| `003` | Container-Codec (Decoder) + Decrypt         | Container-Encoder + Pro-Chunk-AAD stehen             |
| `004` | Audit-Hash-Chain + Verify-Tool              | JSONL-Sink + Port stehen; 004 ergänzt Hash-Chain ohne Port-Bruch |
| `005` | Helm-Chart + Kind-Smoke (inkl. NetworkPolicy) | PKCS#11-Volume + PIN-Secret-Schema definiert; trägt K8s-Secret-Mount-Smoke (`defaultMode: 0440` + `fsGroup`) als eigene Akzeptanz |

## Bezug

- Direkter Implementierungs-/Akzeptanzbezug im Slice 002b:
  [Lastenheft `HSM-FA-ENC-001..003`, `HSM-FA-HSM-001..003`,
  `HSM-FA-CHUNK-001..003`, `HSM-FA-STREAM-001..002`,
  `HSM-FA-KEY-001..002`, `HSM-FA-KEY-004`, `HSM-FA-QUEUE-001`,
  `HSM-FA-RETRY-001..002`, `HSM-FA-AUDIT-003`,
  `HSM-FA-AUDIT-005`, `HSM-ARCH-001`,
  `HSM-OPS-CFG-001..002`, `HSM-NFA-MEM-001`,
  `HSM-NFA-OPS-001..003`](../../../../spec/lastenheft.md)
- Direkter Implementierungs-/Akzeptanzbezug im Slice 002b:
  [Spezifikation `HSM-FA-ENC-004..006`, `HSM-FA-CHUNK-004..008`,
  `HSM-FA-STREAM-003..004`, `HSM-FA-HSM-004..005`,
  `HSM-API-P11-002..003`, `HSM-FMT-001..006`,
  `HSM-DATA-003..004`, `HSM-FA-QUEUE-002..003`,
  `HSM-FA-RETRY-003..004`, `HSM-FA-FAIL-003..004`,
  `HSM-FA-FAIL-008`, `HSM-FA-AUDIT-010`, `HSM-ARCH-003`](../../../../spec/spezifikation.md)
- Struktur-/Identitätsbezug im Slice 002b:
  [Lastenheft `HSM-FA-AUDIT-001`](../../../../spec/lastenheft.md)
  und [Spezifikation `HSM-DATA-001`,
  `HSM-API-GRPC-008`](../../../../spec/spezifikation.md). Alle Audit-
  Pflichtfelder sind für den Encrypt-Pfad vorhanden; `caller` stammt in
  002b aus dem verifizierten mTLS-Peer-Zertifikat. Slice 006 erweitert
  diesen Minimalpfad um Header-Quelle, Peer-Allowlist und die volle
  `identity.*`-Konfigurationsmatrix.
- Nur als Abgrenzung/Folge-Scope erwähnt, **nicht** durch Slice 002b
  erfüllt:
  [Lastenheft `HSM-FA-AUDIT-002`, `HSM-FA-KEY-003`,
  `HSM-FA-KEY-005`, `HSM-FA-FAIL-001`,
  `HSM-TECH-006`](../../../../spec/lastenheft.md)
  sowie [Spezifikation `HSM-FA-DEC-003`, `HSM-FA-FAIL-005..007`,
  `HSM-FA-AUDIT-006..008`, `HSM-FA-KEY-006`,
  `HSM-API-GRPC-006..007`](../../../../spec/spezifikation.md)
- [Architektur Kapitel 2 (Komponenten), 3 (Hexagonale Schichtung), 5.1 (Encrypt-Stream-Sequenz)](../../../../spec/architecture.md)
- [Vorgänger-Slice 002a — CGO-Build-Pipeline](002a-cgo-build-pipeline.md)
- [ADR 0001 §2.3 — Accepted-ADRs sind immutable](../../adr/0001-documentation-and-planning-structure.md)
- [ADR 0002 — Docker-only Build-Pipeline](../../adr/0002-docker-only-build-pipeline.md)
- [`internal/README.md` — Hexagon-Ziel-Layout](../../../../internal/README.md)
- [Offene Arbeitsfäden §1.1 (Sibling-Regel), §2.1 (MaxRecvMsgSize)](../in-progress/offene-arbeitsfaeden.md)
- [Roadmap M1](../in-progress/roadmap.md)
