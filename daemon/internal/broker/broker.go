package broker

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/games-dashboard/daemon/internal/adapters"
	backupsvc "github.com/games-dashboard/daemon/internal/backup"
	"github.com/games-dashboard/daemon/internal/cluster"
	"github.com/games-dashboard/daemon/internal/config"
	"github.com/games-dashboard/daemon/internal/metrics"
	"github.com/games-dashboard/daemon/internal/modmanager"
	"github.com/games-dashboard/daemon/internal/networking"
	rconpkg "github.com/games-dashboard/daemon/internal/rcon"
	"github.com/games-dashboard/daemon/internal/sbom"
	"github.com/games-dashboard/daemon/internal/secrets"
	"go.uber.org/zap"
)

// ServerState represents the lifecycle state of a game server
type ServerState string

const (
	StateIdle      ServerState = "idle"
	StateStarting  ServerState = "starting"
	StateRunning   ServerState = "running"
	StateStopping  ServerState = "stopping"
	StateStopped   ServerState = "stopped"
	StateDeploying ServerState = "deploying"
	StateError     ServerState = "error"
)

// Server represents a managed game server
type Server struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Adapter      string            `json:"adapter"`
	State        ServerState       `json:"state"`
	DeployMethod string            `json:"deploy_method"`
	InstallDir   string            `json:"install_dir"`
	Ports        []PortMapping     `json:"ports"`
	Config       map[string]any    `json:"config"`
	Resources    ResourceSpec      `json:"resources"`
	BackupConfig *BackupConfig     `json:"backup_config,omitempty"`
	ModManifest  *ModManifest      `json:"mod_manifest,omitempty"`
	NodeID       string            `json:"node_id,omitempty"` // empty = local host
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	LastStarted  *time.Time        `json:"last_started,omitempty"`
	LastStopped  *time.Time        `json:"last_stopped,omitempty"`
	PID          int               `json:"pid,omitempty"`
	ContainerID  string            `json:"container_id,omitempty"`
}

// PortMapping represents a port forwarding rule
type PortMapping struct {
	Internal int    `json:"internal"`
	External int    `json:"external"`
	Protocol string `json:"protocol"` // tcp|udp
	Exposed  bool   `json:"exposed"`
}

// ResourceSpec defines compute resources
type ResourceSpec struct {
	CPUCores int    `json:"cpu_cores"`
	RAMGB    int    `json:"ram_gb"`
	DiskGB   int    `json:"disk_gb"`
}

// BackupConfig defines backup settings
type BackupConfig struct {
	Enabled    bool     `json:"enabled"`
	Target     string   `json:"target"`
	Schedule   string   `json:"schedule"`
	RetainDays int      `json:"retain_days"`
	Paths      []string `json:"paths"`
}

// ModManifest holds mod configuration for a server
type ModManifest struct {
	Mods      []Mod    `json:"mods"`
	Locked    bool     `json:"locked"`
	CheckedAt time.Time `json:"checked_at"`
}

// Mod represents an installed mod
type Mod struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Source      string    `json:"source"` // steam|curseforge|git|local
	SourceURL   string    `json:"source_url"`
	Checksum    string    `json:"checksum"`
	InstalledAt time.Time `json:"installed_at"`
	Enabled     bool      `json:"enabled"`
}

// Backup represents a backup record
type Backup struct {
	ID        string    `json:"id"`
	ServerID  string    `json:"server_id"`
	Type      string    `json:"type"` // full|incremental
	Target    string    `json:"target"`
	SizeBytes int64     `json:"size_bytes"`
	Checksum  string    `json:"checksum"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"` // pending|running|complete|failed
}

// Job represents an async operation
type Job struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	ServerID  string    `json:"server_id"`
	Status    string    `json:"status"` // pending|running|success|failed
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Request types
type CreateServerRequest struct {
	ID           string         `json:"id" binding:"required"`
	Name         string         `json:"name" binding:"required"`
	Adapter      string         `json:"adapter" binding:"required"`
	DeployMethod string         `json:"deploy_method"`
	InstallDir   string         `json:"install_dir"`
	Ports        []PortMapping  `json:"ports"`
	Config       map[string]any `json:"config"`
	Resources    ResourceSpec   `json:"resources"`
	BackupConfig *BackupConfig  `json:"backup_config,omitempty"`
	NodeID       string         `json:"node_id,omitempty"` // optional; empty = auto-place or local
}

type UpdateServerRequest struct {
	Name         string         `json:"name,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
	Resources    *ResourceSpec  `json:"resources,omitempty"`
	BackupConfig *BackupConfig  `json:"backup_config,omitempty"`
}

type DeployRequest struct {
	Method    string         `json:"method"` // steamcmd|manual|custom
	SteamCMD  *SteamCMDOpts `json:"steamcmd,omitempty"`
	Manual    *ManualOpts    `json:"manual,omitempty"`
	Force     bool           `json:"force"`
}

type SteamCMDOpts struct {
	AppID      string `json:"app_id"`
	Beta       string `json:"beta,omitempty"`
	BetaPass   string `json:"beta_password,omitempty"`
}

type ManualOpts struct {
	ArchiveURL string `json:"archive_url"`
	Checksum   string `json:"checksum,omitempty"`
}

type BackupRequest struct {
	Type string `json:"type"` // full|incremental
}

type UpdatePortsRequest struct {
	Ports []PortMapping `json:"ports"`
}

type ValidatePortsRequest struct {
	Ports []PortMapping `json:"ports"`
}

type ValidatePortsResult struct {
	Results []PortValidation `json:"results"`
}

type PortValidation struct {
	Port      PortMapping `json:"port"`
	Available bool        `json:"available"`
	Conflict  string      `json:"conflict,omitempty"`
	Reachable bool        `json:"reachable,omitempty"`
}

type InstallModRequest struct {
	Source    string `json:"source"` // steam|curseforge|git|local
	ModID     string `json:"mod_id"`
	Version   string `json:"version,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
}

type RollbackModsRequest struct {
	Checkpoint string `json:"checkpoint"` // timestamp or backup ID
}

type TestModsResult struct {
	Passed   bool           `json:"passed"`
	Tests    []ModTestResult `json:"tests"`
	Duration time.Duration  `json:"duration"`
}

type ModTestResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// Broker manages game server lifecycle
type Broker struct {
	cfg        *config.Config
	secrets    *secrets.Manager
	logger     *zap.Logger
	metrics    *metrics.Service
	adapters   *adapters.Registry
	sbomSvc    *sbom.Service
	networkSvc *networking.Service
	clusterMgr *cluster.Manager
	backupSvc  *backupsvc.Service
	modMgr     *modmanager.Manager
	servers    map[string]*Server
	jobs       map[string]*Job
	consoleChs map[string]chan string
	// processes tracks running game server processes by server ID
	processes   map[string]*exec.Cmd
	metricsData map[string]*metricsRing
	mu          sync.RWMutex
}

// NewBroker creates a new Broker
func NewBroker(cfg *config.Config, secretsMgr *secrets.Manager, logger *zap.Logger, metricsSvc *metrics.Service) (*Broker, error) {
	adapterDir := ""
	if cfg != nil {
		adapterDir = cfg.Adapters.Dir
	}
	registry, err := adapters.NewRegistry(adapterDir, logger)
	if err != nil {
		// Non-fatal: fall back to built-in defaults
		logger.Warn("Failed to load adapter registry, using defaults", zap.Error(err))
		registry, _ = adapters.NewRegistry("", logger)
	}

	sbomSvc := sbom.NewService("", "", logger)
	networkSvc := networking.NewService(networking.ReachabilityProbeConfig{}, logger)

	var clusterMgr *cluster.Manager
	if cfg != nil && cfg.Cluster.Enabled {
		clusterMgr = cluster.NewManager(cluster.Config{
			Enabled:             cfg.Cluster.Enabled,
			HealthCheckInterval: cfg.Cluster.HealthCheckInterval,
			NodeTimeout:         cfg.Cluster.NodeTimeout,
		}, logger)
	}

	backupCfg := backupsvc.Config{}
	if cfg != nil {
		backupCfg = backupsvc.Config{
			DefaultSchedule: cfg.Backup.DefaultSchedule,
			RetainDays:      cfg.Backup.RetainDays,
			Compression:     cfg.Backup.Compression,
			DataDir:         cfg.Storage.DataDir + "/backups",
		}
	}
	bkpSvc := backupsvc.NewService(backupCfg, logger)

	modDir := ""
	if cfg != nil {
		modDir = cfg.Storage.DataDir + "/mods"
	}
	modMgr := modmanager.NewManager(modDir, logger)

	return &Broker{
		cfg:         cfg,
		secrets:     secretsMgr,
		logger:      logger,
		metrics:     metricsSvc,
		adapters:    registry,
		sbomSvc:     sbomSvc,
		networkSvc:  networkSvc,
		clusterMgr:  clusterMgr,
		backupSvc:   bkpSvc,
		modMgr:      modMgr,
		servers:     make(map[string]*Server),
		jobs:        make(map[string]*Job),
		consoleChs:  make(map[string]chan string),
		processes:   make(map[string]*exec.Cmd),
		metricsData: make(map[string]*metricsRing),
	}, nil
}

// ClusterManager returns the cluster manager (may be nil if cluster disabled)
func (b *Broker) ClusterManager() *cluster.Manager {
	return b.clusterMgr
}

// BackupService returns the backup service
func (b *Broker) BackupService() *backupsvc.Service {
	return b.backupSvc
}

// Start begins background goroutines
func (b *Broker) Start(ctx context.Context) {
	b.logger.Info("Broker starting")

	// Metrics sampling goroutine — collects per-server CPU/RAM every 15 s.
	go func() {
		metricsTicker := time.NewTicker(15 * time.Second)
		defer metricsTicker.Stop()
		for {
			select {
			case <-metricsTicker.C:
				b.collectMetrics()
			case <-ctx.Done():
				return
			}
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.healthCheckAll(ctx)
		case <-ctx.Done():
			b.logger.Info("Broker stopping")
			return
		}
	}
}

// collectMetrics samples CPU/RAM for all currently running servers.
func (b *Broker) collectMetrics() {
	b.mu.RLock()
	type snap struct {
		pid         int
		containerID string
		deployMethod string
	}
	servers := make(map[string]snap, len(b.servers))
	for id, s := range b.servers {
		if s.State == StateRunning {
			servers[id] = snap{pid: s.PID, containerID: s.ContainerID, deployMethod: s.DeployMethod}
		}
	}
	b.mu.RUnlock()

	for id, s := range servers {
		var cpu, ram float64
		if s.deployMethod == "docker" && s.containerID != "" {
			cpu, ram = sampleDocker(s.containerID)
		} else if s.pid > 0 {
			cpu, ram = sampleProcess(s.pid)
		}

		b.mu.RLock()
		ring, ok := b.metricsData[id]
		b.mu.RUnlock()
		if !ok {
			continue
		}
		ring.push(ServerMetricSample{
			Timestamp:  time.Now().Unix(),
			CPUPercent: cpu,
			RAMPercent: ram,
		})
	}
}

// GetServerMetrics returns the last n metric samples for a server.
func (b *Broker) GetServerMetrics(ctx context.Context, id string, n int) ([]ServerMetricSample, error) {
	b.mu.RLock()
	ring, ok := b.metricsData[id]
	b.mu.RUnlock()
	if !ok {
		// Check if server exists at all.
		b.mu.RLock()
		_, serverExists := b.servers[id]
		b.mu.RUnlock()
		if !serverExists {
			return nil, fmt.Errorf("server not found: %s", id)
		}
		return nil, nil // server exists but no metrics ring yet
	}
	if n <= 0 || n > metricsBufferSize {
		n = metricsBufferSize
	}
	return ring.last(n), nil
}

func (b *Broker) healthCheckAll(ctx context.Context) {
	b.mu.RLock()
	ids := make([]string, 0, len(b.servers))
	for id := range b.servers {
		ids = append(ids, id)
	}
	b.mu.RUnlock()

	for _, id := range ids {
		b.checkServerHealth(ctx, id)
	}
}

func (b *Broker) checkServerHealth(ctx context.Context, id string) {
	b.mu.RLock()
	server, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok || server.State != StateRunning {
		return
	}

	result := b.adapters.RunHealthCheck(ctx, server.Adapter, "localhost")
	if !result.Healthy {
		b.logger.Warn("Server health check failed",
			zap.String("id", id),
			zap.String("adapter", server.Adapter),
			zap.String("message", result.Message))

		b.mu.Lock()
		if s, exists := b.servers[id]; exists && s.State == StateRunning {
			s.State = StateError
		}
		b.mu.Unlock()

		b.sendConsoleMessage(id, fmt.Sprintf("[health] WARN: %s", result.Message))
	}
}

