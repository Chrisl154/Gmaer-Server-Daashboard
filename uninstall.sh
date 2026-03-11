#!/usr/bin/env bash
# =============================================================================
# Games Dashboard — Uninstaller
# Completely removes everything installed by install.sh.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/uninstall.sh | bash
#
#   Or locally:
#   bash uninstall.sh
# =============================================================================
set -euo pipefail

INSTALL_DIR="/opt/gdash"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

log()     { echo -e "${CYAN}[$(date '+%H:%M:%S')]${NC} $*"; }
ok()      { echo -e "  ${GREEN}✓${NC} $*"; }
info()    { echo -e "  ${YELLOW}ℹ${NC}  $*"; }
section() { echo -e "\n${BOLD}══ $* ══${NC}"; }

SUDO=""
if [[ $EUID -eq 0 ]]; then
  SUDO=""
elif command -v sudo &>/dev/null; then
  SUDO="sudo"
else
  echo -e "${RED}ERROR: root or sudo required.${NC}"; exit 1
fi

echo ""
echo -e "${RED}${BOLD}  This will completely remove the Games Dashboard.${NC}"
echo -e "  The following will be deleted:"
echo -e "    • Systemd service:    gdash-daemon"
echo -e "    • nginx site:         /etc/nginx/sites-*/gdash"
echo -e "    • Install directory:  $INSTALL_DIR"
echo -e "    • CLI symlink:        /usr/local/bin/gdash"
echo ""

if [[ -t 0 ]]; then
  read -r -p "  Continue? [y/N] " CONFIRM
  [[ "$CONFIRM" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 0; }
else
  info "Non-interactive mode — proceeding automatically."
fi

echo ""

# =============================================================================
section "Stopping & Removing Systemd Service"
# =============================================================================

if systemctl list-units --full -all 2>/dev/null | grep -q "gdash-daemon.service"; then
  log "Stopping gdash-daemon..."
  $SUDO systemctl stop gdash-daemon 2>/dev/null || true
  $SUDO systemctl disable gdash-daemon 2>/dev/null || true
  ok "Service stopped and disabled"
else
  info "gdash-daemon service not found — skipping"
fi

if [[ -f /etc/systemd/system/gdash-daemon.service ]]; then
  $SUDO rm -f /etc/systemd/system/gdash-daemon.service
  $SUDO systemctl daemon-reload
  ok "Removed /etc/systemd/system/gdash-daemon.service"
fi

# =============================================================================
section "Removing nginx Configuration"
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
  # Restore default nginx site if it was removed
  if [[ ! -f /etc/nginx/sites-enabled/default ]] && \
     [[ -f /etc/nginx/sites-available/default ]]; then
    $SUDO ln -sf /etc/nginx/sites-available/default \
      /etc/nginx/sites-enabled/default
    info "Restored nginx default site"
  fi

  # Reload nginx if it's running
  if systemctl is-active --quiet nginx 2>/dev/null; then
    if $SUDO nginx -t 2>/dev/null; then
      $SUDO systemctl reload nginx
      ok "nginx reloaded"
    else
      $SUDO systemctl stop nginx
      info "nginx stopped (config invalid without gdash site)"
    fi
  fi
else
  info "No nginx gdash config found — skipping"
fi

# =============================================================================
section "Removing Install Directory"
# =============================================================================

if [[ -d "$INSTALL_DIR" ]]; then
  log "Removing $INSTALL_DIR ..."
  $SUDO rm -rf "$INSTALL_DIR"
  ok "Removed $INSTALL_DIR"
else
  info "$INSTALL_DIR not found — already clean"
fi

# =============================================================================
section "Removing CLI Symlink"
# =============================================================================

if [[ -L /usr/local/bin/gdash ]]; then
  $SUDO rm -f /usr/local/bin/gdash
  ok "Removed /usr/local/bin/gdash"
else
  info "/usr/local/bin/gdash not found — skipping"
fi

# =============================================================================
section "Done"
# =============================================================================

echo ""
echo -e "${GREEN}${BOLD}  Games Dashboard fully uninstalled.${NC}"
echo ""
echo -e "  To reinstall, run:"
echo -e "  ${BOLD}curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/install.sh | bash${NC}"
echo ""
