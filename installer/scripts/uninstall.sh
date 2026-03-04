#!/usr/bin/env bash
# uninstall.sh — Remove Games Dashboard installation.
# Usage: uninstall.sh [--purge] [--install-dir /path]
set -euo pipefail

INSTALL_DIR="/opt/games-dashboard"
PURGE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --purge)       PURGE=true; shift ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

echo "Uninstalling Games Dashboard from $INSTALL_DIR ..."

# Stop and remove Docker containers if running
if command -v docker &>/dev/null; then
  if docker ps -q --filter name=games-dashboard-daemon | grep -q .; then
    echo "Stopping containers..."
    docker stop games-dashboard-daemon games-dashboard-ui 2>/dev/null || true
    docker rm   games-dashboard-daemon games-dashboard-ui 2>/dev/null || true
  fi
  if [[ -f "$INSTALL_DIR/docker-compose.yml" ]]; then
    docker compose -f "$INSTALL_DIR/docker-compose.yml" down 2>/dev/null || true
  fi
fi

# Remove systemd service if present
if systemctl is-active --quiet games-dashboard 2>/dev/null; then
  echo "Stopping systemd service..."
  systemctl stop games-dashboard
  systemctl disable games-dashboard
  rm -f /etc/systemd/system/games-dashboard.service
  systemctl daemon-reload
fi

# Remove binary
rm -f /usr/local/bin/gdash
rm -f /usr/local/bin/games-daemon

if [[ "$PURGE" == "true" ]]; then
  echo "Purging installation directory: $INSTALL_DIR"
  rm -rf "$INSTALL_DIR"
  rm -rf /etc/games-dashboard
  echo "Purge complete."
else
  echo "Installation files kept at $INSTALL_DIR (use --purge to remove data)."
fi

echo "Games Dashboard uninstalled."