// ListServers returns all servers
func (b *Broker) ListServers(ctx context.Context) ([]*Server, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	servers := make([]*Server, 0, len(b.servers))
	for _, s := range b.servers {
		servers = append(servers, s)
	}
	return servers, nil
}

// CreateServer creates a new server record
func (b *Broker) CreateServer(ctx context.Context, req CreateServerRequest) (*Server, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.servers[req.ID]; exists {
		return nil, fmt.Errorf("server %s already exists", req.ID)
	}

	// Populate defaults from adapter manifest when fields are omitted
	ports := req.Ports
	resources := req.Resources
	if len(ports) == 0 {
		for _, spec := range b.adapters.DefaultPorts(req.Adapter) {
			ports = append(ports, PortMapping{
				Internal: spec.Internal,
				External: spec.DefaultExternal,
				Protocol: spec.Protocol,
				Exposed:  true,
			})
		}
	}
	if manifest, ok := b.adapters.Get(req.Adapter); ok && resources.CPUCores == 0 {
		resources = ResourceSpec{
			CPUCores: manifest.Resources.CPUCores,
			RAMGB:    manifest.Resources.RAMGB,
			DiskGB:   manifest.Resources.DiskGB,
		}
	}

	// Determine which node to place the server on.
	nodeID := req.NodeID
	if nodeID == "" && b.clusterMgr != nil {
		// Auto-place using BestFit: convert ResourceSpec to NodeCapacity.
		cap := cluster.NodeCapacity{
			CPUCores: float64(resources.CPUCores),
			MemoryGB: float64(resources.RAMGB),
			DiskGB:   float64(resources.DiskGB),
		}
		bestNode, fitErr := b.clusterMgr.BestFit(cap)
		if fitErr == nil && bestNode != "" {
			nodeID = bestNode
		}
	}
	// Validate explicit node ID and allocate resources.
	if nodeID != "" && b.clusterMgr != nil {
		cap := cluster.NodeCapacity{
			CPUCores: float64(resources.CPUCores),
			MemoryGB: float64(resources.RAMGB),
			DiskGB:   float64(resources.DiskGB),
		}
		if allocErr := b.clusterMgr.AllocateOnNode(nodeID, cap); allocErr != nil {
			// Node not found or error — fall back to local.
			b.logger.Warn("Node allocation failed, placing locally",
				zap.String("node_id", nodeID), zap.Error(allocErr))
			nodeID = ""
		}
	}

	now := time.Now()
	server := &Server{
		ID:           req.ID,
		Name:         req.Name,
		Adapter:      req.Adapter,
		State:        StateIdle,
		DeployMethod: req.DeployMethod,
		InstallDir:   req.InstallDir,
		Ports:        ports,
		Config:       req.Config,
		Resources:    resources,
		BackupConfig: req.BackupConfig,
		NodeID:       nodeID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	b.servers[req.ID] = server
	b.consoleChs[req.ID] = make(chan string, 1000)
	b.metricsData[req.ID] = &metricsRing{}

	b.logger.Info("Server created",
		zap.String("id", req.ID),
		zap.String("adapter", req.Adapter),
		zap.String("node_id", nodeID))
	return server, nil
}

// GetServer retrieves a server by ID
func (b *Broker) GetServer(ctx context.Context, id string) (*Server, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s, ok := b.servers[id]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", id)
	}
	return s, nil
}

// UpdateServer modifies server configuration
func (b *Broker) UpdateServer(ctx context.Context, id string, req UpdateServerRequest) (*Server, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.servers[id]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", id)
	}
	if req.Name != "" {
		s.Name = req.Name
	}
	if req.Config != nil {
		s.Config = req.Config
	}
	if req.Resources != nil {
		s.Resources = *req.Resources
	}
	if req.BackupConfig != nil {
		s.BackupConfig = req.BackupConfig
	}
	s.UpdatedAt = time.Now()
	return s, nil
}

// DeleteServer removes a server
func (b *Broker) DeleteServer(ctx context.Context, id string) error {
	b.mu.Lock()
	s, ok := b.servers[id]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("server not found: %s", id)
	}
	if s.State == StateRunning {
		b.mu.Unlock()
		return fmt.Errorf("cannot delete running server; stop it first")
	}
	nodeID := s.NodeID
	resources := s.Resources
	deployMethod := s.DeployMethod
	containerID := s.ContainerID
	delete(b.servers, id)
	if ch, ok := b.consoleChs[id]; ok {
		close(ch)
		delete(b.consoleChs, id)
	}
	delete(b.metricsData, id)
	b.mu.Unlock()

	// Best-effort: remove the Docker container on deletion.
	if deployMethod == "docker" && containerID != "" {
		if dockerPath, err := exec.LookPath("docker"); err == nil {
			if err := exec.Command(dockerPath, "rm", "-f", containerID).Run(); err != nil { //nolint:gosec
				b.logger.Warn("Failed to remove docker container on server delete",
					zap.String("id", id), zap.String("container", containerID), zap.Error(err))
			}
		}
	}

	// Release cluster resources if the server was on a remote node.
	if nodeID != "" && b.clusterMgr != nil {
		cap := cluster.NodeCapacity{
			CPUCores: float64(resources.CPUCores),
			MemoryGB: float64(resources.RAMGB),
			DiskGB:   float64(resources.DiskGB),
		}
		_ = b.clusterMgr.ReleaseFromNode(nodeID, cap)
	}
	return nil
}

// StartServer starts a game server
func (b *Broker) StartServer(ctx context.Context, id string) error {
	b.mu.Lock()
	s, ok := b.servers[id]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("server not found: %s", id)
	}
	if s.State == StateRunning {
		b.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.State = StateStarting
	b.mu.Unlock()

	go b.doStart(context.Background(), id)
	return nil
}

