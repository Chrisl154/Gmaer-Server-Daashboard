package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func newTestService(validatorURL string) *Service {
	return NewService(ReachabilityProbeConfig{ValidatorURL: validatorURL}, zap.NewNop())
}

// findEphemeralPort returns a free port that we can then intentionally bind.
func findEphemeralPort(t *testing.T, proto string) int {
	t.Helper()
	p, err := FindFreePort(proto, 30000)
	if err != nil {
		t.Fatalf("FindFreePort(%s): %v", proto, err)
	}
	return p
}

// ── ReservePort / ReleaseServer ──────────────────────────────────────────────

func TestReservePort_MarksConflict(t *testing.T) {
	svc := newTestService("")
	svc.ReservePort("server-a", "tcp", 25565)

	results, err := svc.ValidatePorts(context.Background(), []struct {
		Internal int
		External int
		Protocol string
	}{
		{Internal: 25565, External: 25565, Protocol: "tcp"},
	})
	if err != nil {
		t.Fatalf("ValidatePorts: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Available {
		t.Error("port should be marked unavailable due to reservation")
	}
	if r.Conflict == "" {
		t.Error("expected Conflict message for reserved port")
	}
}

func TestReleaseServer_ClearsReservation(t *testing.T) {
	svc := newTestService("")
	svc.ReservePort("server-a", "tcp", 25566)
	svc.ReleaseServer("server-a")

	// After release the port should be available again (OS check may vary,
	// but the reservation conflict should be gone)
	key := "tcp:25566"
	svc.mu.RLock()
	_, stillReserved := svc.reserved[key]
	svc.mu.RUnlock()

	if stillReserved {
		t.Error("ReleaseServer did not remove the reservation")
	}
}

func TestReleaseServer_OnlyRemovesOwnedPorts(t *testing.T) {
	svc := newTestService("")
	svc.ReservePort("server-a", "tcp", 25567)
	svc.ReservePort("server-b", "tcp", 25568)
	svc.ReleaseServer("server-a")

	svc.mu.RLock()
	_, aGone := svc.reserved["tcp:25567"]
	_, bStill := svc.reserved["tcp:25568"]
	svc.mu.RUnlock()

	if aGone {
		t.Error("server-a reservation should be removed")
	}
	if !bStill {
		t.Error("server-b reservation should remain after server-a release")
	}
}

// ── isPortAvailable ───────────────────────────────────────────────────────────

func TestIsPortAvailable_TCP_Free(t *testing.T) {
	port := findEphemeralPort(t, "tcp")
	svc := newTestService("")
	ok, err := svc.isPortAvailable("tcp", port)
	if err != nil {
		t.Fatalf("isPortAvailable error: %v", err)
	}
	if !ok {
		t.Errorf("port %d should be available", port)
	}
}

func TestIsPortAvailable_TCP_InUse(t *testing.T) {
	port := findEphemeralPort(t, "tcp")
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatalf("could not bind port %d for test: %v", port, err)
	}
	defer ln.Close()

	svc := newTestService("")
	ok, err := svc.isPortAvailable("tcp", port)
	if err != nil {
		t.Fatalf("isPortAvailable error: %v", err)
	}
	if ok {
		t.Errorf("port %d should be unavailable (already bound)", port)
	}
}

func TestIsPortAvailable_UDP_Free(t *testing.T) {
	port := findEphemeralPort(t, "udp")
	svc := newTestService("")
	ok, err := svc.isPortAvailable("udp", port)
	if err != nil {
		t.Fatalf("isPortAvailable error: %v", err)
	}
	if !ok {
		t.Errorf("UDP port %d should be available", port)
	}
}

func TestIsPortAvailable_UDP_InUse(t *testing.T) {
	port := findEphemeralPort(t, "udp")
	pc, err := net.ListenPacket("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatalf("could not bind UDP port %d: %v", port, err)
	}
	defer pc.Close()

	svc := newTestService("")
	ok, err := svc.isPortAvailable("udp", port)
	if err != nil {
		t.Fatalf("isPortAvailable error: %v", err)
	}
	if ok {
		t.Errorf("UDP port %d should be unavailable (already bound)", port)
	}
}

