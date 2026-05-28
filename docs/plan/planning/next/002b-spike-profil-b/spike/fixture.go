//go:build spike && cgo && (amd64 || arm64)

package profilbspike

// FixtureIKM ist deterministisches Test-Material für den Profil-B-
// Spike, synchron zu hkdfspike.FixtureIKM aus
// docs/plan/planning/next/002b-spike-hkdf/spike/fixture.go.
//
// **Synchron halten:** wenn dieser Wert oder HeaderHMACInfo geändert
// werden, MUSS die Profil-A-Spike-Konstante synchron mitziehen.
// Sonst weichen die Pure-Go-Referenz (Profil-A-Spike) und der
// HSM-Tag (Profil-B-Spike) voneinander ab.
//
// 32 Byte = 0x00..0x1f, niemals produktiv verwenden — der Wert
// landet im CI-Init-Skript (ci/keys-init/{softhsm,bouncyhsm}.sh)
// als Master-HMAC-Key mit CKA_EXTRACTABLE=false. Dasselbe Master-
// Material trägt Profil A und Profil B; beide Konstruktionen
// liefern denselben Header-HMAC-Tag.
var FixtureIKM = []byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
	0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
}

// HeaderHMACInfo ist die HKDF-Info-Konstante aus HSM-FMT-006 §1
// Profil A/B. Beide Konstruktionen verwenden denselben Wert.
const HeaderHMACInfo = "c-hsm-doc/header-hmac/v1"
