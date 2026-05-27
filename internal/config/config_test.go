package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tlsFiles legt zwei reguläre Files im TempDir an und gibt ihre Pfade
// zurück. config.Load() verlangt Datei-Existenz, nicht TLS-Gültigkeit;
// leerer Inhalt reicht.
func tlsFiles(t *testing.T) (cert, key string) {
	t.Helper()
	dir := t.TempDir()
	cert = filepath.Join(dir, "server.crt")
	key = filepath.Join(dir, "server.key")
	for _, p := range []string{cert, key} {
		if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return cert, key
}

func setEnvBaseline(t *testing.T) (cert, key string) {
	t.Helper()
	cert, key = tlsFiles(t)
	t.Setenv("HSMDOC_TLS_CERT", cert)
	t.Setenv("HSMDOC_TLS_KEY", key)
	// Defensiv: distinkte Ports setzen, damit Tests, die nur eine Variable
	// kippen, nicht versehentlich in die Port-Kollision laufen.
	t.Setenv("HSMDOC_GRPC_PORT", "")
	t.Setenv("HSMDOC_HEALTH_PORT", "")
	return cert, key
}

func TestLoadDefaults(t *testing.T) {
	setEnvBaseline(t)
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
	setEnvBaseline(t)
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

func TestLoadTrimsWhitespace(t *testing.T) {
	cert, key := tlsFiles(t)
	t.Setenv("HSMDOC_TLS_CERT", "  "+cert+"\n")
	t.Setenv("HSMDOC_TLS_KEY", "\t"+key+" ")
	t.Setenv("HSMDOC_GRPC_PORT", " 18443\n")
	t.Setenv("HSMDOC_HEALTH_PORT", "18090 ")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TLSCertPath != cert || cfg.TLSKeyPath != key {
		t.Errorf("paths not trimmed: cert=%q key=%q", cfg.TLSCertPath, cfg.TLSKeyPath)
	}
	if cfg.GRPCPort != 18443 || cfg.HealthPort != 18090 {
		t.Errorf("ports = (%d, %d), want (18443, 18090)", cfg.GRPCPort, cfg.HealthPort)
	}
}

func TestLoadRejectsMissingTLS(t *testing.T) {
	cert, key := tlsFiles(t)
	// Sichern, dass die Port-Kollisions-Prüfung nicht vor der TLS-Prüfung
	// feuert.
	t.Setenv("HSMDOC_GRPC_PORT", "18001")
	t.Setenv("HSMDOC_HEALTH_PORT", "18002")

	cases := []struct {
		name string
		cert string
		key  string
		want string
	}{
		{"missing cert", "", key, "HSMDOC_TLS_CERT"},
		{"missing key", cert, "", "HSMDOC_TLS_KEY"},
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

func TestLoadRejectsUnreadableTLSPath(t *testing.T) {
	setEnvBaseline(t)
	dir := t.TempDir()
	// Pfad auf ein Verzeichnis, nicht auf eine Datei.
	t.Setenv("HSMDOC_TLS_CERT", dir)
	_, err := Load()
	if err == nil {
		t.Fatal("Load: expected directory-not-a-file error, got nil")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("Load error = %q, want substring 'directory'", err.Error())
	}
}

func TestLoadRejectsNonExistentTLSPath(t *testing.T) {
	setEnvBaseline(t)
	t.Setenv("HSMDOC_TLS_CERT", "/tmp/nonexistent-c-hsm-doc-cert-xyz")
	_, err := Load()
	if err == nil {
		t.Fatal("Load: expected not-accessible error, got nil")
	}
	if !strings.Contains(err.Error(), "not accessible") {
		t.Errorf("Load error = %q, want substring 'not accessible'", err.Error())
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
			setEnvBaseline(t)
			t.Setenv(c.env, c.val)
			_, err := Load()
			if err == nil {
				t.Fatalf("Load with %s=%s: expected error, got nil", c.env, c.val)
			}
		})
	}
}

func TestLoadRejectsCollidingPorts(t *testing.T) {
	setEnvBaseline(t)
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
