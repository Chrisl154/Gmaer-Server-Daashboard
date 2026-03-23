#!/bin/bash
# gdash-update.sh — called by the Games Dashboard self-update feature.
# Usage: gdash-update.sh [branch]   (default: main)
#
# Can also be run manually:
#   sudo bash /opt/gdash/bin/gdash-update.sh dev
#
# Or bootstrapped from GitHub:
#   curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/scripts/gdash-update.sh | sudo bash -s main
set -euo pipefail

# Ensure we're in a valid directory — the daemon or a re-install may have
# removed the cwd, causing getcwd() failures in every child process.
cd /tmp

BRANCH="${1:-main}"
if [[ "$BRANCH" != "main" && "$BRANCH" != "dev" ]]; then
  echo "ERROR: branch must be 'main' or 'dev'" >&2
  exit 1
fi

INSTALL_DIR="/opt/gdash"
REPO_DIR="$INSTALL_DIR/repo"
BIN_DIR="$INSTALL_DIR/bin"
UI_SRC="$REPO_DIR/ui"
UI_DST="$INSTALL_DIR/ui"
LOG="$INSTALL_DIR/logs/gdash-update.log"

mkdir -p "$INSTALL_DIR/logs"
exec >> "$LOG" 2>&1
echo ""
echo "=== $(date '+%Y-%m-%d %H:%M:%S') === gdash self-update to branch: $BRANCH ==="
echo "PROGRESS:2"

# ── Find Go ──────────────────────────────────────────────────────────────────
GO_BIN=""
for candidate in \
    "/usr/local/go/bin/go" \
    "$HOME/.local/go/bin/go" \
    "$(command -v go 2>/dev/null || true)"; do
  [[ -x "$candidate" ]] && GO_BIN="$candidate" && break
done
if [[ -z "$GO_BIN" ]]; then
  echo "ERROR: go binary not found. Install Go 1.22+ and retry." >&2
  exit 1
fi
echo "Using Go: $GO_BIN ($($GO_BIN version))"

# ── Find Node / npm ──────────────────────────────────────────────────────────
export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
[[ -s "$NVM_DIR/nvm.sh" ]] && source "$NVM_DIR/nvm.sh"
if ! command -v node &>/dev/null; then
  echo "ERROR: node not found. Install Node 20 LTS and retry." >&2
  exit 1
fi
echo "Using Node: $(node --version)"
echo "PROGRESS:10"

# ── Pull latest code ─────────────────────────────────────────────────────────
echo "Updating repository (branch: $BRANCH)..."
echo "PROGRESS:15"
# Fetch the specific branch — shallow clones (--depth=1) only have refs for
# the branch that was originally cloned. A bare `git fetch origin` won't
# create refs for other branches.
git -C "$REPO_DIR" fetch origin "$BRANCH" --quiet
git -C "$REPO_DIR" checkout "$BRANCH" 2>/dev/null \
  || git -C "$REPO_DIR" checkout -b "$BRANCH" "origin/$BRANCH" 2>/dev/null \
  || true
git -C "$REPO_DIR" reset --hard "origin/$BRANCH"
echo "Repository updated to: $(git -C "$REPO_DIR" rev-parse --short HEAD)"
echo "PROGRESS:30"

# ── Self-update: copy the latest version of this script from the repo ─────
if [[ -f "$REPO_DIR/scripts/gdash-update.sh" ]]; then
  cp "$REPO_DIR/scripts/gdash-update.sh" "$BIN_DIR/gdash-update.sh"
  chmod +x "$BIN_DIR/gdash-update.sh"
  echo "Update script refreshed from repo."
fi

# ── Rebuild daemon ───────────────────────────────────────────────────────────
echo "Building daemon..."
echo "PROGRESS:35"
(cd "${REPO_DIR}/daemon" && GONOSUMDB="*" GOFLAGS="-mod=mod" $GO_BIN mod download 2>&1 | grep -v "^$" || true)
GONOSUMDB="*" $GO_BIN build -o "${BIN_DIR}/games-daemon.new" "${REPO_DIR}/daemon/cmd/daemon"
mv "${BIN_DIR}/games-daemon.new" "${BIN_DIR}/games-daemon"
echo "Daemon binary updated."
echo "PROGRESS:60"

# ── Rebuild CLI ──────────────────────────────────────────────────────────────
echo "Building CLI..."
(cd "${REPO_DIR}/cli" && GONOSUMDB="*" GOFLAGS="-mod=mod" $GO_BIN mod download 2>&1 | grep -v "^$" || true)
GONOSUMDB="*" $GO_BIN build -o "${BIN_DIR}/gdash.new" "${REPO_DIR}/cli/cmd"
mv "${BIN_DIR}/gdash.new" "${BIN_DIR}/gdash"
echo "CLI binary updated."
echo "PROGRESS:70"

# ── Rebuild UI ───────────────────────────────────────────────────────────────
echo "Building UI..."
echo "PROGRESS:75"
cd "$UI_SRC"
npm install --silent 2>/dev/null
chmod +x node_modules/.bin/* 2>/dev/null || true
node_modules/.bin/vite build --outDir "${UI_DST}" --emptyOutDir 2>/dev/null
echo "UI rebuilt."
echo "PROGRESS:90"

# ── Restart service ──────────────────────────────────────────────────────────
echo "Restarting gdash-daemon..."
echo "PROGRESS:95"
sleep 2
sudo systemctl restart gdash-daemon
echo "=== Update complete ==="
echo "PROGRESS:100"