func TestIsPortAvailable_UnsupportedProtocol(t *testing.T) {
	svc := newTestService("")
	_, err := svc.isPortAvailable("sctp", 9999)
	if err == nil {
		t.Error("expected error for unsupported protocol 'sctp'")
	}
}

// ── probeReachability ─────────────────────────────────────────────────────────

func TestProbeReachability_NoValidatorURL(t *testing.T) {
	svc := newTestService("")
	reachable, latency := svc.probeReachability(context.Background(), "tcp", 25565)
	if reachable {
		t.Error("should not be reachable when no validator URL is set")
	}
	if latency != 0 {
		t.Errorf("latency should be 0 when no validator URL, got %d", latency)
	}
}

func TestProbeReachability_ReachableTrue(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/probe" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"reachable": true, "latency_ms": 42})
	}))
	defer ts.Close()

	svc := newTestService(ts.URL)
	reachable, latency := svc.probeReachability(context.Background(), "tcp", 25565)
	if !reachable {
		t.Error("expected reachable=true from validator")
	}
	if latency != 42 {
		t.Errorf("latency = %d, want 42", latency)
	}
}

func TestProbeReachability_ReachableFalse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"reachable": false, "latency_ms": 0})
	}))
	defer ts.Close()

	svc := newTestService(ts.URL)
	reachable, _ := svc.probeReachability(context.Background(), "tcp", 25565)
	if reachable {
		t.Error("expected reachable=false from validator")
	}
}

func TestProbeReachability_ServerError_ReturnsFalse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	svc := newTestService(ts.URL)
	reachable, _ := svc.probeReachability(context.Background(), "tcp", 25565)
	if reachable {
		t.Error("expected reachable=false on server error")
	}
}

func TestProbeReachability_BadJSON_ReturnsFalse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer ts.Close()

	svc := newTestService(ts.URL)
	reachable, _ := svc.probeReachability(context.Background(), "tcp", 25565)
	if reachable {
		t.Error("expected reachable=false on bad JSON")
	}
}

// ── ValidatePorts integration ─────────────────────────────────────────────────

func TestValidatePorts_FreePort_Available(t *testing.T) {
	port := findEphemeralPort(t, "tcp")
	svc := newTestService("")

	results, err := svc.ValidatePorts(context.Background(), []struct {
		Internal int
		External int
		Protocol string
	}{
		{Internal: port, External: port, Protocol: "tcp"},
	})
	if err != nil {
		t.Fatalf("ValidatePorts: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Available {
		t.Errorf("port %d should be available", port)
	}
	if results[0].Conflict != "" {
		t.Errorf("unexpected conflict: %s", results[0].Conflict)
	}
}

func TestValidatePorts_ReservedBeatsOSCheck(t *testing.T) {
	port := findEphemeralPort(t, "tcp")
	svc := newTestService("")
	svc.ReservePort("game-server-1", "tcp", port)

	results, _ := svc.ValidatePorts(context.Background(), []struct {
		Internal int
		External int
		Protocol string
	}{
		{Internal: port, External: port, Protocol: "tcp"},
	})

	if results[0].Available {
		t.Error("reserved port should be unavailable")
	}
	if results[0].Conflict == "" {
		t.Error("expected conflict message for reserved port")
	}
}

// ── FindFreePort ──────────────────────────────────────────────────────────────

func TestFindFreePort_TCP(t *testing.T) {
	port, err := FindFreePort("tcp", 40000)
	if err != nil {
		t.Fatalf("FindFreePort(tcp): %v", err)
	}
	if port < 40000 || port >= 65535 {
		t.Errorf("port %d out of expected range", port)
	}
}

func TestFindFreePort_UDP(t *testing.T) {
	port, err := FindFreePort("udp", 40000)
	if err != nil {
		t.Fatalf("FindFreePort(udp): %v", err)
	}
	if port < 40000 {
		t.Errorf("port %d out of expected range", port)
	}
}
