# Lastenheft – `c-hsm-doc` – Hardwaregestützte Dokumentverschlüsselung mit HSM

| Dokument         | Lastenheft                                                                          |
| ---------------- | ----------------------------------------------------------------------------------- |
| Projektname      | `c-hsm-doc`                                                                         |
| Kurzbeschreibung | HSM-gestützter Krypto-Dienst (Go) mit Java-Streaming-Client zur Dokumentverschlüsselung |
| Zielplattform    | Linux-Container (Bare oder Kubernetes ≥ 1.28, mit oder ohne Service Mesh; siehe `HSM-ENV-004`), PKCS#11-HSM (SoftHSM, Utimaco, Thales) |
| Hauptnutzer      | Backend-Dienste, die regulierungspflichtige Dokumente vertraulich ablegen müssen    |
| Version          | 0.2                                                                                 |
| Status           | Entwurf                                                                             |
| Datum            | 2026-05-27                                                                          |
| Begleitdokument  | [spec/spezifikation.md](spezifikation.md) – Technische Spezifikation                |

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
| Architektur (`ARCH-*`)              | Architekturentscheid (ADR) plus statischer Importcheck                             |
| Prinzipien (`PRINC-*`)              | ADR plus Lint-/Architekturtest                                                     |
| Performance (`NFA-PERF-*`)          | reproduzierbares Benchmark-Skript plus Messprotokoll für die Referenzumgebung      |
| Sicherheit (`NFA-SEC-*`)            | automatisierter Test, Konfigurationsbeleg oder dokumentiertes Reviewprotokoll       |
| Compliance (`COMP-*`)               | Verweis auf einschlägige Norm plus Konfigurations- oder Testbeleg                   |
| Schnittstellen (`API-*`)            | Protobuf-/Javadoc-Artefakt plus Kontrakttest                                       |
| Betrieb (`OPS-*`, `ENV-*`)          | dokumentiertes Runbook, Helm-Chart oder Skript im Repository                       |
| Bedrohungsmodell (`THREAT-*`)       | dokumentierte Risiko- und Mitigationsanalyse im Sicherheitskonzept                  |
| Abnahmekriterien (`ACCEPT-*`)       | im Repository vorhandenes Abnahmeartefakt (Test, Skript, Demo-Lauf)                |

### HSM-LESE-003 – ID-Schema

IDs folgen dem Muster `HSM-<Bereich>-<NNN>` mit dreistelliger Nummer. Bereiche im Lastenheft umfassen:

`LESE`, `ZB`, `PE`, `PUE`, `MVP`, `NONGOAL`,
`FA-ENC`, `FA-DEC`, `FA-CHUNK`, `FA-STREAM`, `FA-HSM`, `FA-KEY`, `FA-QUEUE`, `FA-RETRY`, `FA-AUDIT`, `FA-TENANT`, `FA-FAIL`,
`API-JAVA`, `API-GRPC`, `API-P11`, `API-CFG`,
`NFA-PERF`, `NFA-SCALE`, `NFA-HA`, `NFA-SEC`, `NFA-PRIV`, `NFA-MEM`, `NFA-OBS`, `NFA-OPS`, `NFA-MAINT`, `NFA-PORT`,
`ARCH`, `PRINC`,
`TECH`, `ENV`, `OPS-MON`, `OPS-HC`, `OPS-CFG`,
`COMP`, `THREAT`, `RISK`, `ASSUMP`, `ACCEPT`, `MENGE`, `GLOSS`, `REF`.

Die Technische Spezifikation (spec/spezifikation.md) verwendet denselben ID-Raum. Eine `HSM-…`-ID lebt in genau einem der beiden Dokumente; Cross-Referenzen funktionieren dadurch über beide Dokumente hinweg.

### HSM-LESE-004 – Verhältnis zur Technischen Spezifikation

Dieses Lastenheft beschreibt das fachliche, vertragliche und sicherheitsrelevante **WAS und WARUM**. Es bildet die vertragliche und Abnahme-Grundlage.

Die Technische Spezifikation (`spec/spezifikation.md`) beschreibt das **WIE**: Algorithmen, Datenstrukturen, Protokoll-Details, PKCS#11-Returncodes, Metriknamen, Container-Layout, Retry-Strategien.

Es gilt:

- Anforderungen in diesem Lastenheft sind vertraglich/abnahmebindend.
- Anforderungen in der Technischen Spezifikation sind technisch verbindlich für die Implementierung, aber nicht vertraglich abnahmebindend. Sie können ohne Lastenheft-Änderung weiterentwickelt werden, solange sie keine Lastenheft-Anforderung verletzen.
- Im Konfliktfall hat dieses Lastenheft Vorrang.

### HSM-LESE-005 – Referenzumgebung

Sofern eine Anforderung Performance, Latenz oder Durchsatz benennt, gilt – wenn nicht anders angegeben – folgende Referenzumgebung:

- Linux x86_64, Kernel ≥ 6.1
- 4 vCPU, 8 GiB RAM je Service-Replica
- gRPC über Loopback oder lokales 10-GbE-VLAN
- HSM: SoftHSM v2 lokal (Funktional-Referenz) bzw. Netzwerk-HSM mit < 2 ms RTT (Performance-Referenz)

### HSM-LESE-006 – SoftHSM-Abgrenzung

SoftHSM v2 dient ausschließlich als funktionale Referenz für Entwicklung, Unit-Tests und Smoke-Tests im CI. SoftHSM verhält sich nicht wie ein produktives Hardware- oder Netzwerk-HSM (keine echten Sessionlimits, andere Latenz- und Fehlerprofile, keine echte Schlüsselisolation in Hardware).

Es gilt:

- Funktionale Abnahmen (HSM-ACCEPT-001) DÜRFEN gegen SoftHSM erbracht werden.
- Performance-Abnahmen (HSM-ACCEPT-002), Failure-Abnahmen, Compliance-Abnahmen (HSM-ACCEPT-006) und FIPS-/CC-Nachweise (HSM-COMP-004) DÜRFEN NICHT ausschließlich gegen SoftHSM erbracht werden; sie MÜSSEN für jedes vorgesehene Produktionsprofil (HSM-TECH-006) separat nachgewiesen werden.
- Lasttests gegen SoftHSM gelten als Code-/Architekturnachweis, nicht als Produktionsnachweis.

---

## 1. Zielbestimmung

### HSM-ZB-001 – Projektziel

`c-hsm-doc` MUSS einen hochverfügbaren kryptografischen Dienst bereitstellen, der Dokumente beliebiger Größe hardwaregestützt mittels HSM verschlüsselt und entschlüsselt, ohne dass kryptografisches Schlüsselmaterial das HSM verlässt.

Akzeptanz: Ein Referenzlauf verschlüsselt ein 1-GiB-Dokument gegen SoftHSM, schreibt einen Container im definierten Format und stellt ihn byte-identisch wieder her.

### HSM-ZB-002 – Produktvision

Der Dienst SOLL sich aus Sicht aufrufender Backend-Dienste wie ein streamingfähiger „Crypto-as-a-Service" mit harten HSM-Garantien verhalten: einfacher Java-Client, gRPC-Stream, keine PKCS#11-Details für den Aufrufer, kein Klartext im Speicher des Service über Chunk-Grenzen hinaus.

### HSM-ZB-003 – Muss-/Soll-/Kann-Ziele

