package broker

import (
	"context"
	"testing"
	"time"

	"github.com/games-dashboard/daemon/internal/config"
	"github.com/games-dashboard/daemon/internal/metrics"
	"go.uber.org/zap"
)

func newTestBroker(t *testing.T) *Broker {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	cfg := config.Config{}
	b, err := NewBroker(&cfg, nil, logger, metrics.NewService())
	if err != nil {
		t.Fatalf("failed to create broker: %v", err)
	}
	return b
}

func TestCreateServer(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	req := CreateServerRequest{
		ID:           "valheim-1",
		Name:         "My Valheim Server",
		Adapter:      "valheim",
		DeployMethod: "steamcmd",
		InstallDir:   "/opt/valheim",
		Ports: []PortMapping{
			{Internal: 2456, External: 2456, Protocol: "udp"},
		},
		Resources: ResourceSpec{CPUCores: 2, RAMGB: 4, DiskGB: 5},
	}

	server, err := b.CreateServer(ctx, req)
	if err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	if server.ID != req.ID {
		t.Errorf("expected ID %s, got %s", req.ID, server.ID)
	}
	if server.State != StateIdle {
		t.Errorf("expected idle state, got %s", server.State)
	}
	if server.Adapter != "valheim" {
		t.Errorf("expected valheim adapter, got %s", server.Adapter)
	}
}

func TestCreateServer_Duplicate(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	req := CreateServerRequest{ID: "dup-1", Name: "Dup", Adapter: "minecraft"}
	if _, err := b.CreateServer(ctx, req); err != nil {
		t.Fatal(err)
	}

	_, err := b.CreateServer(ctx, req)
	if err == nil {
		t.Error("expected error for duplicate server ID")
	}
}

func TestGetServer(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	req := CreateServerRequest{ID: "mc-1", Name: "Minecraft", Adapter: "minecraft"}
	_, _ = b.CreateServer(ctx, req)

	server, err := b.GetServer(ctx, "mc-1")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if server.ID != "mc-1" {
		t.Errorf("unexpected server ID: %s", server.ID)
	}
}

func TestGetServer_NotFound(t *testing.T) {
	b := newTestBroker(t)
	_, err := b.GetServer(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for unknown server ID")
	}
}

func TestListServers(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		req := CreateServerRequest{
			ID:      string(rune('a' + i)),
			Name:    "Server",
			Adapter: "valheim",
		}
		if _, err := b.CreateServer(ctx, req); err != nil {
			t.Fatal(err)
		}
	}

	servers, err := b.ListServers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 3 {
		t.Errorf("expected 3 servers, got %d", len(servers))
	}
}

