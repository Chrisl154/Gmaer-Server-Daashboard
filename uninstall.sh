#!/usr/bin/env bash
# =============================================================================
# Games Dashboard — Uninstaller
#
# What this does:
#   1. Stops all running game servers managed by the daemon
#   2. Moves backups to ~/gdash-backups-<timestamp>/ before deleting anything
#   3. Stops & removes the gdash-daemon systemd service
#   4. Removes the nginx gdash site config
#   5. Removes /opt/gdash/ (binaries, data, config, TLS)
#   6. Removes the /usr/local/bin/gdash CLI symlink
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/uninstall.sh | bash
#   bash uninstall.sh
#   GDASH_FORCE=1 bash uninstall.sh   # skip confirmation (non-interactive)
# =============================================================================
set -euo pipefail

INSTALL_DIR="/opt/gdash"
DAEMON_PORT="8443"
BACKUP_DEST="$HOME/gdash-backups-$(date '+%Y%m%d-%H%M%S')"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'

log()     { echo -e "${CYAN}[$(date '+%H:%M:%S')]${NC} $*"; }
ok()      { echo -e "  ${GREEN}✓${NC} $*"; }
warn()    { echo -e "  ${YELLOW}⚠${NC}  $*"; }
info()    { echo -e "  ${DIM}ℹ${NC}  $*"; }
section() { echo -e "\n${BOLD}══ $* ══${NC}"; }

SUDO=""
if [[ $EUID -eq 0 ]]; then
  SUDO=""
elif command -v sudo &>/dev/null; then
  SUDO="sudo"
else
  echo -e "${RED}ERROR: root or sudo required.${NC}"; exit 1
fi

# =============================================================================
# Confirmation
# =============================================================================

echo ""
echo -e "${RED}${BOLD}  ╔══════════════════════════════════════════════════════╗"
echo -e "  ║         Games Dashboard — Uninstaller               ║"
echo -e "  ╚══════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${BOLD}This will:${NC}"
echo -e "    ${RED}•${NC} Stop all running game servers"
echo -e "    ${YELLOW}•${NC} Move your backups to ${BOLD}~/gdash-backups-*/${NC} before deleting"
echo -e "    ${RED}•${NC} Stop & remove the gdash-daemon systemd service"
echo -e "    ${RED}•${NC} Remove the nginx gdash site configuration"
echo -e "    ${RED}•${NC} Remove ${BOLD}$INSTALL_DIR${NC} (binaries, data, config, TLS)"
echo -e "    ${RED}•${NC} Remove ${BOLD}/usr/local/bin/gdash${NC}"
echo ""
echo -e "  ${YELLOW}Your backups will be preserved at:${NC}"
echo -e "    ${BOLD}$BACKUP_DEST${NC}"
echo ""

if [[ -t 0 && -z "${GDASH_FORCE:-}" ]]; then
  read -r -p "  Type YES to confirm uninstall: " CONFIRM
  if [[ "$CONFIRM" != "YES" ]]; then
    echo ""
    echo -e "  Aborted — nothing was changed."
    exit 0
  fi
else
  warn "Non-interactive / forced mode — proceeding automatically."
fi

echo ""

# =============================================================================
section "Step 1: Stop Running Game Servers"
# =============================================================================

# Helper: call the daemon API locally
daemon_api() {
  local method="$1" path="$2" data="${3:-}"
  if command -v curl &>/dev/null; then
    if [[ -n "$data" ]]; then
      curl -sk -X "$method" -H "Content-Type: application/json" -d "$data" \
        "https://127.0.0.1:${DAEMON_PORT}${path}" 2>/dev/null || true
    else
      curl -sk -X "$method" "https://127.0.0.1:${DAEMON_PORT}${path}" 2>/dev/null || true
    fi
  fi
}

