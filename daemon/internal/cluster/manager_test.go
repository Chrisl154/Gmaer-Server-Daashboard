package cluster

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestManager() *Manager {
	return NewManager(Config{
		Enabled:             true,
		HealthCheckInterval: time.Minute,
		NodeTimeout:         90 * time.Second,
	}, zap.NewNop())
}

func registerNode(t *testing.T, m *Manager, hostname, address string, cpu, mem, disk float64) *Node {
	t.Helper()
	n, err := m.Register(RegisterNodeRequest{
		Hostname: hostname,
		Address:  address,
		Capacity: NodeCapacity{CPUCores: cpu, MemoryGB: mem, DiskGB: disk},
	})
	if err != nil {
		t.Fatalf("Register(%s): unexpected error: %v", hostname, err)
	}
	return n
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_NewNode(t *testing.T) {
	m := newTestManager()
	n := registerNode(t, m, "node-1", "10.0.0.1:9090", 8, 16, 200)

	if n.ID == "" {
		t.Error("expected non-empty ID")
	}
	if n.Hostname != "node-1" {
		t.Errorf("hostname = %q, want %q", n.Hostname, "node-1")
	}
	if n.Status != NodeStatusOnline {
		t.Errorf("status = %v, want online", n.Status)
	}
	if n.Capacity.CPUCores != 8 {
		t.Errorf("cpu_cores = %v, want 8", n.Capacity.CPUCores)
	}
}

func TestRegister_DuplicateAddress_UpdatesExisting(t *testing.T) {
	m := newTestManager()

	first := registerNode(t, m, "node-1", "10.0.0.1:9090", 4, 8, 100)
	id := first.ID

	// Re-register same address with different hostname / capacity
	second := registerNode(t, m, "node-1-renamed", "10.0.0.1:9090", 16, 32, 500)

	if second.ID != id {
		t.Errorf("expected same ID %s on re-registration, got %s", id, second.ID)
	}
	if second.Hostname != "node-1-renamed" {
		t.Errorf("hostname not updated: got %s", second.Hostname)
	}
	if second.Capacity.CPUCores != 16 {
		t.Errorf("capacity not updated: cpu_cores = %v", second.Capacity.CPUCores)
	}
	// Should still be only 1 node
	if len(m.List()) != 1 {
		t.Errorf("expected 1 node after re-registration, got %d", len(m.List()))
	}
}

// ── Deregister ────────────────────────────────────────────────────────────────

func TestDeregister_RemovesNode(t *testing.T) {
	m := newTestManager()
	n := registerNode(t, m, "node-1", "10.0.0.1:9090", 4, 8, 100)

	if err := m.Deregister(n.ID); err != nil {
		t.Fatalf("Deregister: unexpected error: %v", err)
	}
	if len(m.List()) != 0 {
		t.Error("expected empty list after deregister")
	}
}

func TestDeregister_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.Deregister("nonexistent-id")
	if err == nil {
		t.Error("expected error for unknown node ID")
	}
}

// ── Heartbeat ─────────────────────────────────────────────────────────────────

func TestHeartbeat_UpdatesFields(t *testing.T) {
	m := newTestManager()
	n := registerNode(t, m, "node-1", "10.0.0.1:9090", 8, 16, 200)

	before := time.Now().Add(-time.Millisecond)
	err := m.Heartbeat(n.ID, HeartbeatRequest{
		Allocated:   NodeCapacity{CPUCores: 2, MemoryGB: 4, DiskGB: 50},
		ServerCount: 3,
		Status:      NodeStatusDraining,
	})
	if err != nil {
		t.Fatalf("Heartbeat: unexpected error: %v", err)
	}

	updated, _ := m.Get(n.ID)
	if updated.Allocated.CPUCores != 2 {
		t.Errorf("allocated cpu_cores = %v, want 2", updated.Allocated.CPUCores)
	}
	if updated.ServerCount != 3 {
		t.Errorf("server_count = %v, want 3", updated.ServerCount)
	}
	if updated.Status != NodeStatusDraining {
		t.Errorf("status = %v, want draining", updated.Status)
	}
	if !updated.LastSeen.After(before) {
		t.Error("LastSeen was not updated")
	}
}

