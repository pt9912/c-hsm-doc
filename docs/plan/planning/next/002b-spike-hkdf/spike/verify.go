//go:build spike && (amd64 || arm64)

package hkdfspike

import (
	"crypto/hmac"
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"
)

// DerivedHeaderKeyLen entspricht CKA_VALUE_LEN=32 aus dem
// C_DeriveKey-Template; HSM-FMT-006 Profil A fordert 32 Byte
// Header-Key.
const DerivedHeaderKeyLen = 32

// DeriveHeaderKey führt die HKDF-Extract+Expand-Stufe in Pure-Go
// aus (RFC 5869 mit HMAC-SHA-256). Spike-/Test-only — produktiver
// Adapter-Code unter internal/adapter/driven/pkcs11/ ruft die
// Derivation ausschließlich am HSM auf und sieht das IKM nie.
func DeriveHeaderKey(ikm, salt, info []byte) ([DerivedHeaderKeyLen]byte, error) {
	r := hkdf.New(sha256.New, ikm, salt, info)
	var out [DerivedHeaderKeyLen]byte
	if _, err := io.ReadFull(r, out[:]); err != nil {
		return out, err
	}
	return out, nil
}

// ExpectedHeaderMAC berechnet den 32-Byte-HMAC-SHA-256-Tag, den
// der HSM-Lauf über headerBytes mit dem HKDF-abgeleiteten
// Header-Key liefern muss: HMAC-SHA256(HKDF(IKM, salt, info,
// L=32), headerBytes). Vergleichswert für die Spike-Erfolgs-
// Kriterien §3 Punkt 2 + 5.
func ExpectedHeaderMAC(ikm, salt, info, headerBytes []byte) ([sha256.Size]byte, error) {
	key, err := DeriveHeaderKey(ikm, salt, info)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	m := hmac.New(sha256.New, key[:])
	m.Write(headerBytes)
	var out [sha256.Size]byte
	copy(out[:], m.Sum(nil))
	return out, nil
}
