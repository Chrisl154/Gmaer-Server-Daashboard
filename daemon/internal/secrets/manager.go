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
	cfg    Config
	logger *zap.Logger
	key    []byte
}

// NewManager creates a new secrets manager
func NewManager(cfg Config, logger *zap.Logger) (*Manager, error) {
	m := &Manager{
		cfg:    cfg,
		logger: logger,
	}

	switch cfg.Backend {
	case "vault":
		// TODO: initialize Vault client
		logger.Warn("Vault backend not yet configured, falling back to local")
		fallthrough
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

// Rotate generates a new encryption key
func (m *Manager) Rotate(ctx context.Context) error {
	m.logger.Info("Rotating secrets encryption key")
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return err
	}
	m.key = newKey
	return m.saveKey()
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
