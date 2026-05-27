//go:build !cgo

// Stub für die Pure-Go-Coverage-Stage (ADR 0002 §2.5,
// CGO_ENABLED=0 go test ./...). Das produktive Smoke-Binary aus
// ADR 0004 §2.5 ist Cgo-pflichtig (dlopen über <dlfcn.h>); dieser
// Stub erlaubt einen sauberen `go test ./...`-Lauf, ohne dass die
// Coverage-Stage am Paket scheitert.
package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(run())
}

func run() int {
	fmt.Fprintln(os.Stderr, "pkcs11-dlopen-smoke requires CGO_ENABLED=1 (see ADR 0004 §2.5)")
	return 2
}
