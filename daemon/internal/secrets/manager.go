package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	vaultapi "github.com/hashicorp/vault/api"
	"go.uber.org/zap"
)

// Config holds secrets manager configuration
type Config struct {
	Backend    string `yaml:"backend" json:"backend"` // local|vault
	KeyFile    string `yaml:"key_file" json:"key_file"`
	VaultAddr  string `yaml:"vault_addr,omitempty" json:"vault_addr,omitempty"`
	VaultToken string `yaml:"vault_token,omitempty" json:"vault_token,omitempty"`
	VaultPath  string `yaml:"vault_path,omitempty" json:"vault_path,omitempty"`
}

// Manager handles encryption/decryption of secrets
type Manager struct {
	cfg         Config
	logger      *zap.Logger
	key         []byte
	vaultClient *vaultapi.Client // non-nil when vault backend is active
}

// NewManager creates a new secrets manager
func NewManager(cfg Config, logger *zap.Logger) (*Manager, error) {
	m := &Manager{
		cfg:    cfg,
		logger: logger,
	}

	switch cfg.Backend {
	case "vault":
		if err := m.initVaultBackend(); err != nil {
			logger.Warn("Vault backend initialization failed, falling back to local AES key",
				zap.Error(err))
			if localErr := m.loadOrCreateKey(); localErr != nil {
				return nil, fmt.Errorf("failed to initialize local KMS: %w", localErr)
			}
		}
	default:
		if err := m.loadOrCreateKey(); err != nil {
			return nil, fmt.Errorf("failed to initialize local KMS: %w", err)
		}
	}

	return m, nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext
func (m *Manager) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext
func (m *Manager) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(m.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// Rotate generates a new encryption key and persists it to whichever backend is active.
func (m *Manager) Rotate(ctx context.Context) error {
	m.logger.Info("Rotating secrets encryption key")
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return err
	}
	m.key = newKey
	if m.vaultClient != nil {
		return m.saveVaultKey()
	}
	return m.saveKey()
}

// initVaultBackend creates a Vault client and loads (or generates) the master key
// from the configured Vault KV path. The master key is used for local AES-256-GCM
// encryption — Vault acts as an external key store (envelope encryption).
func (m *Manager) initVaultBackend() error {
	vaultCfg := vaultapi.DefaultConfig()
	if m.cfg.VaultAddr != "" {
		vaultCfg.Address = m.cfg.VaultAddr
	}

	client, err := vaultapi.NewClient(vaultCfg)
	if err != nil {
		return fmt.Errorf("create vault client: %w", err)
	}

	if m.cfg.VaultToken != "" {
		client.SetToken(m.cfg.VaultToken)
	}

	m.vaultClient = client
	return m.loadOrCreateVaultKey()
}

// vaultKeyPath returns the KV path used to store the master key.
func (m *Manager) vaultKeyPath() string {
	if m.cfg.VaultPath != "" {
		return m.cfg.VaultPath
	}
	return "secret/data/games-dashboard/master-key"
}

// loadOrCreateVaultKey reads the master key from Vault KV.
// If the secret does not exist yet it generates a new key and writes it.
func (m *Manager) loadOrCreateVaultKey() error {
	path := m.vaultKeyPath()

	secret, err := m.vaultClient.Logical().Read(path)
	if err != nil {
		return fmt.Errorf("vault read %s: %w", path, err)
	}

	if secret != nil && secret.Data != nil {
		// KV v2 wraps the payload in a "data" key; KV v1 stores it directly.
		data := secret.Data
		if nested, ok := secret.Data["data"].(map[string]interface{}); ok {
			data = nested
		}
		if keyStr, ok := data["key"].(string); ok && keyStr != "" {
			decoded, decErr := base64.StdEncoding.DecodeString(keyStr)
			if decErr != nil {
				return fmt.Errorf("decode vault key: %w", decErr)
			}
			if len(decoded) != 32 {
				return fmt.Errorf("vault key has wrong length: %d (expected 32)", len(decoded))
			}
			m.key = decoded
			m.logger.Info("Loaded master key from Vault", zap.String("path", path))
			return nil
		}
	}

	// Key not present — generate and store it
	newKey := make([]byte, 32)
	if _, genErr := io.ReadFull(rand.Reader, newKey); genErr != nil {
		return fmt.Errorf("generate master key: %w", genErr)
	}
	m.key = newKey
	if writeErr := m.saveVaultKey(); writeErr != nil {
		return writeErr
	}
	m.logger.Info("Generated and stored master key in Vault", zap.String("path", path))
	return nil
}

// saveVaultKey persists the current master key to Vault KV.
func (m *Manager) saveVaultKey() error {
	path := m.vaultKeyPath()
	encoded := base64.StdEncoding.EncodeToString(m.key)
	// Use KV v2 write format; Vault KV v1 paths will ignore the nested "data" key harmlessly.
	writeData := map[string]interface{}{
		"data": map[string]interface{}{
			"key": encoded,
		},
	}
	if _, err := m.vaultClient.Logical().Write(path, writeData); err != nil {
		return fmt.Errorf("vault write %s: %w", path, err)
	}
	return nil
}

func (m *Manager) loadOrCreateKey() error {
	if m.cfg.KeyFile == "" {
		m.cfg.KeyFile = "/etc/games-dashboard/secrets/master.key"
	}

	data, err := os.ReadFile(m.cfg.KeyFile)
	if os.IsNotExist(err) {
		return m.generateAndSaveKey()
	}
	if err != nil {
		return err
	}

	key, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return err
	}

	if len(key) != 32 {
		return fmt.Errorf("invalid key length: expected 32 bytes")
	}

	m.key = key
	return nil
}

func (m *Manager) generateAndSaveKey() error {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return err
	}
	m.key = key
	return m.saveKey()
}

func (m *Manager) saveKey() error {
	if err := os.MkdirAll(fmt.Sprintf("%s/..", m.cfg.KeyFile), 0700); err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(m.key)
	return os.WriteFile(m.cfg.KeyFile, []byte(encoded), 0600)
}
