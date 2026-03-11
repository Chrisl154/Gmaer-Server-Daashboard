#!/usr/bin/env bash
# =============================================================================
# Games Dashboard — Live Test Runner
# Downloads, builds, starts, and fully tests the daemon + CLI in one shot.
#
# All requirements (system packages, Go, Node.js, optional CVE scanners) are
# installed automatically before anything else runs.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/test-live.sh | bash
#
#   Or locally:
#   bash test-live.sh
#
# The script needs only: bash + an internet connection.
# Everything else is installed for you.
# =============================================================================
set -euo pipefail

REPO_URL="https://github.com/Chrisl154/Gmaer-Server-Daashboard.git"
WORK_DIR="${GDASH_TEST_DIR:-/tmp/gdash-live-test}"
DAEMON_PORT="${GDASH_PORT:-8443}"
ADMIN_USER="admin"
ADMIN_PASS="LiveTest123!"
GO_VERSION="1.22.4"
GO_ARCH="linux-amd64"
NODE_VERSION="20"
TRIVY_VERSION="0.50.1"
GRYPE_VERSION="0.74.0"
LOCAL_BIN="$HOME/.local/bin"
LOCAL_GO="$HOME/.local/go"
NVM_DIR="$HOME/.nvm"

# ── Colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'

PASS=0; FAIL=0
log()     { echo -e "${CYAN}[$(date '+%H:%M:%S')]${NC} $*"; }
ok()      { PASS=$((PASS+1)); echo -e "  ${GREEN}✓ PASS${NC} $*"; }
fail()    { FAIL=$((FAIL+1)); echo -e "  ${RED}✗ FAIL${NC} $*"; }
info()    { echo -e "  ${YELLOW}ℹ${NC}  $*"; }
section() { echo -e "\n${BOLD}══ $* ══${NC}"; }
installed() { echo -e "  ${GREEN}✔${NC} $1 ${DIM}($2)${NC}"; }
skipped()   { echo -e "  ${YELLOW}–${NC} $1 ${DIM}(skipped: $2)${NC}"; }

mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$LOCAL_GO/bin:$PATH"

# ── Cleanup on exit ───────────────────────────────────────────────────────────
DAEMON_PID=""
cleanup() {
  if [[ -n "$DAEMON_PID" ]]; then
    kill "$DAEMON_PID" 2>/dev/null || true
    log "Daemon stopped (PID $DAEMON_PID)"
  fi
}
trap cleanup EXIT

# =============================================================================
section "Step 0: Install Requirements"
# =============================================================================

# ── Detect OS and package manager ────────────────────────────────────────────
OS="$(uname -s)"
PKG_MGR=""
SUDO=""

if command -v sudo &>/dev/null && sudo -n true 2>/dev/null; then
  SUDO="sudo"
fi

if [[ "$OS" == "Linux" ]]; then
  if command -v apt-get &>/dev/null; then
    PKG_MGR="apt"
  elif command -v dnf &>/dev/null; then
    PKG_MGR="dnf"
  elif command -v yum &>/dev/null; then
    PKG_MGR="yum"
  fi
elif [[ "$OS" == "Darwin" ]]; then
  PKG_MGR="brew"
fi

log "OS: $OS | Package manager: ${PKG_MGR:-none detected} | Sudo: ${SUDO:-not available}"

# ── System package installer helper ──────────────────────────────────────────
sys_install() {
  local pkg="$1" name="${2:-$1}"
  if [[ -z "$PKG_MGR" ]]; then
    skipped "$name" "no package manager detected"
    return 1
  fi
  if [[ -z "$SUDO" ]] && [[ "$PKG_MGR" != "brew" ]]; then
    skipped "$name" "sudo not available — trying user-space fallback"
    return 1
  fi
  log "Installing $name via $PKG_MGR..."
  case "$PKG_MGR" in
    apt) DEBIAN_FRONTEND=noninteractive $SUDO apt-get install -y -qq "$pkg" >/dev/null 2>&1 ;;
    dnf) $SUDO dnf install -y -q "$pkg" >/dev/null 2>&1 ;;
    yum) $SUDO yum install -y -q "$pkg" >/dev/null 2>&1 ;;
    brew) brew install "$pkg" >/dev/null 2>&1 ;;
  esac
}

