package adapters

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Manifest describes a game server adapter loaded from manifest.yaml
type Manifest struct {
	ID             string         `yaml:"id"`
	Name           string         `yaml:"name"`
	Engine         string         `yaml:"engine"`
	SteamAppID     string         `yaml:"steam_app_id"`
	Version        string         `yaml:"version"`
	DeployMethods  []string       `yaml:"deploy_methods"`
	StartCommand   string         `yaml:"start_command"`
	StopCommand    string         `yaml:"stop_command"`
	RestartCommand string         `yaml:"restart_command"`
	Console        ConsoleConfig  `yaml:"console"`
	BackupPaths    []string       `yaml:"backup_paths"`
	ConfigTemplates []ConfigTemplate `yaml:"config_templates"`
	Ports          []PortSpec     `yaml:"ports"`
	HealthChecks   []HealthCheckSpec `yaml:"health_checks"`
	ModSupport     bool           `yaml:"mod_support"`
	ModSources     []string       `yaml:"mod_sources"`
	Resources      ResourceSpec   `yaml:"recommended_resources"`
	Notes          string         `yaml:"notes"`
	SteamCMD       SteamCMDSpec   `yaml:"steamcmd"`
	Docker         DockerSpec     `yaml:"docker"`
}

// ConsoleConfig describes how to connect to the server console
type ConsoleConfig struct {
	Type          string `yaml:"type"` // stdio|rcon|websocket
	AttachCommand string `yaml:"attach_command"`
	RCONEnabled   bool   `yaml:"rcon_enabled"`
	RCONPort      int    `yaml:"rcon_port,omitempty"`
}

// PortSpec describes a default port for a game server
type PortSpec struct {
	Internal        int    `yaml:"internal"`
	DefaultExternal int    `yaml:"default_external"`
	Protocol        string `yaml:"protocol"` // tcp|udp
	Description     string `yaml:"description"`
}

// HealthCheckSpec defines a health check to run against a server
type HealthCheckSpec struct {
	Type           string `yaml:"type"` // tcp|command
	Host           string `yaml:"host,omitempty"`
	Port           int    `yaml:"port,omitempty"`
	TimeoutSeconds int    `yaml:"timeout_seconds,omitempty"`
	Command        string `yaml:"command,omitempty"`
	ExpectedOutput string `yaml:"expected_output,omitempty"`
}

// ConfigTemplate describes a config file that can be templated
type ConfigTemplate struct {
	Path        string `yaml:"path"`
	Description string `yaml:"description"`
	Sample      string `yaml:"sample"`
}

// ResourceSpec defines recommended compute resources
type ResourceSpec struct {
	CPUCores int `yaml:"cpu_cores"`
	RAMGB    int `yaml:"ram_gb"`
	DiskGB   int `yaml:"disk_gb"`
}

// SteamCMDSpec holds SteamCMD deployment parameters
type SteamCMDSpec struct {
	Login         string `yaml:"login"`
	AppID         string `yaml:"app_id"`
	Validate      bool   `yaml:"validate"`
	Beta          string `yaml:"beta"`
	InstallDirEnv string `yaml:"install_dir_env"`
}

// DockerSpec holds Docker deployment parameters
type DockerSpec struct {
	Image   string            `yaml:"image"`
	Pull    string            `yaml:"pull"`
	EnvVars map[string]string `yaml:"env_vars"`
}

// HealthResult is the result of a health check
type HealthResult struct {
	Healthy  bool          `json:"healthy"`
	Message  string        `json:"message"`
	Latency  time.Duration `json:"latency_ms"`
	CheckedAt time.Time   `json:"checked_at"`
}

// Registry loads and provides access to game adapter manifests
type Registry struct {
	manifests map[string]*Manifest
	logger    *zap.Logger
}

// NewRegistry creates a new adapter registry and loads manifests from dir
func NewRegistry(dir string, logger *zap.Logger) (*Registry, error) {
	r := &Registry{
		manifests: make(map[string]*Manifest),
		logger:    logger,
	}

	if dir == "" {
		logger.Warn("Adapter directory not configured, using built-in defaults")
		r.loadDefaults()
		return r, nil
	}

	if err := r.loadFromDir(dir); err != nil {
		logger.Warn("Failed to load adapters from directory, using defaults",
			zap.String("dir", dir),
			zap.Error(err))
		r.loadDefaults()
	}

	logger.Info("Adapter registry initialized", zap.Int("count", len(r.manifests)))
	return r, nil
}

