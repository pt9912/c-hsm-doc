//go:build spike && (amd64 || arm64)

package hkdfspike

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"unsafe"
)

// Sentinel-Pointer aus 8-Byte-Mustern — auf der Wire-Ebene
// austauschbar, also reproduzierbar im Test.
const (
	sentinelSaltPtr uintptr = 0xDEADBEEFCAFEBABE
	sentinelInfoPtr uintptr = 0x1122334455667788
)

func TestMarshalReferenceLayout(t *testing.T) {
	p := Params{
		Extract:          true,
		Expand:           true,
		PRFHashMechanism: CKM_SHA256,
		SaltType:         CKF_HKDF_SALT_DATA,
		SaltLen:          16,
		InfoLen:          24,
	}

	got, err := Marshal(p, sentinelSaltPtr, sentinelInfoPtr)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	want := mustDecodeHex(t, strings.Join([]string{
		"0101000000000000",       // bExtract=1, bExpand=1, pad
		"5002000000000000",       // prfHashMechanism = CKM_SHA256 (0x250)
		"0200000000000000",       // ulSaltType = CKF_HKDF_SALT_DATA
		"bebafecaefbeadde",       // pSalt = 0xDEADBEEFCAFEBABE (LE)
		"1000000000000000",       // ulSaltLen = 16
		"0000000000000000",       // hSaltKey = 0
		"8877665544332211",       // pInfo = 0x1122334455667788 (LE)
		"1800000000000000",       // ulInfoLen = 24
	}, ""))

	if !bytes.Equal(got, want) {
		t.Fatalf("Marshal output mismatch\n got %s\nwant %s",
			hex.EncodeToString(got), hex.EncodeToString(want))
	}
	if len(got) != ParamsSize {
		t.Fatalf("Marshal returned %d bytes, want %d", len(got), ParamsSize)
	}
}

func TestMarshalSaltNull(t *testing.T) {
	p := Params{
		Extract:          false,
		Expand:           true,
		PRFHashMechanism: CKM_SHA512,
		SaltType:         CKF_HKDF_SALT_NULL,
		InfoLen:          24,
	}
	got, err := Marshal(p, 0, sentinelInfoPtr)
	if err != nil {
		t.Fatalf("Marshal SALT_NULL returned error: %v", err)
	}
	if got[offBExtract] != ckFalse || got[offBExpand] != ckTrue {
		t.Errorf("Extract/Expand bytes wrong: %x %x", got[offBExtract], got[offBExpand])
	}
	for off := offPSalt; off < offPSalt+8; off++ {
		if got[off] != 0 {
			t.Errorf("pSalt byte %d expected 0, got %x", off, got[off])
		}
	}
	for off := offSaltLen; off < offSaltLen+8; off++ {
		if got[off] != 0 {
			t.Errorf("ulSaltLen byte %d expected 0, got %x", off, got[off])
		}
	}
}

func TestMarshalSaltKey(t *testing.T) {
	p := Params{
		Extract:          true,
		Expand:           true,
		PRFHashMechanism: CKM_SHA384,
		SaltType:         CKF_HKDF_SALT_KEY,
		SaltKeyHandle:    0xCAFE,
		InfoLen:          24,
	}
	got, err := Marshal(p, 0, sentinelInfoPtr)
	if err != nil {
		t.Fatalf("Marshal SALT_KEY returned error: %v", err)
	}
	if got[offHSaltKey] != 0xFE || got[offHSaltKey+1] != 0xCA {
		t.Errorf("hSaltKey LE encoding wrong: %x", got[offHSaltKey:offHSaltKey+8])
	}
}