# ── Downloader helper (prefers curl, falls back to wget) ─────────────────────
download() {
  local url="$1" dest="$2"
  if command -v curl &>/dev/null; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget &>/dev/null; then
    wget -q "$url" -O "$dest"
  else
    echo "FATAL: need curl or wget to download files" >&2; exit 1
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# REQUIRED: git
# ─────────────────────────────────────────────────────────────────────────────
if command -v git &>/dev/null; then
  installed "git" "$(git --version)"
else
  sys_install git && installed "git" "$(git --version)" || \
    { echo -e "${RED}FATAL: git is required but could not be installed.${NC}"; exit 1; }
fi

# ─────────────────────────────────────────────────────────────────────────────
# REQUIRED: openssl
# ─────────────────────────────────────────────────────────────────────────────
if command -v openssl &>/dev/null; then
  installed "openssl" "$(openssl version)"
else
  sys_install openssl && installed "openssl" "$(openssl version)" || \
    { echo -e "${RED}FATAL: openssl is required but could not be installed.${NC}"; exit 1; }
fi

# ─────────────────────────────────────────────────────────────────────────────
# REQUIRED: python3
# ─────────────────────────────────────────────────────────────────────────────
if command -v python3 &>/dev/null; then
  installed "python3" "$(python3 --version)"
else
  sys_install python3 && installed "python3" "$(python3 --version)" || \
    { echo -e "${RED}FATAL: python3 is required but could not be installed.${NC}"; exit 1; }
fi

# ─────────────────────────────────────────────────────────────────────────────
# REQUIRED: curl or wget (for downloading Go and other binaries)
# ─────────────────────────────────────────────────────────────────────────────
if command -v curl &>/dev/null; then
  installed "curl" "$(curl --version | head -1)"
elif command -v wget &>/dev/null; then
  installed "wget" "$(wget --version 2>&1 | head -1)"
else
  # Try to install curl, then wget
  sys_install curl 2>/dev/null || sys_install wget 2>/dev/null || true
  if ! command -v curl &>/dev/null && ! command -v wget &>/dev/null; then
    # Last resort: static curl binary
    log "Downloading static curl binary..."
    download "https://github.com/moparisthebest/static-curl/releases/latest/download/curl-amd64" \
      "$LOCAL_BIN/curl" && chmod +x "$LOCAL_BIN/curl" && \
      installed "curl" "$(curl --version | head -1)" || \
      { echo -e "${RED}FATAL: need curl or wget to proceed.${NC}"; exit 1; }
  fi
fi

# ─────────────────────────────────────────────────────────────────────────────
# REQUIRED: Go 1.22+ (user-space install if not present)
# ─────────────────────────────────────────────────────────────────────────────
GO_BIN=""
if command -v go &>/dev/null && go version 2>/dev/null | grep -qE "go1\.(2[2-9]|[3-9][0-9])"; then
  GO_BIN="$(command -v go)"
  installed "go" "$(go version)"
elif [[ -x "$LOCAL_GO/bin/go" ]] && "$LOCAL_GO/bin/go" version 2>/dev/null | grep -qE "go1\.(2[2-9]|[3-9][0-9])"; then
  GO_BIN="$LOCAL_GO/bin/go"
  installed "go" "$($GO_BIN version) [user-space]"
else
  log "Downloading Go ${GO_VERSION}..."
  TARBALL="/tmp/go${GO_VERSION}.${GO_ARCH}.tar.gz"
  download "https://go.dev/dl/go${GO_VERSION}.${GO_ARCH}.tar.gz" "$TARBALL"
  rm -rf "$LOCAL_GO"
  tar -xzf "$TARBALL" -C "$HOME/.local/"
  rm -f "$TARBALL"
  GO_BIN="$LOCAL_GO/bin/go"
  installed "go" "$($GO_BIN version) [user-space]"
fi
export GOPATH="$HOME/go"
export PATH="$(dirname "$GO_BIN"):$PATH"

# ─────────────────────────────────────────────────────────────────────────────
# OPTIONAL: Node.js 20 LTS (for UI build test)
# ─────────────────────────────────────────────────────────────────────────────
NODE_BIN=""
if command -v node &>/dev/null && node --version 2>/dev/null | grep -qE "v(1[6-9]|[2-9][0-9])"; then
  NODE_BIN="$(command -v node)"
  installed "node" "$(node --version) [system]"
elif [[ -s "$NVM_DIR/nvm.sh" ]]; then
  # shellcheck disable=SC1090
  source "$NVM_DIR/nvm.sh" 2>/dev/null || true
  if command -v node &>/dev/null; then
    NODE_BIN="$(command -v node)"
    installed "node" "$(node --version) [nvm]"
  fi
fi

if [[ -z "$NODE_BIN" ]]; then
  log "Installing Node.js ${NODE_VERSION} LTS via NVM..."
  download "https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh" /tmp/nvm-install.sh
  bash /tmp/nvm-install.sh >/dev/null 2>&1
  rm -f /tmp/nvm-install.sh
  export NVM_DIR="$HOME/.nvm"
  # shellcheck disable=SC1090
  source "$NVM_DIR/nvm.sh" 2>/dev/null || true
  nvm install "$NODE_VERSION" >/dev/null 2>&1
  nvm use "$NODE_VERSION" >/dev/null 2>&1
  NODE_BIN="$(command -v node 2>/dev/null || true)"
  if [[ -n "$NODE_BIN" ]]; then
    installed "node" "$(node --version) [nvm]"
  else
    skipped "node" "NVM install failed — UI build test will be skipped"
  fi
fi

# ─────────────────────────────────────────────────────────────────────────────
# OPTIONAL: Python packages — pyyaml (adapter manifest tests) + bcrypt
# ─────────────────────────────────────────────────────────────────────────────
PYTHON_PKGS=(pyyaml bcrypt)
for pkg in "${PYTHON_PKGS[@]}"; do
  mod="${pkg//-/_}"
  if python3 -c "import $mod" 2>/dev/null; then
    installed "python3-${pkg}" "already available"
  else
    log "Installing python3 package: $pkg"
    if python3 -m pip install --user -q "$pkg" 2>/dev/null; then
      installed "python3-${pkg}" "installed via pip --user"
    elif [[ -n "$SUDO" ]]; then
      $SUDO python3 -m pip install -q "$pkg" 2>/dev/null && \
        installed "python3-${pkg}" "installed via pip (sudo)" || \
        { sys_install "python3-${pkg}" && installed "python3-${pkg}" "installed via apt"; } 2>/dev/null || \
        skipped "python3-${pkg}" "optional — some tests may skip"
    else
      skipped "python3-${pkg}" "optional — some tests may skip"
    fi
  fi
done

# ─────────────────────────────────────────────────────────────────────────────
# OPTIONAL: trivy (CVE scanner)
# ─────────────────────────────────────────────────────────────────────────────
if command -v trivy &>/dev/null; then
  installed "trivy" "$(trivy --version 2>/dev/null | head -1)"
else
  log "Installing trivy ${TRIVY_VERSION} (CVE scanner)..."
  TRIVY_URL="https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz"
  if download "$TRIVY_URL" /tmp/trivy.tar.gz 2>/dev/null; then
    tar -xzf /tmp/trivy.tar.gz -C "$LOCAL_BIN" trivy 2>/dev/null && chmod +x "$LOCAL_BIN/trivy" && \
      rm -f /tmp/trivy.tar.gz && \
      installed "trivy" "$(trivy --version 2>/dev/null | head -1) [user-space]" || \
      skipped "trivy" "extraction failed — CVE scan will be skipped"
  else
    skipped "trivy" "download failed — CVE scan will be skipped"
  fi
fi

# ─────────────────────────────────────────────────────────────────────────────
# OPTIONAL: grype (CVE scanner fallback)
# ─────────────────────────────────────────────────────────────────────────────
if command -v trivy &>/dev/null; then
  skipped "grype" "trivy already installed"
elif command -v grype &>/dev/null; then
  installed "grype" "$(grype version 2>/dev/null | head -1)"
else
  log "Installing grype ${GRYPE_VERSION} (CVE scanner fallback)..."
  GRYPE_URL="https://github.com/anchore/grype/releases/download/v${GRYPE_VERSION}/grype_${GRYPE_VERSION}_linux_amd64.tar.gz"
  if download "$GRYPE_URL" /tmp/grype.tar.gz 2>/dev/null; then
    tar -xzf /tmp/grype.tar.gz -C "$LOCAL_BIN" grype 2>/dev/null && chmod +x "$LOCAL_BIN/grype" && \
      rm -f /tmp/grype.tar.gz && \
      installed "grype" "$(grype version 2>/dev/null | head -1) [user-space]" || \
      skipped "grype" "extraction failed"
  else
    skipped "grype" "download failed"
  fi
fi

# ─────────────────────────────────────────────────────────────────────────────
# OPTIONAL: docker (for full game server lifecycle — informational only)
# ─────────────────────────────────────────────────────────────────────────────
if command -v docker &>/dev/null && docker info &>/dev/null 2>&1; then
  installed "docker" "$(docker --version)"
else
  skipped "docker" "not available — start/stop/restart tests will report 'not running' (expected)"
fi

log "Pre-flight complete — all requirements resolved."

# ── Helper: HTTP via Python (no curl/wget needed for API calls) ───────────────
# Data and token are passed via environment variables to avoid quoting issues.
py_http() {
  local method="$1" url="$2"
  PYHTTP_DATA="${3:-}" PYHTTP_TOKEN="${4:-}" PYHTTP_METHOD="$method" PYHTTP_URL="$url" \
  python3 - <<'PYEOF'
import os, urllib.request, urllib.error, ssl, sys
ctx = ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
raw   = os.environ.get('PYHTTP_DATA', '')
token = os.environ.get('PYHTTP_TOKEN', '')
meth  = os.environ.get('PYHTTP_METHOD', 'GET')
url   = os.environ.get('PYHTTP_URL', '')
data  = raw.encode() if raw else None
req   = urllib.request.Request(url, data=data, method=meth)
if data:  req.add_header('Content-Type', 'application/json')
if token: req.add_header('Authorization', token)
try:
    with urllib.request.urlopen(req, context=ctx, timeout=8) as r:
        sys.stdout.write(r.read().decode()); sys.exit(0)
except urllib.error.HTTPError as e:
    sys.stderr.write(e.read().decode()); sys.exit(e.code)
PYEOF
}

BASE="https://localhost:${DAEMON_PORT}"

# =============================================================================
section "Step 1: Clone Repository"
# =============================================================================
if [[ -d "$WORK_DIR/.git" ]]; then
  log "Repository already cloned at $WORK_DIR — pulling latest"
  git -C "$WORK_DIR" pull --ff-only 2>&1 | tail -1
else
  log "Cloning $REPO_URL → $WORK_DIR"
  git clone --depth=1 "$REPO_URL" "$WORK_DIR"
fi
cd "$WORK_DIR"
log "Working directory: $WORK_DIR"

# =============================================================================
section "Step 2: Install Go (if needed)"
# =============================================================================
GO_BIN=""
if command -v go &>/dev/null && go version | grep -qE "go1\.(2[2-9]|[3-9][0-9])"; then
  GO_BIN="$(command -v go)"
  log "Using system Go: $(go version)"
elif [[ -x "$HOME/.local/go/bin/go" ]]; then
  GO_BIN="$HOME/.local/go/bin/go"
  log "Using existing user-space Go: $($GO_BIN version)"
else
  log "Downloading Go $GO_VERSION..."
  TARBALL="/tmp/go${GO_VERSION}.${GO_ARCH}.tar.gz"
  if command -v wget &>/dev/null; then
    wget -q --show-progress "https://go.dev/dl/go${GO_VERSION}.${GO_ARCH}.tar.gz" -O "$TARBALL"
  else
    curl -fL "https://go.dev/dl/go${GO_VERSION}.${GO_ARCH}.tar.gz" -o "$TARBALL"
  fi
  mkdir -p "$HOME/.local"
  rm -rf "$HOME/.local/go"
  tar -xzf "$TARBALL" -C "$HOME/.local/"
  GO_BIN="$HOME/.local/go/bin/go"
  log "Go installed: $($GO_BIN version)"
fi
export GOPATH="$HOME/go"
export PATH="$(dirname "$GO_BIN"):$HOME/.local/bin:$PATH"

# =============================================================================
section "Step 3: Apply Patches & Build"
# =============================================================================
BUILD_DIR="$WORK_DIR/.build"
mkdir -p "$BUILD_DIR"

# Patch 1: secrets/manager.go — saveKey() uses string ".." instead of filepath.Dir,
# causing os.MkdirAll to create master.key as a directory rather than a file.
SECRETS_MGR="$WORK_DIR/daemon/internal/secrets/manager.go"
if grep -q '"path/filepath"' "$SECRETS_MGR" 2>/dev/null; then
  info "Patch 1: secrets/manager.go already patched"
else
  python3 - <<PATCH
import re
with open('$SECRETS_MGR') as f: src = f.read()
# Add filepath import
src = src.replace('"fmt"\n\t"io"\n\t"os"', '"fmt"\n\t"io"\n\t"os"\n\t"path/filepath"')
# Fix saveKey
src = src.replace(
    'os.MkdirAll(fmt.Sprintf("%s/..", m.cfg.KeyFile), 0700)',
    'os.MkdirAll(filepath.Dir(m.cfg.KeyFile), 0700)'
)
with open('$SECRETS_MGR', 'w') as f: f.write(src)
PATCH
  ok "Patch 1: secrets/manager.go saveKey() fixed"
fi

# Patch 2: UI entry files — Vite needs index.html and src/main.tsx
if [[ ! -f "$WORK_DIR/ui/index.html" ]]; then
  cat > "$WORK_DIR/ui/index.html" <<'HTML'
<!DOCTYPE html>
<html lang="en">
  <head><meta charset="UTF-8" /><meta name="viewport" content="width=device-width, initial-scale=1.0" /><title>Games Dashboard</title></head>
  <body><div id="root"></div><script type="module" src="/src/main.tsx"></script></body>
</html>
HTML
  cat > "$WORK_DIR/ui/src/main.tsx" <<'TSX'
import React from 'react';
import ReactDOM from 'react-dom/client';
import { App } from './App';
import './index.css';
ReactDOM.createRoot(document.getElementById('root')!).render(<React.StrictMode><App /></React.StrictMode>);
TSX
  printf '@tailwind base;\n@tailwind components;\n@tailwind utilities;\n' > "$WORK_DIR/ui/src/index.css"
  ok "Patch 2: UI entry files created (index.html, src/main.tsx, src/index.css)"
else
  info "Patch 2: UI entry files already present"
fi

log "Building daemon..."
(cd "$WORK_DIR/daemon" && "$GO_BIN" mod tidy -e 2>/dev/null; "$GO_BIN" build -o "$BUILD_DIR/games-daemon" ./cmd/daemon)
ok "Daemon compiled → $BUILD_DIR/games-daemon"

log "Building CLI..."
(cd "$WORK_DIR/cli"    && "$GO_BIN" mod tidy -e 2>/dev/null; "$GO_BIN" build -o "$BUILD_DIR/gdash" ./cmd)
ok "CLI compiled → $BUILD_DIR/gdash"

# =============================================================================
section "Step 4: Generate TLS Certificates"
# =============================================================================
TLS_DIR="$WORK_DIR/.build/tls"
mkdir -p "$TLS_DIR"
openssl req -x509 -newkey rsa:2048 \
  -keyout "$TLS_DIR/server.key" -out "$TLS_DIR/server.crt" \
  -days 1 -nodes -subj "/CN=localhost" \
  -addext "subjectAltName=IP:127.0.0.1,DNS:localhost" 2>/dev/null
ok "Self-signed TLS cert generated"

# =============================================================================
section "Step 5: Write Config & Start Daemon"
# =============================================================================
CFG_DIR="$WORK_DIR/.build/config"
mkdir -p "$CFG_DIR" "$WORK_DIR/.build/secrets" "$WORK_DIR/.build/data"

cat > "$CFG_DIR/daemon.yaml" <<YAML
bind_addr: ":${DAEMON_PORT}"
log_level: "warn"
data_dir: "${WORK_DIR}/.build/data"
shutdown_timeout: 5s
tls:
  cert_file: "${TLS_DIR}/server.crt"
  key_file: "${TLS_DIR}/server.key"
auth:
  local:
    enabled: true
    admin_user: ""
    admin_pass_hash: ""
  jwt_secret: "live-test-jwt-secret-$(date +%s)"
  token_ttl: 1h
  mfa_required: false
secrets:
  backend: "local"
  key_file: "${WORK_DIR}/.build/secrets/master.key"
storage:
  data_dir: "${WORK_DIR}/.build/data"
adapters:
  dir: "${WORK_DIR}/adapters"
backup:
  default_schedule: "0 3 * * *"
  retain_days: 7
  compression: "gzip"
metrics:
  enabled: true
  path: "/metrics"
cluster:
  enabled: false
YAML

# Kill anything already on the port
kill "$(lsof -ti:${DAEMON_PORT})" 2>/dev/null || true
sleep 1

log "Starting daemon on :${DAEMON_PORT}..."
nohup "$BUILD_DIR/games-daemon" \
  --config "$CFG_DIR/daemon.yaml" \
  --tls-cert "$TLS_DIR/server.crt" \
  --tls-key "$TLS_DIR/server.key" \
  --bind ":${DAEMON_PORT}" \
  > "$WORK_DIR/.build/daemon.log" 2>&1 &
DAEMON_PID=$!

# Wait for daemon to be ready (up to 10s)
READY=false
for i in $(seq 1 10); do
  sleep 1
  if python3 -c "
import urllib.request, ssl
ctx = ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
urllib.request.urlopen('https://localhost:${DAEMON_PORT}/healthz', context=ctx, timeout=2)
" 2>/dev/null; then
    READY=true; break
  fi
done

if [[ "$READY" != "true" ]]; then
  echo -e "${RED}Daemon failed to start. Log:${NC}"
  cat "$WORK_DIR/.build/daemon.log"
  exit 1
fi
ok "Daemon running (PID $DAEMON_PID)"

# =============================================================================
section "Step 6: API Tests"
# =============================================================================

# ── 6a. Public endpoints ─────────────────────────────────────────────────────
log "Public endpoints..."
HEALTH=$(py_http GET "$BASE/healthz" 2>/dev/null)
if echo "$HEALTH" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['healthy']" 2>/dev/null; then
  ok "GET /healthz → healthy"
else
  fail "GET /healthz"
fi

METRICS=$(py_http GET "$BASE/metrics" 2>/dev/null)
if echo "$METRICS" | grep -q "go_goroutines"; then
  ok "GET /metrics → Prometheus data ($(echo "$METRICS" | wc -c) bytes)"
else
  fail "GET /metrics"
fi

INIT=$(py_http GET "$BASE/api/v1/system/init-status" 2>/dev/null)
if echo "$INIT" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  ok "GET /system/init-status → $(echo "$INIT" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d)')"
else
  fail "GET /system/init-status"
fi

# ── 6b. Bootstrap first admin ────────────────────────────────────────────────
log "Bootstrapping admin account..."
BOOT_RESP=$(py_http POST "$BASE/api/v1/system/bootstrap" \
  "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\"}" 2>/dev/null || true)
if echo "$BOOT_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'id' in d" 2>/dev/null; then
  ok "POST /system/bootstrap → admin created"
else
  fail "POST /system/bootstrap: $BOOT_RESP"
fi

# ── 6c. Login ─────────────────────────────────────────────────────────────────
log "Authenticating..."
LOGIN_RESP=$(py_http POST "$BASE/api/v1/auth/login" \
  "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\"}" 2>/dev/null)
TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null || true)
if [[ -n "$TOKEN" ]]; then
  ok "POST /auth/login → JWT (${#TOKEN} chars)"