func TestHeartbeat_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.Heartbeat("bad-id", HeartbeatRequest{})
	if err == nil {
		t.Error("expected error for unknown node ID")
	}
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestList_ReturnsSnapshot(t *testing.T) {
	m := newTestManager()
	registerNode(t, m, "a", "10.0.0.1:9090", 4, 8, 100)
	registerNode(t, m, "b", "10.0.0.2:9090", 4, 8, 100)

	nodes := m.List()
	if len(nodes) != 2 {
		t.Fatalf("List() returned %d nodes, want 2", len(nodes))
	}

	// Mutations to the returned copy must not affect stored state
	nodes[0].Hostname = "mutated"
	fresh := m.List()
	for _, n := range fresh {
		if n.Hostname == "mutated" {
			t.Error("List() returned a non-copy; mutation affected internal state")
		}
	}
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestGet_Found(t *testing.T) {
	m := newTestManager()
	n := registerNode(t, m, "node-1", "10.0.0.1:9090", 4, 8, 100)

	got, err := m.Get(n.ID)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got.ID != n.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, n.ID)
	}
}

func TestGet_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.Get("nope")
	if err == nil {
		t.Error("expected error for unknown node ID")
	}
}

// ── BestFit ───────────────────────────────────────────────────────────────────

func TestBestFit_SelectsMostAvailableCPU(t *testing.T) {
	m := newTestManager()

	small := registerNode(t, m, "small", "10.0.0.1:9090", 4, 8, 100)
	large := registerNode(t, m, "large", "10.0.0.2:9090", 16, 32, 500)

	req := NodeCapacity{CPUCores: 1, MemoryGB: 1, DiskGB: 1}
	id, err := m.BestFit(req)
	if err != nil {
		t.Fatalf("BestFit: unexpected error: %v", err)
	}
	if id != large.ID {
		t.Errorf("BestFit chose %s (%s), want %s (%s)", id, small.Hostname, large.ID, large.Hostname)
	}
}

func TestBestFit_ExcludesOfflineNodes(t *testing.T) {
	m := newTestManager()

	n := registerNode(t, m, "offline-node", "10.0.0.1:9090", 16, 32, 500)
	_ = m.Heartbeat(n.ID, HeartbeatRequest{Status: NodeStatusOffline})

	id, _ := m.BestFit(NodeCapacity{CPUCores: 1, MemoryGB: 1, DiskGB: 1})
	if id != "" {
		t.Errorf("BestFit returned %s for offline node, expected empty string", id)
	}
}

func TestBestFit_ExcludesDrainingNodes(t *testing.T) {
	m := newTestManager()

	n := registerNode(t, m, "draining-node", "10.0.0.1:9090", 16, 32, 500)
	_ = m.Heartbeat(n.ID, HeartbeatRequest{Status: NodeStatusDraining})

	id, _ := m.BestFit(NodeCapacity{CPUCores: 1, MemoryGB: 1, DiskGB: 1})
	if id != "" {
		t.Errorf("BestFit returned %s for draining node, expected empty string", id)
	}
}

func TestBestFit_ExcludesInsufficientCapacity(t *testing.T) {
	m := newTestManager()
	registerNode(t, m, "small", "10.0.0.1:9090", 2, 4, 50)

	// Request more than available
	id, _ := m.BestFit(NodeCapacity{CPUCores: 8, MemoryGB: 8, DiskGB: 100})
	if id != "" {
		t.Errorf("BestFit returned %s despite insufficient capacity", id)
	}
}

func TestBestFit_NoNodes_ReturnsEmpty(t *testing.T) {
	m := newTestManager()
	id, err := m.BestFit(NodeCapacity{CPUCores: 1, MemoryGB: 1, DiskGB: 1})
	if err != nil {
		t.Fatalf("BestFit: unexpected error: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty string with no nodes, got %s", id)
	}
}

