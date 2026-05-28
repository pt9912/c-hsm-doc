//go:build spike && (amd64 || arm64)

package hkdfspike

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// PKCS#11 v3.0 Mechanismus-IDs (§6.30). Werte werden zusätzlich in
// mechanism_test.go gegen Literale geprüft, damit ein Tippfehler in
// dieser Konstantenliste nicht durch alle Tests durchrutscht
// (z. B. 0x0000402D ist CKM_SALSA20_KEY_GEN, nicht HKDF_DERIVE).
const (
	CKM_SHA256       uint64 = 0x00000250
	CKM_SHA256_HMAC  uint64 = 0x00000251
	CKM_SHA384       uint64 = 0x00000260
	CKM_SHA384_HMAC  uint64 = 0x00000261
	CKM_SHA512       uint64 = 0x00000270
	CKM_SHA512_HMAC  uint64 = 0x00000271
	CKM_HKDF_DERIVE  uint64 = 0x0000402A
)

// CK_BBOOL-Werte (PKCS#11 v3.0 §3.1).
const (
	ckTrue  byte = 0x01
	ckFalse byte = 0x00
)

// CK_HKDF_PARAMS ulSaltType (§6.31.1).
const (
	CKF_HKDF_SALT_NULL uint64 = 0x00000001
	CKF_HKDF_SALT_DATA uint64 = 0x00000002
	CKF_HKDF_SALT_KEY  uint64 = 0x00000004
)

// Field-Offsets im serialisierten CK_HKDF_PARAMS (LP64, little-endian).
// Auf 64-Bit-Linux/AMD64 ergibt das natürliche C-Struct-Alignment
// folgendes Layout (PKCS#11 v3.0 §6.31.1):
//
//	offset 0  bExtract           CK_BBOOL  (1 byte)
//	offset 1  bExpand            CK_BBOOL  (1 byte)
//	offset 2  padding                       (6 bytes, alignment)
//	offset 8  prfHashMechanism   CK_ULONG  (8 bytes)
//	offset 16 ulSaltType         CK_ULONG  (8 bytes)
//	offset 24 pSalt              CK_BYTE_PTR (8 bytes)
//	offset 32 ulSaltLen          CK_ULONG  (8 bytes)
//	offset 40 hSaltKey           CK_OBJECT_HANDLE (8 bytes)
//	offset 48 pInfo              CK_BYTE_PTR (8 bytes)
//	offset 56 ulInfoLen          CK_ULONG  (8 bytes)
//	total    64 bytes
const (
	offBExtract = 0
	offBExpand  = 1
	offPRF      = 8
	offSaltType = 16
	offPSalt    = 24
	offSaltLen  = 32
	offHSaltKey = 40
	offPInfo    = 48
	offInfoLen  = 56
	ParamsSize  = 64
)

// Params trägt die logischen HKDF-Eingaben, aus denen Marshal den
// 64-Byte-CK_HKDF_PARAMS-Block baut. Die Bytewerte für Salt und Info
// werden vom Aufrufer separat in C-Speicher abgelegt; die zugehörigen
// Pointer (saltPtr, infoPtr) reicht der Aufrufer als uintptr nach.
type Params struct {
	Extract          bool
	Expand           bool
	PRFHashMechanism uint64
	SaltType         uint64
	SaltLen          uint64
	SaltKeyHandle    uint64
	InfoLen          uint64
}

// ErrSaltMismatch signalisiert, dass SaltType, SaltLen, saltPtr und
// SaltKeyHandle nicht zueinander passen.
var ErrSaltMismatch = errors.New("hkdfspike: salt type/length/handle inconsistent")

// ErrInfoMismatch signalisiert, dass InfoLen und infoPtr nicht
// zueinander passen (Länge ohne Pointer oder Pointer ohne Länge).
var ErrInfoMismatch = errors.New("hkdfspike: info length/pointer inconsistent")