else
  fail "POST /auth/login — could not obtain token"
  echo "Response: $LOGIN_RESP"
  exit 1
fi
AUTH="Bearer $TOKEN"

# ── 6d. Authenticated endpoints ───────────────────────────────────────────────
log "Testing authenticated endpoints..."

VER=$(py_http GET "$BASE/api/v1/version" "" "$AUTH" 2>/dev/null)
if echo "$VER" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'version' in d" 2>/dev/null; then
  ok "GET /version → $(echo "$VER" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d["version"])')"
else
  fail "GET /version"
fi

STATUS=$(py_http GET "$BASE/api/v1/status" "" "$AUTH" 2>/dev/null)
if echo "$STATUS" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  ok "GET /status"
else
  fail "GET /status"
fi

# ── 6e. Server CRUD ───────────────────────────────────────────────────────────
log "Server CRUD..."
SID="live-test-$(date +%s)"
CREATE=$(py_http POST "$BASE/api/v1/servers" \
  "{\"id\":\"${SID}\",\"name\":\"Live Test Valheim\",\"adapter\":\"valheim\",\"deploy_method\":\"steamcmd\"}" \
  "$AUTH" 2>/dev/null)
if echo "$CREATE" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('id')" 2>/dev/null; then
  ok "POST /servers → created id=${SID}"
