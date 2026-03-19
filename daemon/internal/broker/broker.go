package broker

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/games-dashboard/daemon/internal/notifications"
	rconpkg "github.com/games-dashboard/daemon/internal/rcon"
	"github.com/games-dashboard/daemon/internal/sbom"
	"github.com/games-dashboard/daemon/internal/secrets"
	telnetsvc "github.com/games-dashboard/daemon/internal/telnet"
	webrconpkg "github.com/games-dashboard/daemon/internal/webrcon"
	"github.com/robfig/cron/v3"
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
	// Auto-restart fields
	AutoRestart      bool       `json:"auto_restart,omitempty"`
	MaxRestarts      int        `json:"max_restarts,omitempty"`  // 0 = default (3)
	RestartDelaySecs int        `json:"restart_delay_secs,omitempty"` // 0 = default (10)
	RestartCount     int        `json:"restart_count,omitempty"`
	LastCrashAt      *time.Time `json:"last_crash_at,omitempty"`
	// Last human-readable error message (set on every StateError transition)
	LastError string `json:"last_error,omitempty"`
	// Live resource utilisation — updated every metrics collection cycle (15 s).
	// 0 = unknown / server not running.
	CPUPct     float64 `json:"cpu_pct,omitempty"`
	RAMPct     float64 `json:"ram_pct,omitempty"`
	DiskPct    float64 `json:"disk_pct,omitempty"`
	NetInKbps  float64 `json:"net_in_kbps,omitempty"`
	NetOutKbps float64 `json:"net_out_kbps,omitempty"`
	// Player counts — updated alongside CPU/RAM/Disk every 15 s.
	// PlayerCount == -1 means the adapter does not support player count queries
	// or the RCON password is not set.  PlayerCount >= 0 is the live count.
	PlayerCount int `json:"player_count"`
	MaxPlayers  int `json:"max_players,omitempty"`
	// Auto-update — when enabled the server is re-deployed on AutoUpdateSchedule.
	AutoUpdate         bool       `json:"auto_update,omitempty"`
	AutoUpdateSchedule string     `json:"auto_update_schedule,omitempty"` // cron expr; default "0 4 * * *"
	LastUpdateCheck    *time.Time `json:"last_update_check,omitempty"`
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
	ID               string         `json:"id" binding:"required"`
	Name             string         `json:"name" binding:"required"`
	Adapter          string         `json:"adapter" binding:"required"`
	DeployMethod     string         `json:"deploy_method"`
	InstallDir       string         `json:"install_dir"`
	Ports            []PortMapping  `json:"ports"`
	Config           map[string]any `json:"config"`
	Resources        ResourceSpec   `json:"resources"`
	BackupConfig     *BackupConfig  `json:"backup_config,omitempty"`
	NodeID           string         `json:"node_id,omitempty"` // optional; empty = auto-place or local
	AutoRestart      bool           `json:"auto_restart,omitempty"`
	MaxRestarts      int            `json:"max_restarts,omitempty"`
	RestartDelaySecs int            `json:"restart_delay_secs,omitempty"`
}