# Get auth token if daemon is running
DAEMON_TOKEN=""
if systemctl is-active --quiet gdash-daemon 2>/dev/null; then
  log "Daemon is running — fetching server list..."

  # Try to read JWT secret from config to get a token
  CFG="$INSTALL_DIR/config/daemon.yaml"
  if [[ -f "$CFG" ]]; then
    # Bootstrap won't work if already set up — try a direct login with stored info
    # We'll use the daemon's admin API via its socket if accessible
    HEALTH=$(daemon_api GET /healthz)
    if echo "$HEALTH" | grep -q "healthy"; then
      info "Daemon API reachable — will attempt graceful server shutdown"

      # Get list of running servers and stop each one
      # (We can't authenticate without the password, so we send SIGTERM to game processes)
      GAME_DIRS=$(find "$INSTALL_DIR/data" -maxdepth 2 -name "*.pid" 2>/dev/null || true)
      if [[ -n "$GAME_DIRS" ]]; then
        while IFS= read -r pidfile; do
          PID=$(cat "$pidfile" 2>/dev/null || true)
          SERVER_NAME=$(basename "$(dirname "$pidfile")")
          if [[ -n "$PID" ]] && kill -0 "$PID" 2>/dev/null; then
            log "Stopping game server: $SERVER_NAME (PID $PID)..."
            kill "$PID" 2>/dev/null || true
            sleep 2
            # Force kill if still running
            kill -0 "$PID" 2>/dev/null && kill -9 "$PID" 2>/dev/null || true
            ok "Stopped game server: $SERVER_NAME"
          fi
        done <<< "$GAME_DIRS"
      else
        info "No game server PID files found"
      fi

      # Kill any processes running from the game install dirs
      GAME_DATA="$INSTALL_DIR/data"
      if [[ -d "$GAME_DATA" ]]; then
        # Find and kill any child processes spawned from the data directory
        GAME_PIDS=$(lsof +D "$GAME_DATA" 2>/dev/null | awk 'NR>1 {print $2}' | sort -u || true)
        if [[ -n "$GAME_PIDS" ]]; then
          log "Stopping processes using game data directory..."
          while IFS= read -r pid; do
            PNAME=$(ps -p "$pid" -o comm= 2>/dev/null || echo "unknown")
            # Don't kill the daemon itself — only child game processes
            if [[ "$pid" != "$(pgrep -f games-daemon 2>/dev/null || true)" ]]; then
              kill "$pid" 2>/dev/null || true
              ok "Killed process: $PNAME (PID $pid)"
            fi
          done <<< "$GAME_PIDS"
        else
          info "No active game processes found in data directory"
        fi
      fi
    fi
  fi
else
  info "Daemon is not running — no game servers to stop"
fi

ok "Game server cleanup complete"

# =============================================================================
section "Step 2: Preserve Backups"
# =============================================================================

BACKUP_SRC="$INSTALL_DIR/data/backups"
BACKUP_COUNT=0

if [[ -d "$BACKUP_SRC" ]]; then
  BACKUP_COUNT=$(find "$BACKUP_SRC" -type f 2>/dev/null | wc -l || echo 0)
fi

if [[ "$BACKUP_COUNT" -gt 0 ]]; then
  log "Found $BACKUP_COUNT backup file(s) — moving to $BACKUP_DEST ..."
  mkdir -p "$BACKUP_DEST"
  cp -r "$BACKUP_SRC/." "$BACKUP_DEST/" 2>/dev/null || \
    $SUDO cp -r "$BACKUP_SRC/." "$BACKUP_DEST/" 2>/dev/null || true

  # Verify the copy succeeded
  SAVED=$(find "$BACKUP_DEST" -type f 2>/dev/null | wc -l || echo 0)
  if [[ "$SAVED" -gt 0 ]]; then
    ok "Saved $SAVED backup file(s) to $BACKUP_DEST"
  else
    warn "Backup copy may have failed — check $BACKUP_DEST manually"
  fi
else
  info "No backup files found in $BACKUP_SRC — nothing to preserve"
fi

# Also preserve any game server data that isn't backups (worlds, saves, etc.)
GAME_SERVERS_DIR="$INSTALL_DIR/data/servers"
if [[ -d "$GAME_SERVERS_DIR" ]]; then
  SERVER_COUNT=$(find "$GAME_SERVERS_DIR" -maxdepth 1 -mindepth 1 -type d 2>/dev/null | wc -l || echo 0)
  if [[ "$SERVER_COUNT" -gt 0 ]]; then
    log "Found $SERVER_COUNT game server data director(ies) — preserving world/save files..."
    WORLDS_DEST="$BACKUP_DEST/game-data"
    mkdir -p "$WORLDS_DEST"
    cp -r "$GAME_SERVERS_DIR/." "$WORLDS_DEST/" 2>/dev/null || \
      $SUDO cp -r "$GAME_SERVERS_DIR/." "$WORLDS_DEST/" 2>/dev/null || true
    ok "Game server data preserved to $WORLDS_DEST"
  fi
