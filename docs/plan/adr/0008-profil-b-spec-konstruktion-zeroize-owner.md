# ADR 0008 — Profil-B-Konstruktion gemäß HSM-FMT-006 und Zeroize-Owner-Vertrag

**Status:** Accepted
**Datum:** 2026-05-28
**Bezug:** [Spezifikation HSM-FMT-006](../../../spec/spezifikation.md),
[ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
(ADR-Immutabilität),
[ADR 0007](0007-profil-b-als-m1-default-und-konfigurierbare-profilwahl.md)
(geschärft durch §2 + §3 dieser ADR — §2.1 Cross-Profil-Aussage
+ §4 PRK-Window),
[Slice 002b](../planning/next/002b-pkcs11-encrypt-hexagon.md),
[Spike 002b-Profil-B §1](../planning/next/002b-spike-profil-b/README.md)

---

## 1. Kontext

ADR 0007 ist `Accepted` und nach [ADR 0001 §2.3](0001-documentation-and-planning-structure.md)
inhaltlich unveränderlich. Beim Anlegen des Profil-B-Spike-Plans
sind zwei Lücken im ADR-0007-Text aufgetaucht, die ein erstes
Review (2026-05-28) als Findings markiert hat:

### 1.1 Profil-B-Konstruktion vertauschte HKDF-Argumente

[Spec HSM-FMT-006 §1 Profil B](../../../spec/spezifikation.md)
definiert die Konstruktion als:

```
Extract: HMAC(salt, ikm)
Expand:  HMAC(prk, info || 0x01)
header_key = HKDF-SHA-256(ikm=master_hmac, salt, info, L=32)
```

Profil A und Profil B MÜSSEN denselben `header_key` liefern, weil
Spec explizit `header_key = HKDF-SHA-256(...)` festschreibt und
beide Profile derselben Definition folgen.

Der initiale Profil-B-Spike-Plan hatte die Argumente vertauscht
(`HMAC(IKM, salt)` statt `HMAC(salt, IKM)`) und damit eine
HKDF-inkompatible Konstruktion beschrieben — Profil A und Profil B
hätten unterschiedliche Tags produziert. Das ist nicht spec-konform.

ADR 0007 §2.1 hatte den Gegenfall (Cross-Profil-Inkompatibilität)
direkt eingefügt; dieser Edit ist inzwischen revertiert (Original-
Text wiederhergestellt). Diese ADR 0008 fixiert die spec-konforme
Lesart und nimmt die Cross-Profil-Behauptung damit zurück.

### 1.2 Zeroize-Owner-Vertrag widersprüchlich

ADR 0007 §4 erwähnte einen PRK-Klartext-Buffer + Zeroize-Pflicht
„unmittelbar nach `C_CreateObject`". Der Spike-Plan (002b-spike-
profil-b/README.md) zog daraus zwei nicht-kompatible Lesarten —
Erfolgs-Kriterium §3 sagte „Aufrufer sieht Klartext nie über
Helper-Grenze", Vorgehen §5 sagte „Extract gibt PRK an Aufrufer
zurück, der zeroizen muss". Vor Implementierung muss eine
eindeutige Owner-Definition stehen.

Außerdem deckt ADR 0007 §4 nur den PRK ab — die spec-konforme
Profil-B-Konstruktion macht aber **zwei** HMAC-Schritte; der
Header-Key (Output von HKDF-Expand) ist ein zweiter Klartext-
Wert, der ebenfalls zeroized werden muss.

---

## 2. Entscheidung

### 2.1 Profil-B-Konstruktion ist spec-konform und Cross-Profil-identisch

Slice 002b implementiert Profil B exakt nach
[HSM-FMT-006 §1 Profil B](../../../spec/spezifikation.md):

```
PRK        = HMAC-SHA256( salt, IKM )                  # Extract
header_key = HMAC-SHA256( PRK, info || 0x01 )          # Expand (L=32)
tag        = HMAC-SHA256( header_key, header_bytes )   # Header-HMAC
```

Damit gilt: `header_key` ist **identisch** mit dem RFC-5869-HKDF-
Output `HKDF-SHA-256(IKM, salt, info, 32)`. Profil A (natives
`CKM_HKDF_DERIVE`) und Profil B (zweistufige HMAC-Konstruktion)
liefern denselben Header-Key und damit denselben `tag` über
identische Inputs.

Konsequenz für die Wire-Ebene: `HSM-FMT-001`-Container-Header trägt
das Profil **nicht** und MUSS es nicht tragen — beide Profile sind
über denselben `master_hmac_pkcs11_label` cross-verifizierbar. ADR
0007 §2.1 bleibt damit gültig wie geschrieben.

### 2.2 Implementierungs-Realität ist Spike-Befund-Material

