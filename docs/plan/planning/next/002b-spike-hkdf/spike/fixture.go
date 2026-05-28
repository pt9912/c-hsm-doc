//go:build spike && (amd64 || arm64)

package hkdfspike

// FixtureIKM ist deterministisches Test-Material für den Spike.
// 32 Byte = 0x00..0x1f. Niemals produktiv verwenden — der Wert
// landet im CI-Init-Skript per C_CreateObject mit
// CKA_EXTRACTABLE=false und dient ausschließlich als Vergleichs-
// Vorlage für die Pure-Go-HKDF-Referenz aus verify.go.
var FixtureIKM = []byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
	0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
}

// HeaderHMACInfo ist die HKDF-Info-Konstante aus HSM-FMT-006 §1
// Profil A für den Header-HMAC-Key. Identisch zum produktiven
// Adapter-Pfad in Slice 002b.
const HeaderHMACInfo = "c-hsm-doc/header-hmac/v1"