fi

# =============================================================================
section "Step 3: Stop & Remove gdash-daemon Service"
# =============================================================================

if systemctl list-units --full -all 2>/dev/null | grep -q "gdash-daemon.service"; then
  log "Stopping gdash-daemon..."
  $SUDO systemctl stop gdash-daemon 2>/dev/null || true
  $SUDO systemctl disable gdash-daemon 2>/dev/null || true
  ok "Service stopped and disabled"
else
  info "gdash-daemon service not found"
fi

if [[ -f /etc/systemd/system/gdash-daemon.service ]]; then
  $SUDO rm -f /etc/systemd/system/gdash-daemon.service
  $SUDO systemctl daemon-reload
  ok "Removed /etc/systemd/system/gdash-daemon.service"
fi

# Kill any lingering daemon processes
DAEMON_PIDS=$(pgrep -f "games-daemon" 2>/dev/null || true)
if [[ -n "$DAEMON_PIDS" ]]; then
  log "Killing lingering daemon processes..."
  echo "$DAEMON_PIDS" | xargs kill 2>/dev/null || true
  sleep 1
  echo "$DAEMON_PIDS" | xargs kill -9 2>/dev/null || true
  ok "Daemon processes terminated"
fi

# Free the port
if command -v lsof &>/dev/null; then
  PORT_PIDS=$(lsof -ti:"$DAEMON_PORT" 2>/dev/null || true)
  if [[ -n "$PORT_PIDS" ]]; then
    echo "$PORT_PIDS" | xargs kill -9 2>/dev/null || true
    ok "Freed port $DAEMON_PORT"
  fi
fi

# =============================================================================
section "Step 4: Remove nginx Configuration"
# =============================================================================

NGINX_REMOVED=false
for f in /etc/nginx/sites-enabled/gdash /etc/nginx/sites-available/gdash; do
  if [[ -f "$f" ]]; then
    $SUDO rm -f "$f"
    ok "Removed $f"
    NGINX_REMOVED=true
  fi
done

if [[ "$NGINX_REMOVED" == "true" ]]; then
  if [[ ! -f /etc/nginx/sites-enabled/default ]] && \
     [[ -f /etc/nginx/sites-available/default ]]; then
    $SUDO ln -sf /etc/nginx/sites-available/default /etc/nginx/sites-enabled/default
    info "Restored nginx default site"
  fi
  if systemctl is-active --quiet nginx 2>/dev/null; then
    if $SUDO nginx -t 2>/dev/null; then
      $SUDO systemctl reload nginx
      ok "nginx reloaded"
    else
      $SUDO systemctl stop nginx
      info "nginx stopped (no valid sites remaining)"
    fi
  fi
else
  info "No nginx gdash config found"
fi

# =============================================================================
section "Step 5: Remove Install Directory"
# =============================================================================

if [[ -d "$INSTALL_DIR" ]]; then
  log "Removing $INSTALL_DIR ..."
  $SUDO rm -rf "$INSTALL_DIR"
  ok "Removed $INSTALL_DIR"
else
  info "$INSTALL_DIR not found — already clean"
fi

# =============================================================================
section "Step 6: Remove CLI Symlink"
# =============================================================================

if [[ -L /usr/local/bin/gdash ]]; then
  $SUDO rm -f /usr/local/bin/gdash
  ok "Removed /usr/local/bin/gdash"
else
  info "/usr/local/bin/gdash not found"
fi

# =============================================================================
section "Done"
# =============================================================================

echo ""
echo -e "${GREEN}${BOLD}  ╔══════════════════════════════════════════════════════╗"
echo -e "  ║       Games Dashboard fully uninstalled.            ║"
echo -e "  ╚══════════════════════════════════════════════════════╝${NC}"
echo ""

if [[ -d "$BACKUP_DEST" ]]; then
  TOTAL_SAVED=$(find "$BACKUP_DEST" -type f 2>/dev/null | wc -l || echo 0)
  echo -e "  ${YELLOW}${BOLD}Your data was preserved:${NC}"
  echo -e "    ${BOLD}$BACKUP_DEST${NC}  ($TOTAL_SAVED files)"
  echo -e "    Includes: backups + game world/save files"
  echo ""
fi

echo -e "  To reinstall fresh:"
echo -e "  ${BOLD}curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/install.sh | bash${NC}"
echo ""