Spec sagt „realisiert über `CKM_SHA256_HMAC` auf dem nicht-
extrahierbaren Master-Key". Die Realisierung von `HMAC(salt, IKM)`
mit IKM nicht-extrahierbar im HSM ist nicht-trivial, weil C_Sign-
Daten ein `[]byte` sind und IKM nicht extrahiert werden darf:

- **Variante 1 (Vendor-HKDF):** Wenn das Modul einen HKDF-artigen
  Derive-Mechanismus anbietet (`CKM_NSS_HKDF`, `CKM_HKDF_KEY_GEN`,
  `CKM_SP800_108_COUNTER_KDF`, …), kann Profil B darüber realisiert
  werden — dann ist `CKM_HKDF_DERIVE` zwar formal nicht da, der
  Effekt aber gleich.
- **Variante 2 (Salt-as-Key Pattern):** salt wird als
  `CKK_GENERIC_SECRET` mit `CKA_VALUE=salt` importiert (salt ist
  öffentlich, keine Sensitive-Bedenken); `C_DeriveKey` mit
  Vendor-Mechanismus, der den IKM als Base-Key + den salt-Handle
  + Info kombiniert.
- **Variante 3 (nicht freigegeben):** IKM kurzzeitig als
  extractable-Key behandeln. Verstößt gegen Spec-Pflicht „ohne
  Klartext-Export des Master-Materials" und ist **nicht**
  zulässig.

Welche Variante pro Modul greift, ist genau das, was der
Profil-B-Spike (Slice 002b §Vorbedingung 4) erkundet. Mögliches
Spike-Befund: ein Modul (z. B. SoftHSM 2.7.0) bietet keinen
geeigneten Vendor-Mechanismus — dann ist es für Profil B **nicht
freigegeben** und kann die `HSM-FA-HSM-001`-Akzeptanz nicht
tragen. In dem Fall geht der Spike zurück in die Planung
(Modul-Wechsel oder eigene Folge-ADR).

Der Spike-Plan ist deshalb so geschnitten, dass er die
Realisierungs-Variante **erkundet** und nicht vorgibt — pro
CI-Modul wird Variante 1/2 separat geprüft.

### 2.3 Zeroize-Owner-Vertrag: `defer`-Pattern in Extract/Expand

Profil B produziert **zwei** Klartext-Werte im Server-Heap:

- **PRK** zwischen `Extract`-Aufruf und `ReimportPRK`-Aufruf
- **header_key** zwischen `Expand`-Aufruf und `ReimportHeaderKey`-Aufruf

Beide werden durch denselben Owner-Vertrag geschützt:

- **Owner = `Extract`/`Expand`-Helper-Funktion.** Helper allokiert
  den Klartext-Buffer, ruft `C_Sign`, setzt **vor jedem weiteren
  Statement** ein `defer zeroize(buf)` und gibt eine **Kopie** an
  den Aufrufer zurück.
- **Aufrufer ruft `Reimport*` direkt mit dem zurückgegebenen
  Buffer.** Direkt nach Rückkehr aus `Reimport*` läuft `defer
  zeroize(buf)` am Stack-Frame-Ende des Helpers. Damit ist der
  Klartext-Buffer null, bevor irgendein anderer Code-Pfad ihn
  sehen kann.
- **Kein zusätzlicher Zeroize-Schritt im Aufrufer.** Doppel-Zeroize
  ist unschädlich aber überflüssig; das `defer`-Pattern reicht.
- **Error-Pfade:** `defer` läuft auch bei `panic` und früher
  `return`. Ein Fehler in `Reimport*` führt nicht zu einem
  permanenten Klartext im Heap.

Konkrete Helper-Signatur (verbindlich für Spike + produktiven
Adapter):

```go
// Extract ruft HMAC-SHA256(salt, IKM) im HSM auf und gibt eine
// Kopie des PRK an den Aufrufer zurück. Die Zeroize-Pflicht ist
// im defer am Stack-Frame-Ende dieses Helpers verankert; der
// Aufrufer muss den zurückgegebenen Buffer NICHT selbst zeroizen.
func Extract(ctx *pkcs11.Ctx, session pkcs11.SessionHandle,
    masterKey pkcs11.ObjectHandle, salt []byte) (prk []byte, err error)

// Expand ruft HMAC-SHA256(PRK, info || 0x01) im HSM auf und gibt
// eine Kopie des Header-Keys an den Aufrufer zurück. Selbes
// defer-Pattern.
func Expand(ctx *pkcs11.Ctx, session pkcs11.SessionHandle,
    prkHandle pkcs11.ObjectHandle, info []byte) (headerKey []byte, err error)
```

Adapter-Pflichten zusätzlich: kein Logging, kein Trace, kein
temp-File für PRK oder Header-Key. `gosec`-/Code-Review-Gate
verbietet `fmt.Sprintf("%x", prk)`-artige Aufrufe.