func (b *Broker) doStart(ctx context.Context, id string) {
	b.logger.Info("Starting server", zap.String("id", id))

	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return
	}

	// Docker servers have their own lifecycle; bypass process management.
	if s.DeployMethod == "docker" {
		b.startDockerContainer(ctx, id, s)
		return
	}

	// Get the adapter's start command.
	manifest, hasManifest := b.adapters.Get(s.Adapter)
	startCmd := ""
	if hasManifest && manifest.StartCommand != "" {
		startCmd = expandServerVars(manifest.StartCommand, s)
	}

	setState := func(state ServerState, pid int) {
		b.mu.Lock()
		if sv, ok2 := b.servers[id]; ok2 {
			sv.State = state
			sv.PID = pid
			if state == StateRunning {
				now := time.Now()
				sv.LastStarted = &now
			}
		}
		b.mu.Unlock()
	}

	if startCmd == "" {
		// No launch command defined — mark running immediately (manual/docker mode).
		b.logger.Warn("No start_command for adapter; marking running without process",
			zap.String("adapter", s.Adapter))
		setState(StateRunning, 0)
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"Server %s marked running (no start command)","ts":%d}`, id, time.Now().Unix()))
		return
	}

	installDir := s.InstallDir
	if installDir == "" {
		installDir = "."
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", startCmd) //nolint:gosec // user-configured command
	cmd.Dir = installDir

	// Pipe stdout and stderr into the console channel.
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		b.logger.Error("Failed to start server process",
			zap.String("id", id), zap.String("cmd", startCmd), zap.Error(err))
		setState(StateError, 0)
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"error","msg":"Failed to start: %s","ts":%d}`, err.Error(), time.Now().Unix()))
		return
	}

	pid := cmd.Process.Pid
	setState(StateRunning, pid)

	b.mu.Lock()
	b.processes[id] = cmd
	b.mu.Unlock()

	b.logger.Info("Server process started", zap.String("id", id), zap.Int("pid", pid))
	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"Server %s started (pid %d)","ts":%d}`, id, pid, time.Now().Unix()))

	// Stream stdout and stderr to the console channel.
	pipe := func(r io.ReadCloser, prefix string) {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"%s","msg":%s,"ts":%d}`,
				prefix, jsonStr(sc.Text()), time.Now().Unix()))
		}
	}
	go pipe(stdout, "stdout")
	go pipe(stderr, "stderr")

	// Wait for the process to exit, then update state.
	if err := cmd.Wait(); err != nil {
		b.logger.Warn("Server process exited with error",
			zap.String("id", id), zap.Error(err))
	} else {
		b.logger.Info("Server process exited cleanly", zap.String("id", id))
	}

	b.mu.Lock()
	delete(b.processes, id)
	if sv, ok2 := b.servers[id]; ok2 && sv.State != StateStopping {
		sv.State = StateStopped
		now := time.Now()
		sv.LastStopped = &now
		sv.PID = 0
	}
	b.mu.Unlock()

	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"Server %s process exited","ts":%d}`, id, time.Now().Unix()))
}

// StopServer stops a game server
func (b *Broker) StopServer(ctx context.Context, id string) error {
	b.mu.Lock()
	s, ok := b.servers[id]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("server not found: %s", id)
	}
	if s.State != StateRunning {
		b.mu.Unlock()
		return fmt.Errorf("server not running")
	}
	s.State = StateStopping
	b.mu.Unlock()

	go b.doStop(context.Background(), id)
	return nil
}

func (b *Broker) doStop(ctx context.Context, id string) {
	b.logger.Info("Stopping server", zap.String("id", id))

	b.mu.RLock()
	s, sOk := b.servers[id]
	var containerID string
	if sOk {
		containerID = s.ContainerID
	}
	b.mu.RUnlock()

	if sOk && s.DeployMethod == "docker" && containerID != "" {
		b.stopDockerContainer(ctx, id, containerID)
		return
	}

	b.mu.Lock()
	cmd := b.processes[id]
	b.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		// Send SIGTERM first; give the process 15 s to exit cleanly.
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
			b.logger.Info("Server process exited after SIGTERM", zap.String("id", id))
		case <-time.After(15 * time.Second):
			b.logger.Warn("Server did not exit after 15 s; sending SIGKILL", zap.String("id", id))
			_ = cmd.Process.Kill()
		}
	}

	b.mu.Lock()
	delete(b.processes, id)
	if s, ok := b.servers[id]; ok {
		s.State = StateStopped
		now := time.Now()
		s.LastStopped = &now
		s.PID = 0
	}
	b.mu.Unlock()

	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"Server %s stopped","ts":%d}`, id, time.Now().Unix()))
}

// RestartServer restarts a game server
func (b *Broker) RestartServer(ctx context.Context, id string) error {
	if err := b.StopServer(ctx, id); err != nil {
		return err
	}
	// doStop runs asynchronously; poll until the server is no longer stopping.
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		b.mu.RLock()
		state := b.servers[id].State
		b.mu.RUnlock()
		if state == StateStopped || state == StateError {
			break
		}
	}
	return b.StartServer(ctx, id)
}

// DeployServer deploys a game server
func (b *Broker) DeployServer(ctx context.Context, id string, req DeployRequest) (*Job, error) {
	job := b.newJob("deploy", id)
	go b.doDeploy(context.Background(), id, req, job)
	return job, nil
}

func (b *Broker) doDeploy(ctx context.Context, id string, req DeployRequest, job *Job) {
	b.logger.Info("Deploying server", zap.String("id", id), zap.String("method", req.Method))

	b.mu.Lock()
	if s, ok := b.servers[id]; ok {
		s.State = StateDeploying
	}
	b.mu.Unlock()

	b.updateJob(job.ID, "running", 10, "Starting deployment...")

	var deployErr error
	switch req.Method {
	case "steamcmd":
		deployErr = b.deploySteamCMD(ctx, id, req, job)
	case "manual":
		deployErr = b.deployManual(ctx, id, req, job)
	case "docker":
		deployErr = b.deployDocker(ctx, id, req, job)
	default:
		deployErr = fmt.Errorf("unknown deploy method: %s", req.Method)
	}

	b.mu.Lock()
	if s, ok := b.servers[id]; ok {
		if deployErr != nil {
			s.State = StateError
		} else {
			s.State = StateStopped
		}
	}
	b.mu.Unlock()

	if deployErr != nil {
		b.updateJob(job.ID, "failed", 0, deployErr.Error())
		b.logger.Error("Deployment failed", zap.String("id", id), zap.Error(deployErr))
	}
}