// ── AllocateOnNode / ReleaseFromNode ─────────────────────────────────────────

func TestAllocateOnNode_IncrementsAllocated(t *testing.T) {
	m := newTestManager()
	n := registerNode(t, m, "node-1", "10.0.0.1:9090", 8, 16, 200)

	err := m.AllocateOnNode(n.ID, NodeCapacity{CPUCores: 2, MemoryGB: 4, DiskGB: 20})
	if err != nil {
		t.Fatalf("AllocateOnNode: unexpected error: %v", err)
	}

	got, _ := m.Get(n.ID)
	if got.Allocated.CPUCores != 2 {
		t.Errorf("allocated cpu_cores = %v, want 2", got.Allocated.CPUCores)
	}
	if got.Allocated.MemoryGB != 4 {
		t.Errorf("allocated memory_gb = %v, want 4", got.Allocated.MemoryGB)
	}
	if got.ServerCount != 1 {
		t.Errorf("server_count = %v, want 1", got.ServerCount)
	}
}

func TestAllocateOnNode_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.AllocateOnNode("bad-id", NodeCapacity{CPUCores: 1})
	if err == nil {
		t.Error("expected error for unknown node ID")
	}
}

func TestReleaseFromNode_DecrementsAllocated(t *testing.T) {
	m := newTestManager()
	n := registerNode(t, m, "node-1", "10.0.0.1:9090", 8, 16, 200)

	_ = m.AllocateOnNode(n.ID, NodeCapacity{CPUCores: 4, MemoryGB: 8, DiskGB: 100})
	err := m.ReleaseFromNode(n.ID, NodeCapacity{CPUCores: 2, MemoryGB: 4, DiskGB: 50})
	if err != nil {
		t.Fatalf("ReleaseFromNode: unexpected error: %v", err)
	}

	got, _ := m.Get(n.ID)
	if got.Allocated.CPUCores != 2 {
		t.Errorf("allocated cpu_cores = %v, want 2", got.Allocated.CPUCores)
	}
	if got.Allocated.MemoryGB != 4 {
		t.Errorf("allocated memory_gb = %v, want 4", got.Allocated.MemoryGB)
	}
	if got.ServerCount != 0 {
		t.Errorf("server_count = %v, want 0", got.ServerCount)
	}
}

func TestReleaseFromNode_FloorsAtZero(t *testing.T) {
	m := newTestManager()
	n := registerNode(t, m, "node-1", "10.0.0.1:9090", 8, 16, 200)

	// Release more than allocated — should not go negative
	err := m.ReleaseFromNode(n.ID, NodeCapacity{CPUCores: 100, MemoryGB: 100, DiskGB: 100})
	if err != nil {
		t.Fatalf("ReleaseFromNode: unexpected error: %v", err)
	}

	got, _ := m.Get(n.ID)
	if got.Allocated.CPUCores != 0 {
		t.Errorf("cpu_cores = %v, want 0 (floor)", got.Allocated.CPUCores)
	}
	if got.Allocated.MemoryGB != 0 {
		t.Errorf("memory_gb = %v, want 0 (floor)", got.Allocated.MemoryGB)
	}
	if got.Allocated.DiskGB != 0 {
		t.Errorf("disk_gb = %v, want 0 (floor)", got.Allocated.DiskGB)
	}
}

func TestReleaseFromNode_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.ReleaseFromNode("bad-id", NodeCapacity{CPUCores: 1})
	if err == nil {
		t.Error("expected error for unknown node ID")
	}
}

// ── Node.Available / Node.CanFit ─────────────────────────────────────────────