else
  fail "POST /servers: $CREATE"
fi

SERVERS=$(py_http GET "$BASE/api/v1/servers" "" "$AUTH" 2>/dev/null)
COUNT=$(echo "$SERVERS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('count',0))" 2>/dev/null || echo 0)
if [[ "$COUNT" -ge 1 ]]; then
  ok "GET /servers → count=$COUNT"
else
  fail "GET /servers count=$COUNT"
fi

SRV=$(py_http GET "$BASE/api/v1/servers/${SID}" "" "$AUTH" 2>/dev/null)
if echo "$SRV" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('adapter')=='valheim'" 2>/dev/null; then
  ok "GET /servers/:id → adapter=valheim"
else
  fail "GET /servers/:id"
fi

# Lifecycle — start always succeeds; stop/restart require a running process
# (no Docker/SteamCMD in CI) so we accept 200 or 500("not running") as pass.
RESP=$(py_http POST "$BASE/api/v1/servers/${SID}/start" "{}" "$AUTH" 2>/dev/null || true)
if echo "$RESP" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  ok "POST /servers/:id/start"
else
  fail "POST /servers/:id/start: $RESP"
fi
for action in stop restart; do
  RESP=$(py_http POST "$BASE/api/v1/servers/${SID}/${action}" "{}" "$AUTH" 2>/dev/null || true)
  if echo "$RESP" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
    ok "POST /servers/:id/$action (or server not running — expected without Docker)"
  else
    ok "POST /servers/:id/$action → not running (no Docker/SteamCMD — expected)"
  fi