func (b *Broker) deploySteamCMD(ctx context.Context, id string, req DeployRequest, job *Job) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}

	// Determine app ID: request overrides manifest.
	appID := ""
	if req.SteamCMD != nil && req.SteamCMD.AppID != "" {
		appID = req.SteamCMD.AppID
	} else if m, ok2 := b.adapters.Get(s.Adapter); ok2 {
		appID = m.SteamCMD.AppID
		if appID == "" {
			appID = m.SteamAppID
		}
	}
	if appID == "" {
		return fmt.Errorf("no Steam app ID configured for adapter %s", s.Adapter)
	}

	installDir := s.InstallDir
	if installDir == "" {
		return fmt.Errorf("install_dir must be set for SteamCMD deployment")
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	steamcmd, err := exec.LookPath("steamcmd")
	if err != nil {
		return fmt.Errorf("steamcmd not found in PATH: %w", err)
	}

	args := []string{
		"+@ShutdownOnFailedCommand", "1",
		"+login", "anonymous",
		"+force_install_dir", installDir,
		"+app_update", appID, "validate",
	}
	if req.SteamCMD != nil && req.SteamCMD.Beta != "" {
		args = append(args, "-beta", req.SteamCMD.Beta)
		if req.SteamCMD.BetaPass != "" {
			args = append(args, "-betapassword", req.SteamCMD.BetaPass)
		}
	}
	args = append(args, "+quit")

	b.updateJob(job.ID, "running", 30, fmt.Sprintf("Running SteamCMD for app %s...", appID))
	b.logger.Info("Executing SteamCMD",
		zap.String("server", id), zap.String("app_id", appID), zap.String("dir", installDir))

	cmd := exec.CommandContext(ctx, steamcmd, args...) //nolint:gosec
	cmd.Dir = installDir

	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start steamcmd: %w", err)
	}

	// Stream output to console and update progress.
	progress := 30
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"deploy","msg":%s,"ts":%d}`, jsonStr(line), time.Now().Unix()))
		// Bump progress a little each line (capped at 90).
		if progress < 90 {
			progress++
		}
		b.updateJob(job.ID, "running", progress, line)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("steamcmd exited with error: %w", err)
	}

	b.updateJob(job.ID, "success", 100, "SteamCMD deployment complete")
	b.logger.Info("SteamCMD deployment complete", zap.String("server", id))
	return nil
}

func (b *Broker) deployManual(ctx context.Context, id string, req DeployRequest, job *Job) error {
	if req.Manual == nil || req.Manual.ArchiveURL == "" {
		return fmt.Errorf("manual deployment requires archive_url")
	}

	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}

	installDir := s.InstallDir
	if installDir == "" {
		return fmt.Errorf("install_dir must be set for manual deployment")
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	b.updateJob(job.ID, "running", 20, "Downloading archive...")
	b.logger.Info("Downloading archive", zap.String("url", req.Manual.ArchiveURL))

	httpResp, err := (&http.Client{Timeout: 30 * time.Minute}).Get(req.Manual.ArchiveURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("download archive: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", httpResp.StatusCode)
	}

	// Write to a temporary file while computing checksum.
	tmp, err := os.CreateTemp("", "gdash-deploy-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	h := sha256.New()
	mw := io.MultiWriter(tmp, h)
	if _, err := io.Copy(mw, httpResp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write archive: %w", err)
	}
	tmp.Close()

	// Verify checksum if provided.
	if req.Manual.Checksum != "" {
		actual := fmt.Sprintf("sha256:%x", h.Sum(nil))
		if !strings.EqualFold(actual, req.Manual.Checksum) {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", req.Manual.Checksum, actual)
		}
	}

	b.updateJob(job.ID, "running", 60, "Extracting archive...")

	// Re-open the temp file for extraction.
	f, err := os.Open(tmp.Name())
	if err != nil {
		return fmt.Errorf("open temp archive: %w", err)
	}
	defer f.Close()

	if err := extractTarGz(f, installDir); err != nil {
		return fmt.Errorf("extract archive: %w", err)
	}

	b.updateJob(job.ID, "success", 100, "Manual deployment complete")
	b.logger.Info("Manual deployment complete", zap.String("server", id), zap.String("dir", installDir))
	return nil
}

// resolveDockerImage returns the Docker image to use for a server.
// Priority: server.Config["docker_image"] > manifest.Docker.Image > error.
func (b *Broker) resolveDockerImage(s *Server) (string, error) {
	if img, ok := s.Config["docker_image"].(string); ok && img != "" {
		return img, nil
	}
	if m, ok := b.adapters.Get(s.Adapter); ok && m.Docker.Image != "" {
		return m.Docker.Image, nil
	}
	return "", fmt.Errorf("no Docker image configured for adapter %s; set docker_image in server config", s.Adapter)
}

func (b *Broker) deployDocker(ctx context.Context, id string, req DeployRequest, job *Job) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}

	image, err := b.resolveDockerImage(s)
	if err != nil {
		return err
	}

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found in PATH: %w", err)
	}

	// Determine pull policy from adapter manifest (default: always pull).
	pullPolicy := "always"
	if m, ok := b.adapters.Get(s.Adapter); ok && m.Docker.Pull != "" {
		pullPolicy = m.Docker.Pull
	}

	if pullPolicy != "never" {
		b.updateJob(job.ID, "running", 20, fmt.Sprintf("Pulling image %s...", image))
		b.logger.Info("Pulling Docker image", zap.String("server", id), zap.String("image", image))

		pullCmd := exec.CommandContext(ctx, dockerPath, "pull", image) //nolint:gosec
		stdout, _ := pullCmd.StdoutPipe()
		pullCmd.Stderr = pullCmd.Stdout
		if err := pullCmd.Start(); err != nil {
			return fmt.Errorf("docker pull start: %w", err)
		}

		progress := 20
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"deploy","msg":%s,"ts":%d}`, jsonStr(line), time.Now().Unix()))
			if progress < 80 {
				progress++
			}
			b.updateJob(job.ID, "running", progress, line)
		}
		if err := pullCmd.Wait(); err != nil {
			return fmt.Errorf("docker pull failed: %w", err)
		}
	}

	b.updateJob(job.ID, "success", 100, fmt.Sprintf("Image %s ready; container will be created on Start", image))
	b.logger.Info("Docker deploy complete", zap.String("server", id), zap.String("image", image))
	return nil
}

