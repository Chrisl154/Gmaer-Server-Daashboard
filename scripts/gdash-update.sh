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

# Auto-escalate to root if not already — the update needs write access to
# /opt/gdash and permission to restart the systemd service.
# ORIG_HOME preserves the invoking user's home so we can find Go/Node.
if [[ $EUID -ne 0 ]]; then
  exec sudo ORIG_HOME="$HOME" bash "$0" "$@"
fi
# At this point ORIG_HOME is either passed from the sudo line above,
# or empty if we were already root. Fall back to the systemd service user.
if [[ -z "${ORIG_HOME:-}" ]]; then
  ORIG_HOME="$(getent passwd "$(stat -c '%U' /opt/gdash 2>/dev/null || echo root)" | cut -d: -f6)"
fi

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
    "${ORIG_HOME:+$ORIG_HOME/.local/go/bin/go}" \
    "/home/"*"/.local/go/bin/go" \
    "$(command -v go 2>/dev/null || true)"; do
  [[ -n "$candidate" && -x "$candidate" ]] && GO_BIN="$candidate" && break
done
if [[ -z "$GO_BIN" ]]; then
  echo "ERROR: go binary not found. Install Go 1.22+ and retry." >&2
  exit 1
fi
echo "Using Go: $GO_BIN ($($GO_BIN version))"

# ── Find Node / npm ──────────────────────────────────────────────────────────
# Check both root's and the original user's NVM install.
export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
[[ -s "$NVM_DIR/nvm.sh" ]] && source "$NVM_DIR/nvm.sh"
if ! command -v node &>/dev/null && [[ -n "$ORIG_HOME" && -s "$ORIG_HOME/.nvm/nvm.sh" ]]; then
  export NVM_DIR="$ORIG_HOME/.nvm"
  source "$NVM_DIR/nvm.sh"
fi
if ! command -v node &>/dev/null; then
  echo "ERROR: node not found. Install Node 20 LTS and retry." >&2
  exit 1
fi
echo "Using Node: $(node --version)"
echo "PROGRESS:10"

# ── Pull latest code ─────────────────────────────────────────────────────────
echo "Updating repository (branch: $BRANCH)..."
echo "PROGRESS:15"
# Fetch the specific branch with an explicit refspec so that origin/$BRANCH is
# always created — shallow clones only have refspecs for the originally-cloned
# branch, so `git fetch origin dev` alone writes to FETCH_HEAD but never
# creates refs/remotes/origin/dev.
git -C "$REPO_DIR" fetch origin "+refs/heads/${BRANCH}:refs/remotes/origin/${BRANCH}" --quiet
git -C "$REPO_DIR" checkout "$BRANCH" 2>/dev/null \
  || git -C "$REPO_DIR" checkout -b "$BRANCH" "origin/$BRANCH" 2>/dev/null \
  || true
git -C "$REPO_DIR" reset --hard "origin/$BRANCH"
echo "Repository updated to: $(git -C "$REPO_DIR" rev-parse --short HEAD)"
echo "PROGRESS:30"

# ── Self-update: copy the latest version of this script from the repo ─────
if [[ -f "$REPO_DIR/scripts/gdash-update.sh" ]]; then
  # Write to a temp file then atomic-mv so we don't corrupt the running
  # script's file descriptor (cp truncates the inode bash is reading from).
  cp "$REPO_DIR/scripts/gdash-update.sh" "$BIN_DIR/gdash-update.sh.new"
  chmod +x "$BIN_DIR/gdash-update.sh.new"
  mv "$BIN_DIR/gdash-update.sh.new" "$BIN_DIR/gdash-update.sh"
  echo "Update script refreshed from repo."
fi

# ── Rebuild daemon ───────────────────────────────────────────────────────────
echo "Building daemon..."
echo "PROGRESS:35"
(cd "${REPO_DIR}/daemon" \
  && GONOSUMDB="*" GOFLAGS="-mod=mod" $GO_BIN mod download 2>&1 | grep -v "^$" || true)
(cd "${REPO_DIR}/daemon" \
  && GONOSUMDB="*" $GO_BIN build -o "${BIN_DIR}/games-daemon.new" ./cmd/daemon)
mv "${BIN_DIR}/games-daemon.new" "${BIN_DIR}/games-daemon"
echo "Daemon binary updated."
echo "PROGRESS:60"

# ── Rebuild CLI ──────────────────────────────────────────────────────────────
echo "Building CLI..."
(cd "${REPO_DIR}/cli" \
  && GONOSUMDB="*" GOFLAGS="-mod=mod" $GO_BIN mod download 2>&1 | grep -v "^$" || true)
(cd "${REPO_DIR}/cli" \
  && GONOSUMDB="*" $GO_BIN build -o "${BIN_DIR}/gdash.new" ./cmd)
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
systemctl restart gdash-daemon
echo "=== Update complete ==="
echo "PROGRESS:100"