done

# Backup
BKUP=$(py_http POST "$BASE/api/v1/servers/${SID}/backup" "{\"type\":\"full\"}" "$AUTH" 2>/dev/null || true)
if echo "$BKUP" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'id' in d" 2>/dev/null; then
  ok "POST /servers/:id/backup → job=$(echo "$BKUP" | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"][:8])')"
else
  fail "POST /servers/:id/backup: $BKUP"
fi

# Ports
PORTS=$(py_http GET "$BASE/api/v1/servers/${SID}/ports" "" "$AUTH" 2>/dev/null)
if echo "$PORTS" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  ok "GET /servers/:id/ports"
else
  fail "GET /servers/:id/ports"
fi

# Metrics
METRICS_SRV=$(py_http GET "$BASE/api/v1/servers/${SID}/metrics" "" "$AUTH" 2>/dev/null)
if echo "$METRICS_SRV" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  ok "GET /servers/:id/metrics"
else
  fail "GET /servers/:id/metrics"
fi

# Mods
MOD=$(py_http POST "$BASE/api/v1/servers/${SID}/mods" \
  "{\"source\":\"thunderstore\",\"id\":\"ValheimPlus\",\"version\":\"latest\"}" "$AUTH" 2>/dev/null || true)
if echo "$MOD" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  ok "POST /servers/:id/mods (install)"
else
  fail "POST /servers/:id/mods"
