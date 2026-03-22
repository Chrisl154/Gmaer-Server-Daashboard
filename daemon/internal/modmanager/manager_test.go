package modmanager

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(t.TempDir(), zap.NewNop())
}

func TestNewManager_NotNil(t *testing.T) {
	m := newTestManager(t)
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
}

func TestListMods_Empty(t *testing.T) {
	m := newTestManager(t)
	mods := m.ListMods("server-1")
	if len(mods) != 0 {
		t.Errorf("expected 0 mods, got %d", len(mods))
	}
}

func TestUninstall_NotFound(t *testing.T) {
	m := newTestManager(t)
	err := m.Uninstall(context.Background(), "server-1", "nonexistent")
	if err == nil {
		t.Error("expected error for uninstalling a mod that does not exist")
	}
}

func TestSnapshot_Rollback(t *testing.T) {
	m := newTestManager(t)
	sid := "srv-snap"

	// Add a mod directly
	m.mu.Lock()
	m.mods[sid] = []*Mod{{ID: "mod-a", Checksum: "sha256:abc"}}
	m.mu.Unlock()

	// Snapshot the current state
	m.Snapshot(sid)

	// Clear mods
	m.mu.Lock()
	m.mods[sid] = []*Mod{}
	m.mu.Unlock()

	// Rollback should restore mod-a
	if err := m.Rollback(context.Background(), sid, ""); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	mods := m.ListMods(sid)
	if len(mods) != 1 || mods[0].ID != "mod-a" {
		t.Errorf("expected [mod-a] after rollback, got %v", mods)
	}
}

func TestRollback_NoSnapshot_ClearsMods(t *testing.T) {
	m := newTestManager(t)
	sid := "srv-nosnap"

	m.mu.Lock()
	m.mods[sid] = []*Mod{{ID: "leftover", Checksum: "sha256:x"}}
	m.mu.Unlock()

	if err := m.Rollback(context.Background(), sid, ""); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	mods := m.ListMods(sid)
	if len(mods) != 0 {
		t.Errorf("expected empty mods after rollback with no snapshot, got %d", len(mods))
	}
}

func TestSnapshot_MaxFive(t *testing.T) {
	m := newTestManager(t)
	sid := "srv-max"

	for i := 0; i < 7; i++ {
		m.Snapshot(sid)
	}

	m.mu.RLock()
	count := len(m.snapshots[sid])
	m.mu.RUnlock()

	if count != 5 {
		t.Errorf("snapshot count = %d, want 5 (max retained)", count)
	}
}

func TestRunTests_NoMods(t *testing.T) {
	m := newTestManager(t)
	result, err := m.RunTests(context.Background(), "empty-server")
	if err != nil {
		t.Fatalf("RunTests: %v", err)
	}
	if !result.Passed {
		t.Error("expected RunTests to pass with no mods installed")
	}
	if len(result.Tests) != 3 {
		t.Errorf("expected 3 test entries, got %d", len(result.Tests))
	}
}

func TestRunTests_WithValidMods(t *testing.T) {
	m := newTestManager(t)
	sid := "srv-valid"

	m.mu.Lock()
	m.mods[sid] = []*Mod{
		{ID: "mod-1", Name: "Mod One", Checksum: "sha256:abc123"},
		{ID: "mod-2", Name: "Mod Two", Checksum: "sha256:def456"},
	}
	m.mu.Unlock()

	result, err := m.RunTests(context.Background(), sid)
	if err != nil {
		t.Fatalf("RunTests: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass with valid mods, tests: %+v", result.Tests)
	}
}

func TestRunTests_BadChecksum(t *testing.T) {
	m := newTestManager(t)
	sid := "srv-badchk"

	m.mu.Lock()
	m.mods[sid] = []*Mod{{ID: "bad-mod", Checksum: "sha256:error"}}
	m.mu.Unlock()

	result, err := m.RunTests(context.Background(), sid)
	if err != nil {
		t.Fatalf("RunTests: %v", err)
	}
	if result.Passed {
		t.Error("expected RunTests to fail when a mod has an invalid checksum")
	}
}

func TestGetJob_NotFound(t *testing.T) {
	m := newTestManager(t)
	_, ok := m.GetJob("does-not-exist")
	if ok {
		t.Error("expected ok=false for a missing job ID")
	}
}

func TestInstall_ReturnsJob(t *testing.T) {
	m := newTestManager(t)
	job, err := m.Install(context.Background(), "srv", InstallRequest{
		Source: SourceLocal,
		ModID:  "test-mod",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if job == nil {
		t.Fatal("expected non-nil job")
	}
	if job.ID == "" {
		t.Error("job ID should not be empty")
	}
	if job.Status != "pending" {
		t.Errorf("initial job status = %q, want pending", job.Status)
	}

	// Wait for async goroutine to complete (up to 2 s)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		j, ok := m.GetJob(job.ID)
		if ok && (j.Status == "success" || j.Status == "failed") {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	j, _ := m.GetJob(job.ID)
	t.Errorf("job still in status %q after 2 s", j.Status)
}

func TestBuildSourceURL_Thunderstore(t *testing.T) {
	got := buildSourceURL(SourceThunderstore, "MyNS-MyMod", "1.0.0")
	want := "https://thunderstore.io/package/download/MyNS/MyMod/1.0.0/"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildSourceURL_Modrinth(t *testing.T) {
	got := buildSourceURL(SourceModrinth, "my-mod", "2.0.0")
	want := "https://api.modrinth.com/v2/project/my-mod/version/2.0.0"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildSourceURL_Unknown(t *testing.T) {
	got := buildSourceURL("unknown-source", "mod-id", "1.0.0")
	if got != "" {
		t.Errorf("expected empty URL for unknown source, got %q", got)
	}
}
