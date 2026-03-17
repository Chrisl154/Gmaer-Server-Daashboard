package secrets

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	m, err := NewManager(Config{
		Backend: "local",
		KeyFile: filepath.Join(dir, "master.key"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestNewManager_CreatesKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "master.key")
	m, err := NewManager(Config{Backend: "local", KeyFile: keyFile}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
	if len(m.key) != 32 {
		t.Errorf("key length = %d, want 32", len(m.key))
	}
	if _, err := os.Stat(keyFile); err != nil {
		t.Errorf("key file not created: %v", err)
	}
}

func TestNewManager_LoadsExistingKey(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "master.key")

	m1, err := NewManager(Config{Backend: "local", KeyFile: keyFile}, zap.NewNop())
	if err != nil {
		t.Fatalf("first NewManager: %v", err)
	}
	key1 := make([]byte, 32)
	copy(key1, m1.key)

	m2, err := NewManager(Config{Backend: "local", KeyFile: keyFile}, zap.NewNop())
	if err != nil {
		t.Fatalf("second NewManager: %v", err)
	}
	if string(m2.key) != string(key1) {
		t.Error("loaded key does not match original")
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	m := newTestManager(t)
	cases := []string{"", "hello world", "secret-password-123!", "unicode: \u00e9\u00e0\u00fc"}
	for _, plain := range cases {
		enc, err := m.Encrypt(plain)
		if err != nil {
			t.Errorf("Encrypt(%q): %v", plain, err)
			continue
		}
		got, err := m.Decrypt(enc)
		if err != nil {
			t.Errorf("Decrypt after Encrypt(%q): %v", plain, err)
			continue
		}
		if got != plain {
			t.Errorf("roundtrip %q: got %q", plain, got)
		}
	}
}

func TestEncrypt_RandomNonce(t *testing.T) {
	m := newTestManager(t)
	c1, _ := m.Encrypt("same plaintext")
	c2, _ := m.Encrypt("same plaintext")
	if c1 == c2 {
		t.Error("expected different ciphertexts due to random nonce")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Decrypt("not!!valid!!base64")
	if err == nil {
		t.Error("expected error for invalid base64 input")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	m := newTestManager(t)
	// Encode a single byte — shorter than AES-GCM nonce (12 bytes)
	short := base64.StdEncoding.EncodeToString([]byte("a"))
	_, err := m.Decrypt(short)
	if err == nil {
		t.Error("expected error for ciphertext shorter than nonce size")
	}
}

func TestRotate_ChangesKey(t *testing.T) {
	m := newTestManager(t)
	key1 := make([]byte, 32)
	copy(key1, m.key)

	if err := m.Rotate(context.Background()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if string(m.key) == string(key1) {
		t.Error("key unchanged after rotation")
	}
	if len(m.key) != 32 {
		t.Errorf("key length after rotate = %d, want 32", len(m.key))
	}
}

func TestRotate_NewKeyUsableForEncryption(t *testing.T) {
	m := newTestManager(t)
	if err := m.Rotate(context.Background()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	enc, err := m.Encrypt("post-rotate secret")
	if err != nil {
		t.Fatalf("Encrypt after rotate: %v", err)
	}
	got, err := m.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt after rotate: %v", err)
	}
	if got != "post-rotate secret" {
		t.Errorf("got %q, want post-rotate secret", got)
	}
}
