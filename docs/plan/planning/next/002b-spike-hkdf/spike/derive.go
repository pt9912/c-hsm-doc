//go:build spike && cgo && (amd64 || arm64)

package hkdfspike

/*
#include <stdlib.h>
*/
import "C"

import (
	"fmt"

	"github.com/miekg/pkcs11"
)

// DeriveHeaderKeyHSM ruft C_DeriveKey mit CKM_HKDF_DERIVE auf und gibt
// das Handle des abgeleiteten 32-Byte-Header-Keys zurück. Namenssuffix
// HSM grenzt diese Variante gegen die Pure-Go-Referenz DeriveHeaderKey
// (verify.go) ab — beide laufen im selben Paket.
//
// CK_HKDF_PARAMS hat kein Output-Length-Feld; die 32-Byte-Vorgabe
// kommt über CKA_VALUE_LEN im Template. Salt und Info werden in
// C-Memory allokiert (Go-Slice-Adressen wären für CGO-Aufrufe nicht
// stabil); die C-Buffer werden nach C_DeriveKey wieder freigegeben.
//
// miekg/pkcs11 v1.1.2 exportiert keine CKM_HKDF_DERIVE-Konstante,
// deshalb der uint-Cast aus dem hkdfspike-eigenen Wert (= 0x402A,
// PKCS#11 v3.0 §6.30).
//
// Der Aufrufer ist verpflichtet, das zurückgegebene Handle per
// ctx.DestroyObject zu zerstören (kanonische Trace-Sequenz Schritt 10).
func DeriveHeaderKeyHSM(
	ctx *pkcs11.Ctx,
	session pkcs11.SessionHandle,
	masterKey pkcs11.ObjectHandle,
	salt, info []byte,
) (pkcs11.ObjectHandle, error) {
	if len(salt) == 0 {
		return 0, fmt.Errorf("hkdfspike: salt must not be empty")
	}
	if len(info) == 0 {
		return 0, fmt.Errorf("hkdfspike: info must not be empty")
	}

	saltPtr := C.CBytes(salt)
	defer C.free(saltPtr)
	infoPtr := C.CBytes(info)
	defer C.free(infoPtr)

	params, err := Marshal(Params{
		Extract:          true,
		Expand:           true,
		PRFHashMechanism: CKM_SHA256,
		SaltType:         CKF_HKDF_SALT_DATA,
		SaltLen:          uint64(len(salt)),
		InfoLen:          uint64(len(info)),
	}, uintptr(saltPtr), uintptr(infoPtr))
	if err != nil {
		return 0, fmt.Errorf("hkdfspike: marshal CK_HKDF_PARAMS: %w", err)
	}

	mech := []*pkcs11.Mechanism{
		pkcs11.NewMechanism(uint(CKM_HKDF_DERIVE), params),
	}

	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_SECRET_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_GENERIC_SECRET),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE_LEN, DerivedHeaderKeyLen),
		pkcs11.NewAttribute(pkcs11.CKA_SIGN, true),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, false),
		pkcs11.NewAttribute(pkcs11.CKA_EXTRACTABLE, false),
		pkcs11.NewAttribute(pkcs11.CKA_SENSITIVE, true),
	}

	handle, err := ctx.DeriveKey(session, mech, masterKey, template)
	if err != nil {
		return 0, fmt.Errorf("hkdfspike: C_DeriveKey: %w", err)
	}
	return handle, nil
}
