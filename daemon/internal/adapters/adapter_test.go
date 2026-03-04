package adapters

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func newTestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	l, _ := zap.NewDevelopment()
	return l
}

func TestRegistry_Defaults(t *testing.T) {
	r, err := NewRegistry("", newTestLogger(t))
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}

	expected := []string{"valheim", "minecraft", "satisfactory", "palworld", "eco", "enshrouded", "riftbreaker"}
	for _, id := range expected {
		if _, ok := r.Get(id); !ok {
			t.Errorf("expected adapter %q to be loaded", id)
		}
	}

	if !r.Exists("valheim") {
		t.Error("Exists should return true for valheim")
	}
	if r.Exists("nonexistent") {
		t.Error("Exists should return false for unknown adapter")
	}
}

func TestRegistry_List(t *testing.T) {
	r, _ := NewRegistry("", newTestLogger(t))
	manifests := r.List()
	if len(manifests) == 0 {
		t.Error("List should return non-empty slice")
	}
}

func TestRegistry_DefaultPorts(t *testing.T) {
	r, _ := NewRegistry("", newTestLogger(t))

	ports := r.DefaultPorts("valheim")
	if len(ports) == 0 {
		t.Fatal("valheim should have default ports")
	}

	// Valheim uses UDP
	for _, p := range ports {
		if p.Protocol != "udp" {
			t.Errorf("expected UDP port for valheim, got %s", p.Protocol)
		}
	}

	ports = r.DefaultPorts("minecraft")
	if len(ports) == 0 {
		t.Fatal("minecraft should have default ports")
	}
	if ports[0].Protocol != "tcp" {
		t.Errorf("expected TCP port for minecraft game port, got %s", ports[0].Protocol)
	}
	if ports[0].Internal != 25565 {
		t.Errorf("expected minecraft port 25565, got %d", ports[0].Internal)
	}
}

func TestRegistry_BackupPaths(t *testing.T) {
	r, _ := NewRegistry("", newTestLogger(t))

	paths := r.BackupPaths("minecraft")
	if len(paths) == 0 {
		t.Error("minecraft should have backup paths")
	}
}

func TestRegistry_LoadFromDir(t *testing.T) {
	// Create a temp dir with a minimal manifest
	dir := t.TempDir()
	adapterDir := filepath.Join(dir, "testgame")
	if err := os.MkdirAll(adapterDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `id: testgame
name: Test Game Server
version: "1.0.0"
deploy_methods:
  - manual
ports:
  - internal: 9999
    default_external: 9999
    protocol: udp
    description: Game port
`
	if err := os.WriteFile(filepath.Join(adapterDir, "manifest.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := NewRegistry(dir, newTestLogger(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, ok := r.Get("testgame")
	if !ok {
		t.Fatal("testgame adapter should be loaded from directory")
	}
	if m.Name != "Test Game Server" {
		t.Errorf("unexpected name: %s", m.Name)
	}
	if len(m.Ports) != 1 || m.Ports[0].Internal != 9999 {
		t.Error("port not loaded correctly")
	}
}

func TestRegistry_HealthCheck_UnknownAdapter(t *testing.T) {
	r, _ := NewRegistry("", newTestLogger(t))

	result := r.RunHealthCheck(context.Background(), "nonexistent", "localhost")
	if result.Healthy {
		t.Error("unknown adapter should return unhealthy")
	}
}

func TestRegistry_HealthCheck_KnownAdapter(t *testing.T) {
	r, _ := NewRegistry("", newTestLogger(t))

	// valheim health checks use UDP which will fail since no server is running
	// but the registry call itself should not panic
	result := r.RunHealthCheck(context.Background(), "valheim", "127.0.0.1")
	// We don't assert Healthy here because no server is running in the test environment
	_ = result
}
