# Lastenheft – `c-hsm-doc` – Hardwaregestützte Dokumentverschlüsselung mit HSM

| Dokument         | Lastenheft                                                                          |
| ---------------- | ----------------------------------------------------------------------------------- |
| Projektname      | `c-hsm-doc`                                                                         |
| Kurzbeschreibung | HSM-gestützter Krypto-Dienst (Go) mit Java-Streaming-Client zur Dokumentverschlüsselung |
| Zielplattform    | Linux-Container, Kubernetes, PKCS#11-HSM (SoftHSM, Utimaco, Thales)                 |
| Hauptnutzer      | Backend-Dienste, die regulierungspflichtige Dokumente vertraulich ablegen müssen    |
| Version          | 0.1                                                                                 |
| Status           | Entwurf                                                                             |
| Datum            | 2026-05-26                                                                          |

---

## 0. Lesehinweise

### HSM-LESE-001 – Modalverben

In diesem Dokument haben die in Großbuchstaben geschriebenen Schlüsselwörter folgende normative Bedeutung in Anlehnung an V-Modell XT und RFC 2119:

- **MUSS** / **MÜSSEN** – verbindliche Anforderung für den zugeordneten Abnahmestand.
- **DARF NICHT** / **DÜRFEN NICHT** – ausdrücklich ausgeschlossen.
- **SOLLTE** / **SOLLEN** – geplante Eigenschaft; Abweichungen müssen begründet und dokumentiert werden.
- **KANN** / **KÖNNEN** – optionale Eigenschaft ohne Abnahmeverpflichtung.

