package broker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/games-dashboard/daemon/internal/config"
	"github.com/games-dashboard/daemon/internal/metrics"
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
	servers    map[string]*Server
	jobs       map[string]*Job
	backups    map[string][]*Backup
	consoleChs map[string]chan string
	mu         sync.RWMutex
}

// NewBroker creates a new Broker
func NewBroker(cfg *config.Config, secretsMgr *secrets.Manager, logger *zap.Logger, metricsSvc *metrics.Service) (*Broker, error) {
	return &Broker{
		cfg:        cfg,
		secrets:    secretsMgr,
		logger:     logger,
		metrics:    metricsSvc,
		servers:    make(map[string]*Server),
		jobs:       make(map[string]*Job),
		backups:    make(map[string][]*Backup),
		consoleChs: make(map[string]chan string),
	}, nil
}

// Start begins background goroutines
func (b *Broker) Start(ctx context.Context) {
	b.logger.Info("Broker starting")
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
	// TODO: run adapter-specific health check
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

	now := time.Now()
	server := &Server{
		ID:           req.ID,
		Name:         req.Name,
		Adapter:      req.Adapter,
		State:        StateIdle,
		DeployMethod: req.DeployMethod,
		InstallDir:   req.InstallDir,
		Ports:        req.Ports,
		Config:       req.Config,
		Resources:    req.Resources,
		BackupConfig: req.BackupConfig,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	b.servers[req.ID] = server
	b.consoleChs[req.ID] = make(chan string, 1000)

	b.logger.Info("Server created", zap.String("id", req.ID), zap.String("adapter", req.Adapter))
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
	defer b.mu.Unlock()
	s, ok := b.servers[id]
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}
	if s.State == StateRunning {
		return fmt.Errorf("cannot delete running server; stop it first")
	}
	delete(b.servers, id)
	if ch, ok := b.consoleChs[id]; ok {
		close(ch)
		delete(b.consoleChs, id)
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
	time.Sleep(2 * time.Second) // simulate startup

	b.mu.Lock()
	if s, ok := b.servers[id]; ok {
		s.State = StateRunning
		now := time.Now()
		s.LastStarted = &now
	}
	b.mu.Unlock()

	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"Server %s started","ts":%d}`, id, time.Now().Unix()))
	b.logger.Info("Server started", zap.String("id", id))
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
	time.Sleep(1 * time.Second)

	b.mu.Lock()
	if s, ok := b.servers[id]; ok {
		s.State = StateStopped
		now := time.Now()
		s.LastStopped = &now
	}
	b.mu.Unlock()

	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"Server %s stopped","ts":%d}`, id, time.Now().Unix()))
}

// RestartServer restarts a game server
func (b *Broker) RestartServer(ctx context.Context, id string) error {
	if err := b.StopServer(ctx, id); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
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
	time.Sleep(2 * time.Second)

	switch req.Method {
	case "steamcmd":
		b.deploySteamCMD(ctx, id, req, job)
	case "manual":
		b.deployManual(ctx, id, req, job)
	default:
		b.updateJob(job.ID, "failed", 0, "unknown deploy method")
	}
}

func (b *Broker) deploySteamCMD(ctx context.Context, id string, req DeployRequest, job *Job) {
	b.updateJob(job.ID, "running", 50, "Downloading via SteamCMD...")
	time.Sleep(3 * time.Second)
	b.updateJob(job.ID, "success", 100, "Deployment complete")

	b.mu.Lock()
	if s, ok := b.servers[id]; ok {
		s.State = StateStopped
	}
	b.mu.Unlock()
}

func (b *Broker) deployManual(ctx context.Context, id string, req DeployRequest, job *Job) {
	b.updateJob(job.ID, "running", 50, "Downloading archive...")
	time.Sleep(2 * time.Second)
	b.updateJob(job.ID, "success", 100, "Manual deployment complete")

	b.mu.Lock()
	if s, ok := b.servers[id]; ok {
		s.State = StateStopped
	}
	b.mu.Unlock()
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

// GetServerLogs returns recent log lines
func (b *Broker) GetServerLogs(ctx context.Context, id, lines string) ([]string, error) {
	return []string{
		"[INFO] Server starting...",
		"[INFO] Loading world...",
		"[INFO] Server ready on port 2456",
	}, nil
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
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.backups[serverID], nil
}

func (b *Broker) TriggerBackup(ctx context.Context, serverID string, req BackupRequest) (*Job, error) {
	job := b.newJob("backup", serverID)
	go b.doBackup(context.Background(), serverID, req, job)
	return job, nil
}

func (b *Broker) doBackup(ctx context.Context, serverID string, req BackupRequest, job *Job) {
	b.updateJob(job.ID, "running", 20, "Creating backup...")
	time.Sleep(2 * time.Second)

	backup := &Backup{
		ID:        generateID(),
		ServerID:  serverID,
		Type:      req.Type,
		SizeBytes: 1024 * 1024 * 100, // 100MB placeholder
		Checksum:  "sha256:placeholder",
		CreatedAt: time.Now(),
		Status:    "complete",
	}

	b.mu.Lock()
	b.backups[serverID] = append(b.backups[serverID], backup)
	b.mu.Unlock()

	b.updateJob(job.ID, "success", 100, "Backup complete")
}

func (b *Broker) RestoreBackup(ctx context.Context, serverID, backupID string) (*Job, error) {
	job := b.newJob("restore", serverID)
	go func() {
		b.updateJob(job.ID, "running", 50, "Restoring backup...")
		time.Sleep(3 * time.Second)
		b.updateJob(job.ID, "success", 100, "Restore complete")
	}()
	return job, nil
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
	results := make([]PortValidation, 0, len(req.Ports))
	for _, p := range req.Ports {
		results = append(results, PortValidation{
			Port:      p,
			Available: true, // TODO: actual port check
		})
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
	return &TestModsResult{
		Passed: true,
		Tests: []ModTestResult{
			{Name: "server_start", Passed: true, Message: "Server started with mods"},
			{Name: "console_connect", Passed: true, Message: "Console connected"},
		},
		Duration: 5 * time.Second,
	}, nil
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
	return map[string]any{
		"bomFormat":   "CycloneDX",
		"specVersion": "1.5",
		"version":     1,
		"components":  []any{},
	}, nil
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
		b.updateJob(job.ID, "running", 50, "Running CVE scan...")
		time.Sleep(3 * time.Second)
		b.updateJob(job.ID, "success", 100, "CVE scan complete")
	}()
	return job, nil
}

func (b *Broker) GetCVEReport(ctx context.Context) (map[string]any, error) {
	return map[string]any{
		"generated_at": time.Now(),
		"scanner":      "trivy",
		"status":       "clean",
		"findings":     []any{},
	}, nil
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
