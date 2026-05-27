// Package grpcadapter ist der driving gRPC-Adapter des Servers.
//
// Slice 001 liefert nur das Skeleton: alle RPC-Methoden geben
// codes.Unimplemented zurück. Die echten Use-Cases (Encrypt-Stream
// mit Chunk-State-Machine, Decrypt mit Tag-Mismatch-Behandlung,
// ListKeys mit Tenant-Filter, Health mit echten Pool-/HSM-Werten)
// folgen in M1-Folge-Slices.
//
// Bezug:
//   - HSM-API-GRPC-001..004 (Lastenheft, Spezifikation)
//   - HSM-ARCH-001 (hexagonale Architektur; dieser Adapter ist driving)
package grpcadapter

import (
	chsmdocv1 "github.com/pt9912/c-hsm-doc/internal/gen/chsmdocv1"
)

// Server ist die HsmDocService-Implementierung für Slice 001.
//
// Der eingebettete UnimplementedHsmDocServiceServer liefert für alle
// vier RPCs codes.Unimplemented. Folge-Slices ersetzen ihn schrittweise
// durch echte Methoden.
type Server struct {
	chsmdocv1.UnimplementedHsmDocServiceServer
}

// NewServer erzeugt einen neuen Skeleton-Server.
func NewServer() *Server {
	return &Server{}
}
