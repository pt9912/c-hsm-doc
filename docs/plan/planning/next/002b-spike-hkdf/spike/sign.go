//go:build spike && cgo && (amd64 || arm64)

package hkdfspike

import (
	"fmt"

	"github.com/miekg/pkcs11"
)

// SignHeaderHMAC ruft C_SignInit + C_Sign mit CKM_SHA256_HMAC auf
// dem abgeleiteten Header-Key auf und gibt den 32-Byte-Tag zurück.
//
// miekg/pkcs11.Ctx.Sign macht intern den Two-Call-Wrapper (Längen-
// Probe + Sign). Das ist laut trace/README.md für den Spike
// akzeptiert; nicht für den produktiven C_Encrypt-Pfad.
func SignHeaderHMAC(
	ctx *pkcs11.Ctx,
	session pkcs11.SessionHandle,
	headerKey pkcs11.ObjectHandle,
	headerBytes []byte,
) ([]byte, error) {
	mech := []*pkcs11.Mechanism{
		pkcs11.NewMechanism(pkcs11.CKM_SHA256_HMAC, nil),
	}
	if err := ctx.SignInit(session, mech, headerKey); err != nil {
		return nil, fmt.Errorf("hkdfspike: C_SignInit: %w", err)
	}
	tag, err := ctx.Sign(session, headerBytes)
	if err != nil {
		return nil, fmt.Errorf("hkdfspike: C_Sign: %w", err)
	}
	if len(tag) != 32 {
		return nil, fmt.Errorf("hkdfspike: HMAC-SHA-256 tag length %d, want 32", len(tag))
	}
	return tag, nil
}
