# Technische Spezifikation – `c-hsm-doc`

| Dokument         | Technische Spezifikation                                                       |
| ---------------- | ------------------------------------------------------------------------------ |
| Projektname      | `c-hsm-doc`                                                                    |
| Kurzbeschreibung | Implementierungs- und Verfahrensvorgaben für den HSM-Service und Java-Client   |
| Version          | 0.2                                                                            |
| Status           | Entwurf                                                                        |
| Datum            | 2026-05-26                                                                     |
| Begleitdokument  | [spec/lastenheft.md](lastenheft.md) – Lastenheft (vertraglich abnahmebindend)  |

---

## 0. Lesehinweise

### HSM-SP-LESE-001 – Verhältnis zum Lastenheft

Dieses Dokument konkretisiert die im Lastenheft formulierten WAS-Anforderungen um das WIE: Algorithmen, Datenstrukturen, Protokoll-Codes, Retry-Strategien, Metriknamen, Container-Layout, Audit-Mechanik.

Es gilt:

- Anforderungen in diesem Dokument sind technisch verbindlich für die Implementierung, aber **nicht vertraglich abnahmebindend**.
- Anforderungen können fortgeschrieben werden, solange sie keine Lastenheft-Anforderung verletzen. Solche Änderungen sind keine Lastenheftänderung und benötigen keinen vertraglichen Change Request.
- Im Konfliktfall hat das Lastenheft Vorrang.

### HSM-SP-LESE-002 – Modalverben und ID-Schema

Modalverben (`MUSS`, `SOLLTE`, `KANN`, `DARF NICHT`) folgen der Definition in HSM-LESE-001 des Lastenhefts.

IDs folgen dem Muster `HSM-<Bereich>-<NNN>` und teilen sich den ID-Raum mit dem Lastenheft. Eine ID lebt in genau einem Dokument; Cross-Referenzen funktionieren über beide Dokumente hinweg.

Bereiche, die in diesem Dokument verwendet werden, umfassen:

`SP-LESE`,
`FA-ENC` (ab 004), `FA-DEC` (ab 003), `FA-CHUNK` (ab 004), `FA-STREAM` (ab 003), `FA-HSM` (ab 004), `FA-KEY` (ab 006), `FA-QUEUE` (ab 002), `FA-RETRY` (ab 003 – Detail), `FA-AUDIT` (ab 006), `FA-TENANT` (ab 005), `FA-FAIL` (komplett),
`API-JAVA` (ab 002), `API-GRPC` (ab 004 – Detail), `API-P11` (ab 002),
`FMT`, `DATA`,
`NFA-OBS` (ab 002), `NFA-MEM` (Detail),
`ARCH` (Detail), `CC`.

---

## 1. Container-Format

### HSM-FMT-001 – Container-Header

Der verschlüsselte Container MUSS mit einem Header beginnen, der mindestens enthält:

- `magic` (4 Byte): `0x48 0x53 0x4D 0x43` (`"HSMC"`)
- `version` (1 Byte): aktuell `0x01`
- `cipher` (1 Byte): `0x01` für AES-256-GCM
- `chunk_size` (4 Byte, BE): konfigurierte Chunkgröße
- `tenant_id_hash` (16 Byte): SHA-256-gekürzter Hash der `tenant_id`
- `key_id` (16 Byte): UUID des logischen Schlüssels
- `key_version` (4 Byte, BE)
- `stream_id` (16 Byte): UUIDv4 gemäß HSM-DATA-004
- `header_aad_len` (2 Byte, BE) und optionale anwendungsspezifische AAD
- `header_hmac` (32 Byte): HMAC-SHA-256 über alle vorherigen Header-Felder, mit Header-Key gemäß HSM-FMT-006

Akzeptanz: Schemadokumentation, Encoder/Decoder und Roundtrip-Test liegen vor.

### HSM-FMT-002 – Chunk-Frame

Jeder Chunk MUSS folgendes Frame-Layout besitzen:

- `seq` (8 Byte, BE)
- `nonce` (12 Byte, gemäß HSM-FA-ENC-004)
- `ciphertext_len` (4 Byte, BE)
- `ciphertext` (`ciphertext_len` Byte)
- `tag` (16 Byte, GCM-Tag)

### HSM-FMT-003 – Trailer

Der Container MUSS mit einem Trailer enden, der `total_chunks` (8 Byte, BE) und `final_marker` (`0xFF`) enthält. Fehlt der Trailer beim Entschlüsseln, MUSS der Stream als unvollständig abgelehnt werden.

### HSM-FMT-004 – Versionierung

Das Format MUSS versioniert sein. Der Service MUSS unbekannte `version`-Werte beim Entschlüsseln mit `FAILED_PRECONDITION` und Fehlerklasse `UNSUPPORTED_FORMAT_VERSION` ablehnen.

### HSM-FMT-005 – Endianness

Alle Mehrbyte-Ganzzahlen im Container MÜSSEN Big-Endian codiert sein.

### HSM-FMT-006 – Ableitung des Header-Key