type UpdateServerRequest struct {
	Name               string         `json:"name,omitempty"`
	Config             map[string]any `json:"config,omitempty"`
	Resources          *ResourceSpec  `json:"resources,omitempty"`
	BackupConfig       *BackupConfig  `json:"backup_config,omitempty"`
	AutoRestart        *bool          `json:"auto_restart,omitempty"`
	MaxRestarts        *int           `json:"max_restarts,omitempty"`
	RestartDelaySecs   *int           `json:"restart_delay_secs,omitempty"`
	AutoUpdate         *bool          `json:"auto_update,omitempty"`
	AutoUpdateSchedule *string        `json:"auto_update_schedule,omitempty"`
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
	notify     *notifications.Service
	servers    map[string]*Server
	jobs       map[string]*Job
	consoleChs map[string]chan string
	// processes tracks running game server processes by server ID
	processes    map[string]*exec.Cmd
	metricsData  map[string]*metricsRing
	diskWarnedAt map[string]time.Time // throttle: last time a disk warning was emitted per server
	// prevNetBytes tracks the previous cumulative network byte counters per server [in, out]
	// used to compute kbps rates between metric collection cycles.
	prevNetBytes map[string][2]uint64
	prevNetTime  map[string]time.Time
	// auto-update scheduler (robfig/cron); updateMu protects updateEntries independently
	// of the main mu to avoid lock-ordering issues.
	updateCron    *cron.Cron
	updateEntries map[string]cron.EntryID
	updateMu      sync.Mutex
	// per-server rotating log writers
	logWriters map[string]*rotatingWriter
	logWriteMu sync.Mutex // protects logWriters map
	mu         sync.RWMutex
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
		nodeSavePath := cfg.Cluster.NodeSavePath
		if nodeSavePath == "" {
			nodeSavePath = cfg.Storage.DataDir + "/nodes.json"
		}
		clusterMgr = cluster.NewManager(cluster.Config{
			Enabled:             cfg.Cluster.Enabled,
			HealthCheckInterval: cfg.Cluster.HealthCheckInterval,
			NodeTimeout:         cfg.Cluster.NodeTimeout,
			NodeSavePath:        nodeSavePath,
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

	notifyCfg := notifications.Config{}
	if cfg != nil {
		notifyCfg = notifications.Config{
			WebhookURL:    cfg.Notifications.WebhookURL,
			WebhookFormat: cfg.Notifications.WebhookFormat,
			Events:        cfg.Notifications.Events,
		}
		if e := cfg.Notifications.Email; e != nil {
			notifyCfg.Email = &notifications.EmailConfig{
				Enabled:  e.Enabled,
				SMTPHost: e.SMTPHost,
				SMTPPort: e.SMTPPort,
				Username: e.Username,
				Password: e.Password,
				From:     e.From,
				To:       e.To,
				UseTLS:   e.UseTLS,
			}
		}
	}
	notifySvc := notifications.New(notifyCfg, logger)

	persistedServers := loadServersState(cfg, logger)

	// Pre-populate consoleChs and metricsData for loaded servers.
	consoleChs := make(map[string]chan string)
	metricsData := make(map[string]*metricsRing)
	for id := range persistedServers {
		consoleChs[id] = make(chan string, 1000)
		metricsData[id] = &metricsRing{}
	}

	return &Broker{
		cfg:          cfg,
		secrets:      secretsMgr,
		logger:       logger,
		metrics:      metricsSvc,
		adapters:     registry,
		sbomSvc:      sbomSvc,
		networkSvc:   networkSvc,
		clusterMgr:   clusterMgr,
		backupSvc:    bkpSvc,
		modMgr:       modMgr,
		notify:       notifySvc,
		servers:      persistedServers,
		jobs:         make(map[string]*Job),
		consoleChs:   consoleChs,
		processes:    make(map[string]*exec.Cmd),
		metricsData:  metricsData,
		diskWarnedAt: make(map[string]time.Time),
		prevNetBytes: make(map[string][2]uint64),
		prevNetTime:  make(map[string]time.Time),
		logWriters:   make(map[string]*rotatingWriter),
	}, nil
}

// NotifyService returns the notifications service instance.
func (b *Broker) NotifyService() *notifications.Service {
	return b.notify
}

// ClusterManager returns the cluster manager (may be nil if cluster disabled)
func (b *Broker) ClusterManager() *cluster.Manager {
	return b.clusterMgr
}

// generateServerID returns a short random hex ID suitable for use as a server ID.
func generateServerID() string {
	b := make([]byte, 6)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

// saveServersLocked persists the servers map to disk. Must be called with b.mu held (read or write).
func (b *Broker) saveServersLocked() {
	if b.cfg == nil {
		return
	}
	path := filepath.Join(b.cfg.Storage.DataDir, "servers.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		b.logger.Warn("Failed to create data dir for server state", zap.Error(err))
		return
	}
	data, err := json.Marshal(b.servers)
	if err != nil {
		b.logger.Warn("Failed to marshal server state", zap.Error(err))
		return
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		b.logger.Warn("Failed to write server state", zap.Error(err))
	}
}

// loadServersState reads the persisted servers map from disk. Transient states are reset to stopped.
func loadServersState(cfg *config.Config, logger *zap.Logger) map[string]*Server {
	servers := make(map[string]*Server)
	if cfg == nil {
		return servers
	}
	path := filepath.Join(cfg.Storage.DataDir, "servers.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read server state file", zap.Error(err))
		}
		return servers
	}
	if err := json.Unmarshal(data, &servers); err != nil {
		logger.Warn("Failed to parse server state file — starting fresh", zap.Error(err))
		return make(map[string]*Server)
	}
	// Reset transient states so the UI doesn't show stale starting/running/stopping entries.
	for _, s := range servers {
		if s.State == StateStarting || s.State == StateRunning || s.State == StateStopping {
			s.State = StateStopped
		}
		s.PID = 0
	}
	logger.Info("Loaded persisted server state", zap.Int("count", len(servers)))
	return servers
}

// BackupService returns the backup service
func (b *Broker) BackupService() *backupsvc.Service {
	return b.backupSvc
}

// Start begins background goroutines
func (b *Broker) Start(ctx context.Context) {
	b.logger.Info("Broker starting")

	// Auto-update cron scheduler.
	b.initAutoUpdateScheduler(ctx)

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

// collectMetrics samples CPU/RAM for running servers and disk usage for all servers.
// It also queries player counts via RCON for games that support it.
func (b *Broker) collectMetrics() {
	b.mu.RLock()
	type snap struct {
		pid            int
		containerID    string
		deployMethod   string
		installDir     string
		state          ServerState
		adapter        string
		rconEnabled    bool
		consoleType    string
		rconPort       int
		rconPassword   string
		telnetPassword string
	}
	servers := make(map[string]snap, len(b.servers))
	for id, s := range b.servers {
		var rconEnabled bool
		var consoleType string
		var rconPort int
		if m, ok := b.adapters.Get(s.Adapter); ok && m.Console.RCONEnabled {
			rconEnabled = true
			consoleType = m.Console.Type
			rconPort = m.Console.RCONPort
		}
		rconPassword, _ := s.Config["rcon_password"].(string)
		telnetPassword, _ := s.Config["telnet_password"].(string)
		servers[id] = snap{
			pid:            s.PID,
			containerID:    s.ContainerID,
			deployMethod:   s.DeployMethod,
			installDir:     s.InstallDir,
			state:          s.State,
			adapter:        s.Adapter,
			rconEnabled:    rconEnabled,
			consoleType:    consoleType,
			rconPort:       rconPort,
			rconPassword:   rconPassword,
			telnetPassword: telnetPassword,
		}
	}
	b.mu.RUnlock()

	// Concurrently query player counts for running RCON-enabled servers.
	// Each query has a 2 s timeout; running them in parallel keeps total overhead ≤ 2 s.
	type pcEntry struct{ current, max int }
	pcMap := make(map[string]pcEntry, len(servers))
	var pcMu sync.Mutex
	var pcWg sync.WaitGroup
	for id, s := range servers {
		if s.state == StateRunning && s.rconEnabled && s.rconPort > 0 {
			pcWg.Add(1)
			go func(id string, s snap) {
				defer pcWg.Done()
				pc := queryPlayerCount(s.adapter, s.consoleType, s.rconPort, s.rconPassword, s.telnetPassword)
				pcMu.Lock()
				pcMap[id] = pcEntry{pc.current, pc.max}
				pcMu.Unlock()
			}(id, s)
		} else {
			pcMap[id] = pcEntry{-1, 0}
		}
	}
	pcWg.Wait()

	now := time.Now()
	for id, s := range servers {
		var cpu, ram float64
		var netInKbps, netOutKbps float64
		if s.deployMethod == "docker" && s.containerID != "" {
			ds := sampleDocker(s.containerID)
			cpu, ram = ds.CPUPct, ds.RAMPct
			// Compute kbps from cumulative byte delta
			b.mu.Lock()
			prev := b.prevNetBytes[id]
			prevT := b.prevNetTime[id]
			b.prevNetBytes[id] = [2]uint64{ds.NetInB, ds.NetOutB}
			b.prevNetTime[id] = now
			b.mu.Unlock()
			if !prevT.IsZero() && ds.NetInB >= prev[0] && ds.NetOutB >= prev[1] {
				elapsed := now.Sub(prevT).Seconds()
				if elapsed > 0 {
					netInKbps = float64(ds.NetInB-prev[0]) / elapsed / 1024 * 8
					netOutKbps = float64(ds.NetOutB-prev[1]) / elapsed / 1024 * 8
				}
			}
		} else if s.pid > 0 {
			cpu, ram = sampleProcess(s.pid)
		}

		// Disk usage applies to all servers regardless of running state.
		disk := diskUsagePct(s.installDir)

		pc := pcMap[id]

		b.mu.RLock()
		ring, ok := b.metricsData[id]
		b.mu.RUnlock()
		if !ok {
			continue
		}
		ring.push(ServerMetricSample{
			Timestamp:   now.Unix(),
			CPUPercent:  cpu,
			RAMPercent:  ram,
			DiskPercent: disk,
			PlayerCount: pc.current,
			NetInKbps:   netInKbps,
			NetOutKbps:  netOutKbps,
		})

		// Mirror latest values onto the Server object for direct API access
		// (no extra /metrics API call needed by the UI).
		b.mu.Lock()
		if sv, ok2 := b.servers[id]; ok2 {
			sv.CPUPct = cpu
			sv.RAMPct = ram
			sv.DiskPct = disk
			sv.NetInKbps = netInKbps
			sv.NetOutKbps = netOutKbps
			sv.PlayerCount = pc.current
			if pc.max > 0 {
				sv.MaxPlayers = pc.max
			}
		}
		b.mu.Unlock()

		// Emit throttled console warnings at 80% and 95% thresholds.
		if disk >= 80 && s.installDir != "" {
			b.checkDiskWarning(id, disk)
		}
	}
}

// checkDiskWarning emits a console warning message when disk usage crosses a
// threshold. Warnings are throttled to at most once per hour per server.
func (b *Broker) checkDiskWarning(id string, pct float64) {
	b.mu.Lock()
	last := b.diskWarnedAt[id]
	if time.Since(last) < time.Hour {
		b.mu.Unlock()
		return
	}
	b.diskWarnedAt[id] = time.Now()
	var serverName string
	if sv, ok := b.servers[id]; ok {
		serverName = sv.Name
	}
	b.mu.Unlock()

	var msg string
	if pct >= 95 {
		msg = fmt.Sprintf("CRITICAL: Disk is %.0f%% full — the server will crash when it runs out of space. Free up disk space immediately.", pct)
	} else {
		msg = fmt.Sprintf("WARNING: Disk is %.0f%% full. Consider freeing up space to avoid the server running out of disk.", pct)
	}
	b.logger.Warn("Disk usage threshold exceeded", zap.String("id", id), zap.Float64("disk_pct", pct))
	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":%s,"ts":%d}`, jsonStr(msg), time.Now().Unix()))
	b.notify.Send("disk.warning", serverName, msg)
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

// startupGracePeriod is how long after a server starts before health checks can
// mark it as failed. Many game servers (e.g. Minecraft JVM) take 30–90 s to
// bind their ports, so we skip health checks during this window.
const startupGracePeriod = 90 * time.Second

func (b *Broker) checkServerHealth(ctx context.Context, id string) {
	b.mu.RLock()
	server, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok || server.State != StateRunning {
		return
	}

	// Skip health check while the server is still in its startup grace window.
	if server.LastStarted != nil && time.Since(*server.LastStarted) < startupGracePeriod {
		return
	}

	result := b.adapters.RunHealthCheck(ctx, server.Adapter, "localhost")
	if !result.Healthy {
		b.logger.Warn("Server health check failed",
			zap.String("id", id),
			zap.String("adapter", server.Adapter),
			zap.String("message", result.Message))

		b.mu.RLock()
		stillRunning := false
		if s, exists := b.servers[id]; exists {
			stillRunning = s.State == StateRunning
		}
		b.mu.RUnlock()

		if stillRunning {
			humanMsg := fmt.Sprintf("Health check failed: %s. The server may have crashed or become unresponsive — check the console for recent output.", result.Message)
			b.setServerError(id, humanMsg)
		}
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
		ID:               req.ID,
		Name:             req.Name,
		Adapter:          req.Adapter,
		State:            StateIdle,
		DeployMethod:     req.DeployMethod,
		InstallDir:       req.InstallDir,
		Ports:            ports,
		Config:           req.Config,
		Resources:        resources,
		BackupConfig:     req.BackupConfig,
		NodeID:           nodeID,
		AutoRestart:      req.AutoRestart,
		MaxRestarts:      req.MaxRestarts,
		RestartDelaySecs: req.RestartDelaySecs,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	b.servers[req.ID] = server
	b.consoleChs[req.ID] = make(chan string, 1000)
	b.metricsData[req.ID] = &metricsRing{}
	b.saveServersLocked()

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
	copy := *s
	return &copy, nil
}

// CloneServer creates a copy of an existing server with a new name.
// The clone starts in the stopped state and gets a fresh ID, so the original
// is never affected. Runtime-only fields (PID, container ID, metrics) are not copied.
func (b *Broker) CloneServer(ctx context.Context, id, newName string) (*Server, error) {
	b.mu.RLock()
	src, ok := b.servers[id]
	if !ok {
		b.mu.RUnlock()
		return nil, fmt.Errorf("server not found: %s", id)
	}
	// Deep-copy config map
	cfgCopy := make(map[string]any, len(src.Config))
	for k, v := range src.Config {
		cfgCopy[k] = v
	}
	// Deep-copy ports slice
	portsCopy := make([]PortMapping, len(src.Ports))
	copy(portsCopy, src.Ports)
	// Deep-copy resources
	resCopy := src.Resources
	var backupCopy *BackupConfig
	if src.BackupConfig != nil {
		bc := *src.BackupConfig
		backupCopy = &bc
	}
	b.mu.RUnlock()

	clone := &Server{
		ID:           generateServerID(),
		Name:         newName,
		Adapter:      src.Adapter,
		State:        StateStopped,
		DeployMethod: src.DeployMethod,
		InstallDir:   "", // will need to be re-deployed; leave blank so user sets it
		Ports:        portsCopy,
		Config:       cfgCopy,
		Resources:    resCopy,
		BackupConfig: backupCopy,
		NodeID:       src.NodeID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		AutoRestart:  src.AutoRestart,
		MaxRestarts:  src.MaxRestarts,
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.servers[clone.ID] = clone
	b.consoleChs[clone.ID] = make(chan string, 1000)
	b.metricsData[clone.ID] = &metricsRing{}
	b.saveServersLocked()

	b.logger.Info("Server cloned", zap.String("source_id", id), zap.String("clone_id", clone.ID), zap.String("name", newName))
	return clone, nil
}

// UpdateServer modifies server configuration
func (b *Broker) UpdateServer(ctx context.Context, id string, req UpdateServerRequest) (*Server, error) {
	b.mu.Lock()
	s, ok := b.servers[id]
	if !ok {
		b.mu.Unlock()
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
	if req.AutoRestart != nil {
		s.AutoRestart = *req.AutoRestart
	}
	if req.MaxRestarts != nil {
		s.MaxRestarts = *req.MaxRestarts
	}
	if req.RestartDelaySecs != nil {
		s.RestartDelaySecs = *req.RestartDelaySecs
	}
	if req.AutoUpdate != nil {
		s.AutoUpdate = *req.AutoUpdate
	}
	if req.AutoUpdateSchedule != nil {
		s.AutoUpdateSchedule = *req.AutoUpdateSchedule
	}
	// Capture auto-update state before releasing lock.
	autoUpdate := s.AutoUpdate
	autoUpdateSchedule := s.AutoUpdateSchedule
	autoUpdateChanged := req.AutoUpdate != nil || req.AutoUpdateSchedule != nil
	s.UpdatedAt = time.Now()
	b.saveServersLocked()
	b.mu.Unlock()

	// Re-schedule (or remove) the cron job outside b.mu to avoid lock ordering issues.
	if autoUpdateChanged {
		if autoUpdate {
			b.scheduleAutoUpdate(id, autoUpdateSchedule)
		} else {
			b.unscheduleAutoUpdate(id)
		}
	}
	b.mu.RLock()
	result := b.servers[id]
	b.mu.RUnlock()
	return result, nil
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
	b.saveServersLocked()
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
				sv.LastError = "" // clear any previous error on successful start
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

	// Guard against attempting to start a server that has never been deployed:
	// if the install directory does not exist the binary can't be there either.
	if _, statErr := os.Stat(installDir); os.IsNotExist(statErr) {
		msg := fmt.Sprintf("Server not deployed yet — the install directory %q does not exist. Click Deploy to download and install the server files first.", installDir)
		b.logger.Error("Failed to start server process — not deployed",
			zap.String("id", id), zap.String("install_dir", installDir))
		b.setServerError(id, msg)
		return
	}

	// Pre-flight: re-apply exec_bins chmod from the adapter manifest.
	// This heals servers that were deployed before the exec_bins fix was in place.
	if hasManifest && len(manifest.SteamCMD.ExecBins) > 0 {
		for _, rel := range manifest.SteamCMD.ExecBins {
			target := filepath.Join(installDir, rel)
			if fi, err := os.Stat(target); err == nil && fi.Mode()&0o111 == 0 {
				b.logger.Warn("exec_bin missing execute bit at start time, applying chmod +x",
					zap.String("id", id), zap.String("file", target))
				_ = os.Chmod(target, fi.Mode()|0o111)
			}
		}
	}

	// Pre-flight: scan the full start command for the first ./binary token.
	// Start commands are often compound shell expressions such as:
	//   export LD_LIBRARY_PATH=./linux64:...; ./valheim_server.x86_64 -args
	// Checking only index-0 would land on "export", not the actual binary.
	// Scanning all whitespace tokens (skipping variable assignments) finds it.
	var dotSlashBin string
	for _, tok := range strings.Fields(startCmd) {
		if strings.Contains(tok, "=") {
			continue // skip LD_LIBRARY_PATH=./linux64:... style tokens
		}
		if strings.HasPrefix(tok, "./") {
			dotSlashBin = tok
			break
		}
	}
	if dotSlashBin != "" {
		binPath := filepath.Join(installDir, dotSlashBin[2:])
		if fi, statErr := os.Stat(binPath); os.IsNotExist(statErr) {
			msg := fmt.Sprintf("Game binary %q was not found in %q. The server files may be incomplete — re-run Deploy to reinstall them.", dotSlashBin, installDir)
			b.logger.Error("Pre-start check failed: binary missing",
				zap.String("id", id), zap.String("binary", binPath))
			b.setServerError(id, msg)
			return
		} else if statErr == nil && fi.Mode()&0o111 == 0 {
			b.logger.Warn("Binary missing execute bit, applying chmod +x",
				zap.String("id", id), zap.String("binary", binPath))
			_ = os.Chmod(binPath, fi.Mode()|0o111)
		}
	}

	// Run loop: start the process, wait for exit, optionally restart on crash.
	for {
		b.mu.RLock()
		sv, svOk := b.servers[id]
		if !svOk {
			b.mu.RUnlock()
			return
		}
		autoRestart := sv.AutoRestart
		maxRestarts := sv.MaxRestarts
		if maxRestarts <= 0 {
			maxRestarts = 3
		}
		restartDelay := sv.RestartDelaySecs
		if restartDelay <= 0 {
			restartDelay = 10
		}
		b.mu.RUnlock()

		cmd := exec.CommandContext(ctx, "sh", "-c", startCmd) //nolint:gosec // user-configured command
		cmd.Dir = installDir
		cmd.Env = buildProcessEnv(sv, manifest)

		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			b.logger.Error("Failed to start server process",
				zap.String("id", id), zap.String("cmd", startCmd), zap.Error(err))
			b.setServerError(id, fmt.Sprintf("Failed to launch the server process: %s. Check that the game files are fully deployed and the server has permission to execute them.", err.Error()))
			return
		}

		startedAt := time.Now()
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

		// Wait for the process to exit.
		exitErr := cmd.Wait()
		if exitErr != nil {
			b.logger.Warn("Server process exited with error", zap.String("id", id), zap.Error(exitErr))
		} else {
			b.logger.Info("Server process exited cleanly", zap.String("id", id))
		}

		b.mu.Lock()
		delete(b.processes, id)
		wasIntentional := false
		if sv2, ok2 := b.servers[id]; ok2 {
			wasIntentional = sv2.State == StateStopping
			if wasIntentional {
				sv2.State = StateStopped
			}
			now := time.Now()
			sv2.LastStopped = &now
			sv2.PID = 0
		}
		b.mu.Unlock()

		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"Server %s process exited","ts":%d}`, id, time.Now().Unix()))

		// Stop here if the process was intentionally stopped or context cancelled.
		if wasIntentional || ctx.Err() != nil {
			return
		}

		// Crash detected. If auto-restart is disabled, mark stopped and return.
		if !autoRestart {
			b.mu.Lock()
			if sv2, ok2 := b.servers[id]; ok2 && sv2.State != StateStopped {
				sv2.State = StateStopped
			}
			b.mu.Unlock()
			return
		}

		// If the process ran for more than 60 s, reset the consecutive crash counter
		// so that isolated crashes don't accumulate toward the restart limit.
		b.mu.Lock()
		if sv2, ok2 := b.servers[id]; ok2 {
			if time.Since(startedAt) > 60*time.Second {
				sv2.RestartCount = 0
			}
			sv2.RestartCount++
			now := time.Now()
			sv2.LastCrashAt = &now
			if sv2.RestartCount > maxRestarts {
				b.mu.Unlock()
				b.logger.Error("Server reached max auto-restart attempts",
					zap.String("id", id), zap.Int("max_restarts", maxRestarts))
				b.setServerError(id, fmt.Sprintf(
					"The server crashed %d times in a row and will not be restarted automatically. Check the console for crash details, fix the underlying issue, then start it manually.",
					maxRestarts))
				return
			}
			attempt := sv2.RestartCount
			restartServerName := sv2.Name
			sv2.State = StateStarting
			b.mu.Unlock()

			b.logger.Warn("Server crashed; scheduling auto-restart",
				zap.String("id", id), zap.Int("attempt", attempt), zap.Int("max", maxRestarts), zap.Int("delay_secs", restartDelay))
			b.sendConsoleMessage(id, fmt.Sprintf(
				`{"type":"system","msg":"Server %s crashed — restarting in %ds (attempt %d/%d)","ts":%d}`,
				id, restartDelay, attempt, maxRestarts, time.Now().Unix()))
			b.notify.Send("server.restart", restartServerName, fmt.Sprintf("Server crashed and is restarting (attempt %d/%d, delay %ds).", attempt, maxRestarts, restartDelay))
		} else {
			b.mu.Unlock()
			return
		}

		select {
		case <-time.After(time.Duration(restartDelay) * time.Second):
		case <-ctx.Done():
			return
		}
		// Loop back to start the process again.
	}
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

	// If no method was specified in the request, fall back to the server's
	// configured deploy_method so a bare POST /deploy with no body "just works".
	method := req.Method
	if method == "" {
		b.mu.RLock()
		if sv, ok := b.servers[id]; ok {
			method = sv.DeployMethod
		}
		b.mu.RUnlock()
	}

	var deployErr error
	switch method {
	case "steamcmd":
		deployErr = b.deploySteamCMD(ctx, id, req, job)
	case "manual":
		deployErr = b.deployManual(ctx, id, req, job)
	case "docker":
		deployErr = b.deployDocker(ctx, id, req, job)
	default:
		deployErr = fmt.Errorf("deploy method %q is not supported — valid options are: steamcmd, manual, docker", method)
	}

	b.mu.Lock()
	if s, ok := b.servers[id]; ok {
		if deployErr != nil {
			s.State = StateError
			s.LastError = deployErr.Error()
		} else {
			s.State = StateStopped
			s.LastError = ""
		}
	}
	b.mu.Unlock()

	if deployErr != nil {
		b.updateJob(job.ID, "failed", 0, deployErr.Error())
		b.logger.Error("Deployment failed", zap.String("id", id), zap.Error(deployErr))
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"error","msg":%s,"ts":%d}`, jsonStr(deployErr.Error()), time.Now().Unix()))
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
		return fmt.Errorf("adapter %q has no Steam App ID — check the adapter manifest file or report this game as unsupported", s.Adapter)
	}

	installDir := s.InstallDir
	if installDir == "" {
		return fmt.Errorf("no install directory set — open Server Settings and enter an Install Directory before deploying")
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("could not create install directory %q: %w", installDir, err)
	}

	// Docker is a hard requirement — SteamCMD always runs inside a container so
	// users never need to install SteamCMD or its 32-bit library dependencies.
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf(
			"Docker is required to deploy game servers. " +
				"Please install Docker (https://docs.docker.com/engine/install/) and ensure the daemon can reach the Docker socket.")
	}

	// Create a throw-away directory that the container uses as HOME so SteamCMD
	// can write its internal .steam cache files without touching installDir.
	// It is removed whether the deploy succeeds or fails.
	steamHome, err := os.MkdirTemp("", "gdash-steamhome-*")
	if err != nil {
		return fmt.Errorf("create steamcmd temp dir: %w", err)
	}
	defer os.RemoveAll(steamHome) //nolint:errcheck

	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"deploy","msg":%s,"ts":%d}`,
		jsonStr("[gdash] Starting SteamCMD via Docker (cm2network/steamcmd). First run will pull the image — this may take a minute."),
		time.Now().Unix()))
	b.updateJob(job.ID, "running", 15, "Pulling SteamCMD Docker image (first run only)…")

	// Ensure installDir is writable by the container's steam user (UID 1000).
	if err := os.Chmod(installDir, 0o777); err != nil {
		b.logger.Warn("Could not chmod install dir for steamcmd container", zap.Error(err))
	}

	// Remove any stale container left by a previous failed deploy so --name doesn't conflict.
	containerName := "gdash-steamcmd-" + id
	_ = exec.Command(dockerBin, "rm", "-f", containerName).Run() //nolint:gosec

	// Build docker run arguments.
	// - installDir is mounted at /games inside the container (game files land here)
	// - steamHome  is mounted at /tmp/steamhome for SteamCMD's own cache/config
	// - We do NOT pass --user: cm2network/steamcmd runs its ENTRYPOINT as its own
	//   internal steam user and breaks if the UID is overridden.
	dockerArgs := []string{
		"run", "--rm",
		"--name", containerName,
		"-v", installDir + ":/games",
		"-v", steamHome + ":/tmp/steamhome",
		"-e", "HOME=/tmp/steamhome",
		"cm2network/steamcmd",
		"+@ShutdownOnFailedCommand", "1",
		"+login", "anonymous",
		"+force_install_dir", "/games",
		"+app_update", appID, "validate",
	}
	if req.SteamCMD != nil && req.SteamCMD.Beta != "" {
		dockerArgs = append(dockerArgs, "-beta", req.SteamCMD.Beta)
		if req.SteamCMD.BetaPass != "" {
			dockerArgs = append(dockerArgs, "-betapassword", req.SteamCMD.BetaPass)
		}
	}
	dockerArgs = append(dockerArgs, "+quit")

	b.updateJob(job.ID, "running", 30, fmt.Sprintf("Running SteamCMD (Docker) for app %s…", appID))
	b.logger.Info("Executing SteamCMD via Docker",
		zap.String("server", id), zap.String("app_id", appID), zap.String("dir", installDir))

	cmd := exec.CommandContext(ctx, dockerBin, dockerArgs...) //nolint:gosec
	cmd.Dir = installDir

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start steamcmd container: %w", err)
	}

	// Forward stderr to the console in a background goroutine so the user sees
	// Docker pull progress and any SteamCMD warnings alongside stdout.
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"deploy","msg":%s,"ts":%d}`,
				jsonStr(sc.Text()), time.Now().Unix()))
		}
	}()

	// Stream stdout to the console and nudge the progress bar forward.
	progress := 30
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"deploy","msg":%s,"ts":%d}`, jsonStr(line), time.Now().Unix()))
		if progress < 90 {
			progress++
		}
		b.updateJob(job.ID, "running", progress, line)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("game download failed — check the console output above for details (common causes: Steam servers busy, disk full, or network timeout): %w", err)
	}

	// Ensure declared executable binaries have the execute bit set.
	// SteamCMD sometimes installs files 0644; without +x the kernel returns
	// ENOENT when the ELF interpreter can't be loaded, causing the confusing
	// "not found" error from sh even though the file is present on disk.
	if manifest, ok := b.adapters.Get(s.Adapter); ok {
		for _, rel := range manifest.SteamCMD.ExecBins {
			target := filepath.Join(installDir, rel)
			if err := os.Chmod(target, 0o755); err != nil {
				b.logger.Warn("Could not chmod exec_bin after deploy",
					zap.String("file", target), zap.Error(err))
			} else {
				b.logger.Debug("chmod +x exec_bin", zap.String("file", target))
			}
		}
	}

	b.updateJob(job.ID, "success", 100, "SteamCMD (Docker) deployment complete")
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
		return fmt.Errorf("no install directory set — open Server Settings and enter an Install Directory before deploying")
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("could not create install directory %q: %w", installDir, err)
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
		return fmt.Errorf("Docker is not installed or not in PATH — install Docker CE first (https://docs.docker.com/engine/install/): %w", err)
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
			return fmt.Errorf("failed to pull Docker image %q — check your internet connection and that the image name is correct: %w", image, err)
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
		b.setServerError(id, fmt.Sprintf("No Docker image configured: %s. Set the docker_image field in Server Settings → Config.", err.Error()))
		return
	}

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		b.logger.Error("docker not in PATH", zap.String("id", id), zap.Error(err))
		b.setServerError(id, "Docker is not installed or not found in PATH. Install Docker on this host to run containerised servers (https://docs.docker.com/engine/install/).")
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
			b.setServerError(id, fmt.Sprintf("Docker failed to start the container: %s. Check that the image is correct and Docker has enough permissions.", runErr.Error()))
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
		sv.LastError = ""
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

	// Always append the daemon-written event log stored under the data dir.
	// This captures system messages (deploy output, start/stop errors) even
	// when the server has never been deployed and has no install directory yet.
	if b.cfg != nil {
		dataDir := b.cfg.Storage.DataDir
		if dataDir == "" {
			dataDir = "/opt/gdash/data"
		}
		candidates = append(candidates, filepath.Join(dataDir, "servers", id, "logs", "gdash-events.log"))
	}

	// For Docker-deployed servers, try fetching logs directly from the container
	// first — this provides real-time output even if the event log is empty.
	b.mu.RLock()
	deployMethod := s.DeployMethod
	containerID := s.ContainerID
	b.mu.RUnlock()
	if deployMethod == "docker" && containerID != "" {
		if dockerPath, err := exec.LookPath("docker"); err == nil {
			cmd := exec.CommandContext(ctx, dockerPath, "logs", "--tail", strconv.Itoa(maxLines), containerID) //nolint:gosec
			if out, err := cmd.CombinedOutput(); err == nil && len(out) > 0 {
				var result []string
				for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
					if line != "" {
						result = append(result, line)
					}
				}
				if len(result) > 0 {
					return result, nil
				}
			}
		}
	}

	for _, path := range candidates {
		if result, err := tailFile(path, maxLines); err == nil && len(result) > 0 {
			return result, nil
		}
	}

	// Nothing found at all.
	return []string{"[system] No log entries yet. Deploy or start the server to generate logs."}, nil
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
	// When total <= n the ring was never wrapped: the valid entries start at
	// index 0. When total > n the oldest entry is at idx%n (the next write slot).
	start := 0
	if total > n {
		start = idx % n
	}
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

// SendConsoleCommand sends a command to a running server's console.
// Dispatches to Source RCON, WebRCON (Rust), or Telnet (7DTD) based on
// the adapter's console.type field.
// The command and its response are also echoed into the console stream.
func (b *Broker) SendConsoleCommand(ctx context.Context, id, command string) (string, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("server not found: %s", id)
	}
	if s.State != StateRunning {
		return "", fmt.Errorf("server is not running — start the server before sending console commands")
	}

	manifest, hasManifest := b.adapters.Get(s.Adapter)
	if !hasManifest {
		return "", fmt.Errorf("no adapter named %q is loaded — check the adapters directory", s.Adapter)
	}
	if !manifest.Console.RCONEnabled && manifest.Console.Type != "telnet" {
		return "", fmt.Errorf("console commands are not supported for %q — this game does not use RCON or telnet", s.Adapter)
	}

	addr := fmt.Sprintf("localhost:%d", manifest.Console.RCONPort)

	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"rcon_cmd","msg":%s,"ts":%d}`, jsonStr("> "+command), time.Now().Unix()))

	var (
		response string
		err      error
	)

	switch manifest.Console.Type {
	case "webrcon":
		// Rust WebSocket-based RCON: ws://host:port/<password>
		password, _ := s.Config["rcon_password"].(string)
		if password == "" {
			return "", fmt.Errorf("no rcon_password set — add it under Server Settings → Config before sending commands")
		}
		response, err = webrconpkg.Exec(addr, password, command, 10*time.Second)
		if err != nil {
			return "", fmt.Errorf("WebRCON connection failed — is the server fully started and is the RCON password correct? (%w)", err)
		}

	case "telnet":
		// 7 Days to Die telnet console
		password, _ := s.Config["telnet_password"].(string)
		if password == "" {
			password, _ = s.Config["rcon_password"].(string)
		}
		response, err = telnetsvc.Exec(addr, password, command, 10*time.Second)
		if err != nil {
			return "", fmt.Errorf("telnet console connection failed — check the server is running and telnet is enabled (%w)", err)
		}

	default:
		// Source RCON protocol (Minecraft, CS2, TF2, GMod, ARK, Factorio, etc.)
		password, _ := s.Config["rcon_password"].(string)
		if password == "" {
			return "", fmt.Errorf("no rcon_password set — add it under Server Settings → Config before sending commands")
		}
		response, err = rconpkg.Exec(addr, password, command, 5*time.Second)
		if err != nil {
			return "", fmt.Errorf("RCON connection failed — check the server is running and the password is correct (%w)", err)
		}
	}

	if response != "" {
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"rcon_resp","msg":%s,"ts":%d}`, jsonStr(response), time.Now().Unix()))
	}
	return response, nil
}

// setServerError transitions a server to StateError and records a human-readable
// explanation in LastError so the UI can display it on the server card.
// It also emits the message to the console stream.
func (b *Broker) setServerError(id, humanMsg string) {
	var serverName string
	b.mu.Lock()
	if sv, ok := b.servers[id]; ok {
		serverName = sv.Name
		sv.State = StateError
		sv.PID = 0
		sv.LastError = humanMsg
	}
	b.mu.Unlock()
	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"error","msg":%s,"ts":%d}`, jsonStr(humanMsg), time.Now().Unix()))
	b.notify.Send("server.crash", serverName, humanMsg)
}

