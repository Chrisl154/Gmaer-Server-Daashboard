package cluster

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NodeStatus represents the health state of a cluster node
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
	NodeStatusDraining NodeStatus = "draining"
)

// NodeCapacity describes the resource capacity of a node
type NodeCapacity struct {
	CPUCores float64 `json:"cpu_cores"`
	MemoryGB float64 `json:"memory_gb"`
	DiskGB   float64 `json:"disk_gb"`
}

// Node represents a cluster node (agent host)
type Node struct {
	ID           string            `json:"id"`
	Hostname     string            `json:"hostname"`
	Address      string            `json:"address"` // host:port of node agent API
	Labels       map[string]string `json:"labels,omitempty"`
	Capacity     NodeCapacity      `json:"capacity"`
	Allocated    NodeCapacity      `json:"allocated"`
	ServerCount  int               `json:"server_count"`
	Status       NodeStatus        `json:"status"`
	Version      string            `json:"version,omitempty"`
	RegisteredAt time.Time         `json:"registered_at"`
	LastSeen     time.Time         `json:"last_seen"`
}

// Available returns how much capacity is still free on a node
func (n *Node) Available() NodeCapacity {
	return NodeCapacity{
		CPUCores: n.Capacity.CPUCores - n.Allocated.CPUCores,
		MemoryGB: n.Capacity.MemoryGB - n.Allocated.MemoryGB,
		DiskGB:   n.Capacity.DiskGB - n.Allocated.DiskGB,
	}
}

// CanFit reports whether the requested resources fit on this node
func (n *Node) CanFit(req NodeCapacity) bool {
	avail := n.Available()
	return avail.CPUCores >= req.CPUCores &&
		avail.MemoryGB >= req.MemoryGB &&
		avail.DiskGB >= req.DiskGB
}

// RegisterNodeRequest is the payload sent by a new node registering itself
type RegisterNodeRequest struct {
	Hostname string            `json:"hostname" binding:"required"`
	Address  string            `json:"address" binding:"required"`
	Labels   map[string]string `json:"labels,omitempty"`
	Capacity NodeCapacity      `json:"capacity"`
	Version  string            `json:"version,omitempty"`
}

// HeartbeatRequest is sent periodically by each node agent
type HeartbeatRequest struct {
	Allocated   NodeCapacity `json:"allocated"`
	ServerCount int          `json:"server_count"`
	Status      NodeStatus   `json:"status"`
}

