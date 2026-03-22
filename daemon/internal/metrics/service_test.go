package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewService(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.registry == nil {
		t.Fatal("registry is nil")
	}
}

func TestHandler_ReturnsMetrics(t *testing.T) {
	svc := NewService()
	h := svc.Handler()
	if h == nil {
		t.Fatal("Handler returned nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), "go_goroutines") {
		t.Error("expected go_goroutines in metrics output")
	}
}

func TestRecordDeploy(t *testing.T) {
	svc := NewService()
	// Should not panic
	svc.RecordDeploy("minecraft", "docker", "success")
	svc.RecordDeploy("valheim", "steamcmd", "failed")
}

func TestRecordBackup(t *testing.T) {
	svc := NewService()
	svc.RecordBackup("srv-1", "full", "success")
	svc.RecordBackup("srv-1", "incremental", "failed")
}

func TestSetServersTotal(t *testing.T) {
	svc := NewService()
	svc.SetServersTotal(5)
	svc.SetServersTotal(0)
}

func TestSetServersRunning(t *testing.T) {
	svc := NewService()
	svc.SetServersRunning(3)
	svc.SetServersRunning(0)
}

func TestMetrics_AppearInOutput(t *testing.T) {
	svc := NewService()
	svc.SetServersTotal(7)
	svc.RecordDeploy("minecraft", "docker", "success")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "games_servers_total") {
		t.Error("games_servers_total metric missing from output")
	}
	if !strings.Contains(body, "games_deploys_total") {
		t.Error("games_deploys_total metric missing from output")
	}
}
