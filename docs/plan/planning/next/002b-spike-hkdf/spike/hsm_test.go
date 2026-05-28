//go:build spike && cgo && (amd64 || arm64)

package hkdfspike

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/miekg/pkcs11"
)

// TestHKDFEndToEndAgainstHSM ist der End-to-End-Spike-Test: Modul
// laden → Token finden → Login → Master-Key lookup → C_DeriveKey →
// Attribut-Check → C_Sign → byteweiser Vergleich gegen die
// Pure-Go-HKDF+HMAC-Referenz → C_DestroyObject → Post-Destroy-
// Sanity-Check.
//
// Skip, wenn keine HSM-Konfiguration über Env-Variablen anliegt —
// damit make spike-hkdf-test ohne HSM grün bleibt.
func TestHKDFEndToEndAgainstHSM(t *testing.T) {
	cfg, ok := loadHSMConfig()
	if !ok {
		t.Skip("SPIKE_PKCS11_MODULE not set — skipping HSM end-to-end test")
	}

	ctx, err := LoadModule(cfg.modulePath)
	if err != nil {
		t.Fatalf("LoadModule(%q): %v", cfg.modulePath, err)
	}

	var session pkcs11.SessionHandle
	t.Cleanup(func() {
		if err := Close(ctx, session); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	slot, err := FindTokenSlot(ctx, cfg.tokenLabel)
	if err != nil {
		t.Fatalf("FindTokenSlot: %v", err)
	}

	// Pre-Flight: ohne CKM_HKDF_DERIVE im Modul macht der Test keinen
	// Sinn — Skip mit klarem Hinweis statt CKR_MECHANISM_INVALID-Fail.
	// SoftHSM 2.6.1 + 2.7.0 und OpenCryptoki-Software-Token haben den
	// Mechanismus nicht (Spike-Befund 2026-05-28, siehe README §6).
	hasHKDF, err := HasMechanism(ctx, slot, uint(CKM_HKDF_DERIVE))
	if err != nil {
		t.Fatalf("HasMechanism: %v", err)
	}
	if !hasHKDF {
		t.Skipf("module %q does not advertise CKM_HKDF_DERIVE (0x402a) — Profil A unverfügbar", cfg.modulePath)
	}

	session, err = LoginUser(ctx, slot, cfg.userPIN)
	if err != nil {
		t.Fatalf("LoginUser: %v", err)
	}

	masterKey, err := FindSecretKey(ctx, session, cfg.masterHMACLabel)
	if err != nil {
		t.Fatalf("FindSecretKey(%q): %v", cfg.masterHMACLabel, err)
	}

	salt := []byte("spike-key-id-v1!")
	header := []byte("c-hsm-doc/test-header/v1")

	headerKey, err := DeriveHeaderKeyHSM(ctx, session, masterKey, salt, []byte(HeaderHMACInfo))
	if err != nil {
		t.Fatalf("DeriveHeaderKeyHSM: %v", err)
	}

	// Attribut-Check (kanonische Trace Schritt 6): SIGN, EXTRACTABLE,
	// SENSITIVE, VALUE_LEN müssen anliegen wie im Template gefordert.
	attrs, err := ctx.GetAttributeValue(session, headerKey, []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_SIGN, nil),
		pkcs11.NewAttribute(pkcs11.CKA_EXTRACTABLE, nil),
		pkcs11.NewAttribute(pkcs11.CKA_SENSITIVE, nil),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE_LEN, nil),
	})
	if err != nil {
		t.Fatalf("GetAttributeValue header key: %v", err)
	}
	checkBool := func(name string, idx int, want byte) {
		t.Helper()
		if len(attrs[idx].Value) == 0 {
			t.Errorf("%s value empty", name)
			return
		}
		if attrs[idx].Value[0] != want {
			t.Errorf("%s = %v, want %d", name, attrs[idx].Value, want)
		}
	}
	checkBool("CKA_SIGN", 0, 1)
	checkBool("CKA_EXTRACTABLE", 1, 0)
	checkBool("CKA_SENSITIVE", 2, 1)
	// CKA_VALUE_LEN: CK_ULONG little-endian; auf LP64-LE ist das erste Byte
	// die niedrigste Stelle. Wir erwarten 32 (0x20).
	if len(attrs[3].Value) == 0 || attrs[3].Value[0] != DerivedHeaderKeyLen {
		t.Errorf("CKA_VALUE_LEN = %v, want 32 in first byte", attrs[3].Value)
	}

	// Sensitive-Negativtest (kanonische Trace Schritt 7): CKA_VALUE
	// darf nicht enthüllt werden — miekg gibt entweder einen leeren
	// Value oder eine CKR_ATTRIBUTE_SENSITIVE-Error zurück.
	valAttrs, valErr := ctx.GetAttributeValue(session, headerKey, []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_VALUE, nil),
	})
	switch {
	case valErr == nil && len(valAttrs[0].Value) == 0:
		t.Logf("CKA_VALUE leak check ok: empty value")
	case valErr != nil:
		if pe, ok := valErr.(pkcs11.Error); ok && pe == pkcs11.CKR_ATTRIBUTE_SENSITIVE {
			t.Logf("CKA_VALUE leak check ok: CKR_ATTRIBUTE_SENSITIVE")
		} else {
			t.Errorf("unexpected CKA_VALUE read error: %v", valErr)
		}
	default:
		t.Errorf("CKA_VALUE leak: %d bytes returned", len(valAttrs[0].Value))
	}

	// C_Sign (kanonische Trace Schritt 8+9): HMAC-SHA-256 über
	// headerBytes mit dem abgeleiteten Header-Key.
	hsmTag, err := SignHeaderHMAC(ctx, session, headerKey, header)
	if err != nil {
		t.Fatalf("SignHeaderHMAC: %v", err)
	}

	// Pure-Go-Referenz: HMAC-SHA256(HKDF(FixtureIKM, salt, info, 32), header).
	expectedTag, err := ExpectedHeaderMAC(FixtureIKM, salt, []byte(HeaderHMACInfo), header)
	if err != nil {
		t.Fatalf("ExpectedHeaderMAC: %v", err)
	}

	if !bytes.Equal(hsmTag, expectedTag[:]) {
		t.Fatalf("HSM tag != Pure-Go reference\n hsm:  %s\n want: %s",
			hex.EncodeToString(hsmTag), hex.EncodeToString(expectedTag[:]))
	}
	t.Logf("end-to-end ok: HSM HMAC-SHA-256 tag matches Pure-Go reference (32 bytes)")

	// C_DestroyObject (kanonische Trace Schritt 10).
	if err := ctx.DestroyObject(session, headerKey); err != nil {
		t.Fatalf("C_DestroyObject: %v", err)
	}

	// Post-Destroy-Sanity (kanonische Trace Schritt 11): C_SignInit
	// auf zerstörtem Handle muss CKR_OBJECT_HANDLE_INVALID liefern.
	mech := []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_SHA256_HMAC, nil)}
	postErr := ctx.SignInit(session, mech, headerKey)
	if postErr == nil {
		t.Errorf("C_SignInit on destroyed handle succeeded — expected CKR_OBJECT_HANDLE_INVALID")
	} else {
		var pe pkcs11.Error
		if errors.As(postErr, &pe) && pe == pkcs11.CKR_OBJECT_HANDLE_INVALID {
			t.Logf("post-destroy SignInit ok: CKR_OBJECT_HANDLE_INVALID")
		} else {
			t.Logf("post-destroy SignInit returned %v (accepted — module-specific error class)", postErr)
		}
	}
}

type hsmConfig struct {
	modulePath      string
	tokenLabel      string
	userPIN         string
	masterHMACLabel string
}

func loadHSMConfig() (hsmConfig, bool) {
	module := os.Getenv("SPIKE_PKCS11_MODULE")
	if module == "" {
		return hsmConfig{}, false
	}
	return hsmConfig{
		modulePath:      module,
		tokenLabel:      envOr("SPIKE_PKCS11_TOKEN", "c-hsm-doc-spike"),
		userPIN:         envOr("SPIKE_PKCS11_PIN", "1234"),
		masterHMACLabel: envOr("SPIKE_MASTER_HMAC_LABEL", "spike-master-hmac"),
	}, true
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