func TestMarshalValidation(t *testing.T) {
	cases := []struct {
		name    string
		p       Params
		saltPtr uintptr
		infoPtr uintptr
		wantErr error
	}{
		{
			name: "neither extract nor expand",
			p: Params{
				PRFHashMechanism: CKM_SHA256,
				SaltType:         CKF_HKDF_SALT_NULL,
				InfoLen:          24,
			},
			infoPtr: sentinelInfoPtr,
		},
		{
			name: "SALT_DATA without saltPtr",
			p: Params{
				Extract:          true,
				PRFHashMechanism: CKM_SHA256,
				SaltType:         CKF_HKDF_SALT_DATA,
				SaltLen:          16,
				InfoLen:          24,
			},
			infoPtr: sentinelInfoPtr,
			wantErr: ErrSaltMismatch,
		},
		{
			name: "SALT_DATA with key handle",
			p: Params{
				Extract:          true,
				PRFHashMechanism: CKM_SHA256,
				SaltType:         CKF_HKDF_SALT_DATA,
				SaltLen:          16,
				SaltKeyHandle:    0xBEEF,
				InfoLen:          24,
			},
			saltPtr: sentinelSaltPtr,
			infoPtr: sentinelInfoPtr,
			wantErr: ErrSaltMismatch,
		},
		{
			name: "SALT_KEY without handle",
			p: Params{
				Extract:          true,
				PRFHashMechanism: CKM_SHA256,
				SaltType:         CKF_HKDF_SALT_KEY,
				InfoLen:          24,
			},
			infoPtr: sentinelInfoPtr,
			wantErr: ErrSaltMismatch,
		},
		{
			name: "unknown SaltType",
			p: Params{
				Extract:          true,
				PRFHashMechanism: CKM_SHA256,
				SaltType:         0xFF,
				InfoLen:          24,
			},
			infoPtr: sentinelInfoPtr,
		},
		{
			name: "InfoLen > 0 but infoPtr == 0",
			p: Params{
				Extract:          true,
				PRFHashMechanism: CKM_SHA256,
				SaltType:         CKF_HKDF_SALT_NULL,
				InfoLen:          24,
			},
			wantErr: ErrInfoMismatch,
		},
		{
			name: "infoPtr != 0 but InfoLen == 0",
			p: Params{
				Extract:          true,
				PRFHashMechanism: CKM_SHA256,
				SaltType:         CKF_HKDF_SALT_NULL,
			},
			infoPtr: sentinelInfoPtr,
			wantErr: ErrInfoMismatch,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Marshal(tc.p, tc.saltPtr, tc.infoPtr)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

// TestSizeAndPointerAssumptions schützt die 64-Bit-LP64-Annahme
// (siehe doc.go-Build-Tag). Wenn diese Asserts knacken, ist das
// Target-Layout für CK_HKDF_PARAMS nicht das, was Marshal annimmt.
func TestSizeAndPointerAssumptions(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Fatalf("uintptr size = %d, expected 8 (LP64 only)", unsafe.Sizeof(uintptr(0)))
	}
	if ParamsSize != 64 {
		t.Fatalf("ParamsSize = %d, expected 64", ParamsSize)
	}
}

// TestMechanismLiterals schützt vor Tippfehlern in den
// PKCS#11-Mechanismus-IDs. Andere Tests dieses Pakets nutzen
// dieselben Paketkonstanten und würden einen Tippfehler (z. B.
// 0x402D ist CKM_SALSA20_KEY_GEN, nicht CKM_HKDF_DERIVE) nicht
// erkennen. Werte sind aus dem OASIS-PKCS#11-v3.0-Header
// (pkcs11.h) verifiziert.
func TestMechanismLiterals(t *testing.T) {
	cases := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"CKM_HKDF_DERIVE", CKM_HKDF_DERIVE, 0x0000402A},
		{"CKM_SHA256", CKM_SHA256, 0x00000250},
		{"CKM_SHA256_HMAC", CKM_SHA256_HMAC, 0x00000251},
		{"CKM_SHA384", CKM_SHA384, 0x00000260},
		{"CKM_SHA384_HMAC", CKM_SHA384_HMAC, 0x00000261},
		{"CKM_SHA512", CKM_SHA512, 0x00000270},
		{"CKM_SHA512_HMAC", CKM_SHA512_HMAC, 0x00000271},
		{"CKF_HKDF_SALT_NULL", CKF_HKDF_SALT_NULL, 0x00000001},
		{"CKF_HKDF_SALT_DATA", CKF_HKDF_SALT_DATA, 0x00000002},
		{"CKF_HKDF_SALT_KEY", CKF_HKDF_SALT_KEY, 0x00000004},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = 0x%08X, want 0x%08X", tc.name, tc.got, tc.want)
		}
	}
}

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("invalid hex literal in test: %v", err)
	}
	return b
}