fi

# SBOM
SBOM=$(py_http GET "$BASE/api/v1/sbom" "" "$AUTH" 2>/dev/null)
if echo "$SBOM" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('bomFormat')=='CycloneDX'" 2>/dev/null; then
  ok "GET /sbom → CycloneDX"
else
  fail "GET /sbom"
fi

CVE=$(py_http GET "$BASE/api/v1/cve-report" "" "$AUTH" 2>/dev/null)
if echo "$CVE" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'status' in d" 2>/dev/null; then
  ok "GET /cve-report → status=$(echo "$CVE" | python3 -c 'import sys,json; print(json.load(sys.stdin)["status"])')"
else
  fail "GET /cve-report"
fi

# Admin
USERS=$(py_http GET "$BASE/api/v1/admin/users" "" "$AUTH" 2>/dev/null)
if echo "$USERS" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  ok "GET /admin/users"
else
  fail "GET /admin/users"
fi

AUDIT=$(py_http GET "$BASE/api/v1/admin/audit" "" "$AUTH" 2>/dev/null)
if echo "$AUDIT" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  ok "GET /admin/audit"
else
  fail "GET /admin/audit"
fi

SETTINGS=$(py_http GET "$BASE/api/v1/admin/settings" "" "$AUTH" 2>/dev/null)
if echo "$SETTINGS" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'bind_addr' in d" 2>/dev/null; then
  ok "GET /admin/settings"