// Get returns the manifest for a game adapter by ID
func (r *Registry) Get(id string) (*Manifest, bool) {
	m, ok := r.manifests[id]
	return m, ok
}

// List returns all loaded adapter manifests
func (r *Registry) List() []*Manifest {
	result := make([]*Manifest, 0, len(r.manifests))
	for _, m := range r.manifests {
		result = append(result, m)
	}
	return result
}

// Exists returns true if an adapter with the given ID is registered
func (r *Registry) Exists(id string) bool {
	_, ok := r.manifests[id]
	return ok
}

// DefaultPorts returns the default port specs for an adapter
func (r *Registry) DefaultPorts(adapterID string) []PortSpec {
	m, ok := r.manifests[adapterID]
	if !ok {
		return nil
	}
	return m.Ports
}

// BackupPaths returns the recommended backup paths for an adapter
func (r *Registry) BackupPaths(adapterID string) []string {
	m, ok := r.manifests[adapterID]
	if !ok {
		return nil
	}
	return m.BackupPaths
}

// RunHealthCheck performs health checks for a server using the adapter spec
func (r *Registry) RunHealthCheck(ctx context.Context, adapterID, host string) HealthResult {
	m, ok := r.manifests[adapterID]
	if !ok {
		return HealthResult{Healthy: false, Message: "unknown adapter", CheckedAt: time.Now()}
	}

	for _, check := range m.HealthChecks {
		result := r.runSingleCheck(ctx, check, host)
		if !result.Healthy {
			return result
		}
	}

	return HealthResult{Healthy: true, Message: "all checks passed", CheckedAt: time.Now()}
}

func (r *Registry) runSingleCheck(ctx context.Context, spec HealthCheckSpec, host string) HealthResult {
	start := time.Now()
	checkHost := host
	if checkHost == "" {
		checkHost = "localhost"
	}
	timeout := time.Duration(spec.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	switch spec.Type {
	case "tcp":
		addr := fmt.Sprintf("%s:%d", checkHost, spec.Port)
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return HealthResult{
				Healthy:   false,
				Message:   fmt.Sprintf("TCP connect failed: %s", err),
				Latency:   time.Since(start),
				CheckedAt: time.Now(),
			}
		}
		conn.Close()
		return HealthResult{Healthy: true, Message: "TCP ok", Latency: time.Since(start), CheckedAt: time.Now()}

	case "udp":
		addr := fmt.Sprintf("%s:%d", checkHost, spec.Port)
		conn, err := net.DialTimeout("udp", addr, timeout)
		if err != nil {
			return HealthResult{
				Healthy:   false,
				Message:   fmt.Sprintf("UDP check failed: %s", err),
				Latency:   time.Since(start),
				CheckedAt: time.Now(),
			}
		}
		conn.Close()
		return HealthResult{Healthy: true, Message: "UDP ok", Latency: time.Since(start), CheckedAt: time.Now()}

	default:
		// command checks require a running process context — skip in remote health check
		return HealthResult{Healthy: true, Message: "skipped", CheckedAt: time.Now()}
	}
}

func (r *Registry) loadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read adapter dir %s: %w", dir, err)
	}

	loaded := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "manifest.yaml")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			r.logger.Warn("Skipping adapter (no manifest)", zap.String("adapter", entry.Name()))
			continue
		}

		var m Manifest
		if err := yaml.Unmarshal(data, &m); err != nil {
			r.logger.Warn("Failed to parse adapter manifest",
				zap.String("adapter", entry.Name()),
				zap.Error(err))
			continue
		}

		if m.ID == "" {
			m.ID = entry.Name()
		}

		r.manifests[m.ID] = &m
		r.logger.Debug("Loaded adapter", zap.String("id", m.ID), zap.String("name", m.Name))
		loaded++
	}

	r.logger.Info("Loaded adapters from directory", zap.String("dir", dir), zap.Int("count", loaded))
	return nil
}