// Marshal serialisiert Params in einen 64-Byte-Block, der direkt als
// Mechanism-Parameter an C_DeriveKey übergeben werden kann.
//
// saltPtr und infoPtr müssen für die Lebensdauer des C_DeriveKey-
// Aufrufs gültige C-Adressen sein (typisch über C.CBytes vor dem
// Aufruf allokiert). Marshal selbst allokiert kein C-Memory.
//
// Hinweis: Die Output-Länge des abgeleiteten Schlüssels (HSM-FMT-006
// Profil A fordert 32 Byte) ist **nicht** Teil von CK_HKDF_PARAMS und
// wird daher hier auch nicht serialisiert. Sie muss vom Aufrufer im
// C_DeriveKey-Template als CKA_VALUE_LEN=32 gesetzt werden.
func Marshal(p Params, saltPtr, infoPtr uintptr) ([]byte, error) {
	if err := validate(p, saltPtr, infoPtr); err != nil {
		return nil, err
	}

	buf := make([]byte, ParamsSize)
	if p.Extract {
		buf[offBExtract] = ckTrue
	} else {
		buf[offBExtract] = ckFalse
	}
	if p.Expand {
		buf[offBExpand] = ckTrue
	} else {
		buf[offBExpand] = ckFalse
	}
	binary.LittleEndian.PutUint64(buf[offPRF:], p.PRFHashMechanism)
	binary.LittleEndian.PutUint64(buf[offSaltType:], p.SaltType)
	binary.LittleEndian.PutUint64(buf[offPSalt:], uint64(saltPtr))
	binary.LittleEndian.PutUint64(buf[offSaltLen:], p.SaltLen)
	binary.LittleEndian.PutUint64(buf[offHSaltKey:], p.SaltKeyHandle)
	binary.LittleEndian.PutUint64(buf[offPInfo:], uint64(infoPtr))
	binary.LittleEndian.PutUint64(buf[offInfoLen:], p.InfoLen)
	return buf, nil
}

func validate(p Params, saltPtr, infoPtr uintptr) error {
	if !p.Extract && !p.Expand {
		return fmt.Errorf("hkdfspike: at least one of Extract/Expand must be true")
	}
	switch p.SaltType {
	case CKF_HKDF_SALT_NULL:
		if p.SaltLen != 0 || saltPtr != 0 || p.SaltKeyHandle != 0 {
			return fmt.Errorf("%w: SALT_NULL requires SaltLen=0, saltPtr=0, SaltKeyHandle=0", ErrSaltMismatch)
		}
	case CKF_HKDF_SALT_DATA:
		if p.SaltLen == 0 || saltPtr == 0 {
			return fmt.Errorf("%w: SALT_DATA requires SaltLen>0 and saltPtr!=0", ErrSaltMismatch)
		}
		if p.SaltKeyHandle != 0 {
			return fmt.Errorf("%w: SALT_DATA must not set SaltKeyHandle", ErrSaltMismatch)
		}
	case CKF_HKDF_SALT_KEY:
		if p.SaltKeyHandle == 0 {
			return fmt.Errorf("%w: SALT_KEY requires SaltKeyHandle!=0", ErrSaltMismatch)
		}
		if p.SaltLen != 0 || saltPtr != 0 {
			return fmt.Errorf("%w: SALT_KEY must not set SaltLen/saltPtr", ErrSaltMismatch)
		}
	default:
		return fmt.Errorf("hkdfspike: unknown SaltType 0x%x", p.SaltType)
	}
	if p.InfoLen == 0 && infoPtr != 0 {
		return fmt.Errorf("%w: infoPtr != 0 but InfoLen == 0", ErrInfoMismatch)
	}
	if p.InfoLen != 0 && infoPtr == 0 {
		return fmt.Errorf("%w: InfoLen > 0 but infoPtr == 0", ErrInfoMismatch)
	}
	return nil
}
