//go:build spike && cgo && (amd64 || arm64)

// Package profilbspike trägt den Probe-Code für den 002b-Profil-B-
// Spike (Software-HMAC-Konstruktion gegen SoftHSM v2 + Bouncy HSM).
//
// Build-Tag spike isoliert das Paket vom regulären Repo-Build
// (siehe ../README.md §4 und ADR 0005 §2.2). cgo + amd64||arm64 sind
// für die HSM-CGO-Pfade über github.com/miekg/pkcs11 nötig; die
// Pure-Go-Referenz wird aus
// github.com/pt9912/c-hsm-doc/docs/plan/planning/next/002b-spike-hkdf/spike
// importiert, das exakt dasselbe Build-Tag-Set trägt.
//
// Bezug: HSM-FMT-006 §1 Profil B (Software-HMAC-Konstruktion),
// ADR 0007 §2.2 (Profil B als M1-Default), ADR 0007 §4 (PRK-Zeroize-
// Invariante).
package profilbspike
