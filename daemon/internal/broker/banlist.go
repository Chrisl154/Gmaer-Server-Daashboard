package broker

import (
	"context"
	"fmt"
	"strings"
	"time"

	rconpkg "github.com/games-dashboard/daemon/internal/rcon"
	telnetsvc "github.com/games-dashboard/daemon/internal/telnet"
	webrconpkg "github.com/games-dashboard/daemon/internal/webrcon"
)

// ErrBanlistNotSupported is returned when the game adapter does not support
// banlist or whitelist management via RCON.
var ErrBanlistNotSupported = fmt.Errorf("banlist management is not supported for this game type")

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
// Returns ErrBanlistNotSupported for games that do not support RCON ban management.
func (b *Broker) ListBannedPlayers(ctx context.Context, id string) ([]string, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server not found: %s", id)
	}
	if !supportsBanlist(s.Adapter) {
		return nil, ErrBanlistNotSupported
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
// The command is echoed to the console stream for audit purposes.
func (b *Broker) BanPlayer(ctx context.Context, id, player, reason string) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}
	if !supportsBanlist(s.Adapter) {
		return ErrBanlistNotSupported
	}

	var cmd string
	switch s.Adapter {
	case "minecraft":
		if reason != "" {
			cmd = fmt.Sprintf("ban %s %s", player, reason)
		} else {
			cmd = fmt.Sprintf("ban %s", player)
		}
	case "ark-survival-ascended", "conan-exiles":
		cmd = fmt.Sprintf("banplayer %s", player)
	}

	_, err := b.SendConsoleCommand(ctx, id, cmd)
	return err
}

// UnbanPlayer removes a player from the server's ban list.
func (b *Broker) UnbanPlayer(ctx context.Context, id, player string) error {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}
	if !supportsBanlist(s.Adapter) {
		return ErrBanlistNotSupported
	}

	var cmd string
	switch s.Adapter {
	case "minecraft":
		cmd = fmt.Sprintf("pardon %s", player)
	case "ark-survival-ascended", "conan-exiles":
		cmd = fmt.Sprintf("unbanplayer %s", player)
	}

	_, err := b.SendConsoleCommand(ctx, id, cmd)
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
	_, err := b.SendConsoleCommand(ctx, id, fmt.Sprintf("whitelist add %s", player))
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
	_, err := b.SendConsoleCommand(ctx, id, fmt.Sprintf("whitelist remove %s", player))
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