// Config controls cluster manager behaviour
type Config struct {
	Enabled             bool          `yaml:"enabled" json:"enabled"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	NodeTimeout         time.Duration `yaml:"node_timeout" json:"node_timeout"`
}

// Manager holds the set of registered nodes and performs placement decisions
type Manager struct {
	cfg    Config
	logger *zap.Logger
	mu     sync.RWMutex
	nodes  map[string]*Node
}

// NewManager creates a cluster Manager
func NewManager(cfg Config, logger *zap.Logger) *Manager {
	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 30 * time.Second
	}
	if cfg.NodeTimeout == 0 {
		cfg.NodeTimeout = 90 * time.Second
	}
	return &Manager{
		cfg:    cfg,
		logger: logger,
		nodes:  make(map[string]*Node),
	}
}

// Register adds a new node and returns its assigned ID
func (m *Manager) Register(req RegisterNodeRequest) (*Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Prevent duplicate address registrations
	for _, n := range m.nodes {
		if n.Address == req.Address {
			// Update existing node instead of duplicating
			n.Hostname = req.Hostname
			n.Labels = req.Labels
			n.Capacity = req.Capacity
			n.Version = req.Version
			n.Status = NodeStatusOnline
			n.LastSeen = time.Now()
			return n, nil
		}
	}

	node := &Node{
		ID:           uuid.NewString(),
		Hostname:     req.Hostname,
		Address:      req.Address,
		Labels:       req.Labels,
		Capacity:     req.Capacity,
		Version:      req.Version,
		Status:       NodeStatusOnline,
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
	}
	m.nodes[node.ID] = node
	m.logger.Info("Node registered", zap.String("id", node.ID), zap.String("host", node.Hostname), zap.String("addr", node.Address))
	return node, nil
}

// Deregister removes a node by ID
func (m *Manager) Deregister(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.nodes[nodeID]; !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}
	delete(m.nodes, nodeID)
	m.logger.Info("Node deregistered", zap.String("id", nodeID))
	return nil
}

// Heartbeat updates a node's resource usage and last-seen timestamp
func (m *Manager) Heartbeat(nodeID string, req HeartbeatRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	node, ok := m.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}
	node.Allocated = req.Allocated
	node.ServerCount = req.ServerCount
	node.Status = req.Status
	node.LastSeen = time.Now()
	return nil
}

// List returns a snapshot of all known nodes
func (m *Manager) List() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Node, 0, len(m.nodes))
	for _, n := range m.nodes {
		cp := *n
		out = append(out, &cp)
	}
	return out
}

// Get returns a single node by ID
func (m *Manager) Get(nodeID string) (*Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}
	cp := *n
	return &cp, nil
}

// BestFit returns the ID of the online node with the most available capacity
// that can satisfy req. Returns ("", nil) when no node matches (use local host).
func (m *Manager) BestFit(req NodeCapacity) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var best *Node
	var bestAvailCPU float64

	for _, n := range m.nodes {
		if n.Status != NodeStatusOnline {
			continue
		}
		if !n.CanFit(req) {
			continue
		}
		avail := n.Available()
		if best == nil || avail.CPUCores > bestAvailCPU {
			best = n
			bestAvailCPU = avail.CPUCores
		}
	}
	if best == nil {
		return "", nil
	}
	return best.ID, nil
}

// AllocateOnNode reserves resources on a node (called when server is placed there)
func (m *Manager) AllocateOnNode(nodeID string, res NodeCapacity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}
	n.Allocated.CPUCores += res.CPUCores
	n.Allocated.MemoryGB += res.MemoryGB
	n.Allocated.DiskGB += res.DiskGB
	n.ServerCount++
	return nil
}

// ReleaseFromNode frees resources on a node (called when server is removed)
func (m *Manager) ReleaseFromNode(nodeID string, res NodeCapacity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}
	n.Allocated.CPUCores = max(0, n.Allocated.CPUCores-res.CPUCores)
	n.Allocated.MemoryGB = max(0, n.Allocated.MemoryGB-res.MemoryGB)
	n.Allocated.DiskGB = max(0, n.Allocated.DiskGB-res.DiskGB)
	if n.ServerCount > 0 {
		n.ServerCount--
	}
	return nil
}

// Start runs the background health-check loop until ctx is cancelled
func (m *Manager) Start(ctx context.Context) {
	if !m.cfg.Enabled {
		return
	}
	ticker := time.NewTicker(m.cfg.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkNodes()
		}
	}
}

// checkNodes pings every node and marks timed-out ones as offline
func (m *Manager) checkNodes() {
	m.mu.Lock()
	defer m.mu.Unlock()

	threshold := time.Now().Add(-m.cfg.NodeTimeout)
	client := &http.Client{Timeout: 5 * time.Second}

	for _, n := range m.nodes {
		if n.Status == NodeStatusDraining {
			continue
		}
		// Mark offline if no heartbeat within timeout window
		if n.LastSeen.Before(threshold) {
			if n.Status != NodeStatusOffline {
				m.logger.Warn("Node timed out", zap.String("id", n.ID), zap.String("host", n.Hostname))
				n.Status = NodeStatusOffline
			}
			continue
		}
		// Active ping
		url := "http://" + n.Address + "/healthz"
		resp, err := client.Get(url) //nolint:noctx
		if err != nil || resp.StatusCode != http.StatusOK {
			if n.Status == NodeStatusOnline {
				m.logger.Warn("Node health-check failed", zap.String("id", n.ID), zap.String("addr", n.Address), zap.Error(err))
				n.Status = NodeStatusOffline
			}
		} else {
			if n.Status == NodeStatusOffline {
				m.logger.Info("Node back online", zap.String("id", n.ID), zap.String("addr", n.Address))
				n.Status = NodeStatusOnline
			}
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
