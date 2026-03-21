package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// NotificationsConfig holds webhook and email notification settings.
type NotificationsConfig struct {
	WebhookURL    string       `yaml:"webhook_url" json:"webhook_url"`
	WebhookFormat string       `yaml:"webhook_format" json:"webhook_format"` // discord|slack|generic
	Events        []string     `yaml:"events" json:"events"`                 // server.crash|server.restart|disk.warning|backup.failed|backup.complete
	Email         *EmailConfig `yaml:"email,omitempty" json:"email,omitempty"`
}

// EmailConfig holds SMTP settings for email notifications.
type EmailConfig struct {
	Enabled  bool     `yaml:"enabled" json:"enabled"`
	SMTPHost string   `yaml:"smtp_host" json:"smtp_host"`
	SMTPPort int      `yaml:"smtp_port" json:"smtp_port"` // 587 (STARTTLS) or 465 (implicit TLS)
	Username string   `yaml:"username" json:"username"`
	Password string   `yaml:"password" json:"password"`
	From     string   `yaml:"from" json:"from"`
	To       []string `yaml:"to" json:"to"`
	// UseTLS switches between STARTTLS (false, port 587) and implicit TLS (true, port 465).
	UseTLS bool `yaml:"use_tls" json:"use_tls"`
}

// LogRotationConfig controls per-server daemon event log rotation.
type LogRotationConfig struct {
	MaxSizeMB  int  `yaml:"max_size_mb" json:"max_size_mb"`   // rotate when file exceeds this (default 100)
	MaxBackups int  `yaml:"max_backups" json:"max_backups"`   // keep this many rotated files (default 5)
	MaxAgeDays int  `yaml:"max_age_days" json:"max_age_days"` // delete rotated files older than this (default 30)
	Compress   bool `yaml:"compress" json:"compress"`         // gzip rotated files (default true)
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
	LogRotation     LogRotationConfig   `yaml:"log_rotation" json:"log_rotation"`
	LogLevel        string              `yaml:"log_level" json:"log_level"`
	DataDir         string              `yaml:"data_dir" json:"data_dir"`
	Updates         UpdateConfig        `yaml:"updates" json:"updates"`
}

// UpdateConfig controls self-update behaviour.
type UpdateConfig struct {
	// RequireSignedCommits rejects updates whose tip commit is not GPG-signed.
	// Set to false only if the repository does not use commit signing.
	RequireSignedCommits bool `yaml:"require_signed_commits" json:"require_signed_commits"`
}

type TLSConfig struct {
	CertFile     string `yaml:"cert_file" json:"cert_file"`
	KeyFile      string `yaml:"key_file" json:"key_file"`
	// AutoTLS enables Let's Encrypt certificate issuance and automatic renewal
	// via the ACME HTTP-01 challenge. Requires ACMEDomain and ACMEEmail to be set,
	// and port 80 to be reachable from the internet.
	AutoTLS      bool   `yaml:"auto_tls" json:"auto_tls"`
	ACMEDomain   string `yaml:"acme_domain" json:"acme_domain"`     // domain to issue the cert for
	ACMEEmail    string `yaml:"acme_email" json:"acme_email"`       // contact email for Let's Encrypt account
	ACMECacheDir string `yaml:"acme_cache_dir" json:"acme_cache_dir"` // where to persist ACME state (default /etc/games-dashboard/tls/acme)
	ACMEStaging  bool   `yaml:"acme_staging" json:"acme_staging"`   // use Let's Encrypt staging CA (for testing)
}

type AuthConfig struct {
	Local       LocalAuthConfig  `yaml:"local" json:"local"`
	OIDC        *OIDCConfig      `yaml:"oidc,omitempty" json:"oidc,omitempty"`
	SAML        *SAMLConfig      `yaml:"saml,omitempty" json:"saml,omitempty"`
	Steam       *SteamConfig     `yaml:"steam,omitempty" json:"steam,omitempty"`
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

type SteamConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	APIKey      string `yaml:"api_key" json:"api_key"`
	ReturnURL   string `yaml:"return_url" json:"return_url"`
	Realm       string `yaml:"realm" json:"realm"`
	FrontendURL string `yaml:"frontend_url" json:"frontend_url"`
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
			Local:    LocalAuthConfig{Enabled: true},
			TokenTTL: 2 * time.Hour,
			// JWTSecret intentionally left empty — main.go calls resolveJWTSecret
			// which loads an existing secret from data_dir/jwt_secret or generates
			// a fresh 64-character random secret and persists it.
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
		LogRotation: LogRotationConfig{
			MaxSizeMB:  100,
			MaxBackups: 5,
			MaxAgeDays: 30,
			Compress:   true,
		},
		LogLevel: "info",
		DataDir:  "/var/lib/games-dashboard",
		Updates: UpdateConfig{
			RequireSignedCommits: true,
		},
	}
}
