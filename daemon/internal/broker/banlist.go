package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rconpkg "github.com/games-dashboard/daemon/internal/rcon"
	telnetsvc "github.com/games-dashboard/daemon/internal/telnet"
	webrconpkg "github.com/games-dashboard/daemon/internal/webrcon"
)

// ErrBanlistNotSupported is returned when the game adapter does not support
// banlist or whitelist management via RCON.
var ErrBanlistNotSupported = fmt.Errorf("banlist management is not supported for this game type")

// ── P47: file-backed ban list for RCON-less games ─────────────────────────────

// banlistDir returns the directory used to persist per-server ban files.
func (b *Broker) banlistDir() string {
	dir := "/opt/gdash/data/banlists"
	if b.cfg != nil && b.cfg.Storage.DataDir != "" {
		dir = filepath.Join(b.cfg.Storage.DataDir, "banlists")
	}
	return dir
}

// banlistPath returns the JSON file path for a server's file-backed ban list.
func (b *Broker) banlistPath(serverID string) string {
	return filepath.Join(b.banlistDir(), serverID+".json")
}

// loadFileBanList reads the ban list from disk into the in-memory cache.
// Returns an empty slice when the file doesn't exist yet.
func (b *Broker) loadFileBanList(serverID string) []string {
	data, err := os.ReadFile(b.banlistPath(serverID)) //nolint:gosec
	if err != nil {
		return []string{}
	}
	var players []string
	if err := json.Unmarshal(data, &players); err != nil {
		return []string{}
	}
	return players
}

// saveFileBanList persists the in-memory ban list to disk.
func (b *Broker) saveFileBanList(serverID string, players []string) {
	dir := b.banlistDir()
	_ = os.MkdirAll(dir, 0o700)
	data, err := json.Marshal(players)
	if err != nil {
		return
	}
	tmp := b.banlistPath(serverID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, b.banlistPath(serverID))
}

// getFileBanList returns the cached ban list, loading from disk on first access.
func (b *Broker) getFileBanList(serverID string) []string {
	b.banMu.Lock()
	defer b.banMu.Unlock()
	if players, ok := b.fileBanLists[serverID]; ok {
		return players
	}
	players := b.loadFileBanList(serverID)
	b.fileBanLists[serverID] = players
	return players
}

// addFileBan adds a player to the file-backed ban list.
func (b *Broker) addFileBan(serverID, player string) {
	b.banMu.Lock()
	defer b.banMu.Unlock()
	if _, ok := b.fileBanLists[serverID]; !ok {
		b.fileBanLists[serverID] = b.loadFileBanList(serverID)
	}
	for _, p := range b.fileBanLists[serverID] {
		if strings.EqualFold(p, player) {
			return // already banned
		}
	}
	b.fileBanLists[serverID] = append(b.fileBanLists[serverID], player)
	b.saveFileBanList(serverID, b.fileBanLists[serverID])
}

// removeFileBan removes a player from the file-backed ban list.
func (b *Broker) removeFileBan(serverID, player string) {
	b.banMu.Lock()
	defer b.banMu.Unlock()
	if _, ok := b.fileBanLists[serverID]; !ok {
		b.fileBanLists[serverID] = b.loadFileBanList(serverID)
	}
	updated := b.fileBanLists[serverID][:0]
	for _, p := range b.fileBanLists[serverID] {
		if !strings.EqualFold(p, player) {
			updated = append(updated, p)
		}
	}
	b.fileBanLists[serverID] = updated
	b.saveFileBanList(serverID, updated)
}

// supportsBanlist returns true for adapters with RCON ban support.
func supportsBanlist(adapter string) bool {
	switch adapter {
	case "minecraft", "ark-survival-ascended", "conan-exiles":
		return true
	}
	return false
}

// supportsWhitelist returns true for adapters with RCON whitelist support.
func supportsWhitelist(adapter string) bool {
	return adapter == "minecraft"
}

// execRCON runs a console command silently (no console-stream echo).
// Used for read-only queries such as banlist/whitelist listing.
func (b *Broker) execRCON(ctx context.Context, id, command string) (string, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("server not found: %s", id)
	}
	if s.State != StateRunning {
		return "", fmt.Errorf("server must be running to query player data")
	}
	manifest, ok := b.adapters.Get(s.Adapter)
	if !ok {
		return "", fmt.Errorf("adapter %q not found", s.Adapter)
	}
	addr := fmt.Sprintf("localhost:%d", manifest.Console.RCONPort)

	switch manifest.Console.Type {
	case "webrcon":
		pass, _ := s.Config["rcon_password"].(string)
		return webrconpkg.Exec(addr, pass, command, 5*time.Second)
	case "telnet":
		pass, _ := s.Config["telnet_password"].(string)
		if pass == "" {
			pass, _ = s.Config["rcon_password"].(string)
		}
		return telnetsvc.Exec(addr, pass, command, 5*time.Second)
	default:
		pass, _ := s.Config["rcon_password"].(string)
		return rconpkg.Exec(addr, pass, command, 5*time.Second)
	}
}