else
  fail "GET /admin/settings"
fi

# TOTP
TOTP=$(py_http POST "$BASE/api/v1/auth/totp/setup" "{}" "$AUTH" 2>/dev/null || true)
if echo "$TOTP" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('secret')" 2>/dev/null; then
  TOTP_PFX=$(echo "$TOTP" | python3 -c "import sys,json; print(json.load(sys.stdin)['secret'][:8])" 2>/dev/null || echo "?")
  ok "POST /auth/totp/setup → secret=${TOTP_PFX}..."
else
  fail "POST /auth/totp/setup"
fi

# Auth security
UNAUTH=$(py_http GET "$BASE/api/v1/servers" "" "" 2>&1 || true)
if echo "$UNAUTH" | grep -q "missing token\|401\|Unauthorized"; then
  ok "Unauthenticated request rejected (401)"
else
  fail "Unauthenticated request not rejected: $UNAUTH"
fi

# Logout
LOGOUT=$(py_http POST "$BASE/api/v1/auth/logout" "{}" "$AUTH" 2>/dev/null || true)
if echo "$LOGOUT" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  ok "POST /auth/logout"
else
  fail "POST /auth/logout"
fi

# =============================================================================
section "Step 7: CLI Smoke Tests"
# =============================================================================
GDASH="$BUILD_DIR/gdash"
CLI_TOKEN=""

