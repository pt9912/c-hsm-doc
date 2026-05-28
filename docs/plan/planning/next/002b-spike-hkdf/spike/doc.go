//go:build spike && (amd64 || arm64)

// Package hkdfspike trägt den Probe-Code für den 002b-Spike
// (CKM_HKDF_DERIVE gegen SoftHSM v2 + OpenCryptoki).
//
// Build-Tag spike isoliert das Paket vom regulären Repo-Build
// (siehe ../README.md §4 und ADR 0005 §2.2). Der Tag amd64||arm64
// hält die CK_HKDF_PARAMS-Serialisierung an 64-Bit-LP64-Little-Endian
// gebunden — das CI-Build-Image aus Slice 002a ist debian12 auf amd64.
//
// Konstanten und Struct-Layout stammen aus PKCS#11 v3.0 §6.30
// (Mechanismus-IDs) und §6.31 (CK_HKDF_PARAMS); HSM-FMT-006 §1
// Profil A legt die Verwendung im Encrypt-Pfad fest.
package hkdfspike