func TestNodeAvailable(t *testing.T) {
	n := &Node{
		Capacity:  NodeCapacity{CPUCores: 8, MemoryGB: 16, DiskGB: 200},
		Allocated: NodeCapacity{CPUCores: 3, MemoryGB: 6, DiskGB: 50},
	}
	avail := n.Available()
	if avail.CPUCores != 5 {
		t.Errorf("available cpu_cores = %v, want 5", avail.CPUCores)
	}
	if avail.MemoryGB != 10 {
		t.Errorf("available memory_gb = %v, want 10", avail.MemoryGB)
	}
	if avail.DiskGB != 150 {
		t.Errorf("available disk_gb = %v, want 150", avail.DiskGB)
	}
}

func TestNodeCanFit_True(t *testing.T) {
	n := &Node{
		Capacity:  NodeCapacity{CPUCores: 8, MemoryGB: 16, DiskGB: 200},
		Allocated: NodeCapacity{CPUCores: 2, MemoryGB: 4, DiskGB: 50},
	}
	if !n.CanFit(NodeCapacity{CPUCores: 4, MemoryGB: 8, DiskGB: 100}) {
		t.Error("CanFit should return true when resources fit")
	}
}

func TestNodeCanFit_False_CPU(t *testing.T) {
	n := &Node{
		Capacity:  NodeCapacity{CPUCores: 4, MemoryGB: 16, DiskGB: 200},
		Allocated: NodeCapacity{CPUCores: 3},
	}
	if n.CanFit(NodeCapacity{CPUCores: 2}) {
		t.Error("CanFit should return false when CPU is insufficient")
	}
}

func TestNodeCanFit_False_Memory(t *testing.T) {
	n := &Node{
		Capacity:  NodeCapacity{CPUCores: 8, MemoryGB: 4, DiskGB: 200},
		Allocated: NodeCapacity{MemoryGB: 3},
	}
	if n.CanFit(NodeCapacity{MemoryGB: 2}) {
		t.Error("CanFit should return false when memory is insufficient")
	}
}

func TestNodeCanFit_False_Disk(t *testing.T) {
	n := &Node{
		Capacity:  NodeCapacity{CPUCores: 8, MemoryGB: 16, DiskGB: 50},
		Allocated: NodeCapacity{DiskGB: 40},
	}
	if n.CanFit(NodeCapacity{DiskGB: 20}) {
		t.Error("CanFit should return false when disk is insufficient")
	}
}

// ── checkNodes timeout ────────────────────────────────────────────────────────

func TestCheckNodes_MarksTimedOutNodeOffline(t *testing.T) {
	m := NewManager(Config{
		Enabled:             true,
		HealthCheckInterval: time.Minute,
		NodeTimeout:         100 * time.Millisecond,
	}, zap.NewNop())

	n := registerNode(t, m, "node-1", "127.0.0.1:19999", 4, 8, 100)

	// Force last_seen into the past
	m.mu.Lock()
	m.nodes[n.ID].LastSeen = time.Now().Add(-200 * time.Millisecond)
	m.mu.Unlock()

	m.checkNodes()

	got, _ := m.Get(n.ID)
	if got.Status != NodeStatusOffline {
		t.Errorf("status = %v, want offline after timeout", got.Status)
	}
}

func TestCheckNodes_DrainedNodeSkipped(t *testing.T) {
	m := NewManager(Config{
		Enabled:             true,
		HealthCheckInterval: time.Minute,
		NodeTimeout:         100 * time.Millisecond,
	}, zap.NewNop())

	n := registerNode(t, m, "node-1", "127.0.0.1:19998", 4, 8, 100)
	_ = m.Heartbeat(n.ID, HeartbeatRequest{Status: NodeStatusDraining})

	// Expire last_seen
	m.mu.Lock()
	m.nodes[n.ID].LastSeen = time.Now().Add(-200 * time.Millisecond)
	m.mu.Unlock()

	m.checkNodes()

	// Draining nodes are skipped — status should remain draining
	got, _ := m.Get(n.ID)
	if got.Status != NodeStatusDraining {
		t.Errorf("status = %v, want draining (should be skipped by checkNodes)", got.Status)
	}
}