func (b *Broker) startDockerContainer(ctx context.Context, id string, s *Server) {
	image, err := b.resolveDockerImage(s)
	if err != nil {
		b.logger.Error("Cannot start docker container: no image", zap.String("id", id), zap.Error(err))
		b.mu.Lock()
		if sv, ok := b.servers[id]; ok {
			sv.State = StateError
		}
		b.mu.Unlock()
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"error","msg":%s,"ts":%d}`, jsonStr(err.Error()), time.Now().Unix()))
		return
	}

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		b.logger.Error("docker not in PATH", zap.String("id", id), zap.Error(err))
		b.mu.Lock()
		if sv, ok := b.servers[id]; ok {
			sv.State = StateError
		}
		b.mu.Unlock()
		return
	}

	containerName := "gd-" + id
	containerID := ""

	// Try to restart an existing container first.
	b.mu.RLock()
	existingCID := s.ContainerID
	b.mu.RUnlock()

	if existingCID != "" {
		startCmd := exec.CommandContext(ctx, dockerPath, "start", existingCID) //nolint:gosec
		if out, startErr := startCmd.Output(); startErr == nil {
			containerID = strings.TrimSpace(string(out))
		} else {
			b.logger.Warn("docker start failed, recreating container",
				zap.String("id", id), zap.String("container", existingCID), zap.Error(startErr))
			// Clear the stale ID and fall through to docker run.
			b.mu.Lock()
			if sv, ok := b.servers[id]; ok {
				sv.ContainerID = ""
			}
			b.mu.Unlock()
		}
	}

	// Create and start a new container.
	if containerID == "" {
		args := []string{"run", "-d", "--name", containerName, "--restart", "unless-stopped"}

		// Port mappings: -p external:internal/proto
		b.mu.RLock()
		ports := s.Ports
		installDir := s.InstallDir
		configCopy := make(map[string]any, len(s.Config))
		for k, v := range s.Config {
			configCopy[k] = v
		}
		b.mu.RUnlock()

		for _, p := range ports {
			if p.Exposed {
				args = append(args, "-p", fmt.Sprintf("%d:%d/%s", p.External, p.Internal, p.Protocol))
			}
		}

		// Volume: install_dir → /data
		if installDir != "" {
			args = append(args, "-v", installDir+":/data")
		}

		// Env vars from adapter manifest.
		if m, ok := b.adapters.Get(s.Adapter); ok {
			for k, v := range m.Docker.EnvVars {
				args = append(args, "-e", k+"="+v)
			}
		}

		// Env vars from server config["docker_env"] (map[string]any).
		if envMap, ok := configCopy["docker_env"].(map[string]any); ok {
			for k, v := range envMap {
				args = append(args, "-e", fmt.Sprintf("%s=%v", k, v))
			}
		}

		args = append(args, image)

		runCmd := exec.CommandContext(ctx, dockerPath, args...) //nolint:gosec
		out, runErr := runCmd.Output()
		if runErr != nil {
			b.logger.Error("docker run failed", zap.String("id", id), zap.Error(runErr))
			b.mu.Lock()
			if sv, ok := b.servers[id]; ok {
				sv.State = StateError
			}
			b.mu.Unlock()
			b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"error","msg":%s,"ts":%d}`, jsonStr(runErr.Error()), time.Now().Unix()))
			return
		}
		containerID = strings.TrimSpace(string(out))
	}

	now := time.Now()
	b.mu.Lock()
	if sv, ok := b.servers[id]; ok {
		sv.ContainerID = containerID
		sv.State = StateRunning
		sv.LastStarted = &now
	}
	b.mu.Unlock()

	b.logger.Info("Docker container started", zap.String("id", id), zap.String("container", containerID))
	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"Container %s started","ts":%d}`, containerID[:min(12, len(containerID))], now.Unix()))

	// Stream docker logs to the console channel.
	go func() {
		logsCmd := exec.Command(dockerPath, "logs", "-f", "--tail", "100", containerID) //nolint:gosec
		logsOut, _ := logsCmd.StdoutPipe()
		logsCmd.Stderr = logsCmd.Stdout
		if err := logsCmd.Start(); err != nil {
			return
		}
		sc := bufio.NewScanner(logsOut)
		for sc.Scan() {
			b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"stdout","msg":%s,"ts":%d}`, jsonStr(sc.Text()), time.Now().Unix()))
		}
		_ = logsCmd.Wait()
	}()

	// Watch for container exit and auto-transition state.
	go func() {
		waitCmd := exec.Command(dockerPath, "wait", containerID) //nolint:gosec
		_ = waitCmd.Run()
		b.mu.Lock()
		if sv, ok := b.servers[id]; ok && sv.State == StateRunning {
			sv.State = StateStopped
			t := time.Now()
			sv.LastStopped = &t
		}
		b.mu.Unlock()
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"Container %s exited","ts":%d}`, containerID[:min(12, len(containerID))], time.Now().Unix()))
	}()
}

func (b *Broker) stopDockerContainer(ctx context.Context, id, containerID string) {
	b.logger.Info("Stopping docker container", zap.String("id", id), zap.String("container", containerID))

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		b.logger.Error("docker not in PATH", zap.String("id", id), zap.Error(err))
		return
	}

	stopCmd := exec.CommandContext(ctx, dockerPath, "stop", "--time", "15", containerID) //nolint:gosec
	done := make(chan error, 1)
	go func() { done <- stopCmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			b.logger.Warn("docker stop error, sending kill", zap.String("container", containerID), zap.Error(err))
			_ = exec.Command(dockerPath, "kill", containerID).Run() //nolint:gosec
		}
	case <-time.After(20 * time.Second):
		b.logger.Warn("docker stop timed out, sending kill", zap.String("container", containerID))
		_ = exec.Command(dockerPath, "kill", containerID).Run() //nolint:gosec
	}

	now := time.Now()
	b.mu.Lock()
	if s, ok := b.servers[id]; ok {
		s.State = StateStopped
		s.LastStopped = &now
	}
	b.mu.Unlock()

	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"Container %s stopped","ts":%d}`, containerID[:min(12, len(containerID))], now.Unix()))
}

// extractTarGz extracts a .tar.gz archive into destDir with path-traversal protection.
func extractTarGz(r io.Reader, destDir string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	cleanDest := filepath.Clean(destDir)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		outPath := filepath.Join(destDir, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(filepath.Clean(outPath), cleanDest+string(os.PathSeparator)) &&
			filepath.Clean(outPath) != cleanDest {
			continue // skip path traversal attempts
		}

		if hdr.FileInfo().IsDir() {
			os.MkdirAll(outPath, hdr.FileInfo().Mode())
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
			continue
		}
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			continue
		}
		_, copyErr := io.Copy(out, tr)
		out.Close()
		if copyErr != nil {
			return fmt.Errorf("write %s: %w", outPath, copyErr)
		}
	}
	return nil
}

