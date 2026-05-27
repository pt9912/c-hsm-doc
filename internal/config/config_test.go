package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HSMDOC_TLS_CERT", "/etc/c-hsm-doc/server.crt")
	t.Setenv("HSMDOC_TLS_KEY", "/etc/c-hsm-doc/server.key")
	t.Setenv("HSMDOC_GRPC_PORT", "")
	t.Setenv("HSMDOC_HEALTH_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GRPCPort != defaultGRPCPort {
		t.Errorf("GRPCPort = %d, want %d", cfg.GRPCPort, defaultGRPCPort)
	}
	if cfg.HealthPort != defaultHealthPort {
		t.Errorf("HealthPort = %d, want %d", cfg.HealthPort, defaultHealthPort)
	}
}

func TestLoadCustomPorts(t *testing.T) {
	t.Setenv("HSMDOC_TLS_CERT", "/x.crt")
	t.Setenv("HSMDOC_TLS_KEY", "/x.key")
	t.Setenv("HSMDOC_GRPC_PORT", "18443")
	t.Setenv("HSMDOC_HEALTH_PORT", "18090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GRPCPort != 18443 || cfg.HealthPort != 18090 {
		t.Errorf("ports = (%d, %d), want (18443, 18090)", cfg.GRPCPort, cfg.HealthPort)
	}
}

func TestLoadRejectsMissingTLS(t *testing.T) {
	cases := []struct {
		name string
		cert string
		key  string
		want string
	}{
		{"missing cert", "", "/x.key", "HSMDOC_TLS_CERT"},
		{"missing key", "/x.crt", "", "HSMDOC_TLS_KEY"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("HSMDOC_TLS_CERT", c.cert)
			t.Setenv("HSMDOC_TLS_KEY", c.key)
			_, err := Load()
			if err == nil {
				t.Fatal("Load: expected error, got nil")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("Load error = %q, want substring %q", err.Error(), c.want)
			}
		})
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	cases := []struct {
		name string
		env  string
		val  string
	}{
		{"non-integer", "HSMDOC_GRPC_PORT", "not-a-number"},
		{"zero", "HSMDOC_GRPC_PORT", "0"},
		{"too high", "HSMDOC_HEALTH_PORT", "70000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("HSMDOC_TLS_CERT", "/x.crt")
			t.Setenv("HSMDOC_TLS_KEY", "/x.key")
			t.Setenv(c.env, c.val)
			_, err := Load()
			if err == nil {
				t.Fatalf("Load with %s=%s: expected error, got nil", c.env, c.val)
			}
		})
	}
}

func TestLoadRejectsCollidingPorts(t *testing.T) {
	t.Setenv("HSMDOC_TLS_CERT", "/x.crt")
	t.Setenv("HSMDOC_TLS_KEY", "/x.key")
	t.Setenv("HSMDOC_GRPC_PORT", "8000")
	t.Setenv("HSMDOC_HEALTH_PORT", "8000")
	_, err := Load()
	if err == nil {
		t.Fatal("Load: expected error for colliding ports, got nil")
	}
	if !strings.Contains(err.Error(), "must differ") {
		t.Errorf("error = %q, want 'must differ'", err.Error())
	}
}