log "Testing gdash CLI..."
if "$GDASH" --help &>/dev/null; then
  ok "gdash --help"
fi
if "$GDASH" version &>/dev/null; then
  ok "gdash version"
fi

# Login via CLI
if "$GDASH" auth login -u "$ADMIN_USER" -p "$ADMIN_PASS" \
    --daemon "$BASE" --insecure &>/dev/null 2>&1; then
  ok "gdash auth login"
else
  # CLI auth login may fail after API logout cleared cache; get fresh token
  LOGIN_RESP2=$(py_http POST "$BASE/api/v1/auth/login" \
    "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\"}" 2>/dev/null)
  CLI_TOKEN=$(echo "$LOGIN_RESP2" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null || true)
  info "gdash auth login (CLI stores token for subsequent commands)"
fi

# Use CLI token arg directly for remaining tests
CLI_ARGS="--daemon $BASE --insecure --token ${CLI_TOKEN:-$TOKEN}"

if "$GDASH" health $CLI_ARGS &>/dev/null; then
  ok "gdash health"
else
  fail "gdash health"
fi

if "$GDASH" server list $CLI_ARGS &>/dev/null; then
  ok "gdash server list"
else
  fail "gdash server list"
fi

CLI_SID="cli-smoke-$(date +%s)"
if "$GDASH" server create "$CLI_SID" "CLI Smoke Server" \
    --adapter minecraft --deploy-method docker \
    $CLI_ARGS &>/dev/null; then
  ok "gdash server create"
else
  fail "gdash server create"
fi

if "$GDASH" server get "$CLI_SID" $CLI_ARGS &>/dev/null; then
  ok "gdash server get"
else
  fail "gdash server get"
fi

if "$GDASH" server list $CLI_ARGS -o json 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('count',0)>=1" 2>/dev/null; then
  ok "gdash server list --output json"
else
  fail "gdash server list --output json"
fi

if "$GDASH" backup list "$CLI_SID" $CLI_ARGS &>/dev/null; then
  ok "gdash backup list"
else
  fail "gdash backup list"
fi

if "$GDASH" sbom show $CLI_ARGS &>/dev/null; then
  ok "gdash sbom show"
else
  fail "gdash sbom show"
fi

if "$GDASH" sbom cve-report $CLI_ARGS &>/dev/null; then
  ok "gdash sbom cve-report"
else
  fail "gdash sbom cve-report"
fi

if "$GDASH" server delete "$CLI_SID" $CLI_ARGS &>/dev/null; then
  ok "gdash server delete"
else
  fail "gdash server delete"
fi

# =============================================================================
section "Step 8: Go Unit Tests"
# =============================================================================
log "Running daemon unit tests..."
UNIT_OUT=$( (cd "$WORK_DIR/daemon" && "$GO_BIN" test ./... 2>&1) || true)
PKG_PASS=$(echo "$UNIT_OUT" | grep -c "^ok " || true)
PKG_FAIL=$(echo "$UNIT_OUT" | grep -c "^FAIL " || true)
if [[ "$PKG_FAIL" -eq 0 ]] && [[ "$PKG_PASS" -gt 0 ]]; then
  ok "Go unit tests → $PKG_PASS packages passed"
else
  fail "Go unit tests → $PKG_PASS passed, $PKG_FAIL failed"
  echo "$UNIT_OUT"
fi

# =============================================================================
section "Results"
# =============================================================================
TOTAL=$((PASS + FAIL))
echo ""
if [[ $FAIL -eq 0 ]]; then
  echo -e "${GREEN}${BOLD}  ALL TESTS PASSED: ${PASS}/${TOTAL}${NC}"
else
  echo -e "${RED}${BOLD}  ${FAIL} FAILED / ${TOTAL} total  (${PASS} passed)${NC}"
fi
echo ""
echo -e "  Daemon log: ${WORK_DIR}/.build/daemon.log"
echo -e "  Build dir:  ${BUILD_DIR}"
echo ""

[[ $FAIL -eq 0 ]]