func (b *Broker) sendConsoleMessage(id, msg string) {
	// Deliver to any active WebSocket stream.
	b.mu.RLock()
	ch, ok := b.consoleChs[id]
	b.mu.RUnlock()
	if ok {
		select {
		case ch <- msg:
		default:
		}
	}

	// Persist all console output via a rotating log writer.
	if b.cfg != nil {
		dataDir := b.cfg.Storage.DataDir
		if dataDir == "" {
			dataDir = "/opt/gdash/data"
		}
		logFile := filepath.Join(dataDir, "servers", id, "logs", "gdash-events.log")
		b.getLogWriter(id, logFile).writeLine(msg)
	}
}

// getLogWriter lazily creates and returns the rotating log writer for a server.
func (b *Broker) getLogWriter(id, path string) *rotatingWriter {
	b.logWriteMu.Lock()
	defer b.logWriteMu.Unlock()
	w, ok := b.logWriters[id]
	if !ok {
		maxMB := 100
		maxBack := 5
		compress := true
		if b.cfg != nil && b.cfg.LogRotation.MaxSizeMB > 0 {
			maxMB = b.cfg.LogRotation.MaxSizeMB
		}
		if b.cfg != nil && b.cfg.LogRotation.MaxBackups > 0 {
			maxBack = b.cfg.LogRotation.MaxBackups
		}
		if b.cfg != nil {
			compress = b.cfg.LogRotation.Compress
		}
		w = newRotatingWriter(path, maxMB, maxBack, compress)
		b.logWriters[id] = w
	}
	return w
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

// buildProcessEnv constructs the environment for a game server child process.
// Priority (highest wins): server config values > adapter manifest Docker.EnvVars defaults > inherited daemon env.
func buildProcessEnv(s *Server, manifest *adapters.Manifest) []string {
	// Start from the daemon's own environment.
	env := os.Environ()

	// Helper: set or override a key in the env slice.
	set := func(k, v string) {
		prefix := k + "="
		for i, e := range env {
			if strings.HasPrefix(e, prefix) {
				env[i] = prefix + v
				return
			}
		}
		env = append(env, prefix+v)
	}

	// Apply adapter manifest Docker.EnvVars as baseline defaults.
	if manifest != nil {
		for k, v := range manifest.Docker.EnvVars {
			set(k, v)
		}
	}

	// Computed convenience vars always available in start commands.
	set("INSTALL_DIR", s.InstallDir)
	set("SERVER_NAME", s.Name)
	set("SERVER_ID", s.ID)
	if len(s.Ports) > 0 {
		set("SERVER_PORT", strconv.Itoa(s.Ports[0].External))
	}

	// Apply all string-typed values from s.Config, uppercased as env vars.
	// e.g. config key "rcon_password" → env var "RCON_PASSWORD".
	for k, v := range s.Config {
		if str, ok := v.(string); ok && str != "" {
			set(strings.ToUpper(k), str)
		}
	}

	return env
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

// ── Config file editor ────────────────────────────────────────────────────────

// GetConfigTemplates returns the list of well-known config files declared in the
// adapter manifest for the given server.
func (b *Broker) GetConfigTemplates(ctx context.Context, id string) ([]adapters.ConfigTemplate, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server %q not found", id)
	}
	manifest, hasManifest := b.adapters.Get(s.Adapter)
	if !hasManifest || len(manifest.ConfigTemplates) == 0 {
		return []adapters.ConfigTemplate{}, nil
	}
	return manifest.ConfigTemplates, nil
}

// configFilePath resolves a manifest-relative path to an absolute filesystem
// path under the server's install directory, and validates that the result is
// contained within that directory (path-traversal protection).
func (b *Broker) configFilePath(installDir, relPath string) (string, error) {
	if installDir == "" {
		installDir = "/opt/gdash/data/servers"
	}
	// Strip leading slash so filepath.Join doesn't treat it as absolute.
	rel := strings.TrimPrefix(relPath, "/")
	abs := filepath.Clean(filepath.Join(installDir, rel))
	// Ensure the resolved path is still under installDir.
	prefix := filepath.Clean(installDir) + string(os.PathSeparator)
	if abs != filepath.Clean(installDir) && !strings.HasPrefix(abs, prefix) {
		return "", fmt.Errorf("path %q is outside the server install directory", relPath)
	}
	return abs, nil
}

// ReadConfigFile reads the contents of a config file from the server's install
// directory. relPath should match one of the paths declared in the adapter's
// config_templates (the leading slash is stripped before joining).
func (b *Broker) ReadConfigFile(ctx context.Context, id, relPath string) (string, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("server %q not found", id)
	}
	abs, err := b.configFilePath(s.InstallDir, relPath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			// Return the sample content from the manifest so the editor has
			// something useful to show even before the server is deployed.
			if manifest, ok2 := b.adapters.Get(s.Adapter); ok2 {
				for _, t := range manifest.ConfigTemplates {
					if t.Path == relPath {
						return t.Sample, nil
					}
				}
			}
			return "", nil
		}
		return "", fmt.Errorf("cannot read config file: %w", err)
	}
	return string(data), nil
}

