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
	Manual         ManualSpec     `yaml:"manual"`
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
	AdminPort       bool   `yaml:"admin_port,omitempty"`
}

// HealthCheckSpec defines a health check to run against a server
type HealthCheckSpec struct {
	Type             string `yaml:"type"` // tcp|udp|rcon|command
	Host             string `yaml:"host,omitempty"`
	Port             int    `yaml:"port,omitempty"`
	TimeoutSeconds   int    `yaml:"timeout_seconds,omitempty"`
	Command          string `yaml:"command,omitempty"`
	ExpectedOutput   string `yaml:"expected_output,omitempty"`
	ExpectedContains string `yaml:"expected_contains,omitempty"`
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

// ManualSpec holds parameters for manual (non-Steam) deployments
type ManualSpec struct {
	DownloadURLTemplate string `yaml:"download_url_template"`
	VersionManifest     string `yaml:"version_manifest,omitempty"`
	JavaRequired        string `yaml:"java_required,omitempty"`
	AcceptEULA          bool   `yaml:"accept_eula,omitempty"`
}

// SteamCMDSpec holds SteamCMD deployment parameters
type SteamCMDSpec struct {
	Login         string `yaml:"login"`
	AppID         string `yaml:"app_id"`
	Validate      bool   `yaml:"validate"`
	Beta          string `yaml:"beta"`
	InstallDirEnv string `yaml:"install_dir_env"`
	ExecBins      []string `yaml:"exec_bins"` // files to chmod +x after deploy, relative to install_dir
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
			SteamAppID: "896660", DeployMethods: []string{"steamcmd", "manual", "docker"},
			BackupPaths: []string{"/data/worlds", "/data/characters"},
			Ports: []PortSpec{
				{Internal: 2456, DefaultExternal: 2456, Protocol: "udp", Description: "Game port"},
				{Internal: 2457, DefaultExternal: 2457, Protocol: "udp", Description: "Game port +1"},
				{Internal: 2458, DefaultExternal: 2458, Protocol: "udp", Description: "Game port +2"},
			},
			Resources:  ResourceSpec{CPUCores: 2, RAMGB: 4, DiskGB: 5},
			ModSupport: true, ModSources: []string{"nexusmods", "thunderstore"},
			Console: ConsoleConfig{Type: "stdio"},
			Docker:  DockerSpec{Image: "lloesche/valheim-server:latest", Pull: "missing"},
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
			Docker:  DockerSpec{Image: "itzg/minecraft-server:latest", Pull: "missing"},
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
		{
			ID: "among-us", Name: "Among Us (Impostor Server)",
			SteamAppID: "945360", DeployMethods: []string{"manual", "custom"},
			BackupPaths: []string{"/data/config"},
			Ports: []PortSpec{
				{Internal: 22023, DefaultExternal: 22023, Protocol: "udp", Description: "Game port"},
			},
			Resources:  ResourceSpec{CPUCores: 1, RAMGB: 1, DiskGB: 1},
			ModSupport: true, ModSources: []string{"local"},
			Console: ConsoleConfig{Type: "stdio"},
			Docker:  DockerSpec{Image: "aeonlucid/impostor", Pull: "missing"},
		},
		{
			ID: "dota2", Name: "Dota 2 Dedicated Server",
			SteamAppID: "570", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/opt/dota2/game/dota/cfg"},
			Ports: []PortSpec{
				{Internal: 27015, DefaultExternal: 27015, Protocol: "udp", Description: "Game port"},
				{Internal: 27020, DefaultExternal: 27020, Protocol: "udp", Description: "SourceTV port"},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 8, DiskGB: 30},
			ModSupport: true, ModSources: []string{"steam_workshop"},
			Console:  ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 27015},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "570", Validate: true},
		},
		{
			ID: "counter-strike-2", Name: "Counter-Strike 2 Dedicated Server",
			SteamAppID: "730", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/opt/cs2/game/csgo/cfg", "/opt/cs2/game/csgo/addons"},
			Ports: []PortSpec{
				{Internal: 27015, DefaultExternal: 27015, Protocol: "udp", Description: "Game port"},
				{Internal: 27020, DefaultExternal: 27020, Protocol: "udp", Description: "SourceTV port"},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 8, DiskGB: 60},
			ModSupport: true, ModSources: []string{"steam_workshop"},
			Console:  ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 27015},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "730", Validate: true},
			Docker:   DockerSpec{Image: "joedwards32/cs2", Pull: "missing"},
		},
		{
			ID: "dayz", Name: "DayZ Dedicated Server",
			SteamAppID: "223350", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/profiles", "/data/mpmissions"},
			Ports: []PortSpec{
				{Internal: 2302, DefaultExternal: 2302, Protocol: "udp", Description: "Game port"},
				{Internal: 2303, DefaultExternal: 2303, Protocol: "udp", Description: "Steam query port"},
				{Internal: 2304, DefaultExternal: 2304, Protocol: "udp", Description: "BattlEye port"},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 8, DiskGB: 30},
			ModSupport: true, ModSources: []string{"steam_workshop"},
			Console:  ConsoleConfig{Type: "stdio"},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "223350", Validate: true},
		},
		{
			ID: "terraria", Name: "Terraria Dedicated Server",
			DeployMethods: []string{"manual"},
			BackupPaths:   []string{"/data/worlds"},
			Ports: []PortSpec{
				{Internal: 7777, DefaultExternal: 7777, Protocol: "tcp", Description: "Game port"},
			},
			Resources:  ResourceSpec{CPUCores: 2, RAMGB: 2, DiskGB: 5},
			ModSupport: true, ModSources: []string{"local"},
			Console: ConsoleConfig{Type: "stdio"},
			Docker:  DockerSpec{Image: "beardandbytes/terraria-server", Pull: "missing"},
		},
		{
			ID: "rust", Name: "Rust Dedicated Server",
			SteamAppID: "252490", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/server"},
			Ports: []PortSpec{
				{Internal: 28015, DefaultExternal: 28015, Protocol: "udp", Description: "Game port"},
				{Internal: 28016, DefaultExternal: 28016, Protocol: "tcp", Description: "RCON port", AdminPort: true},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 12, DiskGB: 20},
			ModSupport: true, ModSources: []string{"umod", "oxide", "carbon"},
			Console:  ConsoleConfig{Type: "webrcon", RCONEnabled: true, RCONPort: 28016},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "252490", Validate: true},
			Docker:   DockerSpec{Image: "didstopia/rust-server", Pull: "missing"},
		},
		{
			ID: "team-fortress-2", Name: "Team Fortress 2 Dedicated Server",
			SteamAppID: "232250", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/opt/tf2/tf/cfg", "/opt/tf2/tf/addons"},
			Ports: []PortSpec{
				{Internal: 27015, DefaultExternal: 27015, Protocol: "udp", Description: "Game port"},
				{Internal: 27020, DefaultExternal: 27020, Protocol: "udp", Description: "SourceTV port"},
			},
			Resources:  ResourceSpec{CPUCores: 2, RAMGB: 4, DiskGB: 20},
			ModSupport: true, ModSources: []string{"sourcemod", "metamod"},
			Console:  ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 27015},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "232250", Validate: true},
			Docker:   DockerSpec{Image: "cm2network/tf2", Pull: "missing"},
		},
		{
			ID: "garrys-mod", Name: "Garry's Mod Dedicated Server",
			SteamAppID: "4020", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/opt/gmod/garrysmod/cfg", "/opt/gmod/garrysmod/addons"},
			Ports: []PortSpec{
				{Internal: 27015, DefaultExternal: 27015, Protocol: "udp", Description: "Game port"},
				{Internal: 27005, DefaultExternal: 27005, Protocol: "udp", Description: "Client port"},
			},
			Resources:  ResourceSpec{CPUCores: 2, RAMGB: 4, DiskGB: 20},
			ModSupport: true, ModSources: []string{"steam_workshop"},
			Console:  ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 27015},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "4020", Validate: true},
			Docker:   DockerSpec{Image: "cm2network/garrysmod", Pull: "missing"},
		},
		{
			ID: "ark-survival-ascended", Name: "ARK: Survival Ascended Dedicated Server",
			SteamAppID: "2430930", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/ShooterGame/Saved/SavedArks"},
			Ports: []PortSpec{
				{Internal: 7777, DefaultExternal: 7777, Protocol: "udp", Description: "Game port"},
				{Internal: 7778, DefaultExternal: 7778, Protocol: "udp", Description: "Raw UDP port"},
				{Internal: 27015, DefaultExternal: 27015, Protocol: "udp", Description: "Steam query port"},
				{Internal: 27020, DefaultExternal: 27020, Protocol: "tcp", Description: "RCON port", AdminPort: true},
			},
			Resources:  ResourceSpec{CPUCores: 6, RAMGB: 16, DiskGB: 60},
			ModSupport: true, ModSources: []string{"steam_workshop", "curseforge"},
			Console:  ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 27020},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "2430930", Validate: true},
		},
		{
			ID: "dont-starve-together", Name: "Don't Starve Together Dedicated Server",
			SteamAppID: "343050", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/clusters"},
			Ports: []PortSpec{
				{Internal: 10999, DefaultExternal: 10999, Protocol: "udp", Description: "Game port (Master)"},
				{Internal: 10998, DefaultExternal: 10998, Protocol: "udp", Description: "Game port (Caves)"},
				{Internal: 10888, DefaultExternal: 10888, Protocol: "udp", Description: "Shard inter-comm port"},
			},
			Resources:  ResourceSpec{CPUCores: 2, RAMGB: 4, DiskGB: 5},
			ModSupport: true, ModSources: []string{"steam_workshop"},
			Console:  ConsoleConfig{Type: "stdio"},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "343050", Validate: true},
		},
		{
			ID: "project-zomboid", Name: "Project Zomboid Dedicated Server",
			SteamAppID: "380870", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/Zomboid/Saves", "/data/Zomboid/Server"},
			Ports: []PortSpec{
				{Internal: 16261, DefaultExternal: 16261, Protocol: "udp", Description: "Game port"},
				{Internal: 16262, DefaultExternal: 16262, Protocol: "udp", Description: "Direct UDP port"},
				{Internal: 8766, DefaultExternal: 8766, Protocol: "udp", Description: "Steam port 1"},
				{Internal: 8767, DefaultExternal: 8767, Protocol: "udp", Description: "Steam port 2"},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 8, DiskGB: 10},
			ModSupport: true, ModSources: []string{"steam_workshop"},
			Console:  ConsoleConfig{Type: "stdio"},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "380870", Validate: true},
		},
		{
			ID: "7-days-to-die", Name: "7 Days to Die Dedicated Server",
			SteamAppID: "294420", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/Saves"},
			Ports: []PortSpec{
				{Internal: 26900, DefaultExternal: 26900, Protocol: "udp", Description: "Game port"},
				{Internal: 26901, DefaultExternal: 26901, Protocol: "udp", Description: "Steam query port +1"},
				{Internal: 26902, DefaultExternal: 26902, Protocol: "udp", Description: "Steam query port +2"},
				{Internal: 8081, DefaultExternal: 8081, Protocol: "tcp", Description: "Telnet console port", AdminPort: true},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 8, DiskGB: 15},
			ModSupport: true, ModSources: []string{"nexusmods"},
			Console:  ConsoleConfig{Type: "telnet"},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "294420", Validate: true},
		},
		{
			ID: "left-4-dead-2", Name: "Left 4 Dead 2 Dedicated Server",
			SteamAppID: "222860", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/opt/l4d2/left4dead2/cfg", "/opt/l4d2/left4dead2/addons"},
			Ports: []PortSpec{
				{Internal: 27015, DefaultExternal: 27015, Protocol: "udp", Description: "Game port"},
			},
			Resources:  ResourceSpec{CPUCores: 2, RAMGB: 4, DiskGB: 15},
			ModSupport: true, ModSources: []string{"steam_workshop", "sourcemod"},
			Console:  ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 27015},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "222860", Validate: true},
			Docker:   DockerSpec{Image: "cm2network/l4d2", Pull: "missing"},
		},
		{
			ID: "factorio", Name: "Factorio Dedicated Server",
			SteamAppID: "427520", DeployMethods: []string{"manual", "steamcmd"},
			BackupPaths: []string{"/data/saves", "/data/mods"},
			Ports: []PortSpec{
				{Internal: 34197, DefaultExternal: 34197, Protocol: "udp", Description: "Game port"},
				{Internal: 27015, DefaultExternal: 27015, Protocol: "tcp", Description: "RCON port", AdminPort: true},
			},
			Resources:  ResourceSpec{CPUCores: 4, RAMGB: 4, DiskGB: 5},
			ModSupport: true, ModSources: []string{"factorio_mod_portal"},
			Console:  ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 27015},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "427520", Validate: true},
			Docker:   DockerSpec{Image: "factoriotools/factorio", Pull: "missing"},
		},
		{
			ID: "risk-of-rain-2", Name: "Risk of Rain 2 Dedicated Server",
			SteamAppID: "1180760", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/RiskOfRain2/config"},
			Ports: []PortSpec{
				{Internal: 27015, DefaultExternal: 27015, Protocol: "udp", Description: "Game port"},
				{Internal: 27016, DefaultExternal: 27016, Protocol: "udp", Description: "Steam query port"},
			},
			Resources:  ResourceSpec{CPUCores: 2, RAMGB: 4, DiskGB: 10},
			ModSupport: true, ModSources: []string{"thunderstore"},
			Console:  ConsoleConfig{Type: "stdio"},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "1180760", Validate: true},
		},
		{
			ID: "squad", Name: "Squad Dedicated Server",
			SteamAppID: "403240", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/opt/squad/SquadGame/ServerConfig"},
			Ports: []PortSpec{
				{Internal: 7787, DefaultExternal: 7787, Protocol: "udp", Description: "Game port"},
				{Internal: 27165, DefaultExternal: 27165, Protocol: "udp", Description: "Steam query port"},
				{Internal: 21114, DefaultExternal: 21114, Protocol: "tcp", Description: "RCON port", AdminPort: true},
			},
			Resources:  ResourceSpec{CPUCores: 8, RAMGB: 16, DiskGB: 60},
			ModSupport: true, ModSources: []string{"steam_workshop"},
			Console:  ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 21114},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "403240", Validate: true},
		},
		{
			ID: "conan-exiles", Name: "Conan Exiles Dedicated Server",
			SteamAppID: "443030", DeployMethods: []string{"steamcmd"},
			BackupPaths: []string{"/data/ConanSandbox/Saved/savegames"},
			Ports: []PortSpec{
				{Internal: 7777, DefaultExternal: 7777, Protocol: "udp", Description: "Game port"},
				{Internal: 7778, DefaultExternal: 7778, Protocol: "udp", Description: "Raw UDP port"},
				{Internal: 27015, DefaultExternal: 27015, Protocol: "udp", Description: "Steam query port"},
				{Internal: 25575, DefaultExternal: 25575, Protocol: "tcp", Description: "RCON port", AdminPort: true},
			},
			Resources:  ResourceSpec{CPUCores: 6, RAMGB: 16, DiskGB: 40},
			ModSupport: true, ModSources: []string{"steam_workshop"},
			Console:  ConsoleConfig{Type: "rcon", RCONEnabled: true, RCONPort: 25575},
			SteamCMD: SteamCMDSpec{Login: "anonymous", AppID: "443030", Validate: true},
			Docker:   DockerSpec{Image: "alinmear/docker-conanexiles", Pull: "missing"},
		},
	}

	for _, m := range defaults {
		r.manifests[m.ID] = m
	}
}
