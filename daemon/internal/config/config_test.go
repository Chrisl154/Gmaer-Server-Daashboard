package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLoad_MissingFile_ReturnsDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/path/daemon.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.BindAddr != ":8443" {
		t.Errorf("BindAddr = %q, want :8443", cfg.BindAddr)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", cfg.ShutdownTimeout)
	}
	if cfg.Auth.TokenTTL != 2*time.Hour {
		t.Errorf("TokenTTL = %v, want 2h", cfg.Auth.TokenTTL)
	}
	if cfg.Backup.RetainDays != 30 {
		t.Errorf("Backup.RetainDays = %d, want 30", cfg.Backup.RetainDays)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("bind_addr: [not: valid: yaml"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestWriteAndLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.yaml")

	original := defaults()
	original.BindAddr = ":9443"
	original.LogLevel = "debug"
	original.Backup.RetainDays = 7

	if err := Write(path, original); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if loaded.BindAddr != ":9443" {
		t.Errorf("BindAddr = %q, want :9443", loaded.BindAddr)
	}
	if loaded.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", loaded.LogLevel)
	}
	if loaded.Backup.RetainDays != 7 {
		t.Errorf("Backup.RetainDays = %d, want 7", loaded.Backup.RetainDays)
	}
}

func TestDefaults_MetricsEnabled(t *testing.T) {
	cfg := defaults()
	if !cfg.Metrics.Enabled {
		t.Error("expected Metrics.Enabled=true by default")
	}
	if cfg.Metrics.Path != "/metrics" {
		t.Errorf("Metrics.Path = %q, want /metrics", cfg.Metrics.Path)
	}
}

func TestDefaults_SecretsBackend(t *testing.T) {
	cfg := defaults()
	if cfg.Secrets.Backend != "local" {
		t.Errorf("Secrets.Backend = %q, want local", cfg.Secrets.Backend)
	}
}

func TestWrite_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.yaml")
	if err := Write(path, defaults()); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected non-empty config file")
	}
	// Unix-only: verify restrictive permissions
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", info.Mode().Perm())
	}
}