// WriteConfigFile writes content to a config file within the server's install
// directory, creating parent directories as needed. relPath must resolve to a
// path inside the install directory.
func (b *Broker) WriteConfigFile(ctx context.Context, id, relPath, content string) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server %q not found", id)
	}
	abs, err := b.configFilePath(s.InstallDir, relPath)
	if err != nil {
		return err
	}
	if mkErr := os.MkdirAll(filepath.Dir(abs), 0o755); mkErr != nil {
		return fmt.Errorf("cannot create config directory: %w", mkErr)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return fmt.Errorf("cannot write config file: %w", err)
	}
	return nil
}

// ── File browser ──────────────────────────────────────────────────────────────

// FileEntry describes a single file or directory inside a server's install dir.
type FileEntry struct {
	Name     string    `json:"name"`
	IsDir    bool      `json:"is_dir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	// Path is relative to the server's install directory, starting with "/".
	Path string `json:"path"`
}

// ListFiles returns the directory entries at dirPath inside the server's
// install directory. dirPath is relative (leading slash stripped internally).
func (b *Broker) ListFiles(ctx context.Context, id, dirPath string) ([]FileEntry, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server %q not found", id)
	}
	abs, err := b.configFilePath(s.InstallDir, dirPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return []FileEntry{}, nil // server not yet deployed — return empty listing
		}
		return nil, fmt.Errorf("cannot list directory: %w", err)
	}
	installDir := filepath.Clean(s.InstallDir)
	result := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		info, iErr := e.Info()
		if iErr != nil {
			continue
		}
		full := filepath.Join(abs, e.Name())
		rel, _ := filepath.Rel(installDir, full)
		result = append(result, FileEntry{
			Name:     e.Name(),
			IsDir:    e.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime().UTC(),
			Path:     "/" + rel,
		})
	}
	return result, nil
}

// DeleteFile deletes a file (not a directory) at relPath inside the server's
// install directory.
func (b *Broker) DeleteFile(ctx context.Context, id, relPath string) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server %q not found", id)
	}
	abs, err := b.configFilePath(s.InstallDir, relPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path %q is a directory — use recursive delete explicitly", relPath)
	}
	return os.Remove(abs)
}

// UploadFile writes the contents of an uploaded file to destDir/filename inside
// the server's install directory, creating destDir if needed.
func (b *Broker) UploadFile(ctx context.Context, id, destDir, filename string, data []byte) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server %q not found", id)
	}
	dirAbs, err := b.configFilePath(s.InstallDir, destDir)
	if err != nil {
		return err
	}
	// Validate filename has no path separators.
	if strings.ContainsAny(filename, "/\\") {
		return fmt.Errorf("filename must not contain path separators")
	}
	if err := os.MkdirAll(dirAbs, 0o755); err != nil {
		return fmt.Errorf("cannot create destination directory: %w", err)
	}
	dest := filepath.Join(dirAbs, filename)
	// Re-check the destination is still within the install dir.
	prefix := filepath.Clean(s.InstallDir) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(dest), prefix) {
		return fmt.Errorf("destination path is outside the server install directory")
	}
	return os.WriteFile(dest, data, 0o644)
}