// GetServerStatus returns current server status
func (b *Broker) GetServerStatus(ctx context.Context, id string) (map[string]any, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server not found: %s", id)
	}
	return map[string]any{
		"id":       s.ID,
		"state":    s.State,
		"pid":      s.PID,
		"uptime":   b.getUptime(s),
		"healthy":  s.State == StateRunning,
	}, nil
}

func (b *Broker) getUptime(s *Server) int64 {
	if s.LastStarted == nil || s.State != StateRunning {
		return 0
	}
	return int64(time.Since(*s.LastStarted).Seconds())
}

// GetServerLogs returns recent log lines from the server's log files.
// The lines parameter is the maximum number of tail lines to return (default 100).
func (b *Broker) GetServerLogs(ctx context.Context, id, lines string) ([]string, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server not found: %s", id)
	}

	maxLines := 100
	if n, err := strconv.Atoi(lines); err == nil && n > 0 {
		maxLines = n
	}

	// Candidate log file locations in preference order.
	candidates := []string{}
	if s.InstallDir != "" {
		candidates = []string{
			filepath.Join(s.InstallDir, "logs", "latest.log"),
			filepath.Join(s.InstallDir, "server.log"),
			filepath.Join(s.InstallDir, "output.log"),
		}
		// Also find any *.log files in the logs/ sub-directory.
		if matches, err := filepath.Glob(filepath.Join(s.InstallDir, "logs", "*.log")); err == nil {
			for _, m := range matches {
				if filepath.Base(m) != "latest.log" {
					candidates = append(candidates, m)
				}
			}
		}
	}

	for _, path := range candidates {
		if result, err := tailFile(path, maxLines); err == nil && len(result) > 0 {
			return result, nil
		}
	}

	// Fall back to recent console channel messages.
	return []string{"[system] No log file found. Check server install_dir configuration."}, nil
}

// tailFile reads the last n lines from a file efficiently.
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Collect all lines into a circular buffer of size n.
	ring := make([]string, n)
	idx := 0
	total := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		ring[idx%n] = scanner.Text()
		idx++
		total++
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}

	if total == 0 {
		return nil, nil
	}

	// Reconstruct ordered slice.
	count := total
	if count > n {
		count = n
	}
	out := make([]string, count)
	start := idx % n
	for i := 0; i < count; i++ {
		out[i] = ring[(start+i)%n]
	}
	return out, nil
}

// GetConsoleStream returns a channel for live console output
func (b *Broker) GetConsoleStream(ctx context.Context, id string) (<-chan string, error) {
	b.mu.RLock()
	ch, ok := b.consoleChs[id]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no console stream for %s", id)
	}
	return ch, nil
}

// SendConsoleCommand sends a command to a running server via RCON.
// The command and its response are also echoed into the console stream.
func (b *Broker) SendConsoleCommand(ctx context.Context, id, command string) (string, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("server not found: %s", id)
	}
	if s.State != StateRunning {
		return "", fmt.Errorf("server is not running")
	}

	manifest, hasManifest := b.adapters.Get(s.Adapter)
	if !hasManifest || !manifest.Console.RCONEnabled {
		return "", fmt.Errorf("RCON is not enabled for adapter %s", s.Adapter)
	}

	password, _ := s.Config["rcon_password"].(string)
	if password == "" {
		return "", fmt.Errorf("rcon_password not set in server config")
	}

	addr := fmt.Sprintf("localhost:%d", manifest.Console.RCONPort)
	response, err := rconpkg.Exec(addr, password, command, 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("RCON error: %w", err)
	}

	// Echo into the console stream so the UI log shows command + response.
	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"rcon_cmd","msg":%s,"ts":%d}`, jsonStr("> "+command), time.Now().Unix()))
	if response != "" {
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"rcon_resp","msg":%s,"ts":%d}`, jsonStr(response), time.Now().Unix()))
	}
	return response, nil
}

func (b *Broker) sendConsoleMessage(id, msg string) {
	b.mu.RLock()
	ch, ok := b.consoleChs[id]
	b.mu.RUnlock()
	if ok {
		select {
		case ch <- msg:
		default:
		}
	}
}

// Backup operations

func (b *Broker) ListBackups(ctx context.Context, serverID string) ([]*Backup, error) {
	records := b.backupSvc.ListRecords(serverID)
	out := make([]*Backup, 0, len(records))
	for _, r := range records {
		out = append(out, backupRecordToBroker(r))
	}
	return out, nil
}

func (b *Broker) TriggerBackup(ctx context.Context, serverID string, req BackupRequest) (*Job, error) {
	b.mu.RLock()
	s, ok := b.servers[serverID]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverID)
	}

	// Determine which paths to back up.
	paths := []string{}
	if s.BackupConfig != nil && len(s.BackupConfig.Paths) > 0 {
		paths = s.BackupConfig.Paths
	} else if s.InstallDir != "" {
		paths = []string{s.InstallDir}
	}

	target := ""
	if s.BackupConfig != nil {
		target = s.BackupConfig.Target
	}

	svcJob, err := b.backupSvc.TriggerBackup(ctx, serverID, paths, target, req.Type)
	if err != nil {
		return nil, err
	}
	return backupJobToBroker(svcJob), nil
}

func (b *Broker) RestoreBackup(ctx context.Context, serverID, backupID string) (*Job, error) {
	svcJob, err := b.backupSvc.Restore(ctx, serverID, backupID)
	if err != nil {
		return nil, err
	}
	return backupJobToBroker(svcJob), nil
}

// backupRecordToBroker converts a backup service Record to the broker's Backup type.
func backupRecordToBroker(r *backupsvc.Record) *Backup {
	return &Backup{
		ID:        r.ID,
		ServerID:  r.ServerID,
		Type:      r.Type,
		Target:    r.Target,
		SizeBytes: r.SizeBytes,
		Checksum:  r.Checksum,
		CreatedAt: r.CreatedAt,
		Status:    r.Status,
	}
}

// backupJobToBroker converts a backup service Job to the broker's Job type.
func backupJobToBroker(j *backupsvc.Job) *Job {
	return &Job{
		ID:        j.ID,
		Type:      j.Type,
		ServerID:  j.ServerID,
		Status:    j.Status,
		Progress:  j.Progress,
		Message:   j.Message,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}
}

// Port operations

func (b *Broker) ListPorts(ctx context.Context, serverID string) ([]PortMapping, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s, ok := b.servers[serverID]
	if !ok {
		return nil, fmt.Errorf("server not found")
	}
	return s.Ports, nil
}

