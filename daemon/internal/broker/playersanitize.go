package broker

import (
	"fmt"
	"regexp"
	"strings"
)

// rconDangerousChars are characters that can split or terminate an RCON command
// on virtually every game engine, regardless of the protocol dialect.
// Semicolons split commands in Source-engine RCON; newlines and carriage returns
// create second commands in all implementations; null bytes terminate the string.
var rconDangerousChars = strings.NewReplacer(
	"\n", "",
	"\r", "",
	"\x00", "",
	";", "",
)

// minecraftNameRe covers Mojang's official username rules:
// 3–16 characters, alphanumeric and underscore only.
var minecraftNameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{3,16}$`)

// sanitizePlayerName returns a safe version of a player name for use in an
// RCON command, or an error if the name is fundamentally invalid.
//
// Minecraft (Mojang auth): strict allowlist — only [a-zA-Z0-9_], 3–16 chars.
// All other games (Steam names): strip the four dangerous protocol characters
// and reject if the result is empty.
func sanitizePlayerName(adapter, name string) (string, error) {
	if adapter == "minecraft" {
		if !minecraftNameRe.MatchString(name) {
			return "", fmt.Errorf("invalid Minecraft username %q: must be 3–16 characters, alphanumeric and underscore only", name)
		}
		return name, nil
	}

	// Steam / freeform name — strip dangerous RCON protocol characters only.
	clean := rconDangerousChars.Replace(name)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "", fmt.Errorf("player name is empty after sanitization")
	}
	return clean, nil
}