// ListBannedPlayers returns the list of banned player names for the server.
// For games with RCON ban support, queries the live server. For all others
// (P47) returns the persisted file-backed ban list.
func (b *Broker) ListBannedPlayers(ctx context.Context, id string) ([]string, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server not found: %s", id)
	}
	// P47: fall back to file store for games without RCON ban support.
	if !supportsBanlist(s.Adapter) {
		return b.getFileBanList(id), nil
	}

	switch s.Adapter {
	case "ark-survival-ascended", "conan-exiles":
		// No "list bans" RCON command for ARK/Conan; return empty list.
		return []string{}, nil
	}

	resp, err := b.execRCON(ctx, id, "banlist players")
	if err != nil {
		return nil, err
	}
	return parseMinecraftBanlist(resp), nil
}

// BanPlayer bans a player from the server.
// For RCON-enabled games the command is sent to the server console.
// For all others (P47) the player is recorded in a persisted file-backed list.
func (b *Broker) BanPlayer(ctx context.Context, id, player, reason string) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}
	// P47: persist in file store for games without RCON ban support.
	if !supportsBanlist(s.Adapter) {
		b.addFileBan(id, player)
		return nil
	}

	safePlayer, err := sanitizePlayerName(s.Adapter, player)
	if err != nil {
		return fmt.Errorf("invalid player name: %w", err)
	}

	var cmd string
	switch s.Adapter {
	case "minecraft":
		if reason != "" {
			safeReason := strings.TrimSpace(rconDangerousChars.Replace(reason))
			cmd = fmt.Sprintf("ban %s %s", safePlayer, safeReason)
		} else {
			cmd = fmt.Sprintf("ban %s", safePlayer)
		}
	case "ark-survival-ascended", "conan-exiles":
		cmd = fmt.Sprintf("banplayer %s", safePlayer)
	}

	_, err = b.SendConsoleCommand(ctx, id, cmd)
	return err
}

// UnbanPlayer removes a player from the server's ban list.
// For RCON-less games (P47) the record is removed from the file-backed store.
func (b *Broker) UnbanPlayer(ctx context.Context, id, player string) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}
	// P47: remove from file store for games without RCON ban support.
	if !supportsBanlist(s.Adapter) {
		b.removeFileBan(id, player)
		return nil
	}

	safePlayer, err := sanitizePlayerName(s.Adapter, player)
	if err != nil {
		return fmt.Errorf("invalid player name: %w", err)
	}

	var cmd string
	switch s.Adapter {
	case "minecraft":
		cmd = fmt.Sprintf("pardon %s", safePlayer)
	case "ark-survival-ascended", "conan-exiles":
		cmd = fmt.Sprintf("unbanplayer %s", safePlayer)
	}

	_, err = b.SendConsoleCommand(ctx, id, cmd)
	return err
}

// ListWhitelistPlayers returns the whitelist for the server.
// Returns ErrBanlistNotSupported for games without whitelist support.
func (b *Broker) ListWhitelistPlayers(ctx context.Context, id string) ([]string, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server not found: %s", id)
	}
	if !supportsWhitelist(s.Adapter) {
		return nil, ErrBanlistNotSupported
	}

	resp, err := b.execRCON(ctx, id, "whitelist list")
	if err != nil {
		return nil, err
	}
	return parseMinecraftWhitelist(resp), nil
}

// WhitelistAdd adds a player to the whitelist.
func (b *Broker) WhitelistAdd(ctx context.Context, id, player string) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}
	if !supportsWhitelist(s.Adapter) {
		return ErrBanlistNotSupported
	}
	safePlayer, err := sanitizePlayerName(s.Adapter, player)
	if err != nil {
		return fmt.Errorf("invalid player name: %w", err)
	}
	_, err = b.SendConsoleCommand(ctx, id, fmt.Sprintf("whitelist add %s", safePlayer))
	return err
}

// WhitelistRemove removes a player from the whitelist.
func (b *Broker) WhitelistRemove(ctx context.Context, id, player string) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}
	if !supportsWhitelist(s.Adapter) {
		return ErrBanlistNotSupported
	}
	safePlayer, err := sanitizePlayerName(s.Adapter, player)
	if err != nil {
		return fmt.Errorf("invalid player name: %w", err)
	}
	_, err = b.SendConsoleCommand(ctx, id, fmt.Sprintf("whitelist remove %s", safePlayer))
	return err
}

// parseMinecraftBanlist parses "banlist players" RCON output.
// Format: "There are N bans:\nplayer1\nplayer2"  or  "There are no bans."
func parseMinecraftBanlist(resp string) []string {
	resp = strings.TrimSpace(resp)
	if strings.Contains(resp, "no bans") {
		return []string{}
	}
	lines := strings.Split(resp, "\n")
	var players []string
	for i, l := range lines {
		if i == 0 {
			continue // skip "There are N bans:" header
		}
		if p := strings.TrimSpace(l); p != "" {
			players = append(players, p)
		}
	}
	return players
}

// parseMinecraftWhitelist parses "whitelist list" RCON output.
// Format: "There are N whitelisted players: p1, p2, p3"  or  "There are no whitelisted players"
func parseMinecraftWhitelist(resp string) []string {
	resp = strings.TrimSpace(resp)
	if strings.Contains(resp, "no whitelisted players") {
		return []string{}
	}
	idx := strings.Index(resp, ": ")
	if idx == -1 {
		return []string{}
	}
	var players []string
	for _, p := range strings.Split(resp[idx+2:], ",") {
		if name := strings.TrimSpace(p); name != "" {
			players = append(players, name)
		}
	}
	return players
}