| Klasse | Ziel                                                                                          |
| ------ | --------------------------------------------------------------------------------------------- |
| MUSS   | AES-256-GCM ausschließlich im HSM, Schlüssel nicht extrahierbar.                              |
| MUSS   | Streamingbasierte, chunkfähige Verarbeitung; keine vollständige Dokumentenpufferung im RAM.   |
| MUSS   | Java-21-Clientbibliothek ohne JNI- oder PKCS#11-Abhängigkeit.                                 |
| MUSS   | Horizontal skalierbarer, container- und Kubernetes-fähiger Go-Service.                        |
| MUSS   | Revisionssichere Auditierung jeder kryptografischen Operation.                                |
| MUSS   | Mandantentrennung auf Schlüssel-, Quota- und Audit-Ebene.                                     |
| SOLLTE | Mehrere PKCS#11-Hersteller (SoftHSM, Utimaco, Thales) ohne Codeänderung austauschbar.         |
| SOLLTE | Schlüsselrotation ohne Ausfall laufender Streams.                                             |
| KANN   | Wiederaufnehmbare (resumable) Verschlüsselungs-Streams.                                       |

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

- Linux x86_64 als Container (OCI-Image), lauffähig sowohl als
  Bare-Container (Docker/Podman/containerd) als auch in
  Kubernetes (≥ 1.28) — siehe `HSM-ENV-004` für die vier
  unterstützten Betriebsmodi,
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
- **UC-7 Re-Encrypt**: Bestehender Container wird ohne Klartextfreigabe auf neuen Key umgeschlüsselt.

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
| `c-hsm-doc-proto`     | Proto3  | gRPC-Definitionen, gemeinsame Datenmodelle                                    |
| `c-hsm-doc-container` | –       | Containerfile, Helm-Chart, Probes, Default-Konfiguration                      |

### HSM-PUE-003 – Vertrauensgrenzen

Folgende Vertrauensgrenzen MÜSSEN als solche dokumentiert und in Code/Konfiguration durchgesetzt werden:

- **Client ↔ Service**: TLS 1.3 mit Serverzertifikat (MUSS), Client-Zertifikat (SOLL für interne Aufrufer).
- **Service ↔ HSM**: PKCS#11-Login mit PIN aus externem Secret-Store (MUSS), HSM-Sessions im Pool wiederverwendet.
- **Service ↔ Audit-Sink**: integritätsgesicherte, append-only-Auslieferung (MUSS); Audit-Empfänger DARF NICHT Klartext, Schlüsselmaterial oder PIN sehen.

---

## 4. MVP-Umfang

### HSM-MVP-001 – Lokaler End-to-End-Stream

Der MVP MUSS Verschlüsselung und Entschlüsselung eines Dokuments im Streaming-Modus gegen SoftHSM v2 demonstrieren.

Akzeptanz: Ein Demo-Skript verschlüsselt eine 1-GiB-Datei und entschlüsselt sie wieder; die SHA-256-Summe von Original und Wiederherstellung ist identisch.

### HSM-MVP-002 – Konfigurierbarer Session- und Worker-Pool

Der MVP MUSS einen PKCS#11-Session-Pool und einen Worker-Pool bereitstellen, beide über Konfiguration einstellbar.

Akzeptanz: Ein Lasttest zeigt keine PKCS#11-Session-Leaks und keine Worker-Hangs nach 100.000 Chunks.

### HSM-MVP-003 – Job-Queue mit Backpressure

Der MVP MUSS eine begrenzte Job-Queue mit Backpressure implementieren.

Akzeptanz: Bei Überlauf antwortet der Service mit gRPC-Status `RESOURCE_EXHAUSTED`; der Java-Client bietet einen konfigurierbaren Retry-/Wait-Adapter.

### HSM-MVP-004 – Audit-Log

Der MVP MUSS jede Encrypt/Decrypt-Operation in ein append-only Audit-Log schreiben.

Akzeptanz: Manipulation an einem bestehenden Eintrag wird durch eine Integritätsprüfung erkannt.

### HSM-MVP-005 – Containerisierte Auslieferung

Der MVP MUSS als Container-Image auslieferbar sein und in allen Modi aus `HSM-ENV-004` lauffähig sein. Ein Helm-Chart MUSS für die Kubernetes-Modi bereitstehen; für den Bare-Container-Modus MUSS eine dokumentierte `docker run`- oder `podman run`-Aufrufsequenz vorliegen.

Akzeptanz: (a) `docker run` (bzw. `podman run`) startet den Service erfolgreich gegen SoftHSM; Liveness- und Readiness-Endpoints sind über Port-Mapping erreichbar und grün; eine Demo-Verschlüsselung läuft durch. (b) `helm install` auf einem lokalen Kind-Cluster startet den Service erfolgreich; Liveness- und Readiness-Probes sind grün; eine Demo-Verschlüsselung über `port-forward` läuft durch.

### HSM-MVP-006 – Java-Client ohne JNI

Der MVP MUSS einen Java-21-Client bereitstellen, der den Service über gRPC anspricht und keine JNI- oder PKCS#11-Abhängigkeit besitzt.

Akzeptanz: Eine Maven-Build-Analyse listet keine native- oder JNI-Abhängigkeiten.

---

## 5. Nicht-Ziele und Scope-Grenzen

### HSM-NONGOAL-001 – Kein Schlüsselverwaltungssystem

Der Dienst ist KEIN vollwertiges Key-Management-System. Schlüssel werden ausschließlich im HSM erzeugt und verwaltet; Funktionen wie Quorum-basierte Schlüsselgenerierung, M-of-N-Backup oder hierarchische KEK-Strukturen sind nicht im Scope.

### HSM-NONGOAL-002 – Kein Dokumentenarchiv

Der Dienst persistiert keine Dokumente. Eingabe und Ausgabe erfolgen ausschließlich als Stream.

### HSM-NONGOAL-003 – Keine asymmetrische Kryptografie im MVP

RSA, ECC, Signaturen und Hybridverschlüsselung sind nicht Bestandteil des MVP.

### HSM-NONGOAL-004 – Kein Re-Verschlüsseln gegen externe Speicher

Re-Encrypt (UC-7) findet ausschließlich innerhalb des Dienstes statt; der Service liest oder schreibt keine Dokumente in externe Storage-Backends.

### HSM-NONGOAL-005 – Kein eigenes Identity-Provider-Modul

Der Dienst stellt keinen eigenen IdP bereit. Authentifizierung erfolgt über mTLS und/oder vorgelagerten Token-Issuer.

---

## 6. Funktionale Anforderungen

Implementierungs- und Verfahrensdetails (Algorithmen, Datenstrukturen, Protokoll-Codes) sind in der Technischen Spezifikation festgelegt. Die Detail-Anforderungen werden hier referenziert, sind aber nicht vertraglich abnahmebindend.

### 6.1 Verschlüsselung und Entschlüsselung

#### HSM-FA-ENC-001 – AES-256-GCM-Verschlüsselung

Der Dienst MUSS Dokumente mittels AES-256-GCM verschlüsseln.

Akzeptanz: Ciphertexte sind mit einem Referenz-Tool (z. B. `openssl enc` mit gleichem Key, Nonce und AAD) verifizierbar.

#### HSM-FA-ENC-002 – HSM-residente Schlüssel

