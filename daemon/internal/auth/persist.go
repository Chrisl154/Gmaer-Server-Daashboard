package auth

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ── User persistence ──────────────────────────────────────────────────────────
//
// users.json stores every user including fields tagged json:"-" on the User
// struct (PasswordHash, TOTPSecret, RecoveryCodes, APIKey.Hash). A separate
// storedUser type carries explicit json tags so the secret fields are written
// to disk but never appear in API responses (which use sanitizeUser).

type storedAPIKey struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	Hash      string     `json:"hash"` // SHA-256 hex — safe to store, useless without raw token
	Roles     []string   `json:"roles"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type storedUser struct {
	ID                string             `json:"id"`
	Username          string             `json:"username"`
	PasswordHash      string             `json:"password_hash"`
	Roles             []string           `json:"roles"`
	AllowedServers    []string           `json:"allowed_servers,omitempty"`
	TOTPEnabled       bool               `json:"totp_enabled"`
	TOTPSecret        string             `json:"totp_secret,omitempty"`
	RecoveryCodes     []string           `json:"recovery_codes,omitempty"`
	PushSubscriptions []PushSubscription `json:"push_subscriptions,omitempty"`
	APIKeys           []storedAPIKey     `json:"api_keys,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	LastLogin         time.Time          `json:"last_login"`
}

func toStoredUser(u *User) storedUser {
	keys := make([]storedAPIKey, len(u.APIKeys))
	for i, k := range u.APIKeys {
		keys[i] = storedAPIKey{
			ID:        k.ID,
			Name:      k.Name,
			Prefix:    k.Prefix,
			Hash:      k.Hash,
			Roles:     k.Roles,
			CreatedAt: k.CreatedAt,
			LastUsed:  k.LastUsed,
			ExpiresAt: k.ExpiresAt,
		}
	}
	return storedUser{
		ID:                u.ID,
		Username:          u.Username,
		PasswordHash:      u.PasswordHash,
		Roles:             u.Roles,
		AllowedServers:    u.AllowedServers,
		TOTPEnabled:       u.TOTPEnabled,
		TOTPSecret:        u.TOTPSecret,
		RecoveryCodes:     u.RecoveryCodes,
		PushSubscriptions: u.PushSubscriptions,
		APIKeys:           keys,
		CreatedAt:         u.CreatedAt,
		LastLogin:         u.LastLogin,
	}
}

func fromStoredUser(s storedUser) *User {
	keys := make([]APIKey, len(s.APIKeys))
	for i, k := range s.APIKeys {
		keys[i] = APIKey{
			ID:        k.ID,
			Name:      k.Name,
			Prefix:    k.Prefix,
			Hash:      k.Hash,
			Roles:     k.Roles,
			CreatedAt: k.CreatedAt,
			LastUsed:  k.LastUsed,
			ExpiresAt: k.ExpiresAt,
		}
	}
	return &User{
		ID:                s.ID,
		Username:          s.Username,
		PasswordHash:      s.PasswordHash,
		Roles:             s.Roles,
		AllowedServers:    s.AllowedServers,
		TOTPEnabled:       s.TOTPEnabled,
		TOTPSecret:        s.TOTPSecret,
		RecoveryCodes:     s.RecoveryCodes,
		PushSubscriptions: s.PushSubscriptions,
		APIKeys:           keys,
		CreatedAt:         s.CreatedAt,
		LastLogin:         s.LastLogin,
	}
}

func (s *Service) usersPath() string {
	return filepath.Join(s.cfg.DataDir, "users.json")
}

// saveUsers atomically writes all users to disk. Must be called with s.mu held
// or from a context where no concurrent mutations are possible.
func (s *Service) saveUsers() {
	if s.cfg.DataDir == "" {
		return
	}
	stored := make([]storedUser, 0, len(s.users))
	for _, u := range s.users {
		stored = append(stored, toStoredUser(u))
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		s.logger.Sugar().Warnf("auth: failed to marshal users: %v", err)
		return
	}
	_ = os.MkdirAll(s.cfg.DataDir, 0o700)
	tmp := s.usersPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		s.logger.Sugar().Warnf("auth: failed to write users.json.tmp: %v", err)
		return
	}
	if err := os.Rename(tmp, s.usersPath()); err != nil {
		_ = os.Remove(tmp)
		s.logger.Sugar().Warnf("auth: failed to rename users.json: %v", err)
	}
}

// loadUsers reads users.json and populates s.users. The admin user from the
// config file is always merged in (or updated) so that daemon.yaml remains
// the authoritative source for the admin password.
func (s *Service) loadUsers() {
	if s.cfg.DataDir == "" {
		return
	}
	data, err := os.ReadFile(s.usersPath()) //nolint:gosec
	if err != nil {
		return // file doesn't exist yet — fresh install
	}
	var stored []storedUser
	if err := json.Unmarshal(data, &stored); err != nil {
		s.logger.Sugar().Warnf("auth: failed to parse users.json: %v", err)
		return
	}
	for _, su := range stored {
		s.users[su.Username] = fromStoredUser(su)
	}
}

// mergeAdminFromConfig ensures the admin user from daemon.yaml is always present
// and has the latest password hash from config (in case it was rotated).
func (s *Service) mergeAdminFromConfig() {
	if !s.cfg.Local.Enabled || s.cfg.Local.Admin.Username == "" || s.cfg.Local.Admin.PasswordHash == "" {
		return
	}
	adminUsername := s.cfg.Local.Admin.Username
	if existing, ok := s.users[adminUsername]; ok {
		// Update password hash from config in case it was rotated externally.
		existing.PasswordHash = s.cfg.Local.Admin.PasswordHash
	} else {
		s.users[adminUsername] = &User{
			ID:           "admin-0",
			Username:     adminUsername,
			PasswordHash: s.cfg.Local.Admin.PasswordHash,
			Roles:        []string{"admin"},
			CreatedAt:    time.Now(),
		}
	}
}

// ── Audit log persistence ─────────────────────────────────────────────────────
//
// The audit log is appended to {dataDir}/audit.log as newline-delimited JSON.
// On startup the last 10,000 lines are loaded into the in-memory slice.
// This keeps memory bounded while still allowing long-term on-disk retention.

const maxAuditMemory = 10_000
const maxAuditDisk = 50_000 // rotate file when it exceeds this many entries

func (s *Service) auditLogPath() string {
	return filepath.Join(s.cfg.DataDir, "audit.log")
}

// appendAuditEntry writes a single audit entry to the JSONL file.
func (s *Service) appendAuditEntry(entry AuditEntry) {
	if s.cfg.DataDir == "" {
		return
	}
	_ = os.MkdirAll(s.cfg.DataDir, 0o700)
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(s.auditLogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

// loadAuditLog reads up to maxAuditMemory entries from the JSONL audit file
// into the in-memory slice (newest entries last).
func (s *Service) loadAuditLog() {
	if s.cfg.DataDir == "" {
		return
	}
	f, err := os.Open(s.auditLogPath()) //nolint:gosec
	if err != nil {
		return
	}
	defer f.Close()

	// Collect all lines into a circular buffer of size maxAuditMemory.
	buf := make([]AuditEntry, 0, maxAuditMemory)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry AuditEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if len(buf) >= maxAuditMemory {
			buf = append(buf[1:], entry)
		} else {
			buf = append(buf, entry)
		}
	}
	s.auditLogMu.Lock()
	s.auditLog = buf
	s.auditLogMu.Unlock()
}
