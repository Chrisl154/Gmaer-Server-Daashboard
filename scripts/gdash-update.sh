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
DATA_DIR="$INSTALL_DIR/data"
LOG="$INSTALL_DIR/logs/gdash-update.log"
STATE_FILE="$DATA_DIR/update-state.json"
VERSION_FILE="$INSTALL_DIR/VERSION"

# Detect the install user from the owner of /opt/gdash (the systemd service
# runs as this user). We need their home directory to find Go and Node/NVM
# since those are installed per-user, not system-wide.
INSTALL_USER="$(stat -c '%U' "$INSTALL_DIR" 2>/dev/null || echo "")"
if [[ -n "$INSTALL_USER" && "$INSTALL_USER" != "root" ]]; then
  INSTALL_HOME="$(getent passwd "$INSTALL_USER" | cut -d: -f6)"
else
  INSTALL_HOME=""
fi

mkdir -p "$INSTALL_DIR/logs" "$DATA_DIR"

# ── State file helper ────────────────────────────────────────────────────────
# Writes a JSON state file that the daemon API can read instantly (no git, no
# log parsing).  The UI polls this instead of parsing the update log.
write_state() {
  local status="$1" phase="$2" progress="$3" error="${4:-}"
  local now
  now="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  cat > "${STATE_FILE}.tmp" <<JSONEOF
{"status":"${status}","phase":"${phase}","progress":${progress},"branch":"${BRANCH}","error":"${error}","updated_at":"${now}"}
JSONEOF
  mv "${STATE_FILE}.tmp" "$STATE_FILE"
}

# Mark as running immediately so the UI picks it up even if the log
# hasn't been written yet.
write_state "running" "starting" 2

# On any failure, mark the state file as failed before exiting.
trap 'write_state "failed" "unknown" "$progress" "script exited unexpectedly"' ERR

exec >> "$LOG" 2>&1
echo ""
echo "=== $(date '+%Y-%m-%d %H:%M:%S') === gdash self-update to branch: $BRANCH ==="

progress=2
echo "PROGRESS:$progress"

# ── Find Go ──────────────────────────────────────────────────────────────────
write_state "running" "finding_go" "$progress"
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
  write_state "failed" "finding_go" "$progress" "go binary not found"
  echo "ERROR: go binary not found. Install Go 1.22+ and retry." >&2
  echo "Searched: /usr/local/go/bin/go, ${INSTALL_HOME:-?}/.local/go/bin/go, /home/*/.local/go/bin/go" >&2
  exit 1
fi
echo "Using Go: $GO_BIN ($($GO_BIN version))"

# ── Find Node / npm ──────────────────────────────────────────────────────────
write_state "running" "finding_node" 8
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
  write_state "failed" "finding_node" 8 "node not found"
  echo "ERROR: node not found. Install Node 20 LTS and retry." >&2
  echo "Searched NVM in: ${INSTALL_HOME:-?}/.nvm, $HOME/.nvm, /home/*/.nvm" >&2
  exit 1
fi
echo "Using Node: $(node --version)"
progress=10
echo "PROGRESS:$progress"

# ── Pull latest code ─────────────────────────────────────────────────────────
write_state "running" "pulling_code" 15
echo "Updating repository (branch: $BRANCH)..."
progress=15
echo "PROGRESS:$progress"
git -C "$REPO_DIR" fetch origin "+refs/heads/${BRANCH}:refs/remotes/origin/${BRANCH}" --quiet
git -C "$REPO_DIR" checkout "$BRANCH" 2>/dev/null \
  || git -C "$REPO_DIR" checkout -b "$BRANCH" "origin/$BRANCH" 2>/dev/null \
  || true
git -C "$REPO_DIR" reset --hard "origin/$BRANCH"
echo "Repository updated to: $(git -C "$REPO_DIR" rev-parse --short HEAD)"
progress=30
echo "PROGRESS:$progress"