Der `header_hmac`-Schlüssel (im Folgenden „Header-Key") MUSS deterministisch über HKDF-SHA-256 aus einem im HSM verwahrten Master-HMAC-Schlüssel je logischer `key_id` abgeleitet werden:

```text
header_key = HKDF-SHA-256(
    ikm  = HSM-resident master HMAC key (CKM_GENERIC_SECRET_KEY_GEN,
                                         CKA_EXTRACTABLE = false),
    salt = key_id || key_version,
    info = "c-hsm-doc/header-hmac/v1",
    L    = 32
)
```

Der Master-HMAC-Schlüssel MUSS pro logischer `key_id` existieren, im HSM verwahrt und nicht extrahierbar sein. Bei Schlüsselrotation MUSS auch der Master-HMAC-Schlüssel neu erzeugt und die neue `key_version` einbezogen werden.

Die HKDF-Ableitung MUSS ohne Klartext-Export des Master-Materials erfolgen. PKCS#11-Unterstützung für HKDF ist herstellerabhängig und inkonsistent; daher gelten folgende, in Reihenfolge bevorzugte Profile, die je HSM-Profil (HSM-TECH-006 des Lastenhefts) zu prüfen sind:

1. **Profil A – natives HKDF**: `CKM_HKDF_DERIVE` mit `salt` und `info` über `CK_HKDF_PARAMS`. Falls vom HSM unterstützt, MUSS dieses Profil verwendet werden.
2. **Profil B – HMAC-Konstruktion**: HKDF wird als zwei HMAC-SHA-256-Schritte im HSM nachgebaut (Extract: `HMAC(salt, ikm)`; Expand: `HMAC(prk, info || 0x01)`), realisiert über `CKM_SHA256_HMAC` auf dem nicht-extrahierbaren Master-Key. Das resultierende Handle wird als nicht-extrahierbares `CKM_GENERIC_SECRET` importiert oder durch eine zweite HMAC-Operation direkt als Header-HMAC verwendet, sodass weder PRK noch Header-Key das HSM verlassen.
3. **Profil C – Vendor-Mechanismus**: Vendor-spezifischer KDF-Mechanismus (z. B. Thales- oder Utimaco-eigener KDF), sofern er äquivalente Sicherheitseigenschaften und nicht-extrahierbare Ausgabe garantiert. Vendor-Wahl, Mechanismus-OID und Eignungsnachweis MÜSSEN im jeweiligen HSM-Profil-Dokument festgehalten sein.

Ein HSM-Profil, das keines dieser drei Profile sicher unterstützt, ist nicht freigegeben.

Akzeptanz: Der Header-Key verlässt das HSM nie; der Code verwendet ausschließlich PKCS#11-Operationen über die abgeleiteten Handles. Ein Versuch, den Master-HMAC-Schlüssel oder den Header-Key zu exportieren, schlägt mit `CKR_KEY_UNEXTRACTABLE` fehl. Für jedes freigegebene HSM-Profil ist im Repository dokumentiert, welches der drei HKDF-Profile aktiv ist und mit welchem Smoke-Test es validiert wird.

---

## 2. Datenmodell

### HSM-DATA-001 – Audit-Eintrag

Audit-Einträge MÜSSEN ein eindeutig versioniertes Schema haben mit Pflichtfeldern: `timestamp` (UTC, RFC 3339), `operation` (`encrypt`/`decrypt`/`key-lookup`/`key-rotate`/`error`), `key_id`, `key_version`, `doc_id`, `caller` (Identitätsstring gemäß `HSM-API-GRPC-008`), `tenant_id`, `result` (`ok`/`error`), `error_class`, `attempt` (Versuchszähler), `chunk_count`, `total_bytes`, `request_id`, `trace_id`, `stream_id`.

Optionale Felder sind im Schema markiert.

### HSM-DATA-002 – Key-Info

`KeyInfo` MUSS mindestens `keyId`, `keyVersion`, `status`, `algorithm`, `createdAt`, `rotatedAt`, `usageCount`, `usageLimit` enthalten und DARF NICHT Schlüsselmaterial enthalten.

### HSM-DATA-003 – Health-Status

`HealthResponse` MUSS `serviceStatus` (`UP`/`DEGRADED`/`DOWN`), `hsmStatus` (`UP`/`DEGRADED`/`DOWN`), `sessionsActive`, `sessionsMax`, `queueDepth`, `queueCapacity`, `circuitBreakerState` enthalten.

### HSM-DATA-004 – Stream-ID

Jeder Encrypt-Stream MUSS eine `stream_id` führen, die:

- als UUIDv4 vom Service beim Annehmen des Streams erzeugt wird,
- innerhalb des Service global eindeutig ist,
- mandantengebunden ist (`tenant_id` + `stream_id` bildet den fachlichen Schlüssel; cross-tenant-Verwendung ist verboten),
- in den Pro-Chunk-AAD und in den Container-Header einfließt,
- in jedem Audit-Eintrag, in Tracing-Spans und in Strukturierten Logs erscheint.

Die `stream_id` DARF NICHT als Nonce-Material wiederverwendet werden; die Nonce-Strategie folgt ausschließlich HSM-FA-ENC-004.

Akzeptanz: Jeder Stream im Roundtrip-Test trägt eine eindeutige `stream_id`; ein Chunk eines Streams scheitert bei der Tag-Verifikation, wenn er im Container-Header eines anderen Streams platziert wird.

---

## 3. Stream-Verarbeitung und Chunk-Lebenszyklus

### HSM-FA-ENC-004 – Eindeutige Nonces

Der Dienst MUSS für jede AES-GCM-Operation eine eindeutige 96-Bit-Nonce erzeugen. Die Nonce MUSS aus einem 32-Bit-Random-Prefix und einem monoton steigenden 64-Bit-Zähler je Schlüssel und Stream bestehen oder vollständig kryptografisch zufällig sein.

Akzeptanz: Statistischer Test über 10⁶ Nonces einer Session zeigt keine Kollision; Zähler-Reset bei Restart wird durch das Prefix verhindert.

### HSM-FA-ENC-005 – Authenticated Additional Data

Der Dienst MUSS Additional Authenticated Data (AAD) je Stream unterstützen. Der Container-Header (HSM-FMT-001) MUSS in die AAD jedes Chunks einfließen. Pro-Chunk-AAD MUSS zusätzlich `key_id`, `key_version`, `seq` und `stream_id` enthalten, sodass ein Chunk außerhalb seines Streams nicht erfolgreich entschlüsselt werden kann.

Akzeptanz: Manipulation des Container-Headers oder der Pro-Chunk-AAD-Felder nach der Verschlüsselung führt beim Entschlüsseln zu `CKR_GENERAL_ERROR` bzw. einer Tag-Verifikations-Fehlermeldung. Ein in einen anderen Container kopierter Chunk schlägt bei der Tag-Verifikation fehl.

### HSM-FA-ENC-006 – AEAD-Granularität pro Chunk

Jeder Chunk MUSS eine eigenständige AES-GCM-Operation mit eigenem 96-Bit-Nonce und eigenem 128-Bit-Authentication-Tag darstellen. Ein durchgehender (Multipart-)GCM-Stream über mehrere Chunks oder mehrere PKCS#11-Calls DARF NICHT verwendet werden.

Begründung: Stream-übergreifendes GCM bindet den Tag an die Gesamtlänge und macht streamingbasierte Cancellation, Retry und parallele Chunk-Verarbeitung sicherheitsrelevant fehleranfällig.

Akzeptanz: Codepfad führt je Chunk genau einen `C_EncryptInit`/`C_Encrypt`-Aufruf (oder Vendor-Äquivalent) aus; eine Code-Inspektion und ein PKCS#11-Trace-Test belegen, dass keine `C_EncryptUpdate`-Ketten über Chunk-Grenzen hinweg verwendet werden.

### HSM-FA-DEC-003 – Key-ID-Auflösung

Der Dienst MUSS den zu verwendenden HSM-Schlüssel aus dem Container-Header (Key-ID, Key-Version) auflösen.

Akzeptanz: Unbekannte oder als `destroyed` markierte Key-IDs führen zu gRPC-Status `FAILED_PRECONDITION` mit Fehlerklasse `KEY_NOT_FOUND`.

### HSM-FA-CHUNK-004 – Chunk-Zustandsmodell

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
- `FAILED_TRANSIENT` / `FAILED_PERMANENT`: gemäß HSM-FA-RETRY-001 des Lastenhefts.

Akzeptanz: State-Übergänge sind als Enum/Konstanten im Code definiert, Übergangsregeln werden durch Unit-Tests abgedeckt, und jeder Zustandswechsel wird als Tracing-Event mit `chunk.seq` und `stream_id` exportiert.

### HSM-FA-CHUNK-005 – Parallele Verarbeitung und Reordering

Chunks DÜRFEN parallel im Worker-Pool verarbeitet werden und DÜRFEN out-of-order in den Zustand `SEALED` wechseln. Die Emission (`SEALED → EMITTED`) MUSS jedoch strikt in `seq`-Reihenfolge erfolgen.

Akzeptanz: Ein Reorder-Buffer puffert frühe SEALED-Chunks bis ihr direkter Vorgänger emittiert ist; die Puffertiefe entspricht maximal der Worker-Pool-Größe und wird als Metrik `hsmdoc_reorder_buffer_depth` exportiert.

### HSM-FA-CHUNK-006 – Retry-Semantik

Ein Chunk im Zustand `FAILED_TRANSIENT` MUSS mit identischer Sequenznummer und identischem Klartext-Inhalt wiederholt werden. Bei jedem Retry MUSS eine neue Nonce erzeugt werden (siehe HSM-FA-ENC-004); die vorherige Nonce DARF NICHT wiederverwendet werden.

Akzeptanz: Ein erzwungener Retry-Test zeigt monoton steigende Nonces für denselben `seq` und identischen entschlüsselten Klartext.

### HSM-FA-CHUNK-007 – Commit-Semantik

Die fachliche Wirkung der Chunk-Verarbeitung wird durch genau definierte Commit-Punkte beschrieben. Ein Chunk durchläuft folgende Commits:

| Commit                | Bedingung                                                            | Beobachtbar als                          |
| --------------------- | -------------------------------------------------------------------- | ---------------------------------------- |
| `audit-attempt`       | jeder beendete HSM-Versuch (Erfolg oder Fehler) ist in den Audit-Sink mit `attempt`-Zähler geschrieben und gemäß HSM-FA-AUDIT-010 dauerhaft | Audit-Eintrag mit `result=ok` / `error` |
| `emit-commit`         | Ciphertext-Frame ist in den gRPC-Response-Stream geschrieben und vom Transport bestätigt | Sender-Side ACK des HTTP/2-Frames        |
| `stream-final-commit` | Container-Trailer (HSM-FMT-003) ist erfolgreich emittiert und auf Client-Seite gelesen | erfolgreiche Stream-Schließung           |

Folgende Regeln MÜSSEN gelten:

- `emit-commit` darf erst nach `audit-attempt` mit `result=ok` für denselben `(seq, attempt)` erfolgen.
- Bei Stream-Abbruch (Client-Cancel, Netzwerkfehler) vor `stream-final-commit` gilt der gesamte Container als nicht committed; der Aufrufer DARF den partiellen Container NICHT als gültig akzeptieren. Bereits emittierte Chunks bleiben kryptografisch valide, der Container ist jedoch ohne Trailer ungültig.
- Audit-Einträge MÜSSEN auch bei Stream-Abbruch persistiert werden, soweit ihre `audit-attempt`-Bedingung erreicht wurde. Sie dokumentieren den Abbruch zusätzlich mit Fehlerklasse `STREAM_ABORTED`.

Akzeptanz: Ein Stream-Cancel-Test nach 50 von 100 Chunks zeigt: 50 erfolgreiche `audit-attempt`-Einträge, 50 `emit-commit`-Events, kein `stream-final-commit`, ein zusätzlicher Audit-Eintrag `STREAM_ABORTED`; der Java-Client liefert eine Exception statt eines gültigen Containers.

### HSM-FA-STREAM-003 – Flow Control

Der Dienst MUSS HTTP/2-Flow-Control respektieren und beim Erreichen interner Queue-Grenzen den Sender drosseln.

Akzeptanz: Ein Client mit langsamem Empfang verursacht keinen unbegrenzten Speicheraufbau im Service; ein Lasttest mit künstlich gedrosseltem Receiver zeigt stabile Service-Speicherwerte.

### HSM-FA-STREAM-004 – Cancellation

Bei Cancellation eines Streams durch den Client (gRPC `CANCELLED`, Verbindungsabbruch oder lokales Timeout) MUSS der Dienst:

1. binnen ≤ 100 ms keine neuen HSM-Operationen für diesen Stream mehr starten,
2. den Klartext-Reader und den Response-Writer schließen,
3. Reorder-Buffer, Worker-Slots und stream-eigene Puffer freigeben,
4. alle bereits an das HSM übergebenen Operationen entweder regulär beenden lassen oder, wenn der PKCS#11-Adapter eine sichere Abbruchsemantik bietet (z. B. `C_CancelFunction` oder Vendor-Erweiterung), abbrechen.

PKCS#11 garantiert KEIN synchrones Abbrechen laufender HSM-Operationen. Eine im HSM laufende `C_Encrypt`-Operation wird daher ggf. zu Ende geführt; ihr Ergebnis wird verworfen.

Sessions, die nach Abschluss der laufenden Operation in einem undefinierten Zustand verbleiben, MÜSSEN aus dem Session-Pool entfernt und durch eine neu eingerichtete Session ersetzt werden, bevor sie wieder verwendet werden.

Akzeptanz: Cancel-Test bricht 100 parallele Streams ab. Innerhalb von 100 ms werden keine neuen `C_Encrypt`-Aufrufe für diese Streams beobachtet (PKCS#11-Trace). Bereits laufende HSM-Operationen werden binnen ihrer typischen Laufzeit beendet; danach kehren Session- und Worker-Pool-Metriken in den Ruhestand zurück. Sessions, die im Verlauf des Cancels einen Fehlerzustand melden, werden ersetzt und nicht weiterverwendet.

### HSM-FA-STREAM-005 – Wiederaufnahme (KANN)

Der Dienst KANN wiederaufnehmbare Streams unterstützen, sodass nach Verbindungsabbruch der Stream ohne Wiederholung verschlüsselter Chunks fortgesetzt werden kann.

Akzeptanz (falls implementiert): Stream-ID + letzte bestätigte Sequenznummer reichen, um den Stream binnen 5 s fortzusetzen.

### HSM-FA-CHUNK-008 – Default-Chunkgröße und Grenzen

Die Default-Chunkgröße ist 4 MiB; gültiger Bereich ist 64 KiB bis 64 MiB. Konfigurationswerte außerhalb dieses Bereichs verhindern den Start mit einer eindeutigen Fehlermeldung.

---

## 4. PKCS#11-Anbindung

### HSM-FA-HSM-004 – Session-Pool-Konfiguration

Der Service MUSS einen PKCS#11-Session-Pool mit folgenden Parametern bereitstellen:

| Parameter           | Default | Bedeutung                                                |
| ------------------- | ------- | -------------------------------------------------------- |
| `pool.size`         | 8       | maximale Anzahl Sessions je HSM-Quelle                   |
| `pool.maxIdle`      | 4       | Sessions, die im Leerlauf gehalten werden                |
| `pool.maxLifetime`  | 1 h     | maximale Lebensdauer einer Session vor Recycling         |
| `pool.acquireTimeout` | 5 s   | maximale Wartezeit auf eine Session                      |
| `pool.loginRetry`   | 3       | Retries bei Re-Login pro Session                         |

Akzeptanz: Konfigurationsparameter werden im Start-Log angezeigt; ein Lasttest belegt, dass der Pool unter Last keine Sessions verliert.

### HSM-FA-HSM-005 – Mechanismus-Check

Der Dienst MUSS beim Start prüfen, dass das HSM `CKM_AES_GCM` unterstützt. Fehlende Mechanismen MÜSSEN beim Start mit einer eindeutigen Fehlermeldung erkannt werden.

Akzeptanz: Ein HSM ohne `CKM_AES_GCM` führt zu Start-Abbruch mit Hinweis auf den fehlenden Mechanismus.

### HSM-API-P11-002 – Vendor-Modul-Validierung

Der Pfad zum Vendor-Modul (`*.so`/`*.dll`) MUSS über Konfiguration setzbar sein und beim Start validiert werden (Existenz, ELF-Header, `C_GetInfo`).

### HSM-API-P11-003 – Go-Binding

Als Go-Binding MUSS `github.com/miekg/pkcs11` verwendet werden.

---

## 5. Schlüsselverwaltung (Detail)

### HSM-FA-KEY-006 – Konkrete Usage-Limits (AES-GCM)

Der Dienst MUSS die Anzahl der AES-GCM-Verschlüsselungsoperationen je logischer `key_id` mit folgenden Werten begrenzen:

- **Hard Limit** (Default): 2³² Verschlüsselungsoperationen je `key_id`. Beim Erreichen MUSS der Dienst weitere Encrypt-Anfragen für diese `key_id` mit `FAILED_PRECONDITION` und Fehlerklasse `KEY_USAGE_EXHAUSTED` ablehnen, bis eine Rotation gemäß HSM-FA-KEY-003 des Lastenhefts erfolgt ist.
- **Soft Limit** (Default): 2³¹ Operationen (50 % des Hard Limits). Beim Erreichen MUSS eine Metrik `hsmdoc_key_usage_soft_limit_reached_total` inkrementiert und eine Rotation-Warnung im Log ausgegeben werden.
- **Auto-Rotation** (SOLL): Bei aktivierter Auto-Rotation MUSS der Dienst beim Erreichen des Soft Limits eine Rotation selbsttätig auslösen, sofern ein Rotations-Hook konfiguriert ist.

Operationszähler je `key_id` MÜSSEN persistiert oder beim Start aus dem Audit-Log rekonstruiert werden, um Resets bei Service-Restart zu vermeiden.

Begründung: NIST SP 800-38D begrenzt die sichere Verwendung eines AES-GCM-Schlüssels mit zufälligen 96-Bit-Nonces aufgrund der Kollisionsschranke; eine deutlich frühere Rotation ist gängige Praxis.

Akzeptanz: Ein Lasttest gegen einen Test-Key mit künstlich gesenktem Hard Limit (z. B. 1000 Operationen) zeigt: Soft-Limit-Metrik wird ausgelöst, Hard-Limit führt zu Encrypt-Ablehnung, nach Rotation funktioniert Encrypt mit neuer `key_version` wieder; Operationszähler überlebt Restart.

---

## 6. Queue, Backpressure, Retry (Detail)

### HSM-FA-QUEUE-002 – Queue-Tiefe und Backpressure-Signal

Default-Queue-Tiefe: 256 Jobs. Default-Wartezeit vor Ablehnung: 0 ms (sofortige Ablehnung).

Der Dienst MUSS Backpressure über HTTP/2-Flow-Control und über explizite gRPC-Statuscodes signalisieren.

Akzeptanz: Java-Client erkennt `RESOURCE_EXHAUSTED` und exponiert eine `BackpressureException` mit empfohlener Wartezeit.

### HSM-FA-QUEUE-003 – Wartezeit-Strategie

Die Wartezeit, die der Service vor Ablehnung wartet, MUSS konfigurierbar sein.

### HSM-FA-RETRY-003 – Exponential Backoff

Wiederholungen MÜSSEN mit Exponential Backoff und Jitter ausgeführt werden. Default: Basis = 50 ms, Faktor = 2, max. 5 Versuche.

### HSM-FA-RETRY-004 – Commit-Idempotenz (Detail)

Eine Chunk-Verarbeitung gilt erst dann als committed, wenn der Ciphertext erfolgreich in den Response-Stream geschrieben (`SEALED → EMITTED`, siehe HSM-FA-CHUNK-004) und der zugehörige Audit-Eintrag persistiert (siehe HSM-FA-AUDIT-010) wurde. Nur der final erfolgreiche Versuch eines Chunks gilt als committed.

Hinweis: Jeder Retry erzeugt nach HSM-FA-CHUNK-006 zwangsläufig einen neuen Ciphertext und einen neuen Authentication-Tag, weil eine neue Nonce verwendet wird. Idempotenz bezieht sich daher auf das Commit-Ergebnis (fachliche Wirkung), nicht auf die Bytefolge des Ciphertexts.

Folgende Regeln MÜSSEN gelten:

- Für jede Sequenznummer `seq` MUSS das Audit-Log höchstens einen Eintrag mit `result=ok` enthalten.
- Vorausgehende fehlgeschlagene Versuche MÜSSEN als separate Audit-Einträge mit `result=error`, `error_class` und `attempt`-Zähler protokolliert werden.
- Ciphertext eines nicht erfolgreich committeten Versuchs DARF NICHT in den Response-Stream emittiert werden.
- Bei Stream-Abbruch vor `EMITTED` gilt der Chunk als nicht committed und liefert keinen `result=ok`-Eintrag.

Akzeptanz: Nach drei erzwungenen transienten Fehlern gefolgt von einem Erfolg zeigt das Audit-Log für die betroffene `seq` genau einen `result=ok`-Eintrag und drei `result=error`-Einträge mit `attempt=1..3`; im Response-Stream erscheint genau ein Ciphertext-Chunk für diese `seq`.

---

## 7. Audit (Detail)

### HSM-FA-AUDIT-006 – Signatur der Audit-Segmente

Audit-Einträge MÜSSEN in zeitlich oder mengenmäßig begrenzten Segmenten gebündelt werden (Default: alle 5 min oder alle 10 000 Einträge, je nachdem was zuerst eintritt). Jedes abgeschlossene Segment MUSS mit einem im HSM verwahrten Signaturschlüssel signiert werden.

Akzeptanz: Eine Manipulation innerhalb eines abgeschlossenen Segments lässt die Segmentsignatur ungültig werden; das Verify-Tool meldet das betroffene Segment eindeutig.

### HSM-FA-AUDIT-007 – Externe Verankerung

Der Endwert der Hash-Chain SOLL regelmäßig (Default: stündlich) extern verankert werden. Zulässige Verankerungssenken sind mindestens eine der folgenden, konfigurierbar:

- ein zweiter, organisatorisch getrennter Append-only-Log (z. B. SIEM, dediziertes Verankerungs-Repository),
- ein RFC-3161-Zeitstempeldienst (TSA),
- ein Transparency-Log (z. B. Sigstore Rekor).

Akzeptanz: Das Verify-Tool kann anhand des externen Verankerungsbelegs den letzten verankerten Chain-Endwert nachweisen; ein vollständiger Neuschreib der Audit-Datei wird erkannt, weil der neu berechnete Chain-Endwert nicht mit der externen Verankerung übereinstimmt.

### HSM-FA-AUDIT-008 – Chain-Rotation

Die Hash-Chain MUSS rotierbar sein: nach Erreichen einer konfigurierbaren Größe (Default 1 GiB) oder eines Zeitfensters (Default 30 Tage) wird ein neuer Chain-Abschnitt begonnen. Der letzte Hash und die letzte Segmentsignatur des alten Abschnitts MÜSSEN als erster Eintrag des neuen Abschnitts referenziert und unabhängig verankert werden.

Akzeptanz: Nach einer Rotation gibt es einen lückenlosen Verifikationspfad über die Abschnittsgrenze hinweg.

### HSM-FA-AUDIT-009 – Zeitstempel-Details

Audit-Zeitstempel MÜSSEN aus einer vertrauenswürdigen Zeitquelle stammen. Mindestanforderung: NTP-/chrony-synchronisierte Systemzeit mit dokumentierter Drift-Überwachung. Für regulierte Umgebungen SOLL zusätzlich ein RFC-3161-Zeitstempel je signiertem Segment (HSM-FA-AUDIT-006) eingeholt werden.

Akzeptanz: Eine Zeit-Abweichung von > 1 s gegenüber NTP-Quelle löst die Metrik `hsmdoc_time_drift_seconds` aus und wird im Service-Log gemeldet; für jedes signierte Segment der regulierten Konfiguration liegt ein RFC-3161-Token vor.

### HSM-FA-AUDIT-010 – Durability und Schreibreihenfolge

Audit-Einträge MÜSSEN dauerhaft persistiert sein, bevor sie den `audit-attempt`-Commit gemäß HSM-FA-CHUNK-007 erfüllen:

- Schreibvorgänge MÜSSEN append-only und in `seq`-Reihenfolge je Stream erfolgen.
- Die Dauerhaftigkeit MUSS durch eine konfigurierbare Sync-Strategie sichergestellt werden:
  - `per-entry-fsync` (Default für regulierte Umgebungen): jeder Eintrag wird vor Abschluss von `audit-attempt` über `fsync(2)` oder Backend-Äquivalent durabel gemacht.
  - `batched-fsync` (Default für Standardumgebungen): Einträge werden in Gruppen ≤ 100 ms oder ≤ 1000 Einträge gesyncert; `audit-attempt` schließt erst nach Sync ab.
- Bei Sync-Fehler MUSS der zugehörige Stream sofort abgebrochen werden (`STREAM_ABORTED`, Fehlerklasse `AUDIT_DURABILITY_FAILED`); der Service DARF KEINEN Ciphertext-Chunk emittieren, dessen Audit-Eintrag nicht durabel ist.

Performance-Hinweis: Die Kombination `per-entry-fsync` + Netzwerk-HSM + latenzbehaftetes Storage kann die effektive Pro-Chunk-Latenz dominieren. Der Sync addiert sich seriell auf den HSM-Roundtrip; bei rotierender Festplatte als Audit-Sink sind Pro-Chunk-Latenzen jenseits der Zielwerte aus HSM-NFA-PERF-003 des Lastenhefts realistisch. Empfehlung: für regulierte Umgebungen Audit-Sink auf NVMe-Volume oder dedizierten Append-only-Service mit eigener Latenz-SLA legen; für Hochdurchsatzprofile `batched-fsync` mit kleinem Batch-Fenster (≤ 20 ms) wählen.

Akzeptanz: Ein erzwungener Sync-Fehler (z. B. EIO) führt zum dokumentierten Stream-Abbruch; die Metrik `hsmdoc_audit_durability_failed_total` steigt. Die effektive Pro-Chunk-Latenz unter `per-entry-fsync` ist im Abnahmebericht pro HSM-Profil und Storage-Backend dokumentiert.

### HSM-FA-AUDIT-011 – Zulässige Audit-Senken

Zulässig sind als primäre Audit-Senken:

- ein lokales append-only Dateisystem mit Dateirotation (z. B. Mode 0640, Owner = Service-User, eigenes Volume),
- ein Object Storage mit Append- oder Versioning-Funktion und Write-Once-Read-Many-Semantik (z. B. S3 Object Lock im Compliance-Mode),
- ein SIEM- oder Log-Ingestion-System mit nachweislicher Append-only-Garantie.

Die externe Verankerungssenke (HSM-FA-AUDIT-007) MUSS organisatorisch und technisch von der primären Senke getrennt sein.

### HSM-FA-AUDIT-012 – Export-Format

Audit-Logs MÜSSEN im JSON-Lines-Format exportierbar sein und ein Begleit-Manifest mit Hash-Chain-Endwert und letzter Segmentsignatur tragen.

---

## 8. Mandantenisolation (Detail)

### HSM-FA-TENANT-005 – Fair Scheduling

Der Worker-Pool MUSS Mandanten fair bedienen. Ein einzelner Mandant DARF NICHT den gesamten Pool oder die gesamte HSM-Session-Kapazität dauerhaft monopolisieren.

Empfohlene Algorithmen: Weighted Fair Queueing (DRR/WFQ) auf Stream-Ebene mit Mandant als Klasse; Mindestgewicht je aktivem Mandant.

Akzeptanz: Ein synthetischer Lasttest mit einem aggressiven Mandanten (Mandant A: ≥ 1000 Streams) und einem moderaten Mandanten (Mandant B: 10 Streams) zeigt für Mandant B eine p99-Latenz, die das p99 ohne A-Last um nicht mehr als Faktor 3 überschreitet.

### HSM-FA-TENANT-006 – Tenant-Metriken

`tenant_id` MUSS in den Tenant-relevanten Metriken/Spans enthalten sein. In Metrik-Labels SOLL ein Hash der `tenant_id` verwendet werden, sofern Klartext-IDs personenbezogen oder geschäftskritisch sind.

---

## 9. HSM Failure Semantics

### HSM-FA-FAIL-003 – Fehlerklassen-Mapping PKCS#11

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
| `CKR_MECHANISM_INVALID`          | `MECHANISM_MISSING`         | Permanenter Konfigurationsfehler                          |
| `CKR_BUFFER_TOO_SMALL`           | `INTERNAL`                  | Programmfehler, Chunk-Abbruch, Bug-Report                 |
| `CKR_DATA_INVALID`/`CKR_ENCRYPTED_DATA_INVALID` | `TAG_MISMATCH` | Stream-Abbruch (siehe HSM-FA-DEC-002 im Lastenheft)      |
| sonstige `CKR_*`                 | `HSM_UNKNOWN`               | als `permanent` behandeln                                 |

Akzeptanz: Die Mapping-Tabelle liegt als Code-Konstante und als Unit-Test-Fixture vor; jeder Eintrag wird durch mindestens einen Test exerziert (Mock-PKCS#11-Modul).

### HSM-FA-FAIL-004 – Session-Lebenszyklus bei Fehlern

Eine Session, die einen Fehler aus den Klassen `SESSION_INVALID`, `HSM_DEVICE_ERROR`, `HSM_FUNCTION_FAILED`, `HSM_GENERAL_ERROR` oder `KEY_HANDLE_STALE` zurückgeliefert hat, MUSS unmittelbar aus dem Pool entfernt und durch eine neu eingerichtete Session ersetzt werden.

Akzeptanz: Ein Fehlertest setzt nacheinander jede dieser Klassen auf einer Session; die Metrik `hsmdoc_sessions_recycled_total` steigt entsprechend; die Sessionanzahl im Pool bleibt stabil.

### HSM-FA-FAIL-005 – Circuit Breaker

Der Dienst MUSS pro HSM-Quelle (Slot/Modul) einen Circuit Breaker bereitstellen. Bei einer konfigurierbaren Fehlerrate (Default ≥ 50 % über ein 30-s-Fenster) öffnet der Breaker, Readiness MUSS auf rot wechseln, neue Streams werden mit `UNAVAILABLE` abgelehnt, bestehende Streams werden abgebrochen.

Akzeptanz: Ein simulierter HSM-Ausfall öffnet den Breaker innerhalb des Fensters; `/readyz` liefert nicht-ready; nach Erholung der HSM-Quelle schließt der Breaker nach einer halben-offenen Probe.

### HSM-FA-FAIL-006 – HSM-Reboot und Token-Removal

Bei `CKR_DEVICE_REMOVED` oder `CKR_TOKEN_NOT_PRESENT` MUSS der Dienst:

1. den Session-Pool für die betroffene Quelle drainen,
2. den Circuit Breaker öffnen,
3. eine Reconnect-Schleife mit Exponential Backoff (Basis 1 s, Faktor 2, Cap 60 s) starten,
4. bei erfolgreichem Reconnect (`C_Initialize` + `C_OpenSession` + `C_Login` + Mechanism-Check) den Pool neu auffüllen.

Akzeptanz: Ein simulierter Token-Remove löst den dokumentierten Ablauf aus; nach Token-Re-Insert ist der Service binnen einer Backoff-Periode wieder ready.

### HSM-FA-FAIL-007 – Netzwerkpartition zum Netzwerk-HSM

Bei Netzwerk-HSMs MUSS der Dienst eine TCP-/Heartbeat-Überwachung der HSM-Verbindung implementieren oder vom Vendor-Modul übernehmen. Ein Timeout MUSS als `HSM_DEVICE_ERROR` behandelt werden und HSM-FA-FAIL-005 auslösen.

Akzeptanz: Ein netemulierter Paketverlust > 80 % über 10 s öffnet den Circuit Breaker; ein nachfolgendes Wiederherstellen schließt ihn.

### HSM-FA-FAIL-008 – Re-Login-Strategie

Bei `CKR_USER_NOT_LOGGED_IN` MUSS der Dienst einen kontrollierten Re-Login durchführen, höchstens mit der konfigurierten Frequenz (Default: max. 1 Re-Login pro Session pro 60 s). Übermäßige Re-Logins MÜSSEN vermieden werden, um HSM-spezifische Lockout-Mechanismen nicht auszulösen.

Akzeptanz: Ein erzwungenes Logout führt zu maximal einem Re-Login innerhalb der Default-Periode; die Metrik `hsmdoc_hsm_relogin_total` zählt Re-Logins pro Slot.

### HSM-FA-FAIL-009 – Readiness-Signal

`/readyz` MUSS den Status `not ready` zurückliefern, solange:

- der Session-Pool weniger als 1 funktionsfähige Session besitzt,
- der Circuit Breaker offen ist,
- der `CKM_AES_GCM`-Check beim letzten Reconnect fehlgeschlagen ist,
- ein permanenter Fehler (`HSM_PIN_INVALID`, `MECHANISM_MISSING`) erkannt wurde.

Akzeptanz: Für jeden dieser Zustände existiert ein automatisierter Test, der `/readyz` als nicht-ready beobachtet, ohne dass Liveness verletzt wird.

---

## 10. Java Client API (Detail)

### HSM-API-JAVA-002 – Konkrete Public API

Die Java-Bibliothek MUSS mindestens folgende API exponieren:

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

### HSM-API-JAVA-003 – Builder-Konfiguration

Die Bibliothek MUSS sich vollständig per Builder konfigurieren lassen (Endpoint, TLS-Material, mTLS-Identität, Timeouts, Retry-Policy, Backpressure-Strategie).

### HSM-API-JAVA-004 – Fehlerklassen

Die Bibliothek MUSS typisierte Exceptions exponieren: `HsmDocException` (Basis), `BackpressureException`, `IntegrityException` (Tag-Mismatch, Hash-Chain-Bruch), `KeyStateException`, `TransientException`, `TenantQuotaException`.

### HSM-API-JAVA-005 – Reactive-Variante

Die Bibliothek SOLL eine reaktive Variante auf Basis von `Flow.Publisher` oder Project Reactor anbieten.

---

## 11. gRPC API (Detail)

### HSM-API-GRPC-004 – Proto-Definition

Die gRPC-Schnittstelle MUSS in Proto3 definiert sein, mit mindestens dem Service:

```proto
service HsmDocService {
  rpc Encrypt(stream EncryptRequest) returns (stream EncryptResponse);
  rpc Decrypt(stream DecryptRequest) returns (stream DecryptResponse);
  rpc ListKeys(ListKeysRequest) returns (ListKeysResponse);
  rpc Health(google.protobuf.Empty) returns (HealthResponse);
}
```

### HSM-API-GRPC-005 – Statuscode-Mapping

Der Service MUSS interne Fehler auf gRPC-Statuscodes mappen. Mindestens:

| Fehlerklasse              | gRPC-Status                |
| ------------------------- | -------------------------- |
| `INVALID_INPUT`           | `INVALID_ARGUMENT`         |
| `QUEUE_FULL`              | `RESOURCE_EXHAUSTED`       |
| `TENANT_QUOTA`            | `RESOURCE_EXHAUSTED`       |
| `HSM_UNAVAILABLE`         | `UNAVAILABLE`              |
| `TAG_MISMATCH`            | `DATA_LOSS`                |
| `KEY_NOT_FOUND`           | `FAILED_PRECONDITION`      |
| `KEY_USAGE_EXHAUSTED`     | `FAILED_PRECONDITION`      |
| `UNSUPPORTED_FORMAT_VERSION` | `FAILED_PRECONDITION`   |
| `UNAUTHENTICATED`         | `UNAUTHENTICATED`          |
| `IDENTITY_MISSING`        | `INTERNAL`                 |
| `AUDIT_DURABILITY_FAILED` | `INTERNAL`                 |
| `INTERNAL`                | `INTERNAL`                 |

### HSM-API-GRPC-006 – Identitätsquelle und Konfigurationsschema

Schärft `HSM-API-GRPC-003` (Lastenheft) für die in `HSM-ENV-004` und [ADR 0003](../docs/plan/adr/0003-plattform-und-mesh-neutralitaet.md) §2.2 festgelegten zwei Identitätsquellen.

Der Server MUSS die Identitätsquelle über folgende Konfigurationsschlüssel exponieren (Pfad orientiert sich am bestehenden Config-Layout; die konkrete Repräsentation — YAML/Env/Flag — folgt dem allgemeinen Mechanismus aus `HSM-OPS-CFG-001..002`):

| Schlüssel                           | Typ           | Default          | Bedeutung                                                                 |
| ----------------------------------- | ------------- | ---------------- | ------------------------------------------------------------------------- |
| `identity.source`                   | enum          | `mtls-subject`   | `mtls-subject` (Modi 1–3) oder `header` (Modus 4)                         |
| `identity.mtls.subject_attribute`   | enum          | `subject_dn`     | `subject_dn` (RFC 4514) oder `san_uri` (erste URI-SAN, z. B. SPIFFE-ID)   |
| `identity.header.name`              | string        | `x-spiffe-id`    | Header, aus dem die Identität gelesen wird (nur bei `source=header`)      |
| `identity.header.format`            | enum          | `spiffe`         | `spiffe` (URI), `xfcc` (Envoy X-Forwarded-Client-Cert), `raw`             |
| `identity.peer.allowlist`           | list<string>  | `[]`             | siehe `HSM-API-GRPC-007`                                                  |

Validierung beim Start (`HSM-OPS-CFG-002`):

- `identity.source=mtls-subject` und kein konfiguriertes Server-Client-CA → harter Start-Abbruch mit `INVALID_INPUT`-Klasse.
- `identity.source=header` und `identity.peer.allowlist` leer oder fehlend → harter Start-Abbruch (Begründung: ADR 0003 §2.3, Schließung von `HSM-THREAT-002`-Eskalation).
- Unbekannter Wert in `identity.source` / `identity.header.format` → harter Start-Abbruch.

Default-Wechsel zwischen den beiden Quellen ist nur über Konfiguration zulässig; Auto-Detection ist verboten.

### HSM-API-GRPC-007 – Peer-Allowlist für `header`-Quelle

Bei `identity.source=header` MUSS der Server für jede eingehende Verbindung prüfen, ob die unmittelbare Peer-Identität gegen die konfigurierte `identity.peer.allowlist` matched, bevor der Identitäts-Header gelesen wird.

Eintragsformate in der Allowlist:

| Form                         | Beispiel                                       | Anwendung                                                |
| ---------------------------- | ---------------------------------------------- | -------------------------------------------------------- |
| `ip:<addr>` oder `cidr:<r>`  | `ip:127.0.0.1`, `ip:::1`, `cidr:10.42.0.0/16`  | Loopback aus Sidecar im selben Pod, Cluster-internes CIDR |
| `spiffe:<id>`                | `spiffe://cluster.local/ns/mesh/sa/ztunnel`    | SPIFFE-ID des Mesh-Sidecars aus transportiertem mTLS     |
| `san-uri:<uri>`              | `san-uri:spiffe://cluster.local/...`           | URI-SAN im Peer-Zertifikat                               |
| `san-dns:<name>`             | `san-dns:istio-proxy.istio-system.svc`         | DNS-SAN im Peer-Zertifikat                               |

Auswertungsreihenfolge:

1. Ist der Peer per `ip:`/`cidr:` zulässig, MUSS der Server zusätzlich prüfen, dass der Transport-Layer dieselbe Maschinen-/Pod-Grenze nicht verletzt (Beispiel: bei `ip:127.0.0.1` MUSS der Listener auf Loopback gebunden sein; bei CIDR-Eintrag MUSS Transport-mTLS aktiv sein).
2. Bei `spiffe:`/`san-*` MUSS der TLS-Handshake mit dem Peer mit gültigem Zertifikat abgeschlossen sein; die SAN-Werte werden gegen die Allowlist verglichen.
3. Bei `match=false` MUSS die Anfrage mit gRPC-Status `UNAUTHENTICATED` abgewiesen werden, **bevor** der Identitäts-Header parsed wird. Audit-Eintrag MUSS mit `result=error`, `error_class=UNAUTHENTICATED`, `caller=anonymous@<peer-addr>` erfolgen.

Metrik `hsmdoc_identity_peer_rejected_total` (Counter, Label: `reason` ∈ {`not_in_allowlist`, `tls_handshake_failed`, `header_missing`, `header_malformed`}) MUSS ergänzt werden (Erweiterung von `HSM-NFA-OBS-003`).

### HSM-API-GRPC-008 – Ableitung des Audit-`caller` pro Identitätsquelle

Das Audit-Feld `caller` (`HSM-DATA-001`) MUSS deterministisch aus der gewählten Quelle abgeleitet werden:

| `identity.source` | Format                       | Beispiel-`caller`-String                              |
| ----------------- | ---------------------------- | ----------------------------------------------------- |
| `mtls-subject` mit `subject_attribute=subject_dn` | RFC 4514 DN  | `CN=svc-billing,OU=apps,O=acme,C=DE`                  |
| `mtls-subject` mit `subject_attribute=san_uri`    | URI          | `spiffe://acme.example/ns/billing/sa/api`             |
| `header` mit `format=spiffe`                      | URI          | `spiffe://acme.example/ns/billing/sa/api`             |
| `header` mit `format=xfcc`                        | RFC 4514 DN aus `Subject="..."`-Feld | `CN=svc-billing,OU=apps,O=acme,C=DE`  |
| `header` mit `format=raw`                         | unveränderter Header-Wert    | (frei, durch Betreiber definiert)                     |

`tenant_id`-Ableitung aus dem `caller`-String folgt `HSM-FA-TENANT-001..002` (Lastenheft) und ist von dieser Spezifikation unabhängig: Sie KANN sowohl per Subject-Attribut, per SPIFFE-Path-Komponente als auch per separat konfigurierter Mapping-Regel erfolgen. Eine konkrete Regelpriorität ist Betreiber-Konfiguration, kein Spec-Pflichtteil.

`caller` DARF NICHT leer sein, wenn die Anfrage akzeptiert wird; ein leerer/fehlender Identitätsstring trotz erfolgreicher Peer-Prüfung MUSS als `INTERNAL` behandelt und im Audit als `error_class=IDENTITY_MISSING` festgehalten werden.

---

## 12. Observability (Detail)

### HSM-NFA-OBS-002 – Strukturierte Logs

Logs MÜSSEN strukturiert in JSON ausgegeben werden. Pflichtfelder: `time`, `level`, `service`, `version`, `request_id`, `trace_id`, `stream_id`, `tenant_id` (oder Hash), `caller`, `message`.

### HSM-NFA-OBS-003 – Pflichtmetriken

Folgende Prometheus-Metriken MÜSSEN exponiert sein:

- `hsmdoc_encrypt_total` (Counter, Labels: `result`, `key_id_hash`, `tenant_id_hash`)
- `hsmdoc_decrypt_total` (Counter, Labels: `result`, `key_id_hash`, `tenant_id_hash`)
- `hsmdoc_chunk_duration_seconds` (Histogram, Label: `operation`)
- `hsmdoc_queue_depth` (Gauge)
- `hsmdoc_sessions_active` (Gauge, Label: `hsm_source`)
- `hsmdoc_sessions_max` (Gauge, Label: `hsm_source`)
- `hsmdoc_sessions_recycled_total` (Counter, Label: `reason`)
- `hsmdoc_reorder_buffer_depth` (Gauge)
- `hsmdoc_errors_total` (Counter, Label: `error_class`)
- `hsmdoc_hsm_up` (Gauge, Label: `hsm_source`)
- `hsmdoc_hsm_relogin_total` (Counter, Label: `hsm_source`)
- `hsmdoc_circuit_breaker_state` (Gauge, Werte 0=closed/1=half-open/2=open)
- `hsmdoc_key_usage_soft_limit_reached_total` (Counter, Label: `key_id_hash`)
- `hsmdoc_audit_durability_failed_total` (Counter)
- `hsmdoc_time_drift_seconds` (Gauge)
- `hsmdoc_tenant_streams_active` (Gauge, Label: `tenant_id_hash`)
- `hsmdoc_tenant_quota_rejections_total` (Counter, Label: `tenant_id_hash`, `quota_type`)
- `hsmdoc_identity_peer_rejected_total` (Counter, Label: `reason` ∈ {`not_in_allowlist`, `tls_handshake_failed`, `header_missing`, `header_malformed`}) — siehe `HSM-API-GRPC-007`

### HSM-NFA-OBS-004 – Tracing-Spans

Jeder Chunk MUSS einen eigenen Span unter dem Stream-Root-Span erzeugen, mit Attributen `chunk.seq`, `chunk.bytes`, `key.id_hash`, `tenant.id_hash`, `stream.id`. Jeder State-Übergang gemäß HSM-FA-CHUNK-004 MUSS als Span-Event erscheinen.

---

## 13. Architektur (Detail)

### HSM-ARCH-003 – Worker-Pool

Encrypt/Decrypt-Verarbeitung MUSS in einem Worker-Pool mit konfigurierbarer Größe laufen. Default: `runtime.NumCPU() * 2`, Bereich 1..512.

### HSM-ARCH-004 – Session-Pool-Adapter

Der PKCS#11-Session-Pool MUSS als eigener Adapter implementiert sein, der Sessions liest/leiht/zurückgibt und Re-Login bei Session-Verlust übernimmt.

### HSM-ARCH-005 – Backpressure als Domain-Konzept

Backpressure MUSS im Domain-Kern als explizites Konzept abgebildet sein und sich nicht auf zufälliges Verhalten von gRPC oder Channels verlassen.

### HSM-CC-001 – Keine zyklischen Modulabhängigkeiten

Module DÜRFEN KEINE zyklischen Importe besitzen. Eine automatisierte Architekturprüfung im CI MUSS Zyklen melden.

### HSM-CC-002 – Speichergrenzen

`GOMEMLIMIT` Default 1 GiB, gültiger Bereich 256 MiB bis 8 GiB.

---

## 14. Referenzen

Siehe HSM-REF-001 im Lastenheft. Zusätzlich relevant für die Implementierung:

- RFC 5869 – HMAC-based Extract-and-Expand Key Derivation Function (HKDF)
- OASIS PKCS#11 Mechanisms Specification (insbesondere `CKM_AES_GCM`, `CKM_HKDF_DERIVE`)
- `github.com/miekg/pkcs11` – Go-Binding
- OpenTelemetry Semantic Conventions
- Prometheus Client Libraries (Go, Java)