### 2.4 ADR 0007 §2.1 + §4 bleiben textlich, sind aber durch diese ADR geschärft

ADR 0007 §2.1 bleibt textlich wie geschrieben — die Cross-Profil-
Verifikations-Aussage ist nach §2.1 dieser ADR korrekt (beide Profile
liefern denselben `header_key`). Die im Slice-Plan zwischenzeitlich
behauptete Inkompatibilität war ein Spike-Plan-Fehler, kein ADR-
Inhalt.

ADR 0007 §4 deckt nur den PRK ab. Diese ADR 0008 §2.3 erweitert auf
zwei Klartext-Werte (PRK + Header-Key) und legt den `defer`-Pattern-
Owner-Vertrag verbindlich fest. ADR 0007 §4 bleibt historisch
gültig für die PRK-Hälfte; ADR 0008 §2.3 ist die maßgebliche
Fassung für die Zeroize-Verantwortung beider Buffer.

Der ADR-Index trägt ADR 0008 als Schärfung von ADR 0007 ein.

---

## 3. Konsequenzen

- **Spike-Plan + Slice-Plan korrigieren:** Konstruktion auf
  Spec-Reihenfolge `HMAC(salt, IKM)` umstellen; Pure-Go-Vergleich
  nutzt wieder `HMAC(HKDF-SHA-256(IKM, salt, info, 32),
  headerBytes)`. Die in den Reviews kurzzeitig gegen Spec
  geschriebene Konstruktion `HMAC(HMAC(HMAC(IKM, salt),
  info||0x01), headerBytes)` ist nicht spec-konform und entfällt.
- **Cross-Profil-Vergleich grün:** Profil A und Profil B liefern
  über identisches Master-Material denselben Tag. Pure-Go-
  Referenz aus dem Profil-A-Spike-Paket
  ([`spike/verify.go`](../planning/next/002b-spike-hkdf/spike/verify.go))
  ist Vergleichswert für beide Profile.
- **`HSM-FA-HSM-001`-Akzeptanz hängt am Spike-Erkundungsbefund**:
  Profil-B-Realisation pro Modul. SoftHSM ohne geeigneten
  Vendor-Mechanismus ist **nicht** automatisch durch ADR 0007/0008
  freigegeben — das Spike-Ergebnis entscheidet.
- **Zeroize-Owner-Vertrag verbindlich:** Spike-Plan-Helper-
  Signaturen + Adapter-Code MÜSSEN dem `defer`-Pattern aus §2.3
  folgen. Code-Review-Akzeptanz: `defer zeroize(buf)` steht in
  `extract.go` und `expand.go` (Spike) bzw. `profile_b.go`
  (Adapter) unmittelbar nach `C_Sign` und vor jedem weiteren
  Statement.
- **ADR 0007-Edit revertiert:** der direkte Edit, der gegen
  ADR-Immutabilität verstieß, ist zurückgenommen. ADR 0007 ist
  wieder im Originalzustand vom 2026-05-28.

---

## 4. Pflege-Regeln

- Wenn ein Modul bei der Spike-Erkundung keinen Vendor-Pfad zur
  Realisation von `HMAC(salt, IKM)` mit nicht-extrahierbarem IKM
  bietet, bleibt es für Profil B nicht freigegeben. Eine eigene
  Folge-ADR dokumentiert die Konsequenz für `HSM-FA-HSM-001`.
- Sollte sich herausstellen, dass Profil B in der Realität nur auf
  Modulen läuft, die auch Profil A bedienen, wird die M1-Default-
  Entscheidung in einer eigenen Folge-ADR re-evaluiert (z. B.
  Profil A wieder als M1-Default mit Bouncy-HSM-Only-CI).
- Der `defer`-Zeroize-Owner-Vertrag aus §2.3 ist Code-Review-Gate.
  Eine Änderung der Owner-Verantwortung (z. B. „Aufrufer zeroizt
  zusätzlich") braucht eigene Folge-ADR.

---

## 5. Nicht Gegenstand dieser ADR

- **Spike-Probe-Code** (extract.go, expand.go, reimport.go,
  sign_b.go, verify_b.go, hsm_test.go) — folgt im nächsten
  Inkrement; diese ADR fixiert nur die Konstruktion + den
  Owner-Vertrag.
- **Modul-Wechsel bei fehlender Profil-B-Realisation** — wird im
  Spike-Befund + ggf. eigene Folge-ADR behandelt.
- **Wire-Format-Erweiterung** (Container-Header trägt Profil) ist
  nicht nötig, weil §2.1 die Cross-Profil-Identität wiederherstellt.
- **Profil C (Vendor-KDF)** bleibt M3-Scope; diese ADR betrifft
  Profil A und Profil B.