Die AES-Operation MUSS vollständig im HSM ausgeführt werden. Der Schlüssel DARF NICHT das HSM verlassen.

Akzeptanz: Das PKCS#11-Attribut `CKA_EXTRACTABLE=false` ist auf allen Encrypt-/Decrypt-Schlüsseln gesetzt; ein Wrap-Versuch schlägt mit `CKR_KEY_UNEXTRACTABLE` fehl.

#### HSM-FA-ENC-003 – Streaming-Eingabe

Die Verschlüsselung MUSS streamingbasiert erfolgen. Der Dienst DARF NICHT das gesamte Dokument im Speicher halten.

Akzeptanz: Der Heap-Verbrauch beim Verschlüsseln einer 10-GiB-Datei überschreitet den konfigurierten Maximalwert (siehe HSM-NFA-MEM-001) nicht.

#### HSM-FA-DEC-001 – Entschlüsselung als Inverse

Der Dienst MUSS einen mit ihm verschlüsselten Container vollständig in den Originaldatenstrom zurückführen können.

Akzeptanz: Für 100 zufällig generierte Eingaben gilt `sha256(decrypt(encrypt(x))) == sha256(x)`.

#### HSM-FA-DEC-002 – Authentizitätsprüfung

Der Dienst MUSS die Integrität jedes Ciphertext-Bausteins prüfen und den Stream bei Mismatch sofort abbrechen. Bereits ausgegebener Klartext nachfolgender Bausteine DARF NICHT vor erfolgreicher Prüfung des aktuellen Bausteins ausgeliefert werden.

Akzeptanz: Ein mutierter Ciphertext-Chunk führt zu gRPC-Status `DATA_LOSS`.

Detail-Verfahren: AAD-Bindung, Per-Chunk-AEAD, Tag-Größe – siehe spec/spezifikation.md.

### 6.2 Chunking und Streaming

#### HSM-FA-CHUNK-001 – Konfigurierbare Chunkgröße

Die Chunkgröße MUSS konfigurierbar sein. Konkrete Defaults und gültige Bereiche sind in spec/spezifikation.md festgelegt.

Akzeptanz: Konfigurationswerte außerhalb des gültigen Bereichs verhindern den Start mit einer eindeutigen Fehlermeldung.

#### HSM-FA-CHUNK-002 – Unabhängigkeit von der Dateigröße

Die chunkbasierte Verarbeitung MUSS unabhängig von der Eingangs-Dateigröße funktionieren, einschließlich Streams unbekannter Länge.

#### HSM-FA-CHUNK-003 – Reihenfolge-Sicherung

Chunks MÜSSEN in derselben Reihenfolge wieder ausgegeben werden, in der sie verschlüsselt wurden. Reihenfolgeverletzungen MÜSSEN beim Entschlüsseln erkannt werden.

Akzeptanz: Eine vertauschte Sequenz führt zu Decrypt-Abbruch mit definierter Fehlerklasse.

#### HSM-FA-STREAM-001 – Bidirektionales gRPC-Streaming

Die Übertragung zwischen Java-Client und Go-Service MUSS über bidirektionales gRPC-Streaming erfolgen.

#### HSM-FA-STREAM-002 – Backpressure und Cancellation