# ── Self-update: copy the latest version of this script from the repo ─────
if [[ -f "$REPO_DIR/scripts/gdash-update.sh" ]]; then
  cp "$REPO_DIR/scripts/gdash-update.sh" "$BIN_DIR/gdash-update.sh.new"
  chmod +x "$BIN_DIR/gdash-update.sh.new"
  mv "$BIN_DIR/gdash-update.sh.new" "$BIN_DIR/gdash-update.sh"
  echo "Update script refreshed from repo."
fi

# ── Copy VERSION file from repo ───────────────────────────────────────────
if [[ -f "$REPO_DIR/VERSION" ]]; then
  cp "$REPO_DIR/VERSION" "$VERSION_FILE"
fi

# ── Rebuild daemon ───────────────────────────────────────────────────────────
write_state "running" "building_daemon" 35
echo "Building daemon..."
progress=35
echo "PROGRESS:$progress"
(cd "${REPO_DIR}/daemon" \
  && GONOSUMDB="*" GOFLAGS="-mod=mod" $GO_BIN mod download 2>&1 | grep -v "^$" || true)
(cd "${REPO_DIR}/daemon" \
  && GONOSUMDB="*" $GO_BIN build -o "${BIN_DIR}/games-daemon.new" ./cmd/daemon)
mv "${BIN_DIR}/games-daemon.new" "${BIN_DIR}/games-daemon"
echo "Daemon binary updated."
progress=60
echo "PROGRESS:$progress"

# ── Rebuild CLI ──────────────────────────────────────────────────────────────
write_state "running" "building_cli" 65
echo "Building CLI..."
(cd "${REPO_DIR}/cli" \
  && GONOSUMDB="*" GOFLAGS="-mod=mod" $GO_BIN mod download 2>&1 | grep -v "^$" || true)
(cd "${REPO_DIR}/cli" \
  && GONOSUMDB="*" $GO_BIN build -o "${BIN_DIR}/gdash.new" ./cmd)
mv "${BIN_DIR}/gdash.new" "${BIN_DIR}/gdash"
echo "CLI binary updated."
progress=70
echo "PROGRESS:$progress"

# ── Rebuild UI ───────────────────────────────────────────────────────────────
write_state "running" "building_ui" 75
echo "Building UI..."
progress=75
echo "PROGRESS:$progress"
cd "$UI_SRC"
npm install --silent 2>/dev/null
chmod +x node_modules/.bin/* 2>/dev/null || true
node_modules/.bin/vite build --outDir "${UI_DST}" --emptyOutDir 2>/dev/null
echo "UI rebuilt."
progress=90
echo "PROGRESS:$progress"

# ── Bump version ─────────────────────────────────────────────────────────────
# Read the current version from VERSION file and increment the patch number.
CURRENT_VERSION="1.0.0"
if [[ -f "$VERSION_FILE" ]]; then
  CURRENT_VERSION="$(cat "$VERSION_FILE" | tr -d '[:space:]')"
fi
# Parse major.minor.patch — increment patch by 1.
IFS='.' read -r V_MAJOR V_MINOR V_PATCH <<< "$CURRENT_VERSION"
V_PATCH=$(( ${V_PATCH:-0} + 1 ))
NEW_VERSION="${V_MAJOR:-1}.${V_MINOR:-0}.${V_PATCH}"
echo "$NEW_VERSION" > "$VERSION_FILE"
echo "Version bumped: $CURRENT_VERSION → $NEW_VERSION"

# ── Mark complete and restart ────────────────────────────────────────────────
# Write completion markers BEFORE restarting — the build work is done.
# If systemctl restart fails, the update itself was still successful.
progress=100
echo "PROGRESS:95"
echo "=== Update complete ==="
echo "PROGRESS:100"

write_state "complete" "done" 100

echo "Restarting gdash-daemon..."
sleep 2
systemctl restart gdash-daemon || echo "WARNING: systemctl restart returned non-zero (service may still be starting)"
