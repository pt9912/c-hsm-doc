# ADR 0007 — Profil B als M1-Header-HMAC-Default und konfigurierbare Profil-Wahl

**Status:** Accepted
**Datum:** 2026-05-28
**Bezug:** [Lastenheft](../../../spec/lastenheft.md) (`HSM-FA-HSM-001`,
`HSM-LESE-004`),
[Spezifikation](../../../spec/spezifikation.md)
(`HSM-FA-HSM-005`, `HSM-FMT-006`),
[ADR 0001](0001-documentation-and-planning-structure.md),
[ADR 0004](0004-runtime-base-cgo-pkcs11.md),
[ADR 0006](0006-hkdf-profil-a-binding-und-bouncy-hsm.md)
(setzt diese ADR fort — schließt die in §2.2 dort dokumentierte
`HSM-FA-HSM-001`-Lücke),
[Spezifikation `HSM-FA-HSM-005`](../../../spec/spezifikation.md)
(geschärft durch §2.4 dieser ADR),
[Slice 002b](../planning/next/002b-pkcs11-encrypt-hexagon.md)

---

## 1. Kontext

[ADR 0006 §2.2](0006-hkdf-profil-a-binding-und-bouncy-hsm.md) hat
nach dem HKDF-Spike vier Lösungspfade für die offene
`HSM-FA-HSM-001`-Akzeptanz benannt:

1. **Slice-Plan-Update** (Status-Klarheit, kein Lösungspfad —
   bereits in Spike-README §6.3 Punkt 3 abgeschlossen).
2. **Profil B als M1-Pfad** (Software-HMAC-Konstruktion gemäß
   `HSM-FMT-006` Profil B).
3. **Profil-Wahl als Service-Konfiguration** (Schärfung
   `HSM-FA-HSM-005`).
4. **Lastenheft-Änderung** über `HSM-LESE-004` — politisch
   hochriskant, weil eine grundlegende Akzeptanzlockerung
   keine „technische Detailänderung" mehr wäre.

Diese ADR fixiert die Kombination **2 + 3**: Profil B wird M1-
Default und Pflicht-Konstruktion, Profil A bleibt als
konfigurierbare Alternative für HKDF-fähige Module bestehen, und
der Mechanism-Check (`HSM-FA-HSM-005`) prüft pro Start nur die
Mechanismen, die das aktive Profil verlangt.

Damit ist `HSM-FA-HSM-001` (Service-Start gegen SoftHSM v2 + ein
zweites herstellerfremdes Modul ohne Codeänderung) ohne Lastenheft-
Eingriff erfüllbar — SoftHSM trägt Profil B (`CKM_SHA256_HMAC`,
universell), Bouncy HSM trägt Profil A **oder** B (Operator-Wahl).

`HSM-FA-HSM-005` ist Spezifikation (nicht Lastenheft) und nach
[ADR 0001 §2.5](0001-documentation-and-planning-structure.md) durch
ADR-Sharpening erweiterbar; ADR 0004 §2.6 (Modulwahl) bleibt textlich
unverändert.

---

## 2. Entscheidung

### 2.1 Header-HMAC-Profil-Wahl wird Service-Konfiguration

Slice 002b §Konfiguration erhält eine neue Env-Variable:

- `HSMDOC_HEADER_HMAC_PROFILE` ∈ `{A, B}`, Default `B`
  (siehe §2.2 zur Begründung des B-Defaults).

Der Wert bestimmt zur Startzeit, welcher Adapter-Pfad im
`HeaderMAC`-Port-Slot lebt (siehe Slice-Plan §HeaderMAC-Port). Eine
zweite Aktivierung beider Profile gleichzeitig ist explizit
ausgeschlossen — der Server startet pro Pod-Lauf mit genau einem
aktiven Profil. Profil-Wechsel zur Laufzeit ist nicht zulässig
(Folge-ADR notwendig, falls je relevant).

Code-Layout: zwei parallele HeaderMAC-Implementierungen
`internal/adapter/driven/pkcs11/headermac/{profile_a.go,
profile_b.go}`, beide implementieren denselben `HeaderMAC`-Port
aus Slice 002b.

**Korrektur zur Cross-Profil-Kompatibilität (Spike-Befund
2026-05-28):** Profil A nutzt natives RFC-5869-HKDF; Profil B nutzt
die Spec-Konstruktion
`HMAC(HMAC(HMAC(IKM, salt), info||0x01), headerBytes)` (zwei
HMAC-Schritte für HKDF-Extract+Expand, danach Header-HMAC,
[HSM-FMT-006 §1 Profil B](../../../spec/spezifikation.md)). Diese
zwei Konstruktionen liefern **unterschiedliche** Header-Keys —
ein Container, der mit Profil A erzeugt wurde, kann **nicht** mit
Profil B verifiziert werden und umgekehrt. Damit gilt:

- `HSM-FMT-001`-Wire-Header trägt das Profil **nicht**; ein
  Container ist nur im selben Profil verifizierbar, mit dem er
  erzeugt wurde.
- Profil-Wahl ist eine **deployment-weite** Entscheidung —
  Operator wählt pro Cluster ein Profil; Streams aus anderen
  Clustern mit anderem Profil sind nicht ohne Profil-Wechsel
  lesbar.
- Eine spätere Migration auf das jeweils andere Profil bedingt
  eine Re-Encrypt-Phase oder eine Container-Header-Erweiterung
  (eigene Folge-ADR).

Diese Beschränkung war in der initialen ADR 0007 nicht erkannt
und wird hier nachgezogen. Die im Slice-Plan §HeaderMAC-Port
ursprünglich behauptete Cross-Profil-Verifikation entfällt.

### 2.2 Profil B als M1-Pflicht und Default

Profil B wird die **M1-Pflicht-Konstruktion** und damit der
Server-Default. Begründung:

- **HSM-FA-HSM-001-Akzeptanz:** Profil B nutzt ausschließlich
  `CKM_SHA256_HMAC` (universell in jedem ernsthaften PKCS#11-Modul).
  SoftHSM startet erfolgreich, Bouncy HSM startet erfolgreich, jedes
  M3-Vendor-HSM mit HMAC-SHA-256 startet erfolgreich.
- **Adapter-Pfad-Validierung in M1:** Pro-Modul-Vendor-Smoke gegen
  SoftHSM + Bouncy HSM beweist `HSM-FA-HSM-001` durch denselben
  Adapter-Code-Pfad.
- **Profil-A-Vorarbeit bleibt erhalten:** Die Marshal-/Shim-Arbeit
  aus [ADR 0006 §2.1](0006-hkdf-profil-a-binding-und-bouncy-hsm.md)
  ist nicht verworfen — sie liefert den Profil-A-Adapter-Code, der
  per Config aktivierbar ist (siehe §2.3).

Profil B folgt strikt
[HSM-FMT-006 §1 Profil B](../../../spec/spezifikation.md): HKDF-Extract
via `C_Sign(CKM_SHA256_HMAC, master)` und HKDF-Expand entweder über
PRK-Re-Import (`C_CreateObject(CKK_GENERIC_SECRET, CKA_VALUE=PRK,
CKA_SENSITIVE=true, CKA_EXTRACTABLE=false)` + `C_Sign` mit dem
Re-Importierten Header-Key) oder über iterierte HMAC-Operationen
direkt auf dem PRK-Handle. Die genaue Konstruktion ist pro
PKCS#11-Modul zu validieren und wird im neuen Sub-Spike (siehe §3)
festgelegt.

### 2.3 Profil A bleibt konfigurierbare Alternative

Für Module mit nativem `CKM_HKDF_DERIVE` (in M1: Bouncy HSM 2.x;
in M3: pro Vendor-Modul nach eigener Validierung) ist Profil A
weiter aktivierbar:

```sh
HSMDOC_HEADER_HMAC_PROFILE=A   # explizit; sonst Default B
```

Der Profil-A-Adapter-Code ist der Pfad-(a)-Shim aus
[ADR 0006 §2.1](0006-hkdf-profil-a-binding-und-bouncy-hsm.md);
Marshal-/Mechanism-/Spike-Validierung bleibt maßgeblich. Profil A
wird in M1 weiterhin in CI exerziert (Bouncy HSM-Run im
`make spike-hkdf-bouncyhsm`-Pfad und Slice-002b-Akzeptanz-Lauf),
aber als zweite Bahn neben dem Profil-B-Default.

### 2.4 Mechanism-Check pro aktivem Profil (Schärfung HSM-FA-HSM-005)

`HSM-FA-HSM-005` (Mechanism-Check beim Service-Start) wird so
geschärft, dass der Check pro Profil eine **definierte
Mechanismus-Liste** prüft. Spec-konform sind:

| Profil | Pflicht-Mechanismen                                    |
| ------ | ------------------------------------------------------ |
| A      | `CKM_AES_GCM`, `CKM_HKDF_DERIVE`, `CKM_SHA256_HMAC`    |
| B      | `CKM_AES_GCM`, `CKM_SHA256_HMAC`                       |

Fehlt ein Pflicht-Mechanismus, bricht der Server mit
`STARTUP_HSM_MECHANISM_MISSING` ab (Spike-Klassifikation in
Slice 002b §Startup-Fehlerklassen). Der Hinweis nennt den
fehlenden Mechanismus und das aktive Profil — eine
Konfigurationskorrektur (Profil-Wechsel oder Modul-Wechsel) ist
direkt aus dem Log ableitbar.

Damit ist `HSM-FA-HSM-005` **selektiv** statt ‚alle HSM-FMT-006-
Mechanismen pflichtig'. Die spec-Erweiterung lebt in einem
zusätzlichen Absatz unter `HSM-FA-HSM-005`, der mit dem
Slice-002b-Akzeptanz-Commit im selben PR landet. Die ursprüngliche
Spec-Stelle bleibt textlich erhalten — die Schärfung verweist auf
diese ADR 0007.

### 2.5 ADR 0006 und ADR 0004 bleiben textlich unverändert

ADR 0007 ist Folge-ADR zu ADR 0006 (und damit indirekt zu ADR 0004).
ADR 0006 bleibt für Profil A maßgeblich (Binding-Pfad-(a)-Shim,
Bouncy HSM als Spike-Modul); ADR 0007 ist für Profil B + Profil-Wahl
maßgeblich. Der ADR-Index trägt ADR 0007 als Schärfung von ADR 0006
in die Schärfungs-Spalte.

ADR 0004 (Runtime-Base) bleibt unangetastet — der CGO-Pfad und
Library-Closure-Verifikation gelten unverändert.

---

## 3. Konsequenzen

- **`HSM-FA-HSM-001` wird erfüllbar:** Vendor-Smoke gegen SoftHSM
  (Profil B) + Bouncy HSM (Profil B, optional Profil A) ohne
  Codeänderung. SoftHSM-Modul-Run nicht mehr am Mechanism-Check
  geblockt.
- **Profil B wird M1-Implementierungs-Pflicht.** Bisher in Slice 002b
  §HeaderMAC-Port als M3-Aufschub markiert; mit dieser ADR wird die
  Profil-B-Konstruktion Teil des M1-Slices.
- **Neuer Sub-Spike erforderlich:** `next/002b-spike-profil-b/`
  validiert die Profil-B-Konstruktion gegen SoftHSM **und** Bouncy
  HSM (`CKK_GENERIC_SECRET` als PRK-Re-Import-Typ, Sensitive-
  Durchsetzung pro Modul, PRK-Window-Minimierung). Slice 002b
  bekommt eine neue Vorbedingung 4 für diesen Spike.
- **Slice-002b-Plan-Update folgt:** §HeaderMAC-Port wird auf
  „Profil B Default, Profil A optional via
  `HSMDOC_HEADER_HMAC_PROFILE`" erweitert; §Akzeptanz HSM-FA-HSM-001
  wird grün-gefordert; §Konfiguration nimmt die neue Env-Variable
  auf; §Startup-Fehlerklassen ergänzen `STARTUP_HSM_MECHANISM_MISSING`.
- **Adapter-Code-Layout:** zwei parallele Implementierungen unter
  `internal/adapter/driven/pkcs11/headermac/{profile_a.go,
  profile_b.go}` mit gemeinsamem Port-Interface aus Slice 002b. Der
  Wire-Container-Header trägt das aktive Profil nicht; Decrypt-
  Verifikation hängt am Master-HMAC-Label.
- **CI-Aufwand-Erhöhung:** beide Profile + zwei Module = bis zu vier
  Smoke-Pfade. Pragmatisch wird CI gepinnt auf:
  - Profil B gegen SoftHSM (Pflicht-Akzeptanz)
  - Profil B gegen Bouncy HSM (Vendor-Portabilitäts-Beleg)
  - Profil A gegen Bouncy HSM (Profil-A-Adapter-Smoke)
  Profil A gegen SoftHSM ist absichtlich kein CI-Pfad — würde
  am Mechanism-Check abbrechen und wäre Doppel-Beleg ohne Mehrwert.

---

## 4. Compliance-Implikationen für Profil B

- **Zwei Klartext-Werte leben kurzfristig im Server-RAM:** der PRK
  (Output von HKDF-Extract) zwischen `C_Sign(extract)` und
  `C_CreateObject(prk-reimport)`, und der Header-Key (Output von
  HKDF-Expand) zwischen `C_Sign(expand)` und
  `C_CreateObject(header-key-reimport)`. Die Profil-B-Spec-
  Konstruktion macht zwei HMAC-Schritte explizit, jeder produziert
  einen Klartext-Wert im Heap.
- **Zeroize-Owner-Vertrag (eindeutig, eine Verantwortliche):** Die
  `Extract`- und `Expand`-Helper sind die Owner der jeweiligen
  Klartext-Buffer. Beide setzen `defer zeroize(buf)` **unmittelbar**
  nach `C_Sign` und vor jedem weiteren Statement (`return` inklusive).
  Aufrufer rufen `ReimportPRK` / `ReimportHeaderKey` und müssen den
  übergebenen Buffer **nicht** selbst zeroizen — der `defer`-Loop
  greift am Stack-Frame-Ende des Helpers, was nach dem Re-Import
  erfolgt. Damit gibt es keine Owner-Konflikte und kein Error-Pfad,
  der den Zeroize-Schritt überspringen kann (das `defer` läuft auch
  bei `panic` und früher `return`).
- **Adapter-Pflichten zusätzlich:** kein Logging, kein Trace, kein
  temp-File für PRK oder Header-Key. `gosec`-/Code-Review-Gate
  verbietet `fmt.Sprintf("%x", prk)`-artige Aufrufe.
- **Master-Key bleibt vollständig non-extractable.** Profil B
  verändert die Master-Key-Attribute nicht. Die
  Sensitive-/Extractable-Invarianten auf dem Master-Key sind
  unverändert.
- **Header-Key (Re-Imported PRK)** trägt zwingend
  `CKA_SENSITIVE=true`, `CKA_EXTRACTABLE=false`, `CKA_TOKEN=false`,
  `CKA_SIGN=true`. Nach Stream-Ende: `C_DestroyObject` — analog
  Profil A (Trace-Sequenz Schritt 10 aus
  [Spike-Trace-Sequenz](../planning/next/002b-spike-hkdf/trace/README.md)).
- **Spec-Bezug:** `HSM-FMT-006` Profil B akzeptiert dieses
  PRK-Window explizit — sicherheitstechnisch ist es minimal solange
  der Adapter-Code die Zeroize-/Atomicity-Invarianten hält. Der
  Sub-Spike `002b-spike-profil-b` belegt das.
- **Threat-Modell-Anker:** PRK-im-RAM ist ein neuer Threat gegen
  Heap-Dump-Angriffe; vorhandene Mitigations (Pod-Härtung
  `readOnlyRootFilesystem`, `GOMEMLIMIT`-Cap, `HSM-NFA-MEM-002`)
  reichen für M1 aus, weil der PRK-Window-Range deterministisch
  klein ist (mikrosekunden zwischen den zwei PKCS#11-Calls).

---

## 5. Pflege-Regeln

- **Profil-B-Implementierungsdetails** leben im Spike
  `next/002b-spike-profil-b/` (PRK-Re-Import-Konstruktion, Vendor-
  spezifische Quirks, Zeroize-Pattern). Das Spike-Ergebnis wird in
  den Adapter-Code übernommen und ist Vorbedingung 4 für die
  Slice-002b-Aktivierung.
- **Default-Umkehrung:** Wenn ein weit verbreitetes Software-HSM
  HKDF nativ implementiert (z. B. ein zukünftiges SoftHSM 2.x mit
  HKDF), kann der M1-Default zurück auf Profil A wechseln. Eigene
  Folge-ADR; ADR 0007 selbst bleibt unverändert.
- **Profil-Wechsel zur Laufzeit** ist nicht zulässig. Wenn künftig
  relevant (z. B. Rolling-Upgrade): eigene Folge-ADR, weil sich
  Wire-Verifikations-Erwartungen ändern können.
- **PRK-Zeroize-Garantie** ist Code-Review-Akzeptanzkriterium in
  Slice 002b. Ein Adapter-Patch, der die Zeroize-Invariante aufhebt,
  muss eigene ADR-Schärfung tragen.

---

## 6. Nicht Gegenstand dieser ADR

- **Profil-B-Implementierungsdetails** (Re-Import-Template, Vendor-
  spezifische Mechanismus-Liste, Sub-Spike-Akzeptanz) — leben in
  `next/002b-spike-profil-b/`.
- **M3-Produktionsprofile** bleiben in der separaten Profil-
  Validierung pro Vendor-HSM. M3 entscheidet pro Vendor, welches
  Profil aktiviert wird.
- **TLS-/Audit-/Stream-Pfade** sind unverändert. Profil-Wahl
  betrifft ausschließlich den Header-HMAC-Pfad.
- **Wire-Format-Änderungen** sind ausgeschlossen — Container-Header
  (`HSM-FMT-001`) trägt das Profil nicht; Decrypt-Verifikation hängt
  am Master-HMAC-Label, nicht am Profil.
- **Profil-A-Spike-Output** (Pfad-(a)-Shim, Bouncy HSM,
  Make-Target `spike-hkdf-bouncyhsm`) bleibt unangetastet. ADR 0006
  ist für Profil A maßgeblich.