Der Dienst MUSS Backpressure durchsetzen und Stream-Cancellation durch den Client respektieren. Konkrete Semantik (insbesondere Verhalten gegenüber bereits laufenden PKCS#11-Operationen) ist in spec/spezifikation.md festgelegt.

### 6.3 HSM-Anbindung

#### HSM-FA-HSM-001 – PKCS#11 als Anbindung

Die HSM-Anbindung MUSS über PKCS#11 v2.40 oder höher erfolgen.

Akzeptanz: Modulpfad und Slot/Token-Label sind konfigurierbar; der Service startet erfolgreich gegen SoftHSM v2 und ein zweites herstellerfremdes Modul ohne Codeänderung.

#### HSM-FA-HSM-002 – PIN-Bezug aus Secret-Store

Die HSM-User-PIN MUSS aus einem externen Secret-Store (Kubernetes Secret, HashiCorp Vault, Datei mit Mode 0400) bezogen werden. Sie DARF NICHT im Code, im Container-Image oder in Logs erscheinen.

Akzeptanz: Image-Scan und Log-Scan finden keine PIN; eine Konfiguration ohne Secret-Quelle führt zu definiertem Startfehler.

#### HSM-FA-HSM-003 – Session-Pool

Der Service MUSS einen PKCS#11-Session-Pool mit konfigurierbarer Größe und Lifetime bereitstellen. Pool-Defaults und Re-Login-Strategien sind in spec/spezifikation.md festgelegt.

### 6.4 Schlüsselverwaltung

#### HSM-FA-KEY-001 – Schlüssel-Lebenszyklus

Der Dienst MUSS einen Schlüssel-Lebenszyklus mit den Zuständen `active`, `deprecated`, `destroyed` führen.

Akzeptanz: Nur `active`-Schlüssel können zum Verschlüsseln verwendet werden; `deprecated`-Schlüssel sind nur zum Entschlüsseln zugelassen; `destroyed`-Schlüssel führen zu definiertem Fehler.

#### HSM-FA-KEY-002 – Logische Key-ID

Jeder Schlüssel MUSS eine stabile logische Key-ID tragen, die im Container-Header gespeichert wird.

#### HSM-FA-KEY-003 – Schlüsselrotation

Der Dienst SOLL Schlüsselrotation unterstützen: Ein neuer aktiver Schlüssel ersetzt den alten, der in den Status `deprecated` wechselt. Laufende Streams DÜRFEN NICHT abgebrochen werden.

#### HSM-FA-KEY-004 – Metadaten außerhalb des HSM

Schlüssel-Metadaten (Key-ID, Status, Erzeugungszeit, Rotationszeit, Algorithmus) MÜSSEN außerhalb des HSM gepflegt werden, ohne sensible Inhalte zu duplizieren.

Akzeptanz: Das Verzeichnis enthält weder Klartext-Schlüssel noch Wrap-Keys.

#### HSM-FA-KEY-005 – Begrenzung der Schlüsselnutzung

Der Dienst MUSS die Anzahl der Verschlüsselungsoperationen je logischer `key_id` begrenzen, um die kryptografische Sicherheitsgrenze von AES-GCM einzuhalten. Beim Erreichen einer harten Obergrenze MUSS der Dienst weitere Encrypt-Anfragen für diese `key_id` ablehnen, bis eine Rotation erfolgt ist.

Hard- und Soft-Limits sind in spec/spezifikation.md festgelegt; eine Anpassung ist über Konfiguration möglich.

### 6.5 Queueing und Retry

#### HSM-FA-QUEUE-001 – Begrenzte Job-Queue mit Backpressure

Der Dienst MUSS eine begrenzte interne Job-Queue mit Backpressure bereitstellen. Bei Überschreiten der konfigurierten Tiefe MUSS er weitere Requests mit gRPC-Status `RESOURCE_EXHAUSTED` ablehnen.

Default-Tiefe und Wartezeit-Strategie: siehe spec/spezifikation.md.

#### HSM-FA-RETRY-001 – Klassifizierung transienter Fehler

Der Dienst MUSS Fehler in transient, permanent und client klassifizieren. Nur transient darf intern wiederholt werden.

Akzeptanz: Eine Mapping-Tabelle (HSM-Fehlercode → Klasse) ist im Repository dokumentiert.

#### HSM-FA-RETRY-002 – Commit-Idempotenz pro Chunk

Eine Chunk-Verarbeitung gilt erst dann als committed, wenn der Ciphertext erfolgreich emittiert und der zugehörige Audit-Eintrag persistiert wurde. Für jede Sequenznummer MUSS das Audit-Log höchstens einen `result=ok`-Eintrag enthalten; fehlgeschlagene Versuche MÜSSEN separat protokolliert sein.

Hinweis: Jeder Retry erzeugt zwangsläufig einen neuen Ciphertext und einen neuen Tag, weil eine neue Nonce verwendet wird. Idempotenz bezieht sich auf die fachliche Commit-Wirkung, nicht auf die Bytefolge.

### 6.6 Auditierung

#### HSM-FA-AUDIT-001 – Audit-Pflichtfelder

Jeder Audit-Eintrag MUSS mindestens folgende Felder enthalten: Zeitstempel (UTC), Operation, Key-ID, Key-Version, Doc-ID, Caller-Identität, Tenant-ID, Resultat, Fehlerklasse, Stream-ID, Request-ID, Trace-ID.

Die Tenant-ID erfüllt zusammen mit `HSM-FA-TENANT-004` die Mandantenkontext-Pflicht. Im Single-Tenant-Bootstrap-Modus ist ein definierter Default-Wert (z. B. `default`) zulässig.

Konkretes Schema: siehe spec/spezifikation.md.

#### HSM-FA-AUDIT-002 – Revisionssicherheit

Audit-Einträge MÜSSEN append-only und manipulationsgeschützt geschrieben werden. Der Schutz MUSS mindestens eine Hash-Chain umfassen; in regulierten Umgebungen MÜSSEN zusätzlich Signatur, externe Verankerung und Chain-Rotation eingesetzt werden.

Akzeptanz: Eine Manipulation eines beliebigen Eintrags wird durch ein automatisches Verify-Tool erkannt. Konkretes Verankerungsverfahren (RFC 3161, Transparency-Log, SIEM): siehe spec/spezifikation.md.

#### HSM-FA-AUDIT-003 – Klartextverbot

Audit-Einträge DÜRFEN NICHT Klartext, Schlüsselmaterial, PINs oder vollständige Ciphertexte enthalten.

#### HSM-FA-AUDIT-004 – Aufbewahrung

Die Aufbewahrungsfrist MUSS konfigurierbar sein und so wählbar, dass branchenspezifische Anforderungen (GoBD, MaRisk, AO § 147) erfüllt werden können.

#### HSM-FA-AUDIT-005 – Vertrauenswürdige Zeitquelle und Durability

Audit-Zeitstempel MÜSSEN aus einer vertrauenswürdigen, NTP-synchronisierten Zeitquelle stammen.

Audit-Einträge MÜSSEN dauerhaft persistiert sein, bevor der zugehörige Ciphertext-Chunk emittiert wird. Bei Persistenz-Fehler MUSS der Stream abgebrochen werden.

Konkrete Sync-Strategien, RFC-3161-Verwendung und zulässige Audit-Senken: siehe spec/spezifikation.md.

### 6.7 Mandantenisolation

#### HSM-FA-TENANT-001 – Tenant als erstklassiges Konzept

Der Dienst MUSS Mandanten (`tenant_id`) als erstklassiges Konzept führen. Die `tenant_id` MUSS aus dem mTLS-Subject, einem Token-Claim oder einem expliziten gRPC-Header abgeleitet werden.

Akzeptanz: Requests ohne auflösbare `tenant_id` werden mit `UNAUTHENTICATED` bzw. `FAILED_PRECONDITION` abgelehnt.

#### HSM-FA-TENANT-002 – Schlüsseltrennung

Ein Mandant DARF NICHT auf Schlüssel anderer Mandanten zugreifen.

Akzeptanz: Ein Decrypt-Versuch mit einer Key-ID eines fremden Mandanten schlägt mit `FAILED_PRECONDITION` und Fehlerklasse `KEY_NOT_FOUND` fehl; ein Audit-Eintrag mit `result=error` wird geschrieben.

#### HSM-FA-TENANT-003 – Quotas pro Mandant

Der Dienst MUSS pro Mandant konfigurierbare Quotas für mindestens folgende Größen unterstützen: maximale parallele Streams, maximale Queue-Tiefe, maximaler Durchsatz pro Zeitfenster.

Akzeptanz: Quota-Überschreitung führt zu `RESOURCE_EXHAUSTED` mit Fehlerklasse `TENANT_QUOTA`.

#### HSM-FA-TENANT-004 – Mandantenkontext in Audit und Telemetrie

`tenant_id` MUSS in jedem Audit-Eintrag und in den Tenant-relevanten Metriken/Spans enthalten sein.

Detail (Hashing von IDs in Metrik-Labels, Fair-Scheduling-Algorithmus): siehe spec/spezifikation.md.

### 6.8 Verfügbarkeits- und Fehlerverhalten

#### HSM-FA-FAIL-001 – Resilienz gegen HSM-Fehler

Der Dienst MUSS gegen die folgenden HSM-Fehlersituationen widerstandsfähig sein und sie kontrolliert behandeln:

- ungültige Session,
- HSM-Device-Fehler,
- Token-Removal und HSM-Reboot,
- Netzwerkpartition zum Netzwerk-HSM,
- fehlende oder verlorene Login-Sitzung,
- nicht unterstützte Mechanismen.

Akzeptanz: Für jede Situation existiert ein dokumentiertes Verhalten und ein automatisierter Failure-Test.

Konkrete PKCS#11-Returncode-Tabelle, Circuit-Breaker-Parameter, Reconnect-Backoff, Re-Login-Throttling, Session-Recycling: siehe spec/spezifikation.md.

#### HSM-FA-FAIL-002 – Trennung Liveness und Readiness

`/healthz` (Liveness) DARF NICHT auf HSM-Fehler reagieren, solange der Service-Prozess selbst korrekt arbeitet. HSM-Ausfälle MÜSSEN ausschließlich auf Readiness wirken.

Akzeptanz: Ein simulierter HSM-Ausfall lässt `/healthz` grün und `/readyz` rot; ein die Endpunkte konsumierender Orchestrator (Kubernetes-Probes, Docker-Healthcheck, externer Watchdog) restartet den Container im Test nicht.

---

## 7. Schnittstellen

### HSM-API-JAVA-001 – Streamingfähige Java-API

Die Java-Bibliothek MUSS eine streamingfähige API für Encrypt und Decrypt bereitstellen und MUSS Java 21 unterstützen. Sie DARF NICHT direkt PKCS#11, JNI oder native Krypto-Libraries einbinden.

Akzeptanz: JAR enthält keine PKCS#11- oder JNI-Symbole; ein Beispielprogramm kompiliert und läuft gegen den Demo-Service.

Konkrete Schnittstellen-Signaturen, Exception-Klassen und reaktive Variante: siehe spec/spezifikation.md.

### HSM-API-GRPC-001 – gRPC-Schnittstelle

Die Kommunikation zwischen Java-Client und Go-Service MUSS über gRPC mit bidirektionalem Streaming für Encrypt und Decrypt erfolgen.

Proto-Definitionen, gRPC-Statuscode-Mapping: siehe spec/spezifikation.md.

### HSM-API-GRPC-002 – TLS 1.3

Der gRPC-Endpoint MUSS TLS 1.3 verlangen. TLS 1.2 KANN als Übergangsoption per Konfiguration aktiviert werden; Default ist TLS 1.3 only.

### HSM-API-GRPC-003 – Mutual TLS

Mutual TLS MUSS unterstützt und über Konfiguration einschaltbar sein.

Die Quelle der Caller-Identität (Audit-Feld `caller`, Ableitung von `tenant_id`) MUSS konfigurierbar sein:

- `mtls-subject` (Default; gültig in den Modi 1–3 aus `HSM-ENV-004`): Subject-Name bzw. SAN aus dem am Server terminierten Client-Zertifikat.
- `header` (gültig in Modus 4 aus `HSM-ENV-004` bei mesh-terminiertem mTLS): konfigurierbarer Header (z. B. `x-forwarded-client-cert`, SPIFFE-ID-Header, JWT-Claim).

Bei Identitätsquelle `header` MUSS der Server die unmittelbare Peer-Adresse und/oder das Peer-Zertifikat gegen eine explizit konfigurierte Allowlist vertrauenswürdiger Peers (z. B. Mesh-Sidecar-SAN, Loopback aus demselben Pod) prüfen. Liegt diese Allowlist nicht vor oder ist sie leer, MUSS der Start gemäß `HSM-OPS-CFG-002` mit eindeutigem Fehler abbrechen.

Akzeptanz: Mit aktiviertem mTLS und Identitätsquelle `mtls-subject` schlagen Clients ohne gültiges Zertifikat mit `UNAUTHENTICATED` fehl; der Subject-Name aus dem Client-Zertifikat erscheint im Audit-Log als `caller`. Mit Identitätsquelle `header` und nicht-vertrauenswürdigem Peer wird die Anfrage mit `UNAUTHENTICATED` abgewiesen; mit vertrauenswürdigem Peer erscheint die im Header transportierte Identität als `caller`.

### HSM-API-P11-001 – PKCS#11 v2.40

Die HSM-Anbindung MUSS PKCS#11 v2.40 oder höher verwenden.

Konkretes Go-Binding und Vendor-Modul-Validierung: siehe spec/spezifikation.md.

### HSM-API-CFG-001 – Health- und Ready-Endpoint

Der Dienst MUSS HTTP-Endpoints `/healthz` (Liveness) und `/readyz` (Readiness inkl. HSM-Verfügbarkeit) bereitstellen.

### HSM-API-CFG-002 – Metrics-Endpoint

Der Dienst MUSS einen `/metrics`-Endpoint im Prometheus-Format bereitstellen.

---

## 8. Nichtfunktionale Anforderungen

### 8.1 Performance

#### HSM-NFA-PERF-001 – Zielwert Durchsatz Encrypt (SoftHSM)

Auf der Referenzumgebung SOLL der Service je Replica mindestens 200 MiB/s Encrypt-Durchsatz bei 4-MiB-Chunks gegen SoftHSM v2 erreichen. Abweichungen MÜSSEN im Abnahmebericht dokumentiert werden.

#### HSM-NFA-PERF-002 – Zielwert Durchsatz Netzwerk-HSM

Mit Netzwerk-HSM SOLL je Replica ein Encrypt-Durchsatz erreicht werden, der hardwareprofilspezifisch im jeweiligen Abnahmebericht festgelegt wird. Orientierungswert: ≥ 50 MiB/s bei 4-MiB-Chunks, RTT < 2 ms und mindestens 16 parallelen Sessions.

#### HSM-NFA-PERF-003 – Latenz pro Chunk (Zielwert)

Die p99-Latenz pro 4-MiB-Chunk-Roundtrip SOLL ≤ 50 ms (SoftHSM) bzw. ≤ 200 ms (Netzwerk-HSM-Referenzprofil) sein.

#### HSM-NFA-PERF-004 – Parallele Streams (Zielwert)

Der Service SOLL pro Replica mindestens 64 parallele Streams verarbeiten können, ohne dass die p99-Latenz aus HSM-NFA-PERF-003 um mehr als Faktor 2 verletzt wird.

### 8.2 Skalierbarkeit

#### HSM-NFA-SCALE-001 – Horizontale Skalierung

Der Service MUSS horizontal skalierbar sein.

Akzeptanz: 1, 3 und 10 Replicas erbringen jeweils annähernd lineare Durchsatzsteigerung (≥ 80 % linearer Skalierfaktor bis zur HSM-Kapazitätsgrenze).

#### HSM-NFA-SCALE-002 – Statefulness

Der Service DARF NICHT zwischen Requests persistenten Lokalzustand führen, der für die Korrektheit notwendig ist.

### 8.3 Hochverfügbarkeit

#### HSM-NFA-HA-001 – Verfügbarkeitsziel

Der Dienst MUSS bei N ≥ 2 Replicas ein monatliches Verfügbarkeitsziel von ≥ 99,9 % erreichen, gemessen auf gRPC-Endpoint-Ebene und ohne HSM-Ausfall.

#### HSM-NFA-HA-002 – Rolling Restart

Rolling Restart einzelner Replicas DARF NICHT laufende Streams auf anderen Replicas beeinträchtigen. Ein Replica MUSS laufende Streams nach Erhalt von `SIGTERM` graceful abschließen (bis Timeout, Default 30 s).

#### HSM-NFA-HA-003 – HSM-Failover

Bei mehreren konfigurierten HSM-Slots/-Modulen SOLL der Service nach einem HSM-Fehler innerhalb der konfigurierten Backoff-Zeit auf eine alternative HSM-Quelle umschalten.

### 8.4 Sicherheit

#### HSM-NFA-SEC-001 – Transportverschlüsselung

Die Kommunikation zwischen Java-Client und Go-Service MUSS TLS-1.3-gesichert sein.

#### HSM-NFA-SEC-002 – Mutual TLS

Mutual TLS MUSS unterstützt werden.

#### HSM-NFA-SEC-003 – Geheimnisverwaltung

Geheimnisse (HSM-PIN, TLS-Schlüssel) MÜSSEN aus externen Quellen stammen und DÜRFEN NICHT in Container-Image, Code, Konfigurationsdateien des Images oder Logs erscheinen.

#### HSM-NFA-SEC-004 – Speicher-Hygiene

Klartext-Buffer im Service MÜSSEN nach Verarbeitung explizit überschrieben werden, soweit die Go-Runtime und Garbage Collection dies erlauben.

#### HSM-NFA-SEC-005 – SBOM und CVE-Scanning

Jeder Release MUSS eine SBOM (CycloneDX oder SPDX) sowie einen CVE-Scan-Bericht enthalten.

#### HSM-NFA-SEC-006 – Image-Signierung

Container-Images MÜSSEN signiert ausgeliefert werden (z. B. cosign + Sigstore).

#### HSM-NFA-SEC-007 – Minimaler Base-Layer

Der Service-Container MUSS auf einem minimalen Base-Image (Distroless oder vergleichbar) basieren und keine Shell, kein `cp`, kein `curl` enthalten.

#### HSM-NFA-SEC-008 – Pod-Härtung

Der Service-Pod MUSS mit `runAsNonRoot`, `readOnlyRootFilesystem`, `allowPrivilegeEscalation=false`, `seccompProfile=RuntimeDefault` und ohne Capabilities laufen.

### 8.5 Datenschutz

#### HSM-NFA-PRIV-001 – Klartext-Verbot in Logs

Logs DÜRFEN NICHT Klartext, Schlüsselmaterial, AAD-Inhalte mit personenbezogenem Bezug oder PINs enthalten.

#### HSM-NFA-PRIV-002 – Doc-ID-Hashing

Doc-IDs SOLLEN vor dem Loggen mit einem service-internen Salt gehasht werden, sofern sie personenbezogene Hinweise enthalten können.

### 8.6 Speicher und Ressourcen

#### HSM-NFA-MEM-001 – Maximale Speichergröße je Replica

Der Service MUSS den Speicherverbrauch je Replica begrenzen. Default-Obergrenze und gültiger Bereich: siehe spec/spezifikation.md.

#### HSM-NFA-MEM-002 – Keine vollständige Dokumentenpufferung

Der Service DARF NICHT Dokumente vollständig im Hauptspeicher halten.

### 8.7 Observability

#### HSM-NFA-OBS-001 – OpenTelemetry und Prometheus

Der Dienst MUSS OpenTelemetry für Traces, Metriken und Logs unterstützen sowie Prometheus-kompatible Metriken bereitstellen.

Konkrete Metriknamen, Label-Konventionen, Pflicht-Spans und Pflicht-Logfelder: siehe spec/spezifikation.md.

### 8.8 Betreibbarkeit

#### HSM-NFA-OPS-001 – 12-Factor-Konfiguration

Konfiguration MUSS über Umgebungsvariablen oder eine validierte YAML-Datei erfolgen. Geheimnisse MÜSSEN aus separaten Secret-Quellen kommen.

#### HSM-NFA-OPS-002 – Graceful Shutdown

`SIGTERM` MUSS einen Graceful Shutdown auslösen: keine neuen Streams annehmen, laufende abschließen bis Timeout, Session-Pool sauber schließen.

#### HSM-NFA-OPS-003 – Probes

Liveness-, Readiness- und Startup-Probes MÜSSEN definiert und im Helm-Chart vorkonfiguriert sein.

### 8.9 Wartbarkeit und Portabilität

#### HSM-NFA-MAINT-001 – Modularität

Der Service MUSS modular aufgebaut sein.

#### HSM-NFA-MAINT-002 – Erweiterbarkeit neuer HSMs

Die Integration weiterer PKCS#11-Module DARF NICHT Codeänderungen außerhalb dünner Adapter erfordern.

#### HSM-NFA-PORT-001 – Linux x86_64

Der Service MUSS auf Linux x86_64 lauffähig sein.

#### HSM-NFA-PORT-002 – Linux ARM64

Der Service SOLL auf Linux ARM64 lauffähig sein.

#### HSM-NFA-PORT-003 – Container-Standard

Container-Images MÜSSEN OCI-konform sein.

---

## 9. Architekturvorgaben

### HSM-ARCH-001 – Hexagonale Architektur

Der Go-Service MUSS einer hexagonalen Architektur folgen. Der Domain-Kern (Stream-Orchestrierung, Chunking, Container-Codec) DARF NICHT direkt von PKCS#11-, gRPC- oder Storage-Bibliotheken abhängen.

### HSM-ARCH-002 – Java-Abstraktion

Die Java-Bibliothek DARF NICHT direkt PKCS#11, JNI oder native Krypto-Libraries einbinden.

### HSM-PRINC-001 – SOLID

Die Implementierung MUSS nach SOLID-Prinzipien erfolgen; Reviews und ADRs dokumentieren Entscheidungen.

### HSM-PRINC-002 – Kleine Schnittstellen

Adapter (PKCS#11, gRPC, Audit, Metrics) MÜSSEN je eine kleine, fachlich getrennte Schnittstelle exponieren.

### HSM-PRINC-003 – Explizite Fehlerbehandlung

Fehler MÜSSEN typisiert und klassifiziert sein; sie DÜRFEN NICHT stillschweigend verschluckt werden.

Konkrete Worker-Pool-, Session-Pool- und Backpressure-Realisierung sowie Code-Conventions: siehe spec/spezifikation.md.

---

## 10. Technologievorgaben

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

- Telemetrie: OpenTelemetry SDK (Go und Java) (MUSS).
- Metriken: Prometheus-Client-Bibliotheken (MUSS).
- Container-Runtime: OCI-konform (MUSS).

Go-Bindings, konkrete Bibliotheken und Versionen: siehe spec/spezifikation.md.

### HSM-TECH-006 – HSM-Profile

Folgende HSMs MÜSSEN unterstützt werden:

- SoftHSM v2 (Funktional-Referenz),
- Utimaco CryptoServer (Produktionsprofil A),
- Thales Luna HSM (Produktionsprofil B).

Akzeptanz: Für jedes Profil existiert eine Konfigurationsvorlage und ein dokumentierter Smoke-Test.

---

## 11. Umgebungs- und Betriebsanforderungen

### HSM-ENV-001 – Containerfähigkeit

Der Service MUSS als Container-Image ausgeliefert werden.

### HSM-ENV-002 – Kubernetes

Das Deployment MUSS Kubernetes-kompatibel sein; ein Helm-Chart MUSS im Repository liegen. Kubernetes ist einer von mehreren unterstützten Betriebsmodi (siehe `HSM-ENV-004`); die Kubernetes-Kompatibilität ist eine Auslieferungseigenschaft und DARF KEINE Voraussetzung für die funktionalen und sicherheitsrelevanten Eigenschaften des Service sein.

### HSM-ENV-003 – Lokale Entwicklung

Für lokale Entwicklung MUSS SoftHSM v2 unterstützt werden; ein `docker-compose.dev.yml` MUSS Service und SoftHSM startfähig kombinieren.

### HSM-ENV-004 – Plattform-Neutralität

Der Service MUSS in vier Betriebsmodi lauffähig sein und seine funktionalen sowie sicherheitsrelevanten Eigenschaften (mTLS, Audit-`caller`, `tenant_id`, Health-/Ready-Probes, Prometheus-Metriken, OTel-Export) in allen vier Modi gleichwertig erfüllen:

1. **Bare-Container** (Docker/Podman/containerd) ohne Orchestrator,
2. **Kubernetes ohne Service Mesh**,
3. **Kubernetes mit L4-Passthrough-Mesh** (z. B. Istio Ambient ztunnel-only, Linkerd ohne mTLS-Termination),
4. **Kubernetes mit L7-/mTLS-terminierendem Mesh** (z. B. Istio mit `PeerAuthentication STRICT`, Linkerd-Proxy mit Mesh-mTLS).

Helm-Chart, NetworkPolicies und Mesh-Konfiguration sind Auslieferungsartefakte, nicht Laufzeitvoraussetzungen. Der Service DARF KEINE Funktionalität, Sicherheitsgarantie oder Identitätsableitung ausschließlich vom Vorhandensein eines Orchestrators oder Service Mesh abhängig machen.

In Modi 1–3 sieht der Server das originale Client-Zertifikat; die Identitätsableitung folgt `HSM-API-GRPC-003`. In Modus 4 terminiert das Mesh den TLS-Handshake; die Identitätsableitung folgt der Header-Variante aus `HSM-API-GRPC-003` und ist nur mit konfigurierter Peer-Vertrauensprüfung zulässig.

### HSM-OPS-MON-001 – Prometheus

Der Dienst MUSS Prometheus-kompatible Metriken bereitstellen.

### HSM-OPS-HC-001 – Probes

Liveness, Readiness und Startup MÜSSEN als Probes bereitgestellt werden.

### HSM-OPS-CFG-001 – Externe Konfiguration

Alle HSM-, Queue-, Worker-, Pool-, TLS-, Audit- und Telemetrieparameter MÜSSEN extern konfigurierbar sein.

### HSM-OPS-CFG-002 – Konfigurations-Validierung

Konfigurationsfehler MÜSSEN beim Start mit eindeutiger Fehlermeldung erkannt werden; der Service DARF NICHT mit ungültiger Konfiguration starten.

---

## 12. Compliance

### HSM-COMP-001 – BSI-Vorgaben

Die kryptografischen Verfahren MÜSSEN BSI TR-02102-1 (aktuelle Fassung) entsprechen.

### HSM-COMP-002 – TLS-Konfiguration

Die TLS-Konfiguration MUSS BSI TR-03116-4 entsprechen.

### HSM-COMP-003 – DSGVO

Die Architektur SOLL technische und organisatorische Maßnahmen gemäß DSGVO Art. 32 belegen, insbesondere Verschlüsselung ruhender Daten und Schlüsseltrennung.

### HSM-COMP-004 – HSM-Zertifizierung

Eingesetzte produktive HSMs SOLLEN FIPS 140-3 Level 3 oder Common Criteria EAL4+ zertifiziert sein.

### HSM-COMP-005 – Audit-Aufbewahrung

Die Aufbewahrungsdauer von Audit-Logs MUSS so wählbar sein, dass branchenspezifische Anforderungen (GoBD, MaRisk, AO §147) erfüllt werden können.

---

## 13. Bedrohungsmodell

Dieses Kapitel skizziert ein orientierendes Threat Model nach Art von STRIDE; eine ausführliche, mit dem Sicherheitsbeauftragten abgestimmte Variante MUSS im Sicherheitskonzept des Projekts entstehen.

### HSM-THREAT-001 – Scope und Vertrauensanker

Innerhalb des Vertrauensankers: HSM (physisch geschützt, zertifiziert), HSM-PIN aus Secret-Store, gepflegte TLS-PKI, Audit-Verankerungssenke.

Außerhalb des Vertrauensankers: jeder Service-Prozess, Container-Image vor Signaturprüfung, jeder Cluster-Knoten, jedes Klartext-Backend, jeder Client-Aufrufer.

### HSM-THREAT-002 – Insider mit Cluster-Zugriff

Bedrohung: Ein Insider mit Kubernetes-Cluster-Admin-Rechten kann Pods exec'en, Secrets lesen, Sidecars injizieren.

Mitigation: HSM-PIN aus separat berechtigtem Secret-Store, `readOnlyRootFilesystem`, keine Shell im Image, organisatorisches 4-Augen-Prinzip für Secret-Zugriffe, RBAC-Trennung Plattform/Crypto-Officer. Restrisiko: Cluster-Admin kann Pod-Identität imitieren; nur durch HSM-seitige Bindung an Pod-Attestierung (z. B. SPIFFE/SPIRE) weiter reduzierbar.

### HSM-THREAT-003 – Kompromittierter Client

Bedrohung: Ein aufrufender Backend-Dienst wurde übernommen und ruft Encrypt/Decrypt mit fremden Doc-IDs oder Mandantenkontext auf.

Mitigation: mTLS mit Per-Service-Identität, `tenant_id` aus mTLS-Subject, Quotas, Audit-Sichtbarkeit, Anomalie-Erkennung.

### HSM-THREAT-004 – Replay verschlüsselter Container

Bedrohung: Ein Angreifer spielt einen alten Container erneut in den Speicher des aufrufenden Systems ein.

Mitigation: AAD im Header bindet Container an Doc-ID, Mandant und Versionskette; die aufrufende Anwendung MUSS die Bindung serverseitig prüfen. Der Dienst selbst kann Replay nicht erkennen, weil er stateless ist.

### HSM-THREAT-005 – Queue/Resource Exhaustion (DoS)

Bedrohung: Ein Angreifer öffnet sehr viele parallele Streams oder sendet Pseudo-Klartext mit künstlich kleiner Chunk-Konfiguration.

Mitigation: Queue-Limits, Tenant-Quotas, Chunkgröße-Validierung, Ingress-Rate-Limits.

### HSM-THREAT-006 – HSM-DoS

Bedrohung: Aggressive Aufrufer treiben die HSM-Session- oder Operationskapazität ans Limit.

Mitigation: Fair Scheduling, Backpressure, Circuit Breaker, Capacity-Planning gegen HSM-Datenblatt.

### HSM-THREAT-007 – Memory Scraping

Bedrohung: Ein Angreifer mit Speicherzugriff auf den Service-Container liest Klartext-Chunks oder Buffer aus.

Mitigation: minimale Pufferzeit, explizites Überschreiben sensibler Buffer, `readOnlyRootFilesystem`, keine Coredumps, Pod-Härtung. Restrisiko: vollständige Memory-Scrubs sind in Go nicht garantiert.

### HSM-THREAT-008 – Node Compromise

Bedrohung: Ein Cluster-Knoten ist kompromittiert; der Angreifer hat root-Zugriff.

Mitigation: dediziertes NodePool mit erhöhter Härtung, mTLS, kurze Session-Lifetimes, regelmäßige Knoten-Rotation. Restrisiko: nur durch Confidential-Compute-Ansätze (AMD SEV-SNP, Intel TDX) weiter reduzierbar.

### HSM-THREAT-009 – Audit-Manipulation

Bedrohung: Ein Angreifer mit Schreibrecht auf den Audit-Sink versucht, Einträge zu manipulieren oder die Datei vollständig neu zu schreiben.

Mitigation: Hash-Chain, Segmentsignatur, externe Verankerung, Chain-Rotation, vertrauenswürdige Zeitquelle.

### HSM-THREAT-010 – Supply Chain

Bedrohung: Kompromittierte Dependency injiziert bösartigen Code.

Mitigation: SBOM, Image-Signierung, Pinning aller Abhängigkeiten, Verifikation der Vendor-Module beim Start, reproducible builds als Ziel.

---

## 14. Risiken

### HSM-RISK-001 – HSM-Kapazitätsgrenzen

HSMs besitzen begrenzte Session- und Durchsatzkapazitäten.

Mitigation: Session-Pool-Konfiguration, Backpressure, horizontale Skalierung, Lasttests gegen Zielhardware vor Produktivnahme.

### HSM-RISK-002 – PKCS#11-Herstellerunterschiede

PKCS#11-Implementierungen unterscheiden sich erheblich.

Mitigation: pro Hersteller ein Adapter-Profil, eigene Smoke-Test-Suite je Profil, Mapping-Tabelle für Fehlercodes.

### HSM-RISK-003 – Netzwerk-HSM-Latenz

Netzwerk-HSMs verursachen zusätzliche Latenzen.

Mitigation: konfigurierbare Chunkgröße, parallele Streams, Profil-spezifische Performance-Ziele.

### HSM-RISK-004 – Schlüsselverlust

Verlust eines HSM-Schlüssels macht die damit verschlüsselten Dokumente unwiederbringlich unbrauchbar.

Mitigation: HSM-spezifische, herstellergeprüfte Backup-Verfahren (M-of-N-Wrap, Cloning); diese sind NICHT Bestandteil dieses Dienstes (HSM-NONGOAL-001), MÜSSEN aber im Betriebskonzept dokumentiert sein.

### HSM-RISK-005 – PIN-Leakage

Eine geleakte HSM-PIN ermöglicht missbräuchliche Nutzung des HSM.

Mitigation: PIN aus Secret-Store, kein PIN in Logs/Images, Rotationsprozess.

### HSM-RISK-006 – Replay verschlüsselter Container

Ein Angreifer könnte einen vollständigen Container wiedereinspielen.

Mitigation: AAD enthält anwendungsspezifische Kontextinformation; aufrufende Anwendung MUSS die Bindung von Container an Doc-ID prüfen.

---

## 15. Annahmen

### HSM-ASSUMP-001 – HSM verfügbar

Es wird angenommen, dass mindestens ein PKCS#11-fähiges HSM bereitsteht und vom Crypto-Officer initialisiert wurde.

### HSM-ASSUMP-002 – Netzwerkkonnektivität

Es wird angenommen, dass zwischen Service und HSM eine stabile Verbindung mit RTT < 5 ms vorliegt; für Netzwerk-HSMs gilt der dokumentierte Wertebereich.

### HSM-ASSUMP-003 – Time Source

Es wird angenommen, dass alle Replicas eine vertrauenswürdige, NTP-synchronisierte Zeitquelle nutzen.

### HSM-ASSUMP-004 – Aufrufer authentifiziert

Es wird angenommen, dass aufrufende Backend-Dienste über mTLS oder einen vorgelagerten Token-Issuer authentifiziert sind.

---

## 16. Abnahmekriterien

### HSM-ACCEPT-001 – Funktionale Abnahme

Das Demo-Skript verschlüsselt und entschlüsselt eine 1-GiB-Datei gegen SoftHSM mit identischer SHA-256-Summe.

### HSM-ACCEPT-002 – Performance-Abnahme

Das Benchmark-Skript erreicht die Werte aus HSM-NFA-PERF-001 und HSM-NFA-PERF-003 in der Referenzumgebung; für jedes Produktionsprofil liegt ein eigenes Messprotokoll vor (siehe HSM-LESE-006).

### HSM-ACCEPT-003 – Security-Abnahme

mTLS-Reject-Test (`HSM-API-GRPC-003`) wird in **Modus 1** (Bare-Container) und **Modus 2** (Kubernetes ohne Mesh) aus `HSM-ENV-004` ausgeführt und schlägt für Clients ohne gültiges Zertifikat in beiden Modi mit `UNAUTHENTICATED` fehl. Für **Modus 4** (mesh-terminiertes mTLS) wird ein zweiter Test mit Identitätsquelle `header` ausgeführt: Anfragen von nicht in der Peer-Allowlist eingetragenen Quellen werden mit `UNAUTHENTICATED` abgewiesen; bei vertrauenswürdigem Peer erscheint die Header-Identität als `caller` im Audit-Log. PIN-Scan über Image und Logs ist negativ; SBOM und Image-Signatur liegen vor.

### HSM-ACCEPT-004 – Audit-Abnahme

Audit-Verifikationstool meldet Manipulation an einem geänderten Audit-Eintrag; Export im definierten Format liegt vor; externe Verankerung ist je nach Konfiguration nachweisbar.

### HSM-ACCEPT-005 – Betriebsabnahme

In **Modus 1** (Bare-Container, `HSM-ENV-004`) startet der Service über `docker run`/`podman run`; Liveness, Readiness und Startup sind über die HTTP-Endpoints grün; Prometheus-Endpoint liefert die Pflichtmetriken. In **Modus 2** (Kubernetes ohne Mesh) deployed das Helm-Chart erfolgreich auf einem Kind-Cluster; Liveness-, Readiness- und Startup-Probes sind grün; Prometheus-Endpoint liefert die Pflichtmetriken.

### HSM-ACCEPT-006 – Compliance-Abnahme

Konfiguration belegt TLS 1.3, AES-256-GCM, BSI-TR-02102-konforme Cipher-Suites; Datenschutz-Stichprobe an Logs zeigt keine PII-Klartexte; für jedes Produktionsprofil liegen die geforderten FIPS-/CC-Nachweise vor.

---

## 17. Mengengerüst

### HSM-MENGE-001 – Lastannahmen MVP

Für den MVP wird folgendes Mengengerüst angenommen:

- bis zu 50 aufrufende Backend-Dienste,
- bis zu 64 parallele Streams je Replica,
- typische Dokumentgröße 100 KiB bis 100 MiB, maximal 10 GiB,
- bis zu 100.000 Encrypt-Operationen pro Tag und Replica.

### HSM-MENGE-002 – Schlüsselanzahl

Es wird angenommen, dass typische Installationen 1 bis 100 logische Schlüssel verwalten. Skalierung auf > 1.000 Schlüssel ist KEIN MVP-Ziel.

---

## 18. Glossar

### HSM-GLOSS-001 – Begriffe

| Begriff             | Bedeutung                                                                                  |
| ------------------- | ------------------------------------------------------------------------------------------ |
| HSM                 | Hardware Security Module                                                                   |
| PKCS#11             | Standardisierte Krypto-API für HSMs und Tokens (OASIS)                                     |
| AES-GCM             | AES im Galois/Counter Mode mit Authentication-Tag                                          |
| AAD                 | Additional Authenticated Data                                                              |
| Nonce               | Pro Verschlüsselungsoperation einmaliger Initialisierungsvektor                            |
| Tag                 | Authentication-Tag der AES-GCM-Operation                                                   |
| Chunk               | Fester Block des Streams, einzeln verschlüsselt mit eigenem Tag                            |
| Container           | Vollständiger verschlüsselter Datenstrom: Header + Chunks + Trailer                        |
| Session             | Aktive PKCS#11-Verbindung zu einem Token                                                   |
| Backpressure        | Mechanismus zur Lastdrosselung                                                              |
| mTLS                | Mutual TLS, bidirektionale Zertifikatsprüfung                                              |
| Crypto-Officer      | Rolle, die HSM-Schlüssel und PINs administriert                                            |
| Tenant              | Mandant; logische Trennungseinheit innerhalb des Dienstes                                  |

---

## 19. Referenzen

### HSM-REF-001 – Normen und Standards

- NIST SP 800-38D – Galois/Counter Mode of Operation
- NIST SP 800-57 – Recommendation for Key Management
- OASIS PKCS#11 Cryptographic Token Interface Base Specification v2.40 / v3.0
- RFC 8446 – TLS 1.3
- RFC 5116 – AEAD-Interfaces
- RFC 3161 – Time-Stamp Protocol
- BSI TR-02102-1 – Kryptographische Verfahren
- BSI TR-03116-4 – Kryptographische Vorgaben für TLS
- BSI TR-03125 (TR-ESOR) – Beweiswerterhaltung
- DSGVO Art. 32 – Sicherheit der Verarbeitung
- FIPS 140-3 – Security Requirements for Cryptographic Modules

### HSM-REF-002 – Begleitdokumente

- [spec/spezifikation.md](spezifikation.md) – Technische Spezifikation (verbindlich für Implementierung, nicht abnahmebindend)
