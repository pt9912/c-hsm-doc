//go:build cgo

// Package main implementiert das pkcs11-dlopen-smoke-Hilfsbinary aus
// ADR 0004 §2.5: ein winziges Cgo-Programm, das dlopen() auf ein
// PKCS#11-Modul ausführt und damit die Library-Closure des
// Runtime-Images echt zur Laufzeit verifiziert (Distroless hat keine
// Shell und kein ldd, deshalb ein eigenes Binary).
//
// Aufrufpfade:
//   - make smoke-dlopen (CI-Pfad, manuelle Diagnose)
//   - Slice 002b: synchron-blockierender Startup-Hook in cmd/hsmdoc
//     vor C_Initialize/Pool-Aufbau; Exit ≠ 0 → STARTUP_PKCS11_DLOPEN_FAILED.
package main

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

func main() {
	os.Exit(run(os.Args))
}

// run kapselt die Hauptlogik, damit defer-Statements (C.free) vor
// dem Prozess-Exit garantiert greifen — gocritic exitAfterDefer
// schlägt sonst an, weil os.Exit innerhalb von main den Stack
// abreißt.
func run(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: pkcs11-dlopen-smoke <module-path>")
		return 2
	}
	module := args[1]

	cModule := C.CString(module)
	defer C.free(unsafe.Pointer(cModule))

	handle := C.dlopen(cModule, C.RTLD_NOW)
	if handle == nil {
		errMsg := "<no dlerror>"
		if cErr := C.dlerror(); cErr != nil {
			errMsg = C.GoString(cErr)
		}
		fmt.Fprintf(os.Stderr, "pkcs11-dlopen-smoke: dlopen failed for %s: %s\n", module, errMsg)
		return 1
	}

	if rc := C.dlclose(handle); rc != 0 {
		errMsg := "<no dlerror>"
		if cErr := C.dlerror(); cErr != nil {
			errMsg = C.GoString(cErr)
		}
		fmt.Fprintf(os.Stderr, "pkcs11-dlopen-smoke: dlclose failed for %s: %s\n", module, errMsg)
		return 1
	}

	fmt.Printf("pkcs11-dlopen-smoke: dlopen ok for %s\n", module)
	return 0
}