// loadDefaults seeds the registry with well-known adapter metadata
// used when the adapter directory is missing or empty.
func (r *Registry) loadDefaults() {
	defaults := []*Manifest{
		{
			ID: "valheim", Name: "Valheim Dedicated Server",
			SteamAppID: "896660", DeployMethods: []string{"steamcmd", "manual"},
			BackupPaths: []string{"/data/worlds", "/data/characters"},
			Ports: []PortSpec{
				{Internal: 2456, DefaultExternal: 2456, Protocol: "udp", Description: "Game port"},
				{Internal: 2457, DefaultExternal: 2457, Protocol: "udp", Description: "Game port +1"},
				{Internal: 2458, DefaultExternal: 2458, Protocol: "udp", Description: "Game port +2"},
			},
			Resources:  ResourceSpec{CPUCores: 2, RAMGB: 4, DiskGB: 5},
			ModSupport: true, ModSources: []string{"nexusmods", "thunderstore"},
			Console: ConsoleConfig{Type: "stdio"},
		},
		{
			ID: "minecraft", Name: "Minecraft Java Edition Server",
			DeployMethods: []string{"manual", "docker"},
			BackupPaths:   []string{"/data/world", "/opt/minecraft"},
			Ports: []PortSpec{
				{Internal: 25565, DefaultExternal: 25565, Protocol: "tcp", Description: "Game port"},
			},
			Resources:  ResourceSpec{CPUCores: 2, RAMGB: 4, DiskGB: 20},
			ModSupport: true, ModSources: []string{"curseforge", "modrinth"},
			Console: ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 25575},
		},
		{
			ID: "satisfactory", Name: "Satisfactory Dedicated Server",
			SteamAppID: "1690800", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/games/satisfactory"},
			Ports: []PortSpec{
				{Internal: 7777, DefaultExternal: 7777, Protocol: "udp", Description: "Game port"},
				{Internal: 15000, DefaultExternal: 15000, Protocol: "tcp", Description: "Beacon port"},
				{Internal: 15777, DefaultExternal: 15777, Protocol: "udp", Description: "Query port"},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 8, DiskGB: 20},
			ModSupport: true, ModSources: []string{"ficsit.app"},
			Console: ConsoleConfig{Type: "stdio"},
		},
		{
			ID: "palworld", Name: "Palworld Dedicated Server",
			SteamAppID: "2394010", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/palworld/SaveGames"},
			Ports: []PortSpec{
				{Internal: 8211, DefaultExternal: 8211, Protocol: "udp", Description: "Game port"},
				{Internal: 25575, DefaultExternal: 25575, Protocol: "tcp", Description: "RCON port"},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 16, DiskGB: 40},
			ModSupport: false,
			Console:    ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 25575},
		},
		{
			ID: "eco", Name: "Eco Global Survival",
			SteamAppID: "382310", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/eco/Storage"},
			Ports: []PortSpec{
				{Internal: 3000, DefaultExternal: 3000, Protocol: "tcp", Description: "Game port"},
				{Internal: 3001, DefaultExternal: 3001, Protocol: "tcp", Description: "Web port"},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 8, DiskGB: 20},
			ModSupport: true, ModSources: []string{"eco-mod-kit"},
			Console: ConsoleConfig{Type: "websocket"},
		},
		{
			ID: "enshrouded", Name: "Enshrouded Dedicated Server",
			SteamAppID: "2278520", DeployMethods: []string{"steamcmd", "manual"},
			BackupPaths: []string{"/data/enshrouded"},
			Ports: []PortSpec{
				{Internal: 15636, DefaultExternal: 15636, Protocol: "udp", Description: "Game port"},
				{Internal: 15637, DefaultExternal: 15637, Protocol: "udp", Description: "Query port"},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 16, DiskGB: 40},
			ModSupport: false,
			Console:    ConsoleConfig{Type: "stdio"},
		},
		{
			ID: "riftbreaker", Name: "The Riftbreaker",
			DeployMethods: []string{"manual"},
			BackupPaths:   []string{"/data/riftbreaker"},
			Ports: []PortSpec{
				{Internal: 27015, DefaultExternal: 27015, Protocol: "udp", Description: "Game port"},
			},
			Resources:  ResourceSpec{CPUCores: 2, RAMGB: 4, DiskGB: 10},
			ModSupport: true, ModSources: []string{"steam-workshop"},
			Console: ConsoleConfig{Type: "stdio"},
		},
	}

	for _, m := range defaults {
		r.manifests[m.ID] = m
	}
}
