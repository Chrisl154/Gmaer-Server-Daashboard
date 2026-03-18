package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// NotificationsConfig holds webhook notification settings.
type NotificationsConfig struct {
	WebhookURL    string   `yaml:"webhook_url" json:"webhook_url"`
	WebhookFormat string   `yaml:"webhook_format" json:"webhook_format"` // discord|slack|generic
	Events        []string `yaml:"events" json:"events"`                 // server.crash|server.restart|disk.warning|backup.failed|backup.complete
}

// Config is the top-level daemon configuration
type Config struct {
	BindAddr        string              `yaml:"bind_addr" json:"bind_addr"`
	TLS             TLSConfig           `yaml:"tls" json:"tls"`
	Auth            AuthConfig          `yaml:"auth" json:"auth"`
	Secrets         SecretsConfig       `yaml:"secrets" json:"secrets"`
	Storage         StorageConfig       `yaml:"storage" json:"storage"`
	ShutdownTimeout time.Duration       `yaml:"shutdown_timeout" json:"shutdown_timeout"`
	Adapters        AdaptersConfig      `yaml:"adapters" json:"adapters"`
	Backup          BackupConfig        `yaml:"backup" json:"backup"`
	Metrics         MetricsConfig       `yaml:"metrics" json:"metrics"`
	Cluster         ClusterConfig       `yaml:"cluster" json:"cluster"`
	Notifications   NotificationsConfig `yaml:"notifications" json:"notifications"`
	LogLevel        string              `yaml:"log_level" json:"log_level"`
	DataDir         string              `yaml:"data_dir" json:"data_dir"`
}

type TLSConfig struct {
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file" json:"key_file"`
	AutoTLS  bool   `yaml:"auto_tls" json:"auto_tls"`
}

type AuthConfig struct {
	Local       LocalAuthConfig  `yaml:"local" json:"local"`
	OIDC        *OIDCConfig      `yaml:"oidc,omitempty" json:"oidc,omitempty"`
	SAML        *SAMLConfig      `yaml:"saml,omitempty" json:"saml,omitempty"`
	JWTSecret   string           `yaml:"jwt_secret" json:"jwt_secret"`
	TokenTTL    time.Duration    `yaml:"token_ttl" json:"token_ttl"`
	MFARequired bool             `yaml:"mfa_required" json:"mfa_required"`
}

type LocalAuthConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	AdminUser string `yaml:"admin_user" json:"admin_user"`
	AdminPassHash string `yaml:"admin_pass_hash" json:"admin_pass_hash"`
}

type OIDCConfig struct {
	Issuer       string `yaml:"issuer" json:"issuer"`
	ClientID     string `yaml:"client_id" json:"client_id"`
	ClientSecret string `yaml:"client_secret" json:"client_secret"`
	RedirectURL  string `yaml:"redirect_url" json:"redirect_url"`
}

type SAMLConfig struct {
	MetadataURL string `yaml:"metadata_url" json:"metadata_url"`
	EntityID    string `yaml:"entity_id" json:"entity_id"`
}

type SecretsConfig struct {
	Backend     string `yaml:"backend" json:"backend"` // local|vault
	KeyFile     string `yaml:"key_file" json:"key_file"`
	VaultAddr   string `yaml:"vault_addr,omitempty" json:"vault_addr,omitempty"`
	VaultToken  string `yaml:"vault_token,omitempty" json:"vault_token,omitempty"`
	VaultPath   string `yaml:"vault_path,omitempty" json:"vault_path,omitempty"`
}

type StorageConfig struct {
	DataDir   string       `yaml:"data_dir" json:"data_dir"`
	NFS       []NFSMount   `yaml:"nfs_mounts" json:"nfs_mounts"`
	S3        *S3Config    `yaml:"s3,omitempty" json:"s3,omitempty"`
}

type NFSMount struct {
	Server     string `yaml:"server" json:"server"`
	Path       string `yaml:"path" json:"path"`
	MountPoint string `yaml:"mount_point" json:"mount_point"`
	Options    string `yaml:"options" json:"options"`
}

type S3Config struct {
	Endpoint        string `yaml:"endpoint" json:"endpoint"`
	Bucket          string `yaml:"bucket" json:"bucket"`
	AccessKey       string `yaml:"access_key" json:"access_key"`
	SecretKey       string `yaml:"secret_key" json:"secret_key"`
	Region          string `yaml:"region" json:"region"`
	UseSSL          bool   `yaml:"use_ssl" json:"use_ssl"`
}

type AdaptersConfig struct {
	Dir string `yaml:"dir" json:"dir"`
}

type BackupConfig struct {
	DefaultSchedule string `yaml:"default_schedule" json:"default_schedule"`
	RetainDays      int    `yaml:"retain_days" json:"retain_days"`
	Compression     string `yaml:"compression" json:"compression"` // gzip|zstd
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Path    string `yaml:"path" json:"path"`
}

type ClusterConfig struct {
	Enabled             bool          `yaml:"enabled" json:"enabled"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	NodeTimeout         time.Duration `yaml:"node_timeout" json:"node_timeout"`
	// NodeSavePath is where registered nodes are persisted across daemon restarts.
	// Defaults to <data_dir>/nodes.json when cluster is enabled.
	NodeSavePath string `yaml:"node_save_path" json:"node_save_path"`
}

// Write serialises cfg to a YAML file at path, creating or truncating it.
func Write(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaults(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	cfg := defaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		BindAddr:        ":8443",
		ShutdownTimeout: 30 * time.Second,
		TLS: TLSConfig{
			CertFile: "/etc/games-dashboard/tls/server.crt",
			KeyFile:  "/etc/games-dashboard/tls/server.key",
		},
		Auth: AuthConfig{
			Local: LocalAuthConfig{Enabled: true},
			TokenTTL: 24 * time.Hour,
			JWTSecret: "change-me-in-production",
		},
		Secrets: SecretsConfig{
			Backend: "local",
			KeyFile: "/etc/games-dashboard/secrets/master.key",
		},
		Storage: StorageConfig{
			DataDir: "/var/lib/games-dashboard",
		},
		Adapters: AdaptersConfig{
			Dir: "/etc/games-dashboard/adapters",
		},
		Backup: BackupConfig{
			DefaultSchedule: "0 3 * * *",
			RetainDays:      30,
			Compression:     "zstd",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
		},
		LogLevel: "info",
		DataDir:  "/var/lib/games-dashboard",
	}
}