func TestUpdateServer(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	_, _ = b.CreateServer(ctx, CreateServerRequest{ID: "upd-1", Name: "Old Name", Adapter: "eco"})

	updated, err := b.UpdateServer(ctx, "upd-1", UpdateServerRequest{
		Name:      "New Name",
		Resources: &ResourceSpec{CPUCores: 4, RAMGB: 8, DiskGB: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected updated name, got %s", updated.Name)
	}
	if updated.Resources.CPUCores != 4 {
		t.Errorf("expected 4 CPU cores, got %d", updated.Resources.CPUCores)
	}
}

func TestDeleteServer(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	_, _ = b.CreateServer(ctx, CreateServerRequest{ID: "del-1", Name: "ToDelete", Adapter: "palworld"})

	if err := b.DeleteServer(ctx, "del-1"); err != nil {
		t.Fatalf("DeleteServer failed: %v", err)
	}

	_, err := b.GetServer(ctx, "del-1")
	if err == nil {
		t.Error("server should not exist after deletion")
	}
}

func TestDeleteServer_Running(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	_, _ = b.CreateServer(ctx, CreateServerRequest{ID: "run-1", Name: "Running", Adapter: "valheim"})

	// Force running state
	b.mu.Lock()
	b.servers["run-1"].State = StateRunning
	b.mu.Unlock()

	err := b.DeleteServer(ctx, "run-1")
	if err == nil {
		t.Error("expected error when deleting running server")
	}
}

func TestStartStopServer(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	_, _ = b.CreateServer(ctx, CreateServerRequest{ID: "ss-1", Name: "SS", Adapter: "minecraft"})

	if err := b.StartServer(ctx, "ss-1"); err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}

	// Allow goroutine to transition state
	time.Sleep(100 * time.Millisecond)

	b.mu.RLock()
	state := b.servers["ss-1"].State
	b.mu.RUnlock()

	if state != StateStarting && state != StateRunning {
		t.Errorf("unexpected state after start: %s", state)
	}

	// Wait for running
	time.Sleep(3 * time.Second)

	b.mu.RLock()
	state = b.servers["ss-1"].State
	b.mu.RUnlock()

	if state != StateRunning {
		t.Errorf("expected running state, got %s", state)
	}

	if err := b.StopServer(ctx, "ss-1"); err != nil {
		t.Fatalf("StopServer failed: %v", err)
	}
}

func TestStartServer_AlreadyRunning(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	_, _ = b.CreateServer(ctx, CreateServerRequest{ID: "ar-1", Name: "AR", Adapter: "valheim"})
	b.mu.Lock()
	b.servers["ar-1"].State = StateRunning
	b.mu.Unlock()

	err := b.StartServer(ctx, "ar-1")
	if err == nil {
		t.Error("expected error when starting already-running server")
	}
}

func TestGetServerStatus(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	_, _ = b.CreateServer(ctx, CreateServerRequest{ID: "stat-1", Name: "Stat", Adapter: "satisfactory"})

	status, err := b.GetServerStatus(ctx, "stat-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := status["id"]; !ok {
		t.Error("status should contain id field")
	}
	if _, ok := status["state"]; !ok {
		t.Error("status should contain state field")
	}
}

func TestTriggerBackup(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	_, _ = b.CreateServer(ctx, CreateServerRequest{ID: "bk-1", Name: "BK", Adapter: "valheim"})

	job, err := b.TriggerBackup(ctx, "bk-1", BackupRequest{Type: "full"})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" {
		t.Error("expected non-empty job ID")
	}
	if job.ServerID != "bk-1" {
		t.Errorf("expected server ID bk-1, got %s", job.ServerID)
	}
}

func TestListBackups(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	_, _ = b.CreateServer(ctx, CreateServerRequest{ID: "lb-1", Name: "LB", Adapter: "minecraft"})
	_, _ = b.TriggerBackup(ctx, "lb-1", BackupRequest{Type: "full"})

	// Wait for backup goroutine
	time.Sleep(3 * time.Second)

	backups, err := b.ListBackups(ctx, "lb-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) == 0 {
		t.Error("expected at least one backup after triggering")
	}
}

func TestInstallMod(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	_, _ = b.CreateServer(ctx, CreateServerRequest{ID: "mod-1", Name: "Mod", Adapter: "minecraft"})

	job, err := b.InstallMod(ctx, "mod-1", InstallModRequest{
		Source:  "local",
		ModID:   "test-mod",
		Version: "1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" {
		t.Error("expected non-empty job ID")
	}
}

func TestValidatePorts(t *testing.T) {
	b := newTestBroker(t)
	ctx := context.Background()

	result, err := b.ValidatePorts(ctx, ValidatePortsRequest{
		Ports: []PortMapping{
			{Internal: 25565, External: 25565, Protocol: "tcp"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(result.Results))
	}
}

func TestGetSBOM(t *testing.T) {
	b := newTestBroker(t)
	sbom, err := b.GetSBOM(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sbom == nil {
		t.Error("expected non-nil SBOM")
	}
	if _, ok := sbom["bomFormat"]; !ok {
		t.Error("SBOM should contain bomFormat field")
	}
}

func TestGetCVEReport(t *testing.T) {
	b := newTestBroker(t)
	report, err := b.GetCVEReport(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report == nil {
		t.Error("expected non-nil CVE report")
	}
}
