// Package config manages the gdash CLI configuration file at ~/.gdash/config.yaml.
// It persists daemon URL, auth token, and preferred output format across invocations.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const configFileName = "config.yaml"

// Config holds all persisted CLI settings.
type Config struct {
	// DaemonURL is the base URL of the games-dashboard daemon.
	DaemonURL string `yaml:"daemon_url"`
	// Token is the Bearer token saved after a successful login.
	Token string `yaml:"token,omitempty"`
	// Output is the default output format: text | json.
	Output string `yaml:"output"`
	// Insecure skips TLS certificate verification.
	Insecure bool `yaml:"insecure"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DaemonURL: "https://localhost:8443",
		Output:    "text",
		Insecure:  false,
	}
}

// ConfigDir returns the path to the gdash configuration directory (~/.gdash).
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".gdash"), nil
}

// configPath returns the full path to the config file.
func configPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// Load reads the config file, returning defaults if the file does not exist.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config to disk, creating the directory if necessary.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}
	return nil
}

// Set updates a single key in the config file and saves it.
func Set(key, value string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	switch key {
	case "daemon_url", "daemon":
		cfg.DaemonURL = value
	case "token":
		cfg.Token = value
	case "output":
		if value != "text" && value != "json" {
			return fmt.Errorf("invalid output format %q: must be text or json", value)
		}
		cfg.Output = value
	case "insecure":
		cfg.Insecure = value == "true" || value == "1" || value == "yes"
	default:
		return fmt.Errorf("unknown config key %q: valid keys: daemon_url, token, output, insecure", key)
	}
	return Save(cfg)
}

// ClearToken removes the saved auth token from the config.
func ClearToken() error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Token = ""
	return Save(cfg)
}

// SaveToken persists a new auth token, loading existing config first.
func SaveToken(token string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Token = token
	return Save(cfg)
}