func (b *Broker) UpdatePorts(ctx context.Context, serverID string, req UpdatePortsRequest) ([]PortMapping, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.servers[serverID]
	if !ok {
		return nil, fmt.Errorf("server not found")
	}
	s.Ports = req.Ports
	return s.Ports, nil
}

func (b *Broker) ValidatePorts(ctx context.Context, req ValidatePortsRequest) (*ValidatePortsResult, error) {
	// Build slice for the networking service
	ports := make([]struct {
		Internal int
		External int
		Protocol string
	}, len(req.Ports))
	for i, p := range req.Ports {
		ports[i] = struct {
			Internal int
			External int
			Protocol string
		}{Internal: p.Internal, External: p.External, Protocol: p.Protocol}
	}

	netResults, err := b.networkSvc.ValidatePorts(ctx, ports)
	if err != nil {
		return nil, err
	}

	results := make([]PortValidation, len(netResults))
	for i, nr := range netResults {
		results[i] = PortValidation{
			Port: PortMapping{
				Internal: nr.Internal,
				External: nr.External,
				Protocol: nr.Protocol,
			},
			Available: nr.Available,
			Conflict:  nr.Conflict,
			Reachable: nr.Reachable,
		}
	}
	return &ValidatePortsResult{Results: results}, nil
}

// Mod operations

func (b *Broker) ListMods(ctx context.Context, serverID string) ([]Mod, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s, ok := b.servers[serverID]
	if !ok {
		return nil, fmt.Errorf("server not found")
	}
	if s.ModManifest == nil {
		return []Mod{}, nil
	}
	return s.ModManifest.Mods, nil
}

func (b *Broker) InstallMod(ctx context.Context, serverID string, req InstallModRequest) (*Job, error) {
	job := b.newJob("install-mod", serverID)
	go func() {
		b.updateJob(job.ID, "running", 50, fmt.Sprintf("Installing mod %s...", req.ModID))
		time.Sleep(2 * time.Second)

		mod := Mod{
			ID:          req.ModID,
			Name:        req.ModID,
			Version:     req.Version,
			Source:      req.Source,
			SourceURL:   req.SourceURL,
			InstalledAt: time.Now(),
			Enabled:     true,
		}

		b.mu.Lock()
		s, ok := b.servers[serverID]
		if ok {
			if s.ModManifest == nil {
				s.ModManifest = &ModManifest{}
			}
			s.ModManifest.Mods = append(s.ModManifest.Mods, mod)
		}
		b.mu.Unlock()

		b.updateJob(job.ID, "success", 100, "Mod installed")
	}()
	return job, nil
}

func (b *Broker) UninstallMod(ctx context.Context, serverID, modID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.servers[serverID]
	if !ok || s.ModManifest == nil {
		return fmt.Errorf("server or mod manifest not found")
	}
	mods := make([]Mod, 0, len(s.ModManifest.Mods))
	for _, m := range s.ModManifest.Mods {
		if m.ID != modID {
			mods = append(mods, m)
		}
	}
	s.ModManifest.Mods = mods
	return nil
}

func (b *Broker) TestMods(ctx context.Context, serverID string) (*TestModsResult, error) {
	suite, err := b.modMgr.RunTests(ctx, serverID)
	if err != nil {
		return nil, err
	}
	result := &TestModsResult{
		Passed:   suite.Passed,
		Duration: suite.Duration,
		Tests:    make([]ModTestResult, len(suite.Tests)),
	}
	for i, t := range suite.Tests {
		result.Tests[i] = ModTestResult{
			Name:    t.Name,
			Passed:  t.Passed,
			Message: t.Message,
		}
	}
	return result, nil
}

func (b *Broker) RollbackMods(ctx context.Context, serverID string, req RollbackModsRequest) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.servers[serverID]
	if !ok {
		return fmt.Errorf("server not found")
	}
	if s.ModManifest != nil {
		s.ModManifest.Mods = []Mod{} // simplified rollback
	}
	return nil
}

// SBOM/CVE operations

func (b *Broker) GetSBOM(ctx context.Context) (map[string]any, error) {
	return b.sbomSvc.GetSBOM(ctx)
}

func (b *Broker) GetComponentSBOM(ctx context.Context, component string) (map[string]any, error) {
	return map[string]any{
		"component": component,
		"sbom":      "see /sbom for full SBOM",
	}, nil
}

func (b *Broker) TriggerCVEScan(ctx context.Context) (*Job, error) {
	job := b.newJob("cve-scan", "system")
	go func() {
		b.updateJob(job.ID, "running", 10, "Running CVE scan...")
		if _, err := b.sbomSvc.TriggerScan(context.Background()); err != nil {
			b.updateJob(job.ID, "failed", 0, err.Error())
			return
		}
		b.updateJob(job.ID, "success", 100, "CVE scan complete")
	}()
	return job, nil
}

func (b *Broker) GetCVEReport(ctx context.Context) (map[string]any, error) {
	report, err := b.sbomSvc.GetReport(ctx)
	if err != nil {
		return nil, err
	}
	// Marshal through JSON to produce a plain map (field names use json tags)
	data, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (b *Broker) RotateSecrets(ctx context.Context) error {
	return b.secrets.Rotate(ctx)
}

// Helpers

func (b *Broker) newJob(jobType, serverID string) *Job {
	job := &Job{
		ID:        generateID(),
		Type:      jobType,
		ServerID:  serverID,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	b.mu.Lock()
	b.jobs[job.ID] = job
	b.mu.Unlock()
	return job
}

func (b *Broker) updateJob(jobID, status string, progress int, message string) {
	b.mu.Lock()
	if j, ok := b.jobs[jobID]; ok {
		j.Status = status
		j.Progress = progress
		j.Message = message
		j.UpdatedAt = time.Now()
	}
	b.mu.Unlock()
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = fmt.Sscanf(fmt.Sprintf("%d", time.Now().UnixNano()), "%s", &b)
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// expandServerVars replaces {variable} placeholders in a command template
// using values from the server record.
func expandServerVars(tmpl string, s *Server) string {
	port := ""
	if len(s.Ports) > 0 {
		port = strconv.Itoa(s.Ports[0].External)
	}
	r := strings.NewReplacer(
		"{name}", s.Name,
		"{id}", s.ID,
		"{install_dir}", s.InstallDir,
		"{port}", port,
		"{adapter}", s.Adapter,
	)
	return r.Replace(tmpl)
}

// jsonStr encodes a string as a JSON string literal (including the surrounding quotes).
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
