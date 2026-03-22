package broker

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	rconpkg "github.com/games-dashboard/daemon/internal/rcon"
	telnetsvc "github.com/games-dashboard/daemon/internal/telnet"
	webrconpkg "github.com/games-dashboard/daemon/internal/webrcon"
)

// playerCounts holds the result of a player count query.
type playerCounts struct {
	current int // current online players; -1 = not available
	max     int // server max; 0 = unknown
}

// queryPlayerCount tries to retrieve the current player count from a running
// game server via RCON, WebRCON, or Telnet. It uses a short 2-second timeout
// so it never blocks the metrics collection cycle for long.
// Returns current=-1 when the adapter doesn't support player count queries
// or when the query fails (e.g. server just started, RCON not ready).
func queryPlayerCount(adapterID, consoleType string, rconPort int, rconPassword, telnetPassword string) playerCounts {
	unknown := playerCounts{-1, 0}
	if rconPort == 0 {
		return unknown
	}
	addr := fmt.Sprintf("localhost:%d", rconPort)

	switch adapterID {
	case "minecraft":
		if rconPassword == "" {
			return unknown
		}
		resp, err := rconpkg.Exec(addr, rconPassword, "list", 2*time.Second)
		if err != nil {
			return unknown
		}
		return parseMinecraftList(resp)

	case "palworld":
		if rconPassword == "" {
			return unknown
		}
		resp, err := rconpkg.Exec(addr, rconPassword, "ShowPlayers", 2*time.Second)
		if err != nil {
			return unknown
		}
		return parsePalworldShowPlayers(resp)

	case "counter-strike-2", "team-fortress-2", "garrys-mod", "left-4-dead-2", "dota2":
		if rconPassword == "" {
			return unknown
		}
		resp, err := rconpkg.Exec(addr, rconPassword, "status", 2*time.Second)
		if err != nil {
			return unknown
		}
		return parseSourceStatus(resp)

	case "factorio":
		if rconPassword == "" {
			return unknown
		}
		resp, err := rconpkg.Exec(addr, rconPassword, "/players online", 2*time.Second)
		if err != nil {
			return unknown
		}
		return parseFactorioPlayers(resp)

	case "squad":
		if rconPassword == "" {
			return unknown
		}
		resp, err := rconpkg.Exec(addr, rconPassword, "AdminListPlayers", 2*time.Second)
		if err != nil {
			return unknown
		}
		return parseRCONLineCount(resp)

	case "ark-survival-ascended", "conan-exiles":
		if rconPassword == "" {
			return unknown
		}
		resp, err := rconpkg.Exec(addr, rconPassword, "ListPlayers", 2*time.Second)
		if err != nil {
			return unknown
		}
		return parseRCONLineCount(resp)

	case "rust":
		if consoleType != "webrcon" || rconPassword == "" {
			return unknown
		}
		resp, err := webrconpkg.Exec(addr, rconPassword, "serverinfo", 2*time.Second)
		if err != nil {
			return unknown
		}
		return parseRustServerInfo(resp)

	case "7-days-to-die":
		pass := telnetPassword
		if pass == "" {
			pass = rconPassword
		}
		if pass == "" {
			return unknown
		}
		resp, err := telnetsvc.Exec(addr, pass, "lp", 2*time.Second)
		if err != nil {
			return unknown
		}
		return parse7DTDListPlayers(resp)

	default:
		return unknown
	}
}

var minecraftListRe = regexp.MustCompile(`There are (\d+) of a max(?: of)? (\d+) players online`)

func parseMinecraftList(resp string) playerCounts {
	m := minecraftListRe.FindStringSubmatch(resp)
	if len(m) < 3 {
		return playerCounts{-1, 0}
	}
	current, _ := strconv.Atoi(m[1])
	max, _ := strconv.Atoi(m[2])
	return playerCounts{current, max}
}

func parsePalworldShowPlayers(resp string) playerCounts {
	// Response format: header line ("name,playeruid,steamid") then one CSV line per player.
	lines := strings.Split(strings.TrimSpace(resp), "\n")
	count := 0
	for i, l := range lines {
		if i == 0 {
			continue // skip header
		}
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	return playerCounts{count, 0}
}

// sourcePlayersRe matches the "players" line in Source engine `status` output.
// Example: "players   : 5 humans, 0 bots (24 max)"
var sourcePlayersRe = regexp.MustCompile(`players\s*:\s*(\d+)\s+humans.*\((\d+)\s+max\)`)

func parseSourceStatus(resp string) playerCounts {
	m := sourcePlayersRe.FindStringSubmatch(resp)
	if len(m) < 3 {
		return playerCounts{-1, 0}
	}
	current, _ := strconv.Atoi(m[1])
	max, _ := strconv.Atoi(m[2])
	return playerCounts{current, max}
}

// factorioPlayersRe matches "Online players (N):" in Factorio `/players online` output.
var factorioPlayersRe = regexp.MustCompile(`\((\d+)\)`)

func parseFactorioPlayers(resp string) playerCounts {
	m := factorioPlayersRe.FindStringSubmatch(resp)
	if len(m) < 2 {
		return playerCounts{-1, 0}
	}
	count, _ := strconv.Atoi(m[1])
	return playerCounts{count, 0}
}

// parseRCONLineCount counts non-empty lines as players (used for Squad, ARK, Conan).
func parseRCONLineCount(resp string) playerCounts {
	resp = strings.TrimSpace(resp)
	if resp == "" {
		return playerCounts{0, 0}
	}
	lines := strings.Split(resp, "\n")
	count := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	return playerCounts{count, 0}
}

func parseRustServerInfo(resp string) playerCounts {
	// Rust `serverinfo` WebRCON response is a JSON object with Players and MaxPlayers.
	var info struct {
		Players    int `json:"Players"`
		MaxPlayers int `json:"MaxPlayers"`
	}
	if err := json.Unmarshal([]byte(resp), &info); err != nil {
		return playerCounts{-1, 0}
	}
	return playerCounts{info.Players, info.MaxPlayers}
}

func parse7DTDListPlayers(resp string) playerCounts {
	// `lp` output: lines starting with digits are player entries;
	// skip header/footer lines ("Total of N in the game", "---", etc.)
	lines := strings.Split(resp, "\n")
	count := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "Total of") || strings.HasPrefix(l, "---") {
			continue
		}
		// Player lines start with a digit (their index)
		if len(l) > 0 && l[0] >= '0' && l[0] <= '9' {
			count++
		}
	}
	return playerCounts{count, 0}
}
