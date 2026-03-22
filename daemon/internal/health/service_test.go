package health

import (
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.checks == nil {
		t.Fatal("checks map is nil")
	}
	if svc.startTime.IsZero() {
		t.Fatal("startTime not set")
	}
}

func TestCheck_NoChecks(t *testing.T) {
	svc := NewService()
	status := svc.Check()

	if status == nil {
		t.Fatal("Check returned nil")
	}
	if !status.Healthy {
		t.Error("expected Healthy=true with no registered checks")
	}
	if len(status.Components) != 0 {
		t.Errorf("expected 0 components, got %d", len(status.Components))
	}
	if status.Timestamp.IsZero() {
		t.Error("Timestamp not set")
	}
}

func TestCheck_AllHealthy(t *testing.T) {
	svc := NewService()
	svc.RegisterCheck("db", func() Check { return Check{Healthy: true, Message: "ok"} })
	svc.RegisterCheck("cache", func() Check { return Check{Healthy: true} })

	status := svc.Check()

	if !status.Healthy {
		t.Error("expected Healthy=true when all checks pass")
	}
	if len(status.Components) != 2 {
		t.Errorf("expected 2 components, got %d", len(status.Components))
	}
	if !status.Components["db"].Healthy {
		t.Error("db component should be healthy")
	}
}

func TestCheck_OneUnhealthy(t *testing.T) {
	svc := NewService()
	svc.RegisterCheck("db", func() Check { return Check{Healthy: true} })
	svc.RegisterCheck("queue", func() Check { return Check{Healthy: false, Message: "connection refused"} })

	status := svc.Check()

	if status.Healthy {
		t.Error("expected Healthy=false when any check fails")
	}
	if status.Components["queue"].Healthy {
		t.Error("queue component should be unhealthy")
	}
	if status.Components["queue"].Message != "connection refused" {
		t.Errorf("unexpected message: %s", status.Components["queue"].Message)
	}
}

func TestRegisterCheck_Overwrites(t *testing.T) {
	svc := NewService()
	svc.RegisterCheck("probe", func() Check { return Check{Healthy: false} })
	svc.RegisterCheck("probe", func() Check { return Check{Healthy: true} })

	status := svc.Check()
	if !status.Components["probe"].Healthy {
		t.Error("second RegisterCheck should overwrite the first")
	}
}

func TestSystemStatus(t *testing.T) {
	svc := NewService()
	ss := svc.SystemStatus()

	if ss == nil {
		t.Fatal("SystemStatus returned nil")
	}
	if ss.Version == "" {
		t.Error("Version should not be empty")
	}
	if ss.Uptime < 0 {
		t.Error("Uptime should be non-negative")
	}
	if ss.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}
}

func TestSystemStatus_UptimeIncreases(t *testing.T) {
	svc := NewService()
	before := svc.SystemStatus().Uptime
	time.Sleep(1100 * time.Millisecond)
	after := svc.SystemStatus().Uptime
	if after <= before {
		t.Errorf("Uptime should increase over time: before=%d after=%d", before, after)
	}
}
