// Package config liest die Server-Konfiguration aus Environment-
// Variablen (12-Factor; HSM-NFA-OPS-001).
//
// Slice 001 deckt nur den Skeleton-Umfang: gRPC-Port, Health-Port,
// TLS-Material-Pfade. mTLS, Identity-Source (`HSM-API-GRPC-006..008`)
// und HSM/PKCS#11-Konfiguration folgen in späteren Slices.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Defaults für Slice 001 (siehe docs/plan/planning/in-progress/001-grpc-skeleton.md).
const (
	defaultGRPCPort   = 9443
	defaultHealthPort = 9090
)

// Config ist die zur Laufzeit aufgelöste Server-Konfiguration.
type Config struct {
	// GRPCPort ist der TCP-Port des gRPC-Listeners (`HSMDOC_GRPC_PORT`).
	GRPCPort int
	// HealthPort ist der TCP-Port des HTTP-Health-Listeners
	// (`HSMDOC_HEALTH_PORT`); bedient /healthz und /readyz
	// (HSM-API-CFG-001).
	HealthPort int
	// TLSCertPath ist der Dateipfad zum PEM-Server-Zertifikat
	// (`HSMDOC_TLS_CERT`). Pflicht.
	TLSCertPath string
	// TLSKeyPath ist der Dateipfad zum PEM-Server-Private-Key
	// (`HSMDOC_TLS_KEY`). Pflicht.
	TLSKeyPath string
}

// Load liest die Konfiguration aus dem Environment.
// Fehlende Pflichtwerte oder ungültige Ports führen zu einem
// Start-Abbruch gemäß HSM-OPS-CFG-002.
func Load() (Config, error) {
	cfg := Config{
		GRPCPort:    defaultGRPCPort,
		HealthPort:  defaultHealthPort,
		TLSCertPath: os.Getenv("HSMDOC_TLS_CERT"),
		TLSKeyPath:  os.Getenv("HSMDOC_TLS_KEY"),
	}

	if v := os.Getenv("HSMDOC_GRPC_PORT"); v != "" {
		p, err := parsePort("HSMDOC_GRPC_PORT", v)
		if err != nil {
			return Config{}, err
		}
		cfg.GRPCPort = p
	}
	if v := os.Getenv("HSMDOC_HEALTH_PORT"); v != "" {
		p, err := parsePort("HSMDOC_HEALTH_PORT", v)
		if err != nil {
			return Config{}, err
		}
		cfg.HealthPort = p
	}

	if cfg.GRPCPort == cfg.HealthPort {
		return Config{}, fmt.Errorf("config: HSMDOC_GRPC_PORT and HSMDOC_HEALTH_PORT must differ (both = %d)", cfg.GRPCPort)
	}
	if cfg.TLSCertPath == "" {
		return Config{}, errors.New("config: HSMDOC_TLS_CERT is required (TLS 1.3, HSM-API-GRPC-002)")
	}
	if cfg.TLSKeyPath == "" {
		return Config{}, errors.New("config: HSMDOC_TLS_KEY is required (TLS 1.3, HSM-API-GRPC-002)")
	}

	return cfg, nil
}

func parsePort(name, v string) (int, error) {
	p, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s = %q is not a valid integer: %w", name, v, err)
	}
	if p < 1 || p > 65535 {
		return 0, fmt.Errorf("config: %s = %d out of range [1, 65535]", name, p)
	}
	return p, nil
}
