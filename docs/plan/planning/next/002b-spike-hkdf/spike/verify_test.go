//go:build spike && (amd64 || arm64)

package hkdfspike

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestDeriveHeaderKeyRFC5869 verifiziert die HKDF-Stufe gegen
// RFC 5869 Appendix A.1 (Test Case 1, SHA-256). DeriveHeaderKey
// liefert 32 Byte, also vergleichen wir gegen die ersten 32 Byte
// des 42-Byte-OKM aus dem RFC-Vektor.
func TestDeriveHeaderKeyRFC5869(t *testing.T) {
	ikm := mustHex(t, "0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b")
	salt := mustHex(t, "000102030405060708090a0b0c")
	info := mustHex(t, "f0f1f2f3f4f5f6f7f8f9")

	// RFC 5869 Appendix A.1 OKM (42 Byte, erste 32 davon prüfen wir).
	wantHex := "3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf"
	want := mustHex(t, wantHex)

	got, err := DeriveHeaderKey(ikm, salt, info)
	if err != nil {
		t.Fatalf("DeriveHeaderKey returned error: %v", err)
	}
	if !bytes.Equal(got[:], want) {
		t.Fatalf("HKDF OKM[:32] mismatch\n got %s\nwant %s",
			hex.EncodeToString(got[:]), wantHex)
	}
}

// TestExpectedHeaderMACDeterminism schützt davor, dass die Funktion
// versehentlich randomisierte Zwischenwerte erzeugt. Zweimal mit
// identischen Inputs muss byteweise identischer Output kommen.
func TestExpectedHeaderMACDeterminism(t *testing.T) {
	salt := bytes.Repeat([]byte{0xAA}, 16)
	header := []byte("c-hsm-doc/test-header")

	a, err := ExpectedHeaderMAC(FixtureIKM, salt, []byte(HeaderHMACInfo), header)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	b, err := ExpectedHeaderMAC(FixtureIKM, salt, []byte(HeaderHMACInfo), header)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if a != b {
		t.Fatalf("non-deterministic output:\n a=%x\n b=%x", a, b)
	}
	if len(a) != sha256.Size {
		t.Fatalf("HMAC output length = %d, want %d", len(a), sha256.Size)
	}
}

// TestExpectedHeaderMACSaltSensitivity belegt, dass der Salt im
// HMAC-Output landet — ein anderer Salt liefert einen anderen Tag.
func TestExpectedHeaderMACSaltSensitivity(t *testing.T) {
	saltA := bytes.Repeat([]byte{0xAA}, 16)
	saltB := bytes.Repeat([]byte{0xBB}, 16)
	header := []byte("c-hsm-doc/test-header")

	a, err := ExpectedHeaderMAC(FixtureIKM, saltA, []byte(HeaderHMACInfo), header)
	if err != nil {
		t.Fatalf("saltA error: %v", err)
	}
	b, err := ExpectedHeaderMAC(FixtureIKM, saltB, []byte(HeaderHMACInfo), header)
	if err != nil {
		t.Fatalf("saltB error: %v", err)
	}
	if a == b {
		t.Fatalf("expected different outputs for different salts, got identical %x", a)
	}
}

// TestDerivedHeaderKeyLenMatchesProfile hält den Vertrag, dass die
// Pure-Go-Referenz exakt die in HSM-FMT-006 Profil A geforderte
// Header-Key-Länge liefert (32 Byte = CKA_VALUE_LEN-Wert im
// C_DeriveKey-Template).
func TestDerivedHeaderKeyLenMatchesProfile(t *testing.T) {
	if DerivedHeaderKeyLen != 32 {
		t.Fatalf("DerivedHeaderKeyLen = %d, want 32 (HSM-FMT-006 Profil A)", DerivedHeaderKeyLen)
	}
	out, err := DeriveHeaderKey(FixtureIKM, []byte("salt"), []byte(HeaderHMACInfo))
	if err != nil {
		t.Fatalf("DeriveHeaderKey error: %v", err)
	}
	if len(out) != DerivedHeaderKeyLen {
		t.Fatalf("output length = %d, want %d", len(out), DerivedHeaderKeyLen)
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("invalid hex literal: %v", err)
	}
	return b
}
