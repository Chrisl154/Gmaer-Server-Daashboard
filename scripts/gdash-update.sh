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
if [[ $EUID -ne 0 ]]; then
  exec sudo bash "$0" "$@"
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

# Detect the install user from the owner of /opt/gdash (the systemd service
# runs as this user). We need their home directory to find Go and Node/NVM
# since those are installed per-user, not system-wide.
INSTALL_USER="$(stat -c '%U' "$INSTALL_DIR" 2>/dev/null || echo "")"
if [[ -n "$INSTALL_USER" && "$INSTALL_USER" != "root" ]]; then
  INSTALL_HOME="$(getent passwd "$INSTALL_USER" | cut -d: -f6)"
else
  INSTALL_HOME=""
fi

mkdir -p "$INSTALL_DIR/logs"
exec >> "$LOG" 2>&1
echo ""
echo "=== $(date '+%Y-%m-%d %H:%M:%S') === gdash self-update to branch: $BRANCH ==="
echo "PROGRESS:2"

# ── Find Go ──────────────────────────────────────────────────────────────────
GO_BIN=""
for candidate in \
    "/usr/local/go/bin/go" \
    "${INSTALL_HOME:+$INSTALL_HOME/.local/go/bin/go}" \
    "$HOME/.local/go/bin/go" \
    /home/*/.local/go/bin/go \
    "$(command -v go 2>/dev/null || true)"; do
  [[ -n "$candidate" && -x "$candidate" ]] && GO_BIN="$candidate" && break
done
if [[ -z "$GO_BIN" ]]; then
  echo "ERROR: go binary not found. Install Go 1.22+ and retry." >&2
  echo "Searched: /usr/local/go/bin/go, ${INSTALL_HOME:-?}/.local/go/bin/go, /home/*/.local/go/bin/go" >&2
  exit 1
fi
echo "Using Go: $GO_BIN ($($GO_BIN version))"

# ── Find Node / npm ──────────────────────────────────────────────────────────
# Try the install user's NVM first, then root's, then any user's.
for nvm_candidate in \
    "${INSTALL_HOME:+$INSTALL_HOME/.nvm/nvm.sh}" \
    "$HOME/.nvm/nvm.sh" \
    /home/*/.nvm/nvm.sh; do
  if [[ -n "$nvm_candidate" && -s "$nvm_candidate" ]]; then
    export NVM_DIR="$(dirname "$nvm_candidate")"
    source "$nvm_candidate"
    command -v node &>/dev/null && break
  fi
done
if ! command -v node &>/dev/null; then
  echo "ERROR: node not found. Install Node 20 LTS and retry." >&2
  echo "Searched NVM in: ${INSTALL_HOME:-?}/.nvm, $HOME/.nvm, /home/*/.nvm" >&2
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
# Mark the update as complete BEFORE restarting — the pull/build/UI rebuild
# are all done at this point.  If systemctl restart fails or times out,
# set -e would kill the script before writing these markers, making the UI
# think the update is still running forever.
echo "PROGRESS:95"
echo "=== Update complete ==="
echo "PROGRESS:100"
echo "Restarting gdash-daemon..."
sleep 2
systemctl restart gdash-daemon || echo "WARNING: systemctl restart returned non-zero (service may still be starting)"
