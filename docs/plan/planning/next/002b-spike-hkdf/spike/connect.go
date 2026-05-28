//go:build spike && cgo && (amd64 || arm64)

package hkdfspike

import (
	"fmt"
	"strings"

	"github.com/miekg/pkcs11"
)

// LoadModule lädt das PKCS#11-Modul und führt C_Initialize aus.
// Der Aufrufer ist verpflichtet, Close zu rufen.
func LoadModule(modulePath string) (*pkcs11.Ctx, error) {
	ctx := pkcs11.New(modulePath)
	if ctx == nil {
		return nil, fmt.Errorf("hkdfspike: pkcs11.New(%q) returned nil", modulePath)
	}
	if err := ctx.Initialize(); err != nil {
		ctx.Destroy()
		return nil, fmt.Errorf("hkdfspike: C_Initialize: %w", err)
	}
	return ctx, nil
}

// Close fährt das Modul deterministisch runter. Logout darf scheitern
// (z. B. wenn nicht eingeloggt) — der Rest läuft trotzdem durch.
func Close(ctx *pkcs11.Ctx, session pkcs11.SessionHandle) error {
	var firstErr error
	if session != 0 {
		if err := ctx.Logout(session); err != nil {
			if pe, ok := err.(pkcs11.Error); !ok || pe != pkcs11.CKR_USER_NOT_LOGGED_IN {
				firstErr = err
			}
		}
		if err := ctx.CloseSession(session); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := ctx.Finalize(); err != nil && firstErr == nil {
		firstErr = err
	}
	ctx.Destroy()
	return firstErr
}

// FindTokenSlot sucht den Slot, dessen Token-Label exakt matcht.
// PKCS#11-Token-Labels werden mit Spaces (und teils NULs) auf
// 32 Byte gepadded — beide werden vor dem Vergleich getrimmt.
func FindTokenSlot(ctx *pkcs11.Ctx, tokenLabel string) (uint, error) {
	slots, err := ctx.GetSlotList(true)
	if err != nil {
		return 0, fmt.Errorf("hkdfspike: C_GetSlotList: %w", err)
	}
	for _, slot := range slots {
		info, err := ctx.GetTokenInfo(slot)
		if err != nil {
			continue
		}
		if strings.TrimRight(info.Label, " \x00") == tokenLabel {
			return slot, nil
		}
	}
	return 0, fmt.Errorf("hkdfspike: token %q not found in initialized slots", tokenLabel)
}

// LoginUser öffnet eine RW-Session und logt mit der User-PIN ein.
func LoginUser(ctx *pkcs11.Ctx, slot uint, pin string) (pkcs11.SessionHandle, error) {
	session, err := ctx.OpenSession(slot, pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		return 0, fmt.Errorf("hkdfspike: C_OpenSession: %w", err)
	}
	if err := ctx.Login(session, pkcs11.CKU_USER, pin); err != nil {
		_ = ctx.CloseSession(session)
		return 0, fmt.Errorf("hkdfspike: C_Login: %w", err)
	}
	return session, nil
}

// HasMechanism prüft via C_GetMechanismList, ob das Modul den
// gegebenen Mechanismus im angegebenen Slot anbietet. Spike nutzt
// das als Pre-Flight-Check vor C_DeriveKey/C_SignInit, damit
// Module ohne CKM_HKDF_DERIVE deterministisch geskippt werden
// statt mit CKR_MECHANISM_INVALID zu failen.
func HasMechanism(ctx *pkcs11.Ctx, slot uint, want uint) (bool, error) {
	mechs, err := ctx.GetMechanismList(slot)
	if err != nil {
		return false, fmt.Errorf("hkdfspike: C_GetMechanismList: %w", err)
	}
	for _, m := range mechs {
		if uint(m.Mechanism) == want {
			return true, nil
		}
	}
	return false, nil
}

// FindSecretKey sucht ein Secret-Key-Objekt mit dem gegebenen Label.
// Genau ein Treffer wird verlangt — null oder mehrere ist ein Fehler.
func FindSecretKey(ctx *pkcs11.Ctx, session pkcs11.SessionHandle, label string) (pkcs11.ObjectHandle, error) {
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_SECRET_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
	}
	if err := ctx.FindObjectsInit(session, template); err != nil {
		return 0, fmt.Errorf("hkdfspike: C_FindObjectsInit: %w", err)
	}
	handles, _, findErr := ctx.FindObjects(session, 8)
	finalErr := ctx.FindObjectsFinal(session)
	if findErr != nil {
		return 0, fmt.Errorf("hkdfspike: C_FindObjects: %w", findErr)
	}
	if finalErr != nil {
		return 0, fmt.Errorf("hkdfspike: C_FindObjectsFinal: %w", finalErr)
	}
	switch len(handles) {
	case 0:
		return 0, fmt.Errorf("hkdfspike: secret key %q not found", label)
	case 1:
		return handles[0], nil
	default:
		return 0, fmt.Errorf("hkdfspike: %d secret keys match label %q (expected exactly 1)", len(handles), label)
	}
}