Klein geschriebene Formen („muss", „soll", „kann") sind beschreibend und nicht normativ.

MVP-blockierend sind ausschließlich Anforderungen, die in Kapitel 4 (MVP-Umfang) explizit gelistet oder in ihrem Text mit dem Zusatz `MVP` versehen sind.

### HSM-LESE-002 – Abnahme und Belegtypen

Eine Anforderung gilt nur dann als erfüllt, wenn der zugehörige Belegtyp im Repository vorliegt:

| Anforderungsklasse                  | Zulässiger Beleg                                                                   |
| ----------------------------------- | ---------------------------------------------------------------------------------- |
| Funktionale Anforderungen (`FA-*`)  | automatisierter Integrationstest gegen SoftHSM oder reproduzierbarer manueller Test |
| Datenmodell / Format (`DATA-*`, `FMT-*`) | Schemadatei (Protobuf, JSON-Schema) plus Roundtrip-Test                       |
| Architektur (`ARCH-*`)              | Architekturentscheid (ADR) plus statischer Importcheck                             |
| Prinzipien (`PRINC-*`, `CC-*`)      | ADR plus Lint-/Architekturtest                                                     |
| Performance (`NFA-PERF-*`)          | reproduzierbares Benchmark-Skript plus Messprotokoll für die Referenzumgebung      |
| Sicherheit (`NFA-SEC-*`)            | automatisierter Test, Konfigurationsbeleg oder dokumentiertes Reviewprotokoll       |
| Compliance (`COMP-*`)               | Verweis auf einschlägige Norm plus Konfigurations- oder Testbeleg                   |
| Schnittstellen (`API-*`)            | Protobuf-/Javadoc-/OpenAPI-Artefakt plus Kontrakttest                              |
| Betrieb (`OPS-*`)                   | dokumentiertes Runbook, Helm-Chart oder Skript im Repository                       |
| Abnahmekriterien (`ACCEPT-*`)       | im Repository vorhandenes Abnahmeartefakt (Test, Skript, Demo-Lauf)                |

Ein ADR allein genügt nur für `ARCH-*`- und `PRINC-*`-Anforderungen.

### HSM-LESE-003 – ID-Schema

IDs folgen dem Muster `HSM-<Bereich>-<NNN>` mit dreistelliger Nummer. Bereiche umfassen:

`LESE`, `ZB`, `PE`, `PUE`, `MVP`, `NONGOAL`,
`FA-ENC`, `FA-DEC`, `FA-CHUNK`, `FA-STREAM`, `FA-HSM`, `FA-KEY`, `FA-QUEUE`, `FA-RETRY`, `FA-AUDIT`, `FA-TENANT`, `FA-FAIL`,
`API-JAVA`, `API-GRPC`, `API-P11`, `API-CFG`,
`FMT`, `DATA`,
`NFA-PERF`, `NFA-SCALE`, `NFA-HA`, `NFA-SEC`, `NFA-PRIV`, `NFA-MEM`, `NFA-OBS`, `NFA-OPS`, `NFA-MAINT`, `NFA-PORT`,
`ARCH`, `PRINC`, `CC`,
`TECH`, `ENV`, `OPS-MON`, `OPS-HC`, `OPS-CFG`,
`COMP`, `THREAT`, `RISK`, `ASSUMP`, `ACCEPT`, `MENGE`, `GLOSS`, `REF`.

### HSM-LESE-004 – Verhältnis zu einem Pflichtenheft

Das Dokument fixiert in den Kapiteln 7 (Schnittstellen), 8 (Dokumentformat), 11 (Architekturvorgaben) und 12 (Technologievorgaben) bewusst auch Lösungsanteile. Es ist damit größer geschnitten als ein klassisches Lastenheft. Diese Festlegungen sind beabsichtigt; sie gelten als Anforderungen mit Belegtyp `ARCH-*` oder `TECH-*` gemäß HSM-LESE-002.

### HSM-LESE-005 – Referenzumgebung

Sofern eine Anforderung Performance, Latenz oder Durchsatz benennt, gilt – wenn nicht anders angegeben – folgende Referenzumgebung:

- Linux x86_64, Kernel ≥ 6.1
- 4 vCPU, 8 GiB RAM je Service-Replica
- gRPC über Loopback oder lokales 10-GbE-VLAN
- HSM: SoftHSM v2 lokal (Funktional-Referenz) bzw. Netzwerk-HSM mit < 2 ms RTT (Performance-Referenz)

---

## 1. Zielbestimmung

### HSM-ZB-001 – Projektziel

`c-hsm-doc` MUSS einen hochverfügbaren kryptografischen Dienst bereitstellen, der Dokumente beliebiger Größe hardwaregestützt mittels HSM verschlüsselt und entschlüsselt, ohne dass kryptografisches Schlüsselmaterial das HSM verlässt.

Akzeptanz: Ein Referenzlauf verschlüsselt ein 1-GiB-Dokument gegen SoftHSM, schreibt einen Container gemäß Kapitel 8 und stellt ihn byte-identisch wieder her.

### HSM-ZB-002 – Produktvision

Der Dienst SOLL sich aus Sicht aufrufender Backend-Dienste wie ein streamingfähiger „Crypto-as-a-Service" mit harten HSM-Garantien verhalten: einfacher Java-Client, gRPC-Stream, keine PKCS#11-Details für den Aufrufer, kein Klartext im Speicher des Service über Chunk-Grenzen hinaus.

### HSM-ZB-003 – Muss-/Soll-/Kann-Ziele

| Klasse | Ziel                                                                                          |
| ------ | --------------------------------------------------------------------------------------------- |
| MUSS   | AES-256-GCM ausschließlich im HSM, Schlüssel nicht extrahierbar (`CKA_EXTRACTABLE=false`).    |
| MUSS   | Streamingbasierte, chunkfähige Verarbeitung; keine vollständige Dokumentenpufferung im RAM.   |
| MUSS   | Java-21-Clientbibliothek ohne JNI- oder PKCS#11-Abhängigkeit.                                 |
| MUSS   | Horizontal skalierbarer, container- und Kubernetes-fähiger Go-Service.                        |
| MUSS   | Revisionssichere Auditierung jeder kryptografischen Operation.                                |
| SOLLTE | Mehrere PKCS#11-Hersteller (SoftHSM, Utimaco, Thales) ohne Codeänderung austauschbar.         |
| SOLLTE | Schlüsselrotation ohne Ausfall laufender Streams.                                             |
| KANN   | Wiederaufnehmbare (resumable) Verschlüsselungs-Streams.                                       |
| KANN   | Hardware-beschleunigte Integritätsprüfung (z. B. AES-NI-Bypass für SoftHSM-Profil).          |

---

## 2. Produkteinsatz

### HSM-PE-001 – Anwendungsbereich

Der Dienst MUSS in Umgebungen einsetzbar sein, in denen Dokumente aus regulatorischen oder vertraglichen Gründen ausschließlich mit hardwaregeschütztem Schlüsselmaterial verschlüsselt abgelegt werden dürfen, insbesondere:

- Archivierung medizinischer, finanzieller oder behördlicher Dokumente,
- Langzeitablage gemäß BSI TR-03125 (TR-ESOR) als Krypto-Vorstufe,
- Mandantenisolierte Dokumentenspeicher mit eigenem HSM-Schlüssel je Mandant.

### HSM-PE-002 – Zielgruppen und Stakeholder

| Rolle                  | Interesse                                                                          |
| ---------------------- | ---------------------------------------------------------------------------------- |
| Aufrufendes Backend    | einfache Streaming-API, deterministische Fehler, Backpressure statt OOM            |
| Plattform-/SRE-Team    | Container, Kubernetes-Manifeste, Health-Probes, Prometheus-Metriken, Tracing       |
| Security-/Crypto-Officer | HSM-Konfiguration, PIN-Handling, Schlüssellebenszyklus, Auditierung               |
| Datenschutzbeauftragter | Nachweis, dass kein Klartext und keine Schlüssel das HSM unkontrolliert verlassen |
| Auditor / Revision     | Lückenlose, manipulationsgeschützte Audit-Logs mit Schlüssel-/Doc-IDs              |

### HSM-PE-003 – Betriebsumgebung

Die primäre Betriebsumgebung MUSS sein:

- Linux x86_64 als Container in Kubernetes (≥ 1.28),
- ein PKCS#11-fähiges HSM (Hardware oder SoftHSM v2),
- TLS-1.3-fähige Netzwerkinfrastruktur zwischen Java-Client und Go-Service.

Sekundäre Umgebungen, die im CI mitgeführt werden SOLLEN:

- lokale Entwicklerumgebung mit SoftHSM v2 in Docker Compose,
- ARM64-Container-Build als Best-Effort, ohne MVP-Abnahmepflicht.

### HSM-PE-004 – Anwendungsfälle

Folgende Use Cases MÜSSEN unterstützt werden:

- **UC-1 Encrypt-Stream**: Aufrufer streamt ein Dokument an den Service und empfängt streamweise den verschlüsselten Container.
- **UC-2 Decrypt-Stream**: Aufrufer streamt einen Container an den Service und empfängt streamweise das entschlüsselte Dokument.
- **UC-3 Key-Lookup**: Aufrufer fragt verfügbare Key-IDs ab, ohne Schlüsselmaterial zu erhalten.
- **UC-4 Health/Ready**: Plattform fragt Liveness und HSM-Ready ab.
- **UC-5 Audit-Export**: Auditor exportiert das Audit-Log eines Zeitraums in einem definierten Format.

Folgende Use Cases SOLLEN unterstützt werden:

- **UC-6 Key-Rotate**: Crypto-Officer rotiert einen logischen Key auf ein neues HSM-Objekt.
- **UC-7 Re-Encrypt**: Bestehender Container wird ohne Klartextfreigabe auf neuen Key umgeschlüsselt (Decrypt-Encrypt im selben Service, Klartext nur in HSM-Sessions).

---

## 3. Produktübersicht

### HSM-PUE-001 – Systemkontext

```text
+----------------------+        gRPC/TLS 1.3        +----------------------+
|  Aufrufendes Backend |  <---------------------->  |   Go HSM-Service     |
|  (Java 21)           |   Bidi-Streaming           |   (Worker- + Session-|
|  + c-hsm-doc-client  |                            |    Pool)             |
+----------------------+                            +----------+-----------+
                                                               |
                                                               | PKCS#11
                                                               v
                                                    +----------------------+
                                                    |        HSM           |
                                                    |  (SoftHSM/Utimaco/   |
                                                    |   Thales Luna)       |
                                                    +----------------------+
```

### HSM-PUE-002 – Komponenten

| Komponente            | Sprache | Verantwortung                                                                 |
| --------------------- | ------- | ----------------------------------------------------------------------------- |
| `c-hsm-doc-server`    | Go      | gRPC-Endpoint, Job-Queue, Worker-Pool, PKCS#11-Sessions, Audit, Metriken      |
| `c-hsm-doc-client`    | Java 21 | Streaming-API, Konnektor, TLS/mTLS, Retry-Logik, Backpressure-Adapter         |
| `c-hsm-doc-proto`     | Proto3  | gRPC-Definitionen, gemeinsame Datenmodelle für Header, Audit, Status          |
| `c-hsm-doc-container` | –       | Containerfile, Helm-Chart, Probes, Default-Konfiguration                      |

### HSM-PUE-003 – Vertrauensgrenzen

Folgende Vertrauensgrenzen MÜSSEN als solche dokumentiert und in Code/Konfiguration durchgesetzt werden:

- **Client ↔ Service**: TLS 1.3 mit Serverzertifikat (MUSS), Client-Zertifikat (SOLL für interne Aufrufer).
- **Service ↔ HSM**: PKCS#11-Login mit PIN aus externem Secret-Store (MUSS), HSM-Sessions im Pool wiederverwendet.
- **Service ↔ Audit-Sink**: signierte Append-only-Logs (MUSS); Audit-Empfänger DARF NICHT Klartext, Schlüsselmaterial oder PIN sehen.

---

## 4. MVP-Umfang

### HSM-MVP-001 – Lokaler End-to-End-Stream

Der MVP MUSS Verschlüsselung und Entschlüsselung eines Dokuments im Streaming-Modus gegen SoftHSM v2 demonstrieren.

Akzeptanz: Ein Demo-Skript verschlüsselt eine 1-GiB-Datei und entschlüsselt sie wieder; die SHA-256-Summe von Original und Wiederherstellung ist identisch.

### HSM-MVP-002 – Konfigurierbarer Session- und Worker-Pool

Der MVP MUSS einen PKCS#11-Session-Pool und einen Worker-Pool bereitstellen, beide über Konfiguration einstellbar.

Akzeptanz: Konfiguration setzt Session-Pool=4 und Worker-Pool=8; ein Lasttest zeigt keine PKCS#11-Session-Leaks und keine Worker-Hangs nach 100.000 Chunks.

### HSM-MVP-003 – Job-Queue mit Backpressure

Der MVP MUSS eine begrenzte Job-Queue mit Backpressure implementieren.

Akzeptanz: Bei Überlauf antwortet der Service mit gRPC-Status `RESOURCE_EXHAUSTED`; der Java-Client bietet einen konfigurierbaren Retry-/Wait-Adapter.

### HSM-MVP-004 – Audit-Log

Der MVP MUSS jede Encrypt/Decrypt-Operation in ein append-only Audit-Log schreiben.

Akzeptanz: Audit-Einträge enthalten Zeitstempel, Operation, Key-ID, Doc-ID, Aufrufer, Resultat und Fehlerklasse; Manipulation an einem bestehenden Eintrag wird durch eine Integritätsprüfung erkannt.

### HSM-MVP-005 – Containerisierte Auslieferung

Der MVP MUSS als Container-Image mit Helm-Chart auslieferbar sein und in Kubernetes laufen.

Akzeptanz: `helm install` auf einem lokalen Kind-Cluster startet den Service erfolgreich, Liveness- und Readiness-Probes sind grün, eine Demo-Verschlüsselung über `port-forward` läuft durch.

### HSM-MVP-006 – Java-Client ohne JNI

Der MVP MUSS einen Java-21-Client bereitstellen, der den Service über gRPC anspricht und keine JNI- oder PKCS#11-Abhängigkeit besitzt.

Akzeptanz: Eine Maven-Build-Analyse listet keine native- oder JNI-Abhängigkeiten; die API-Beispiele in `examples/` kompilieren und laufen gegen den Demo-Service.

---

## 5. Nicht-Ziele und Scope-Grenzen

### HSM-NONGOAL-001 – Kein Schlüsselverwaltungssystem

Der Dienst ist KEIN vollwertiges Key-Management-System. Schlüssel werden ausschließlich im HSM erzeugt und verwaltet; Funktionen wie Quorum-basierte Schlüsselgenerierung, M-of-N-Backup oder hierarchische KEK-Strukturen sind nicht im Scope.

Abgrenzung: Der Service KANN Key-IDs ablesen und Rotationen anstoßen, übernimmt aber keine HSM-Administration.

### HSM-NONGOAL-002 – Kein Dokumentenarchiv

Der Dienst persistiert keine Dokumente. Eingabe und Ausgabe erfolgen ausschließlich als Stream.

### HSM-NONGOAL-003 – Keine asymmetrische Kryptografie im MVP

RSA, ECC, Signaturen und Hybridverschlüsselung sind nicht Bestandteil des MVP. Eine spätere Erweiterung ist nicht ausgeschlossen.

### HSM-NONGOAL-004 – Kein Re-Verschlüsseln gegen externe Speicher

Re-Encrypt (UC-7) findet ausschließlich innerhalb des Dienstes statt. Der Service liest oder schreibt keine Dokumente in externe Storage-Backends.

### HSM-NONGOAL-005 – Kein eigenes Identity-Provider-Modul

Der Dienst stellt keinen eigenen IdP bereit. Authentifizierung erfolgt über mTLS und/oder vorgelagerten Token-Issuer; die Token-Validierung erfolgt im Sidecar oder Ingress.

---

## 6. Funktionale Anforderungen

### 6.1 Verschlüsselung und Entschlüsselung

#### HSM-FA-ENC-001 – AES-256-GCM-Verschlüsselung

Der Dienst MUSS Dokumente mittels AES-256-GCM verschlüsseln.

Akzeptanz: Ciphertexte sind mit einem Referenz-Tool (z. B. `openssl enc` mit gleichem Key, Nonce und AAD) verifizierbar.

#### HSM-FA-ENC-002 – HSM-residente Schlüssel

Die AES-Operation MUSS vollständig im HSM ausgeführt werden. Der Schlüssel DARF NICHT das HSM verlassen.

Akzeptanz: PKCS#11-Attribut `CKA_EXTRACTABLE=false` ist auf allen Encrypt-/Decrypt-Schlüsseln gesetzt; ein Versuch, den Schlüssel zu wrappen, schlägt mit `CKR_KEY_UNEXTRACTABLE` fehl.

#### HSM-FA-ENC-003 – Streaming-Eingabe

Die Verschlüsselung MUSS streamingbasiert erfolgen. Der Dienst DARF NICHT das gesamte Dokument im Speicher halten.

Akzeptanz: Der Heap-Verbrauch beim Verschlüsseln einer 10-GiB-Datei überschreitet den konfigurierten Maximalwert (siehe HSM-NFA-MEM-001) nicht.

#### HSM-FA-ENC-004 – Eindeutige Nonces

Der Dienst MUSS für jede AES-GCM-Operation eine eindeutige 96-Bit-Nonce erzeugen. Die Nonce MUSS aus einem 32-Bit-Random-Prefix und einem monoton steigenden 64-Bit-Zähler je Schlüssel und Stream bestehen oder vollständig kryptografisch zufällig sein.

Akzeptanz: Statistischer Test über 10⁶ Nonces einer Session zeigt keine Kollision; Zähler-Reset bei Restart wird durch das Prefix verhindert.

#### HSM-FA-ENC-005 – Authenticated Additional Data

Der Dienst MUSS Additional Authenticated Data (AAD) je Stream unterstützen. Der Container-Header (siehe HSM-FMT-001) MUSS in die AAD jedes Chunks einfließen. Pro-Chunk-AAD MUSS zusätzlich `key_id`, `key_version`, `seq` und `stream_id` enthalten, sodass ein Chunk außerhalb seines Streams nicht erfolgreich entschlüsselt werden kann.

Akzeptanz: Manipulation des Container-Headers oder der Pro-Chunk-AAD-Felder nach der Verschlüsselung führt beim Entschlüsseln zu `CKR_GENERAL_ERROR` bzw. einer Tag-Verifikations-Fehlermeldung. Ein in einen anderen Container kopierter Chunk schlägt bei der Tag-Verifikation fehl.

#### HSM-FA-ENC-006 – AEAD-Granularität pro Chunk

Jeder Chunk MUSS eine eigenständige AES-GCM-Operation mit eigenem 96-Bit-Nonce und eigenem 128-Bit-Authentication-Tag darstellen. Ein durchgehender (Multipart-)GCM-Stream über mehrere Chunks oder mehrere PKCS#11-Calls DARF NICHT verwendet werden.

Begründung: Stream-übergreifendes GCM bindet den Tag an die Gesamtlänge und macht streamingbasierte Cancellation, Retry und parallele Chunk-Verarbeitung sicherheitsrelevant fehleranfällig.

Akzeptanz: Codepfad führt je Chunk genau einen `C_EncryptInit`/`C_Encrypt`-Aufruf (oder Vendor-Äquivalent) aus; eine Code-Inspektion und ein PKCS#11-Trace-Test belegen, dass keine `C_EncryptUpdate`-Ketten über Chunk-Grenzen hinweg verwendet werden.

#### HSM-FA-DEC-001 – Entschlüsselung als Inverse

Der Dienst MUSS einen mit ihm verschlüsselten Container vollständig in den Originaldatenstrom zurückführen können.

Akzeptanz: Für 100 zufällig generierte Eingaben gilt `sha256(decrypt(encrypt(x))) == sha256(x)`.

#### HSM-FA-DEC-002 – Tag-Verifikation

Der Dienst MUSS bei jedem Chunk den GCM-Authentication-Tag prüfen und den Stream bei Mismatch sofort abbrechen.

Akzeptanz: Ein mutierter Ciphertext-Chunk führt zu gRPC-Status `DATA_LOSS`; bereits ausgegebener Klartext nachfolgender Chunks DARF NICHT vor erfolgreicher Tag-Verifikation des aktuellen Chunks ausgeliefert werden.

#### HSM-FA-DEC-003 – Key-ID-Auflösung

Der Dienst MUSS den zu verwendenden HSM-Schlüssel aus dem Container-Header (Key-ID, Key-Version) auflösen.

Akzeptanz: Unbekannte oder als `destroyed` markierte Key-IDs führen zu gRPC-Status `FAILED_PRECONDITION` mit definierter Fehlerklasse.

### 6.2 Chunkbasierte Verarbeitung

#### HSM-FA-CHUNK-001 – Konfigurierbare Chunkgröße

Die Chunkgröße MUSS konfigurierbar sein. Default ist 4 MiB; gültiger Bereich ist 64 KiB bis 64 MiB.

Akzeptanz: Konfigurationswerte außerhalb des gültigen Bereichs verhindern den Start mit einer eindeutigen Fehlermeldung.

#### HSM-FA-CHUNK-002 – Unabhängigkeit von der Dateigröße

Die chunkbasierte Verarbeitung MUSS unabhängig von der Eingangs-Dateigröße funktionieren, einschließlich Streams unbekannter Länge.

Akzeptanz: Verschlüsselung läuft, ohne dass der Aufrufer eine Gesamtlänge angibt.

#### HSM-FA-CHUNK-003 – Reihenfolge-Sicherung

Chunks MÜSSEN in derselben Reihenfolge wieder ausgegeben werden, in der sie verschlüsselt wurden. Reihenfolgeverletzungen MÜSSEN beim Entschlüsseln erkannt werden.

Akzeptanz: Jeder Chunk-Header trägt eine Sequenznummer; eine vertauschte Sequenz führt zu Decrypt-Abbruch mit definierter Fehlerklasse.

#### HSM-FA-CHUNK-004 – Chunk-Zustandsmodell

Jeder Chunk MUSS pro Stream einen der folgenden Zustände einnehmen und MUSS deterministisch zwischen ihnen wechseln:

```text
PENDING  --> ASSIGNED  --> IN_HSM  --> SEALED  --> EMITTED
                              |
                              +-----> FAILED_TRANSIENT --> ASSIGNED (Retry)
                              +-----> FAILED_PERMANENT --> STREAM_ABORT
```

- `PENDING`: aus dem Klartext-Stream gelesen, Sequenznummer zugewiesen, nicht verarbeitet.
- `ASSIGNED`: einem Worker zugeteilt.
- `IN_HSM`: HSM-Operation läuft.
- `SEALED`: Ciphertext und Tag vorhanden, noch nicht emittiert.
- `EMITTED`: in den gRPC-Response-Stream geschrieben.
- `FAILED_TRANSIENT` / `FAILED_PERMANENT`: gemäß HSM-FA-RETRY-001.

Akzeptanz: State-Übergänge sind als Enum/Konstanten im Code definiert, Übergangsregeln werden durch Unit-Tests abgedeckt, und jeder Zustandswechsel wird als Tracing-Event mit `chunk.seq` und `stream_id` exportiert.

#### HSM-FA-CHUNK-005 – Parallele Verarbeitung und Reordering

Chunks DÜRFEN parallel im Worker-Pool verarbeitet werden und DÜRFEN out-of-order in den Zustand `SEALED` wechseln. Die Emission (`SEALED → EMITTED`) MUSS jedoch strikt in `seq`-Reihenfolge erfolgen.

Akzeptanz: Ein Reorder-Buffer puffert frühe SEALED-Chunks bis ihr direkter Vorgänger emittiert ist; die Puffertiefe entspricht maximal der Worker-Pool-Größe und wird als Metrik `hsmdoc_reorder_buffer_depth` exportiert.

#### HSM-FA-CHUNK-006 – Retry-Semantik

Ein Chunk im Zustand `FAILED_TRANSIENT` MUSS mit identischer Sequenznummer und identischem Klartext-Inhalt wiederholt werden. Bei jedem Retry MUSS eine neue Nonce erzeugt werden (siehe HSM-FA-ENC-004); die vorherige Nonce DARF NICHT wiederverwendet werden.

Akzeptanz: Ein erzwungener Retry-Test zeigt monoton steigende Nonces für denselben `seq` und identischen entschlüsselten Klartext.

### 6.3 Streaming-Verarbeitung

#### HSM-FA-STREAM-001 – Bidirektionales gRPC-Streaming

Die Übertragung zwischen Java-Client und Go-Service MUSS über bidirektionales gRPC-Streaming erfolgen.

Akzeptanz: Proto-Definitionen verwenden `stream` in beide Richtungen für Encrypt und Decrypt.

#### HSM-FA-STREAM-002 – Flow Control

Der Dienst MUSS HTTP/2-Flow-Control respektieren und beim Erreichen interner Queue-Grenzen den Sender drosseln.

Akzeptanz: Ein Client mit langsamem Empfang verursacht keinen unbegrenzten Speicheraufbau im Service; ein Lasttest mit künstlich gedrosseltem Receiver zeigt stabile Service-Speicherwerte.

#### HSM-FA-STREAM-003 – Cancellation

Bei Cancellation eines Streams durch den Client (gRPC `CANCELLED`, Verbindungsabbruch oder lokales Timeout) MUSS der Dienst:

1. binnen ≤ 100 ms keine neuen HSM-Operationen für diesen Stream mehr starten,
2. den Klartext-Reader und den Response-Writer schließen,
3. Reorder-Buffer, Worker-Slots und stream-eigene Puffer freigeben,
4. alle bereits an das HSM übergebenen Operationen entweder regulär beenden lassen oder, wenn der PKCS#11-Adapter eine sichere Abbruchsemantik bietet (z. B. `C_CancelFunction` oder Vendor-Erweiterung), abbrechen.

PKCS#11 garantiert KEIN synchrones Abbrechen laufender HSM-Operationen. Eine im HSM laufende `C_Encrypt`-Operation wird daher ggf. zu Ende geführt; ihr Ergebnis wird verworfen.

Sessions, die nach Abschluss der laufenden Operation in einem undefinierten Zustand verbleiben, MÜSSEN aus dem Session-Pool entfernt und durch eine neu eingerichtete Session ersetzt werden, bevor sie wieder verwendet werden.

Akzeptanz: Cancel-Test bricht 100 parallele Streams ab. Innerhalb von 100 ms werden keine neuen `C_Encrypt`-Aufrufe für diese Streams beobachtet (PKCS#11-Trace). Bereits laufende HSM-Operationen werden binnen ihrer typischen Laufzeit beendet; danach kehren Session- und Worker-Pool-Metriken in den Ruhestand zurück. Sessions, die im Verlauf des Cancels einen Fehlerzustand melden, werden ersetzt und nicht weiterverwendet.

#### HSM-FA-STREAM-004 – Wiederaufnahme (KANN)

Der Dienst KANN wiederaufnehmbare Streams unterstützen, sodass nach Verbindungsabbruch der Stream ohne Wiederholung verschlüsselter Chunks fortgesetzt werden kann.

Akzeptanz (falls implementiert): Stream-ID + letzte bestätigte Sequenznummer reichen, um den Stream binnen 5 s fortzusetzen.

### 6.4 HSM-Anbindung

#### HSM-FA-HSM-001 – PKCS#11 als Anbindung

Die HSM-Anbindung MUSS über PKCS#11 v2.40 oder höher erfolgen.

Akzeptanz: Modulpfad und Slot/Token-Label sind konfigurierbar; der Service startet erfolgreich gegen SoftHSM v2 und ein zweites herstellerfremdes Modul (z. B. CryptoServer-Simulator) ohne Codeänderung.

#### HSM-FA-HSM-002 – Konfigurierbarer Session-Pool

Der Service MUSS einen PKCS#11-Session-Pool bereitstellen, dessen Größe, Lifetime und Re-Login-Strategie konfigurierbar sind.

Akzeptanz: Konfigurationsparameter `pool.size`, `pool.maxIdle`, `pool.maxLifetime`, `pool.loginRetry` werden im Start-Log angezeigt; ein Lasttest belegt, dass der Pool unter Last keine Sessions verliert.

#### HSM-FA-HSM-003 – PIN-Bezug aus Secret-Store

Die HSM-User-PIN MUSS aus einem externen Secret-Store (Kubernetes Secret, HashiCorp Vault, Datei mit Mode 0400) bezogen werden. Sie DARF NICHT im Code, im Container-Image oder in Logs erscheinen.

Akzeptanz: Image-Scan und Log-Scan finden keine PIN; eine Konfiguration ohne Secret-Quelle führt zu definiertem Startfehler.

#### HSM-FA-HSM-004 – Mechanismen

Der Dienst MUSS den Mechanismus `CKM_AES_GCM` verwenden und prüfen, dass das HSM diesen unterstützt. Fehlende Mechanismen MÜSSEN beim Start mit einer eindeutigen Fehlermeldung erkannt werden.

Akzeptanz: Ein HSM ohne `CKM_AES_GCM` führt zu Start-Abbruch mit Hinweis auf den fehlenden Mechanismus.

### 6.5 Schlüsselverwaltung

#### HSM-FA-KEY-001 – Schlüssel-Lebenszyklus

Der Dienst MUSS einen Schlüssel-Lebenszyklus mit den Zuständen `active`, `deprecated`, `destroyed` führen.

Akzeptanz: Nur `active`-Schlüssel können zum Verschlüsseln verwendet werden; `deprecated`-Schlüssel sind nur zum Entschlüsseln zugelassen; `destroyed`-Schlüssel führen zu definiertem Fehler.

#### HSM-FA-KEY-002 – Logische Key-ID

Jeder Schlüssel MUSS eine stabile logische Key-ID (`UUID` oder Label) tragen, die im Container-Header gespeichert wird.

Akzeptanz: Roundtrip-Test schreibt die Key-ID, liest sie zurück und löst den HSM-Schlüssel darüber auf.

#### HSM-FA-KEY-003 – Schlüsselrotation (SOLL)

Der Dienst SOLL Schlüsselrotation unterstützen: Ein neuer aktiver Schlüssel ersetzt den alten, der in den Status `deprecated` wechselt. Laufende Streams DÜRFEN NICHT abgebrochen werden.

Akzeptanz: Während eines Encrypt-Streams wird der Schlüssel rotiert; der laufende Stream beendet mit dem alten Schlüssel, der nächste Stream verwendet den neuen.

#### HSM-FA-KEY-004 – Schlüssel-Metadatenverzeichnis

Metadaten zu logischen Schlüsseln (Key-ID, HSM-Handle/Label, Status, Erzeugungszeit, Rotationszeit, Algorithmus) MÜSSEN außerhalb des HSM gepflegt werden, ohne sensible Inhalte zu duplizieren.

Akzeptanz: Schlüssel-Metadaten sind über einen Read-only-Endpoint abrufbar; das Verzeichnis enthält weder Klartext-Schlüssel noch Wrap-Keys.

### 6.6 Queueing und Backpressure

#### HSM-FA-QUEUE-001 – Begrenzte Job-Queue

Der Dienst MUSS eine begrenzte interne Job-Queue mit konfigurierbarer Tiefe bereitstellen. Default ist 256 Jobs.

Akzeptanz: Beim Überschreiten der Tiefe werden weitere Requests mit gRPC-Status `RESOURCE_EXHAUSTED` und Fehlerklasse `QUEUE_FULL` abgelehnt.

#### HSM-FA-QUEUE-002 – Backpressure-Signal

Der Dienst MUSS Backpressure über HTTP/2-Flow-Control und über explizite gRPC-Statuscodes signalisieren.

Akzeptanz: Java-Client erkennt `RESOURCE_EXHAUSTED` und exponiert eine `BackpressureException` mit empfohlener Wartezeit.

#### HSM-FA-QUEUE-003 – Wartezeit-Strategie

Die Wartezeit, die der Service vor Ablehnung wartet, MUSS konfigurierbar sein (Default 0 ms = sofortige Ablehnung).

### 6.7 Retry und Resilienz

#### HSM-FA-RETRY-001 – Klassifizierung transienter Fehler

Der Dienst MUSS Fehler in `transient`, `permanent` und `client` klassifizieren. Nur `transient` darf intern wiederholt werden.

Akzeptanz: Eine Mapping-Tabelle (HSM-Fehlercode → Klasse) ist im Repository dokumentiert und durch Unit-Tests abgedeckt.

#### HSM-FA-RETRY-002 – Exponential Backoff

Wiederholungen MÜSSEN mit Exponential Backoff und Jitter ausgeführt werden. Default ist Basis = 50 ms, Faktor = 2, max. 5 Versuche.

#### HSM-FA-RETRY-003 – Idempotenz pro Chunk

Retries auf Chunk-Ebene MÜSSEN idempotent sein. Eine wiederholte Verschlüsselung desselben Klartext-Chunks mit derselben Sequenznummer DARF NICHT zu zwei verschiedenen Ciphertexten im Audit-Log führen.

Akzeptanz: Nach erzwungenem Retry zeigt das Audit-Log genau einen erfolgreichen Eintrag pro Chunk.

### 6.8 Auditierung

#### HSM-FA-AUDIT-001 – Audit-Pflichtfelder

Jeder Audit-Eintrag MUSS mindestens folgende Felder enthalten: `timestamp` (UTC, RFC 3339), `operation` (`encrypt`/`decrypt`/`key-lookup`/`key-rotate`/`error`), `key_id`, `key_version`, `doc_id`, `caller` (Subject aus mTLS oder Token), `result` (`ok`/`error`), `error_class`, `chunk_count`, `total_bytes`, `request_id`, `trace_id`.

#### HSM-FA-AUDIT-002 – Revisionssicherheit

Audit-Einträge MÜSSEN append-only und manipulationsgeschützt geschrieben werden. Der Schutz MUSS mindestens eine Hash-Chain (jeder Eintrag enthält den Hash des Vorgängers) umfassen.

Hinweis: Eine Hash-Chain allein verhindert keinen vollständigen Neuschreib der Log-Datei. Sie wird daher durch HSM-FA-AUDIT-006 (Signatur), HSM-FA-AUDIT-007 (externe Verankerung), HSM-FA-AUDIT-008 (Chain-Rotation) und HSM-FA-AUDIT-009 (Zeitstempelquelle) ergänzt; erst deren Zusammenspiel ergibt Revisionssicherheit im Sinne typischer Aufsichtsanforderungen.

Akzeptanz: Manipulation eines beliebigen Eintrags wird durch ein automatisches Verify-Tool erkannt; ein vollständiger Neuschreib der Datei wird durch die Verankerungsprüfung gemäß HSM-FA-AUDIT-007 erkannt.

#### HSM-FA-AUDIT-003 – Klartextverbot

Audit-Einträge DÜRFEN NICHT Klartext, Schlüsselmaterial, PINs oder vollständige Ciphertexte enthalten.

Akzeptanz: Statische Prüfung der Audit-Schreiberfunktion und Stichprobenkontrolle der Logs bestätigen das Verbot.

#### HSM-FA-AUDIT-004 – Aufbewahrung

Die Aufbewahrungsfrist MUSS konfigurierbar sein; Default ist 365 Tage, Minimum 90 Tage.

#### HSM-FA-AUDIT-005 – Export-Format

Audit-Logs MÜSSEN im JSON-Lines-Format exportierbar sein und ein optionales Begleit-Manifest mit Hash-Chain-Endwert tragen.

#### HSM-FA-AUDIT-006 – Signatur der Audit-Segmente

Audit-Einträge MÜSSEN in zeitlich oder mengenmäßig begrenzten Segmenten gebündelt werden (Default: alle 5 min oder alle 10 000 Einträge, je nachdem was zuerst eintritt). Jedes abgeschlossene Segment MUSS mit einem im HSM verwahrten Signaturschlüssel signiert werden.

Akzeptanz: Eine Manipulation innerhalb eines abgeschlossenen Segments lässt die Segmentsignatur ungültig werden; das Verify-Tool meldet das betroffene Segment eindeutig.

#### HSM-FA-AUDIT-007 – Externe Verankerung

Der Endwert der Hash-Chain SOLL regelmäßig (Default: stündlich) extern verankert werden. Zulässige Verankerungssenken sind mindestens eine der folgenden, konfigurierbar:

- ein zweiter, organisatorisch getrennter Append-only-Log (z. B. SIEM, dediziertes Verankerungs-Repository),
- ein RFC-3161-Zeitstempeldienst (TSA),
- ein Transparency-Log (z. B. Sigstore Rekor).

Akzeptanz: Das Verify-Tool kann anhand des externen Verankerungsbelegs den letzten verankerten Chain-Endwert nachweisen; ein vollständiger Neuschreib der Audit-Datei wird erkannt, weil der neu berechnete Chain-Endwert nicht mit der externen Verankerung übereinstimmt.

#### HSM-FA-AUDIT-008 – Chain-Rotation

Die Hash-Chain MUSS rotierbar sein: nach Erreichen einer konfigurierbaren Größe (Default 1 GiB) oder eines Zeitfensters (Default 30 Tage) wird ein neuer Chain-Abschnitt begonnen. Der letzte Hash und die letzte Segmentsignatur des alten Abschnitts MÜSSEN als erster Eintrag des neuen Abschnitts referenziert und unabhängig verankert werden.

Akzeptanz: Nach einer Rotation gibt es einen lückenlosen Verifikationspfad über die Abschnittsgrenze hinweg; das Verify-Tool durchläuft alle Abschnitte einer Aufbewahrungsperiode ohne Bruch.

#### HSM-FA-AUDIT-009 – Zeitstempelquelle

Audit-Zeitstempel MÜSSEN aus einer vertrauenswürdigen Zeitquelle stammen. Mindestanforderung: NTP-/chrony-synchronisierte Systemzeit mit dokumentierter Drift-Überwachung. Für regulierte Umgebungen SOLL zusätzlich ein RFC-3161-Zeitstempel je signiertem Segment (HSM-FA-AUDIT-006) eingeholt werden.

Akzeptanz: Eine Zeit-Abweichung von > 1 s gegenüber NTP-Quelle löst eine Metrik `hsmdoc_time_drift_seconds` aus und wird im Service-Log gemeldet; für jedes signierte Segment der regulierten Konfiguration liegt ein RFC-3161-Token vor.

### 6.9 Mandantenisolation

#### HSM-FA-TENANT-001 – Tenant als erstklassiges Konzept

Der Dienst MUSS Mandanten (`tenant_id`) als erstklassiges Konzept führen. `tenant_id` MUSS aus dem mTLS-Subject, einem Token-Claim oder einem expliziten gRPC-Header abgeleitet werden und MUSS für jeden Stream eindeutig zugeordnet sein.

Akzeptanz: Requests ohne auflösbare `tenant_id` werden mit `UNAUTHENTICATED` bzw. `FAILED_PRECONDITION` abgelehnt; die Mapping-Regel ist konfigurierbar und dokumentiert.

#### HSM-FA-TENANT-002 – Schlüsseltrennung

Ein Mandant DARF NICHT auf Schlüssel anderer Mandanten zugreifen. Die Schlüsselauflösung (siehe HSM-FA-DEC-003) MUSS `tenant_id` als Pflichtfilter einsetzen.

Akzeptanz: Ein Decrypt-Versuch mit einer Key-ID eines fremden Mandanten schlägt mit `FAILED_PRECONDITION` und Fehlerklasse `KEY_NOT_FOUND` fehl; ein Audit-Eintrag mit Resultat `error` wird geschrieben.

#### HSM-FA-TENANT-003 – Quotas pro Mandant

Der Dienst MUSS pro Mandant konfigurierbare Quotas für mindestens folgende Größen unterstützen: maximale parallele Streams, maximale Queue-Tiefe, maximale Sessions im Session-Pool, maximaler Durchsatz pro Zeitfenster.

Akzeptanz: Quota-Überschreitung führt zu `RESOURCE_EXHAUSTED` mit Fehlerklasse `TENANT_QUOTA`; eine Mandanten-Übersicht ist als Metrik (`hsmdoc_tenant_*`) verfügbar.

#### HSM-FA-TENANT-004 – Fair Scheduling

Der Worker-Pool MUSS Mandanten fair bedienen. Ein einzelner Mandant DARF NICHT den gesamten Pool oder die gesamte HSM-Session-Kapazität dauerhaft monopolisieren.

Akzeptanz: Ein synthetischer Lasttest mit einem aggressiven Mandanten (Mandant A: ≥ 1000 Streams) und einem moderaten Mandanten (Mandant B: 10 Streams) zeigt für Mandant B eine p99-Latenz, die das p99 ohne A-Last um nicht mehr als Faktor 3 überschreitet.

#### HSM-FA-TENANT-005 – Mandantenkontext in Audit und Telemetrie

`tenant_id` MUSS in jedem Audit-Eintrag und in den Tenant-relevanten Metriken/Spans enthalten sein. In Metrik-Labels SOLL ein Hash der `tenant_id` verwendet werden, sofern Klartext-IDs personenbezogen oder geschäftskritisch sind.

### 6.10 HSM Failure Semantics

#### HSM-FA-FAIL-001 – Fehlerklassen-Mapping PKCS#11

Der Dienst MUSS PKCS#11-Returncodes auf interne Fehlerklassen mappen. Mindestens folgende Codes MÜSSEN behandelt werden:

| PKCS#11-Returncode               | Fehlerklasse                | Reaktion                                                  |
| -------------------------------- | --------------------------- | --------------------------------------------------------- |
| `CKR_OK`                         | –                           | Erfolg                                                    |
| `CKR_SESSION_HANDLE_INVALID`     | `SESSION_INVALID`           | Session verwerfen, neue Session anfordern, Chunk-Retry    |
| `CKR_SESSION_CLOSED`             | `SESSION_INVALID`           | wie oben                                                  |
| `CKR_DEVICE_ERROR`               | `HSM_DEVICE_ERROR`          | Session verwerfen, Circuit Breaker prüfen, Chunk-Retry    |
| `CKR_DEVICE_REMOVED`             | `HSM_REMOVED`               | Pool drainen, Readiness rot, Reconnect-Schleife           |
| `CKR_TOKEN_NOT_PRESENT`          | `HSM_TOKEN_GONE`            | wie `CKR_DEVICE_REMOVED`                                  |
| `CKR_FUNCTION_FAILED`            | `HSM_FUNCTION_FAILED`       | Session verwerfen, Chunk-Retry (mit Klassifikation)       |
| `CKR_GENERAL_ERROR`              | `HSM_GENERAL_ERROR`         | Session verwerfen, Chunk-Retry, Counter `permanent` prüfen |
| `CKR_USER_NOT_LOGGED_IN`         | `HSM_NOT_LOGGED_IN`         | Re-Login gemäß Policy, dann Chunk-Retry                   |
| `CKR_PIN_INCORRECT`              | `HSM_PIN_INVALID`           | Permanenter Fehler, Readiness rot, Alarmierung            |
| `CKR_KEY_HANDLE_INVALID`         | `KEY_HANDLE_STALE`          | Key-Cache invalidieren, Handle neu auflösen, Chunk-Retry  |
| `CKR_MECHANISM_INVALID`          | `MECHANISM_MISSING`         | Permanenter Konfigurationsfehler, Start abbrechen / Stream abbrechen |
| `CKR_BUFFER_TOO_SMALL`           | `INTERNAL`                  | Programmfehler, Chunk-Abbruch, Bug-Report                 |
| `CKR_DATA_INVALID`/`CKR_ENCRYPTED_DATA_INVALID` | `TAG_MISMATCH` | Stream-Abbruch (siehe HSM-FA-DEC-002)                    |
| sonstige `CKR_*`                 | `HSM_UNKNOWN`               | als `permanent` behandeln                                 |

Akzeptanz: Die Mapping-Tabelle liegt als Code-Konstante und als Unit-Test-Fixture vor; jeder Eintrag wird durch mindestens einen Test exerziert (Mock-PKCS#11-Modul).

#### HSM-FA-FAIL-002 – Session-Lebenszyklus bei Fehlern

Eine Session, die einen Fehler aus den Klassen `SESSION_INVALID`, `HSM_DEVICE_ERROR`, `HSM_FUNCTION_FAILED`, `HSM_GENERAL_ERROR` oder `KEY_HANDLE_STALE` zurückgeliefert hat, MUSS unmittelbar aus dem Pool entfernt und durch eine neu eingerichtete Session ersetzt werden.

Akzeptanz: Ein Fehlertest setzt nacheinander jede dieser Klassen auf einer Session; die Metrik `hsmdoc_sessions_recycled_total` steigt entsprechend; die Sessionanzahl im Pool bleibt stabil.

#### HSM-FA-FAIL-003 – Circuit Breaker

Der Dienst MUSS pro HSM-Quelle (Slot/Modul) einen Circuit Breaker bereitstellen. Bei einer konfigurierbaren Fehlerrate (Default ≥ 50 % über ein 30-s-Fenster) öffnet der Breaker, Readiness MUSS auf rot wechseln, neue Streams werden mit `UNAVAILABLE` abgelehnt, bestehende Streams werden nach HSM-FA-STREAM-003 abgebrochen.

Akzeptanz: Ein simulierter HSM-Ausfall öffnet den Breaker innerhalb des Fensters; `/readyz` liefert nicht-ready; nach Erholung der HSM-Quelle schließt der Breaker nach einer halben-offenen Probe.

#### HSM-FA-FAIL-004 – HSM-Reboot und Token-Removal

Bei `CKR_DEVICE_REMOVED` oder `CKR_TOKEN_NOT_PRESENT` MUSS der Dienst:

1. den Session-Pool für die betroffene Quelle drainen,
2. den Circuit Breaker öffnen,
3. eine Reconnect-Schleife mit Exponential Backoff (Basis 1 s, Faktor 2, Cap 60 s) starten,
4. bei erfolgreichem Reconnect (`C_Initialize` + `C_OpenSession` + `C_Login` + Mechanism-Check) den Pool neu auffüllen.

Akzeptanz: Ein simulierter Token-Remove löst den dokumentierten Ablauf aus; nach Token-Re-Insert ist der Service binnen einer Backoff-Periode wieder ready.

#### HSM-FA-FAIL-005 – Netzwerkpartition zum Netzwerk-HSM

Bei Netzwerk-HSMs MUSS der Dienst eine TCP-/Heartbeat-Überwachung der HSM-Verbindung implementieren oder vom Vendor-Modul übernehmen. Ein Timeout MUSS als `HSM_DEVICE_ERROR` behandelt werden und HSM-FA-FAIL-003 auslösen.

Akzeptanz: Ein netemulierter Paketverlust > 80 % über 10 s öffnet den Circuit Breaker; ein nachfolgendes Wiederherstellen schließt ihn.

#### HSM-FA-FAIL-006 – Re-Login-Strategie

Bei `CKR_USER_NOT_LOGGED_IN` MUSS der Dienst einen kontrollierten Re-Login durchführen, höchstens mit der konfigurierten Frequenz (Default: max. 1 Re-Login pro Session pro 60 s). Übermäßige Re-Logins MÜSSEN vermieden werden, um HSM-spezifische Lockout-Mechanismen nicht auszulösen.

Akzeptanz: Ein erzwungenes Logout führt zu maximal einem Re-Login innerhalb der Default-Periode; die Metrik `hsmdoc_hsm_relogin_total` zählt Re-Logins pro Slot.

#### HSM-FA-FAIL-007 – Readiness-Signal

`/readyz` MUSS den Status `not ready` zurückliefern, solange:

- der Session-Pool weniger als 1 funktionsfähige Session besitzt,
- der Circuit Breaker offen ist,
- der `CKM_AES_GCM`-Check beim letzten Reconnect fehlgeschlagen ist,
- ein permanenter Fehler (`HSM_PIN_INVALID`, `MECHANISM_MISSING`) erkannt wurde.

Akzeptanz: Für jeden dieser Zustände existiert ein automatisierter Test, der `/readyz` als nicht-ready beobachtet, ohne dass Liveness verletzt wird.

#### HSM-FA-FAIL-008 – Liveness vs. Readiness

`/healthz` (Liveness) DARF NICHT auf HSM-Fehler reagieren, solange der Service-Prozess selbst korrekt arbeitet. HSM-Ausfälle MÜSSEN ausschließlich auf Readiness und Circuit Breaker wirken, damit Kubernetes den Pod nicht in einer Reconnect-Phase neu startet.

Akzeptanz: Ein simulierter HSM-Ausfall lässt `/healthz` grün und `/readyz` rot; Kubernetes restartet den Pod im Test nicht.

---

## 7. Schnittstellen

### 7.1 Java Client API

#### HSM-API-JAVA-001 – Public API

Die Java-Bibliothek MUSS eine streamingfähige API bereitstellen. Mindestumfang:

```java
public interface HsmDocClient extends AutoCloseable {

    EncryptionResult encrypt(InputStream plaintext,
                             EncryptOptions options,
                             OutputStream encryptedContainer) throws HsmDocException;

    DecryptionResult decrypt(InputStream encryptedContainer,
                             DecryptOptions options,
                             OutputStream plaintext) throws HsmDocException;

    List<KeyInfo> listKeys() throws HsmDocException;

    HealthStatus health();
}
```

Akzeptanz: Javadoc liegt vor, JAR enthält keine PKCS#11- oder JNI-Symbole, ein Beispiel im `examples/`-Modul kompiliert mit Java 21.

#### HSM-API-JAVA-002 – Konfiguration

Die Bibliothek MUSS sich vollständig per Builder konfigurieren lassen (Endpoint, TLS-Material, mTLS-Identität, Timeouts, Retry-Policy, Backpressure-Strategie).

#### HSM-API-JAVA-003 – Fehlerklassen

Die Bibliothek MUSS typisierte Exceptions exponieren: `HsmDocException` (Basis), `BackpressureException`, `IntegrityException` (Tag-Mismatch, Hash-Chain-Bruch), `KeyStateException`, `TransientException`.

#### HSM-API-JAVA-004 – Reactive-Variante (SOLL)

Die Bibliothek SOLL eine reaktive Variante auf Basis von `Flow.Publisher` oder Project Reactor anbieten.

### 7.2 gRPC-Schnittstelle

#### HSM-API-GRPC-001 – Proto-Definition

Die gRPC-Schnittstelle MUSS in Proto3 definiert sein, mit mindestens dem Service:

```proto
service HsmDocService {
  rpc Encrypt(stream EncryptRequest) returns (stream EncryptResponse);
  rpc Decrypt(stream DecryptRequest) returns (stream DecryptResponse);
  rpc ListKeys(ListKeysRequest) returns (ListKeysResponse);
  rpc Health(google.protobuf.Empty) returns (HealthResponse);
}
```

#### HSM-API-GRPC-002 – TLS 1.3

Der gRPC-Endpoint MUSS TLS 1.3 verlangen. TLS 1.2 KANN als Übergangsoption per Konfiguration aktiviert werden; Default ist TLS 1.3 only.

#### HSM-API-GRPC-003 – Mutual TLS

Mutual TLS MUSS unterstützt und über Konfiguration einschaltbar sein.

Akzeptanz: Mit aktiviertem mTLS schlagen Clients ohne gültiges Zertifikat mit `UNAUTHENTICATED` fehl; der Subject-Name aus dem Client-Zertifikat erscheint im Audit-Log als `caller`.

#### HSM-API-GRPC-004 – Statuscode-Mapping

Der Service MUSS interne Fehler auf gRPC-Statuscodes mappen. Mindestens:

| Fehlerklasse        | gRPC-Status                |
| ------------------- | -------------------------- |
| `INVALID_INPUT`     | `INVALID_ARGUMENT`         |
| `QUEUE_FULL`        | `RESOURCE_EXHAUSTED`       |
| `HSM_UNAVAILABLE`   | `UNAVAILABLE`              |
| `TAG_MISMATCH`      | `DATA_LOSS`                |
| `KEY_NOT_FOUND`     | `FAILED_PRECONDITION`      |
| `UNAUTHENTICATED`   | `UNAUTHENTICATED`          |
| `INTERNAL`          | `INTERNAL`                 |

### 7.3 PKCS#11

#### HSM-API-P11-001 – PKCS#11 v2.40

Die HSM-Anbindung MUSS PKCS#11 v2.40 oder höher verwenden.

#### HSM-API-P11-002 – Vendor-Modul

Der Pfad zum Vendor-Modul (`*.so`/`*.dll`) MUSS über Konfiguration setzbar sein und beim Start validiert werden (Existenz, ELF-Header, `C_GetInfo`).

#### HSM-API-P11-003 – miekg/pkcs11

Als Go-Binding MUSS `github.com/miekg/pkcs11` verwendet werden.

### 7.4 Konfigurations- und Health-Schnittstelle

#### HSM-API-CFG-001 – Health- und Ready-Endpoint

Der Dienst MUSS HTTP-Endpoints `/healthz` (Liveness) und `/readyz` (Readiness inkl. HSM-Verfügbarkeit) bereitstellen.

#### HSM-API-CFG-002 – Metrics-Endpoint

Der Dienst MUSS einen `/metrics`-Endpoint im Prometheus-Format bereitstellen.

---

## 8. Container-Format

#### HSM-FMT-001 – Container-Header

Der verschlüsselte Container MUSS mit einem Header beginnen, der mindestens enthält:

- `magic` (4 Byte): `0x48 0x53 0x4D 0x43` (`"HSMC"`)
- `version` (1 Byte): aktuell `0x01`
- `cipher` (1 Byte): `0x01` für AES-256-GCM
- `chunk_size` (4 Byte, BE): konfigurierte Chunkgröße
- `key_id` (16 Byte): UUID des logischen Schlüssels
- `key_version` (4 Byte, BE)
- `header_aad_len` (2 Byte, BE) und optionale anwendungsspezifische AAD
- `header_hmac` (32 Byte): HMAC-SHA-256 über alle vorherigen Header-Felder, mit einem aus dem HSM abgeleiteten Header-Key

Akzeptanz: Schemadokumentation, Encoder/Decoder und Roundtrip-Test liegen vor.

#### HSM-FMT-002 – Chunk-Frame

Jeder Chunk MUSS folgendes Frame-Layout besitzen:

- `seq` (8 Byte, BE)
- `nonce` (12 Byte, gemäß HSM-FA-ENC-004)
- `ciphertext_len` (4 Byte, BE)
- `ciphertext` (`ciphertext_len` Byte)
- `tag` (16 Byte, GCM-Tag)

#### HSM-FMT-003 – Trailer

Der Container MUSS mit einem Trailer enden, der `total_chunks` (8 Byte, BE) und `final_marker` (`0xFF`) enthält. Fehlt der Trailer beim Entschlüsseln, MUSS der Stream als unvollständig abgelehnt werden.

#### HSM-FMT-004 – Versionierung

Das Format MUSS versioniert sein. Der Service MUSS unbekannte `version`-Werte beim Entschlüsseln mit `FAILED_PRECONDITION` und Fehlerklasse `UNSUPPORTED_FORMAT_VERSION` ablehnen.

#### HSM-FMT-005 – Endianness

Alle Mehrbyte-Ganzzahlen im Container MÜSSEN Big-Endian codiert sein.

---

## 9. Datenmodell

#### HSM-DATA-001 – Audit-Eintrag

Audit-Einträge MÜSSEN ein eindeutig versioniertes Schema haben. Pflichtfelder gemäß HSM-FA-AUDIT-001. Optionale Felder sind im Schema markiert.

#### HSM-DATA-002 – Key-Info

`KeyInfo` MUSS mindestens `keyId`, `keyVersion`, `status`, `algorithm`, `createdAt`, `rotatedAt` enthalten und DARF NICHT Schlüsselmaterial enthalten.

#### HSM-DATA-003 – Health-Status

`HealthResponse` MUSS `serviceStatus` (`UP`/`DEGRADED`/`DOWN`), `hsmStatus` (`UP`/`DEGRADED`/`DOWN`), `sessionsActive`, `sessionsMax`, `queueDepth`, `queueCapacity` enthalten.

---

## 10. Nichtfunktionale Anforderungen

### 10.1 Performance

#### HSM-NFA-PERF-001 – Zielwert Durchsatz Encrypt (SoftHSM)

Auf der Referenzumgebung (HSM-LESE-005) SOLL der Service je Replica mindestens 200 MiB/s Encrypt-Durchsatz bei 4-MiB-Chunks gegen SoftHSM v2 erreichen. Dieser Wert ist ein Zielwert; abweichende Messergebnisse MÜSSEN im Abnahmebericht mit Hardware-, Kernel- und Konfigurationsangaben dokumentiert werden.

Akzeptanz: Benchmark-Skript `bench/encrypt-soft.sh` liefert eine reproduzierbare Messung; das Messprotokoll dokumentiert p50/p95/p99-Durchsatz und Konfiguration.

#### HSM-NFA-PERF-002 – Zielwert Durchsatz Netzwerk-HSM

Mit Netzwerk-HSM SOLL je Replica ein Encrypt-Durchsatz erreicht werden, der hardwareprofilspezifisch im jeweiligen Abnahmebericht festgelegt wird. Als Orientierungswert gilt ≥ 50 MiB/s bei 4-MiB-Chunks, RTT < 2 ms und mindestens 16 parallelen Sessions.

Hinweis: Der tatsächlich erreichbare Durchsatz hängt stark von HSM-Modell, AES-Implementierung, Sessionanzahl und RTT ab. Verbindliche Werte werden pro Hardwareprofil festgelegt; siehe HSM-RISK-001 und HSM-RISK-003.

Akzeptanz: Für jedes Produktionsprofil (siehe HSM-TECH-006) liegt ein Messprotokoll vor, das den real erreichten Wert dokumentiert und mit dem im Profil festgelegten Zielwert vergleicht.

#### HSM-NFA-PERF-003 – Latenz pro Chunk (Zielwert)

Die p99-Latenz pro 4-MiB-Chunk-Roundtrip SOLL ≤ 50 ms (SoftHSM) bzw. ≤ 200 ms (Netzwerk-HSM-Referenzprofil) sein. Für andere Hardwareprofile gilt der jeweils festgelegte Profil-Zielwert.

Akzeptanz: Messprotokoll für jedes Profil weist die p99-Latenz aus.

#### HSM-NFA-PERF-004 – Parallele Streams (Zielwert)

Der Service SOLL pro Replica mindestens 64 parallele Streams verarbeiten können, ohne dass die p99-Latenz aus HSM-NFA-PERF-003 um mehr als den Faktor 2 verletzt wird. Die tatsächlich erreichbare Parallelität wird durch die HSM-Sessionkapazität begrenzt (siehe HSM-RISK-001).

Akzeptanz: Lasttest mit 64 parallelen Streams in der Referenzumgebung; Ergebnis und HSM-Sessionauslastung sind dokumentiert.

### 10.2 Skalierbarkeit

#### HSM-NFA-SCALE-001 – Horizontale Skalierung

Der Service MUSS horizontal skalierbar sein.

Akzeptanz: 1, 3 und 10 Replicas erbringen jeweils annähernd lineare Durchsatzsteigerung (≥ 80 % linearer Skalierfaktor bis zur HSM-Kapazitätsgrenze).

#### HSM-NFA-SCALE-002 – Statefulness

Der Service DARF NICHT zwischen Requests persistenten Lokalzustand führen, der für die Korrektheit notwendig ist. Sessions, Pools und Queues sind Laufzeitzustand pro Replica.

### 10.3 Hochverfügbarkeit

#### HSM-NFA-HA-001 – Verfügbarkeitsziel

Der Dienst MUSS bei N ≥ 2 Replicas ein monatliches Verfügbarkeitsziel von ≥ 99,9 % erreichen, gemessen auf gRPC-Endpoint-Ebene und ohne HSM-Ausfall.

#### HSM-NFA-HA-002 – Rolling Restart

Rolling Restart einzelner Replicas DARF NICHT laufende Streams auf anderen Replicas beeinträchtigen. Ein Replica MUSS laufende Streams nach Erhalt von `SIGTERM` graceful abschließen (bis Timeout, Default 30 s).

#### HSM-NFA-HA-003 – HSM-Failover

Bei mehreren konfigurierten HSM-Slots/-Modulen SOLL der Service nach einem HSM-Fehler innerhalb der konfigurierten Backoff-Zeit auf eine alternative HSM-Quelle umschalten.

### 10.4 Sicherheit

#### HSM-NFA-SEC-001 – Transportverschlüsselung

Die Kommunikation zwischen Java-Client und Go-Service MUSS TLS-1.3-gesichert sein (siehe HSM-API-GRPC-002).

#### HSM-NFA-SEC-002 – Mutual TLS

Mutual TLS MUSS unterstützt werden (siehe HSM-API-GRPC-003).

#### HSM-NFA-SEC-003 – Geheimnisverwaltung

Geheimnisse (HSM-PIN, TLS-Schlüssel) MÜSSEN aus externen Quellen stammen und DÜRFEN NICHT in Container-Image, Code, Konfigurationsdateien des Images oder Logs erscheinen.

#### HSM-NFA-SEC-004 – Speicher-Hygiene

Klartext-Buffer im Service MÜSSEN nach Verarbeitung explizit überschrieben werden, soweit Go-Runtime und Garbage Collection dies erlauben (z. B. via `crypto/subtle.ConstantTimeCopy`-Patterns für sensible Pfade).

#### HSM-NFA-SEC-005 – SBOM und CVE-Scanning

Jeder Release MUSS eine SBOM (CycloneDX oder SPDX) sowie einen CVE-Scan-Bericht enthalten.

#### HSM-NFA-SEC-006 – Image-Signierung

Container-Images MÜSSEN signiert ausgeliefert werden (z. B. cosign + Sigstore).

#### HSM-NFA-SEC-007 – Minimaler Base-Layer

Der Service-Container MUSS auf einem minimalen Base-Image (Distroless oder vergleichbar) basieren und keine Shell, kein `cp`, kein `curl` enthalten.

#### HSM-NFA-SEC-008 – Härtung

Der Service-Pod MUSS mit `runAsNonRoot`, `readOnlyRootFilesystem`, `allowPrivilegeEscalation=false`, `seccompProfile=RuntimeDefault` und ohne Capabilities laufen.

### 10.5 Datenschutz

#### HSM-NFA-PRIV-001 – Klartext-Verbot in Logs

Logs DÜRFEN NICHT Klartext, Schlüsselmaterial, AAD-Inhalte mit personenbezogenem Bezug oder PINs enthalten.

#### HSM-NFA-PRIV-002 – Doc-ID-Hashing

Doc-IDs SOLLEN vor dem Loggen mit einem service-internen Salt gehasht werden, sofern sie personenbezogene Hinweise enthalten können.

### 10.6 Speicher und Ressourcen

#### HSM-NFA-MEM-001 – Maximale Speichergröße je Replica

Der Service MUSS den heap- und buffergewachsenen Speicherverbrauch je Replica begrenzen. Default-Obergrenze ist 1 GiB (`GOMEMLIMIT`), gültiger Bereich 256 MiB bis 8 GiB.

#### HSM-NFA-MEM-002 – Keine vollständige Dokumentenpufferung

Der Service DARF NICHT Dokumente vollständig im Hauptspeicher halten (siehe HSM-FA-ENC-003).

### 10.7 Observability

#### HSM-NFA-OBS-001 – OpenTelemetry

Der Dienst MUSS OpenTelemetry für Traces, Metriken und Logs unterstützen (OTLP gRPC, konfigurierbarer Endpoint).

#### HSM-NFA-OBS-002 – Strukturierte Logs

Logs MÜSSEN strukturiert in JSON ausgegeben werden. Pflichtfelder: `time`, `level`, `service`, `version`, `request_id`, `trace_id`, `caller`, `message`.

#### HSM-NFA-OBS-003 – Pflichtmetriken

Folgende Prometheus-Metriken MÜSSEN exponiert sein:

- `hsmdoc_encrypt_total` (Counter, Labels: `result`, `key_id_hash`)
- `hsmdoc_decrypt_total` (Counter, Labels: `result`, `key_id_hash`)
- `hsmdoc_chunk_duration_seconds` (Histogram)
- `hsmdoc_queue_depth` (Gauge)
- `hsmdoc_sessions_active` (Gauge)
- `hsmdoc_sessions_max` (Gauge)
- `hsmdoc_errors_total` (Counter, Labels: `error_class`)
- `hsmdoc_hsm_up` (Gauge, 0/1)

#### HSM-NFA-OBS-004 – Tracing-Spans

Jeder Chunk MUSS einen eigenen Span unter dem Stream-Root-Span erzeugen, mit Attributen `chunk.seq`, `chunk.bytes`, `key.id_hash`.

### 10.8 Betreibbarkeit

#### HSM-NFA-OPS-001 – 12-Factor-Konfiguration

Konfiguration MUSS über Umgebungsvariablen oder eine validierte YAML-Datei erfolgen. Geheimnisse MÜSSEN aus separaten Secret-Quellen kommen.

#### HSM-NFA-OPS-002 – Graceful Shutdown

`SIGTERM` MUSS einen Graceful Shutdown auslösen: keine neuen Streams annehmen, laufende abschließen bis Timeout, Session-Pool sauber schließen.

#### HSM-NFA-OPS-003 – Probes

Liveness-, Readiness- und Startup-Probes MÜSSEN definiert sein und im Helm-Chart vorkonfiguriert sein.

### 10.9 Wartbarkeit und Erweiterbarkeit

#### HSM-NFA-MAINT-001 – Modularität

Der Service MUSS modular aufgebaut sein (siehe Kapitel 11).

#### HSM-NFA-MAINT-002 – Erweiterbarkeit neuer HSMs

Die Integration weiterer PKCS#11-Module DARF NICHT Codeänderungen außerhalb dünner Adapter erfordern.

### 10.10 Portabilität

#### HSM-NFA-PORT-001 – Linux x86_64

Der Service MUSS auf Linux x86_64 lauffähig sein.

#### HSM-NFA-PORT-002 – Linux ARM64 (SOLL)

Der Service SOLL auf Linux ARM64 lauffähig sein.

#### HSM-NFA-PORT-003 – Container-Standard

Container-Images MÜSSEN OCI-konform sein.

---

## 11. Architekturvorgaben und Prinzipien

### HSM-ARCH-001 – Hexagonale Architektur

Der Go-Service MUSS einer hexagonalen Architektur folgen. Der Domain-Kern (Stream-Orchestrierung, Chunking, Container-Codec) DARF NICHT direkt von PKCS#11-, gRPC- oder Storage-Bibliotheken abhängen.

### HSM-ARCH-002 – Worker-Pool

Encrypt/Decrypt-Verarbeitung MUSS in einem Worker-Pool mit konfigurierbarer Größe laufen.

### HSM-ARCH-003 – Session-Pool

Der PKCS#11-Session-Pool MUSS als eigener Adapter implementiert sein, der Sessions liest/leiht/zurückgibt und Re-Login bei Session-Verlust übernimmt.

### HSM-ARCH-004 – Backpressure als Domain-Konzept

Backpressure MUSS im Domain-Kern als explizites Konzept abgebildet sein und sich nicht auf zufälliges Verhalten von gRPC oder Channels verlassen.

### HSM-ARCH-005 – Java-Abstraktion

Die Java-Bibliothek DARF NICHT direkt PKCS#11, JNI oder native Krypto-Libraries einbinden.

Akzeptanz: Build-Analyse zeigt keine JNI-/Native-Abhängigkeiten.

### HSM-PRINC-001 – SOLID

Die Implementierung MUSS nach SOLID-Prinzipien erfolgen; Reviews und ADRs dokumentieren Entscheidungen.

### HSM-PRINC-002 – Kleine Schnittstellen

Adapter (PKCS#11, gRPC, Audit, Metrics) MÜSSEN je eine kleine, fachlich getrennte Schnittstelle exponieren.

### HSM-PRINC-003 – Explizite Fehlerbehandlung

Fehler MÜSSEN typisiert und klassifiziert sein (siehe HSM-FA-RETRY-001 und HSM-API-GRPC-004); sie DÜRFEN NICHT stillschweigend verschluckt werden.

### HSM-CC-001 – Keine zyklischen Modulabhängigkeiten

Module DÜRFEN KEINE zyklischen Importe besitzen. Eine automatisierte Architekturprüfung im CI MUSS Zyklen melden.

---

## 12. Technologievorgaben

### HSM-TECH-001 – Go-Service

Der Service MUSS in Go (Version ≥ 1.22) implementiert werden.

### HSM-TECH-002 – Java-Client

Die Clientbibliothek MUSS Java 21 (LTS) unterstützen.

### HSM-TECH-003 – Transport

Als Kommunikationsprotokoll MUSS gRPC über HTTP/2 verwendet werden.

### HSM-TECH-004 – Kryptografie

Folgende Standards MÜSSEN verwendet werden:

- AES-256-GCM (NIST SP 800-38D),
- PKCS#11 v2.40 oder höher,
- TLS 1.3 (RFC 8446).

### HSM-TECH-005 – Bibliotheken

- Go-PKCS#11: `github.com/miekg/pkcs11` (MUSS).
- Telemetrie: OpenTelemetry SDK (Go und Java) (MUSS).
- Metriken: Prometheus-Client-Bibliotheken (MUSS).
- Container-Runtime: OCI-konform (MUSS).

### HSM-TECH-006 – HSM-Profile

Folgende HSMs MÜSSEN unterstützt werden:

- SoftHSM v2 (Funktional-Referenz),
- Utimaco CryptoServer (Produktionsprofil A),
- Thales Luna HSM (Produktionsprofil B).

Akzeptanz: Für jedes Profil existiert eine Konfigurationsvorlage und ein dokumentierter Smoke-Test.

---

## 13. Umgebungs- und Betriebsanforderungen

### HSM-ENV-001 – Containerfähigkeit

Der Service MUSS als Container-Image ausgeliefert werden.

### HSM-ENV-002 – Kubernetes

Das Deployment MUSS Kubernetes-kompatibel sein; ein Helm-Chart MUSS im Repository liegen.

### HSM-ENV-003 – Lokale Entwicklung

Für lokale Entwicklung MUSS SoftHSM v2 unterstützt werden; ein `docker-compose.dev.yml` MUSS Service und SoftHSM startfähig kombinieren.

### HSM-OPS-MON-001 – Prometheus

Der Dienst MUSS Prometheus-kompatible Metriken bereitstellen (siehe HSM-NFA-OBS-003).

### HSM-OPS-MON-002 – Dashboards

Ein Beispiel-Grafana-Dashboard SOLL im Repository liegen.

### HSM-OPS-HC-001 – Probes

Liveness, Readiness und Startup MÜSSEN als Probes bereitgestellt werden (siehe HSM-API-CFG-001 und HSM-NFA-OPS-003).

### HSM-OPS-CFG-001 – Externe Konfiguration

Alle HSM-, Queue-, Worker-, Pool-, TLS-, Audit- und Telemetrieparameter MÜSSEN extern konfigurierbar sein (Umgebungsvariablen oder YAML).

### HSM-OPS-CFG-002 – Konfigurations-Validierung

Konfigurationsfehler MÜSSEN beim Start mit eindeutiger Fehlermeldung erkannt werden; der Service DARF NICHT mit ungültiger Konfiguration starten.

---

## 14. Compliance

### HSM-COMP-001 – BSI-Vorgaben

Die kryptografischen Verfahren MÜSSEN BSI TR-02102-1 (aktuelle Fassung) entsprechen.

### HSM-COMP-002 – TLS-Konfiguration

Die TLS-Konfiguration MUSS BSI TR-03116-4 entsprechen.

### HSM-COMP-003 – DSGVO

Die Architektur SOLL technische und organisatorische Maßnahmen gemäß DSGVO Art. 32 (Stand der Technik) belegen, insbesondere Verschlüsselung ruhender Daten und Schlüsseltrennung.

### HSM-COMP-004 – HSM-Zertifizierung

Eingesetzte produktive HSMs SOLLEN FIPS 140-3 Level 3 oder Common Criteria EAL4+ zertifiziert sein.

### HSM-COMP-005 – Audit-Aufbewahrung

Die Aufbewahrungsdauer von Audit-Logs MUSS so wählbar sein, dass branchenspezifische Anforderungen (z. B. GoBD, MaRisk, AO §147) erfüllt werden können (siehe HSM-FA-AUDIT-004).

---

## 15. Bedrohungsmodell

Dieses Kapitel skizziert ein orientierendes Threat Model nach Art von STRIDE; eine ausführliche, mit dem Sicherheitsbeauftragten abgestimmte Variante MUSS im Sicherheitskonzept des Projekts entstehen.

### HSM-THREAT-001 – Scope und Vertrauensanker

Innerhalb des Vertrauensankers: HSM (physisch geschützt, zertifiziert), HSM-PIN aus Secret-Store, gepflegte TLS-PKI, Audit-Verankerungssenke.

Außerhalb des Vertrauensankers: jeder Service-Prozess (kompromittierbar), Container-Image vor Signaturprüfung, jeder Cluster-Knoten, jedes Klartext-Backend, jeder Client-Aufrufer.

Akzeptanz: Die Liste der vertrauenswürdigen und nicht-vertrauenswürdigen Komponenten ist im Sicherheitskonzept dokumentiert und mit der Architektur konsistent.

### HSM-THREAT-002 – Insider mit Cluster-Zugriff

Bedrohung: Ein Insider mit Kubernetes-Cluster-Admin-Rechten kann Pods exec'en, Secrets lesen, Sidecars injizieren.

Mitigation: HSM-PIN aus separat berechtigtem Secret-Store, `readOnlyRootFilesystem`, keine Shell im Image (HSM-NFA-SEC-007), 4-Augen-Prinzip für Secret-Zugriffe organisatorisch, RBAC-Trennung Plattform/Crypto-Officer. Restrisiko: Cluster-Admin kann Pod-Identität imitieren — Mitigation nur durch HSM-seitige Bindung an Pod-Attestierung (z. B. SPIFFE/SPIRE + HSM-Login-Policy) erreichbar, KANN als Erweiterung berücksichtigt werden.

### HSM-THREAT-003 – Kompromittierter Client

Bedrohung: Aufrufender Backend-Dienst wurde übernommen und ruft Encrypt/Decrypt mit fremden Doc-IDs oder Mandantenkontext auf.

Mitigation: mTLS mit Per-Service-Identität (HSM-API-GRPC-003), `tenant_id` aus mTLS-Subject (HSM-FA-TENANT-001), Quotas (HSM-FA-TENANT-003), Audit-Sichtbarkeit aller Operationen mit `caller` und `tenant_id`, Anomalie-Erkennung über Auswertung des Audit-Logs.

### HSM-THREAT-004 – Replay verschlüsselter Container

Bedrohung: Ein Angreifer spielt einen alten Container erneut in den Speicher des aufrufenden Systems ein.

Mitigation: AAD im Header bindet Container an Doc-ID, Mandant und ggf. Versionskette (HSM-FA-ENC-005, HSM-FMT-001); die aufrufende Anwendung MUSS die Bindung serverseitig prüfen. Der Dienst selbst kann Replay nicht erkennen, weil er stateless ist.

Akzeptanz: Risiko ist explizit benannt und im Java-Client-Beispiel ist die Bindungsprüfung als Empfehlung dokumentiert.

### HSM-THREAT-005 – Queue/Resource Exhaustion (DoS)

Bedrohung: Ein Angreifer öffnet sehr viele parallele Streams oder sendet Pseudo-Klartext mit künstlich kleiner Chunk-Konfiguration.

Mitigation: Queue-Limits (HSM-FA-QUEUE-001), Tenant-Quotas (HSM-FA-TENANT-003), Chunkgröße-Validierung (HSM-FA-CHUNK-001), Connection-/Stream-Limits am Ingress, Rate-Limit pro `caller`.

### HSM-THREAT-006 – HSM-DoS

Bedrohung: Aggressive Aufrufer treiben die HSM-Session- oder Operations-Kapazität ans Limit, sodass andere Mandanten oder Aufrufer keine Operationen mehr durchführen können.

Mitigation: Fair Scheduling (HSM-FA-TENANT-004), Backpressure (HSM-FA-QUEUE-002), Circuit Breaker (HSM-FA-FAIL-003), Capacity-Planning gegen HSM-Datenblatt.

### HSM-THREAT-007 – Memory Scraping

Bedrohung: Ein Angreifer mit Speicherzugriff auf den Service-Container liest Klartext-Chunks oder Buffer aus.

Mitigation: minimale Pufferzeit pro Chunk, explizites Überschreiben sensibler Buffer (HSM-NFA-SEC-004), `readOnlyRootFilesystem`, keine Coredumps, `MADV_DONTDUMP` für sensible Bereiche, Pod-Härtung (HSM-NFA-SEC-008). Restrisiko: vollständige Memory-Scrubs sind in Go nicht garantiert; das Threat Model dokumentiert dieses Restrisiko.

### HSM-THREAT-008 – Node Compromise

Bedrohung: Ein Cluster-Knoten ist kompromittiert; der Angreifer hat root-Zugriff und liest Prozessspeicher, Filesystem und Netzwerk-Traffic.

Mitigation: Service-Pod als Workload sensibel klassifizieren (z. B. dediziertes NodePool mit erhöhten Härtungsanforderungen), mTLS zwischen Komponenten, getrennte Secret-Store-Berechtigungen, kurze HSM-Session-Lifetime, regelmäßige Knoten-Rotation. Restrisiko: ein root-Angreifer kann jede laufende Encrypt-Operation kompromittieren — dieses Restrisiko ist nur durch Confidential-Compute-Ansätze (z. B. AMD SEV-SNP, Intel TDX) weiter reduzierbar und KANN als Roadmap-Punkt berücksichtigt werden.

### HSM-THREAT-009 – Audit-Manipulation

Bedrohung: Ein Angreifer mit Schreibrecht auf den Audit-Sink versucht, Einträge zu manipulieren oder die Datei vollständig neu zu schreiben.

Mitigation: Hash-Chain (HSM-FA-AUDIT-002), Segmentsignatur (HSM-FA-AUDIT-006), externe Verankerung (HSM-FA-AUDIT-007), Chain-Rotation (HSM-FA-AUDIT-008), Zeitstempelquelle (HSM-FA-AUDIT-009).

### HSM-THREAT-010 – Supply Chain

Bedrohung: Kompromittierte Dependency (Go-Modul, Java-Library, PKCS#11-Vendor-Modul) injiziert bösartigen Code.

Mitigation: SBOM (HSM-NFA-SEC-005), Image-Signierung (HSM-NFA-SEC-006), Pinning aller Abhängigkeiten, Verifikation der Vendor-Module beim Start (HSM-API-P11-002), reproducible builds als Ziel.

---

## 16. Risiken

### HSM-RISK-001 – HSM-Kapazitätsgrenzen

HSMs besitzen begrenzte Session- und Durchsatzkapazitäten.

Mitigation: Session-Pool-Konfiguration, Backpressure, horizontale Skalierung des Service, Lasttests gegen Zielhardware vor Produktivnahme.

### HSM-RISK-002 – PKCS#11-Herstellerunterschiede

PKCS#11-Implementierungen unterscheiden sich erheblich (Mechanismus-Verfügbarkeit, Fehler-Mapping, GCM-IV-Handling).

Mitigation: pro Hersteller ein Adapter-Profil, eigene Smoke-Test-Suite je Profil, Mapping-Tabelle für Fehlercodes.

### HSM-RISK-003 – Netzwerk-HSM-Latenz

Netzwerk-HSMs verursachen zusätzliche Latenzen.

Mitigation: konfigurierbare Chunkgröße, parallele Streams, Profil-spezifische Performance-Ziele (siehe HSM-NFA-PERF-002).

### HSM-RISK-004 – Schlüsselverlust

Verlust eines HSM-Schlüssels macht die damit verschlüsselten Dokumente unwiederbringlich unbrauchbar.

Mitigation: HSM-spezifische, herstellergeprüfte Backup-Verfahren (M-of-N-Wrap, Cloning); diese Verfahren sind NICHT Bestandteil dieses Dienstes (HSM-NONGOAL-001), MÜSSEN aber im Betriebskonzept dokumentiert sein.

### HSM-RISK-005 – PIN-Leakage

Eine geleakte HSM-PIN ermöglicht missbräuchliche Nutzung des HSM.

Mitigation: PIN aus Secret-Store, kein PIN in Logs/Images, Rotationsprozess im Betriebskonzept.

### HSM-RISK-006 – Replay verschlüsselter Container

Ein Angreifer könnte einen vollständigen Container wiedereinspielen.

Mitigation: AAD enthält anwendungsspezifische Kontextinformation (z. B. Doc-ID); aufrufende Anwendung MUSS die Bindung von Container an Doc-ID prüfen.

---

## 17. Annahmen

### HSM-ASSUMP-001 – HSM verfügbar

Es wird angenommen, dass mindestens ein PKCS#11-fähiges HSM bereitsteht und vom Crypto-Officer initialisiert wurde (Token, Slot, User-PIN, Schlüsselgenerierung).

### HSM-ASSUMP-002 – Netzwerkkonnektivität

Es wird angenommen, dass zwischen Service und HSM eine stabile Verbindung mit RTT < 5 ms vorliegt; für Netzwerk-HSMs gilt der dokumentierte Wertebereich.

### HSM-ASSUMP-003 – Time Source

Es wird angenommen, dass alle Replicas eine vertrauenswürdige, NTP-synchronisierte Zeitquelle nutzen (für Audit-Zeitstempel).

### HSM-ASSUMP-004 – Aufrufer authentifiziert

Es wird angenommen, dass aufrufende Backend-Dienste über mTLS oder einen vorgelagerten Token-Issuer authentifiziert sind.

---

## 18. Abnahmekriterien

### HSM-ACCEPT-001 – Funktionale Abnahme

Das Demo-Skript `demo/encrypt-decrypt.sh` verschlüsselt und entschlüsselt eine 1-GiB-Datei gegen SoftHSM mit identischer SHA-256-Summe.

### HSM-ACCEPT-002 – Performance-Abnahme

Das Benchmark-Skript `bench/encrypt-soft.sh` erreicht die Werte aus HSM-NFA-PERF-001 und HSM-NFA-PERF-003 in der Referenzumgebung.

### HSM-ACCEPT-003 – Security-Abnahme

mTLS-Test schlägt für Clients ohne Zertifikat mit `UNAUTHENTICATED` fehl; PIN-Scan über Image und Logs ist negativ; SBOM und Image-Signatur liegen vor.

### HSM-ACCEPT-004 – Audit-Abnahme

Audit-Verifikationstool meldet Manipulation an einem geänderten Audit-Eintrag; Export im JSON-Lines-Format liegt vor.

### HSM-ACCEPT-005 – Betriebsabnahme

Helm-Chart deployed erfolgreich auf einem Kind-Cluster; Liveness, Readiness und Startup-Probes sind grün; Prometheus-Endpoint liefert alle Pflichtmetriken.

### HSM-ACCEPT-006 – Compliance-Abnahme

Konfiguration belegt TLS 1.3, AES-256-GCM, BSI-TR-02102-konforme Cipher-Suites; Datenschutz-Stichprobe an Logs zeigt keine PII-Klartexte.

---

## 19. Mengengerüst

### HSM-MENGE-001 – Lastannahmen MVP

Für den MVP wird folgendes Mengengerüst angenommen:

- bis zu 50 aufrufende Backend-Dienste,
- bis zu 64 parallele Streams je Replica,
- typische Dokumentgröße 100 KiB bis 100 MiB, maximal 10 GiB,
- bis zu 100.000 Encrypt-Operationen pro Tag und Replica.

### HSM-MENGE-002 – Schlüsselanzahl

Es wird angenommen, dass typische Installationen 1 bis 100 logische Schlüssel verwalten. Skalierung auf > 1.000 Schlüssel ist KEIN MVP-Ziel.

---

## 20. Glossar

### HSM-GLOSS-001 – Begriffe

| Begriff             | Bedeutung                                                                                  |
| ------------------- | ------------------------------------------------------------------------------------------ |
| HSM                 | Hardware Security Module                                                                   |
| PKCS#11             | Standardisierte Krypto-API für HSMs und Tokens (OASIS)                                     |
| AES-GCM             | AES im Galois/Counter Mode mit Authentication-Tag                                          |
| AAD                 | Additional Authenticated Data (in GCM mitgeschützte, aber nicht verschlüsselte Daten)      |
| Nonce               | „Number used once" – pro Verschlüsselungsoperation einmaliger Initialisierungsvektor       |
| Tag                 | Authentication-Tag der AES-GCM-Operation (Integritätsprüfung)                              |
| Chunk               | Fester Block des Streams, der einzeln verschlüsselt und mit eigenem Tag versehen wird      |
| Container           | Vollständiger verschlüsselter Datenstrom: Header + Chunks + Trailer                        |
| Session             | Aktive PKCS#11-Verbindung zu einem Token nach erfolgreichem `C_Login`                      |
| Worker              | Goroutine, die einen Chunk durch die HSM-Operation führt                                   |
| Backpressure        | Mechanismus zur Lastdrosselung: Sender wird verlangsamt, statt Speicher unbegrenzt zu füllen |
| mTLS                | Mutual TLS, bidirektionale Zertifikatsprüfung                                              |
| Crypto-Officer      | Rolle, die HSM-Schlüssel und PINs administriert                                            |

---

## 21. Referenzen

### HSM-REF-001 – Normen und Standards

- NIST SP 800-38D – Galois/Counter Mode of Operation
- NIST SP 800-57 – Recommendation for Key Management
- OASIS PKCS#11 Cryptographic Token Interface Base Specification v2.40 / v3.0
- RFC 8446 – TLS 1.3
- RFC 5116 – AEAD-Interfaces
- BSI TR-02102-1 – Kryptographische Verfahren: Empfehlungen und Schlüssellängen
- BSI TR-03116-4 – Kryptographische Vorgaben für TLS
- BSI TR-03125 (TR-ESOR) – Beweiswerterhaltung kryptografisch signierter Dokumente
- DSGVO Art. 32 – Sicherheit der Verarbeitung
- FIPS 140-3 – Security Requirements for Cryptographic Modules

### HSM-REF-002 – Werkzeuge

- `github.com/miekg/pkcs11`
- SoftHSM v2 (OpenDNSSEC)
- OpenTelemetry SDK (Go, Java)
- Prometheus Client Libraries
- cosign / Sigstore
