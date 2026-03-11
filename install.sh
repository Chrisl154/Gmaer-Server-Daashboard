#!/usr/bin/env bash
# =============================================================================
# Games Dashboard — Production Installer
# Deploys the daemon + UI and leaves everything running.
#
# Usage (Ubuntu 22.04/24.04):
#   curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/install.sh | bash
#
#   Or locally:
#   bash install.sh
#
# After install, open https://<your-server-ip> in a browser.
# Default login: admin / (shown at end of install)
# =============================================================================
set -euo pipefail

REPO_URL="https://github.com/Chrisl154/Gmaer-Server-Daashboard.git"
INSTALL_DIR="/opt/gdash"
DAEMON_PORT="8443"
UI_PORT="443"
GO_VERSION="1.22.4"
# UI talks to nginx (443), nginx proxies /api/* to daemon internally.
# Do NOT point UI directly at daemon port — it binds to 127.0.0.1 only.
GO_ARCH="linux-amd64"
NODE_VERSION="20"
LOCAL_BIN="$HOME/.local/bin"
LOCAL_GO="$HOME/.local/go"
NVM_DIR="$HOME/.nvm"

# ── Colours ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'

log()     { echo -e "${CYAN}[$(date '+%H:%M:%S')]${NC} $*"; }
ok()      { echo -e "  ${GREEN}✓${NC} $*"; }
fail()    { echo -e "  ${RED}✗ ERROR:${NC} $*"; exit 1; }
info()    { echo -e "  ${YELLOW}ℹ${NC}  $*"; }
section() { echo -e "\n${BOLD}══ $* ══${NC}"; }

mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$LOCAL_GO/bin:$PATH"

# ── Sudo check ────────────────────────────────────────────────────────────────
SUDO=""
if [[ $EUID -eq 0 ]]; then
  SUDO=""
elif command -v sudo &>/dev/null && sudo -n true 2>/dev/null; then
  SUDO="sudo"
elif command -v sudo &>/dev/null; then
  info "Some steps require sudo — you may be prompted for your password."
  SUDO="sudo"
else
  fail "This installer needs root or sudo access to install nginx and write to /opt/gdash."
fi

# ── Detect server IP ──────────────────────────────────────────────────────────
detect_ip() {
  # Try multiple methods, prefer non-loopback
  local ip=""
  ip=$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if ($i=="src") {print $(i+1); exit}}') || true
  if [[ -z "$ip" ]]; then
    ip=$(hostname -I 2>/dev/null | awk '{print $1}') || true
  fi
  if [[ -z "$ip" ]]; then
    ip="localhost"
  fi
  echo "$ip"
}

SERVER_IP="${GDASH_HOST:-$(detect_ip)}"
DAEMON_URL="https://${SERVER_IP}:${DAEMON_PORT}"   # internal only
UI_API_URL="https://${SERVER_IP}"                  # nginx on 443 — what browser sees

# =============================================================================
section "Step 0: Install System Requirements"
# =============================================================================

pkg_install() {
  DEBIAN_FRONTEND=noninteractive $SUDO apt-get install -y -qq "$@" >/dev/null 2>&1
}

download() {
  local url="$1" dest="$2"
  if command -v curl &>/dev/null; then
    curl -fsSL "$url" -o "$dest"
  else
    wget -q "$url" -O "$dest"
  fi
}

log "Updating package index..."
$SUDO apt-get update -qq >/dev/null 2>&1

for pkg in git openssl python3 curl nginx lsof; do
  if ! command -v "$pkg" &>/dev/null && ! dpkg -l "$pkg" &>/dev/null 2>&1; then
    log "Installing $pkg..."
    pkg_install "$pkg"
  fi
done
ok "System packages ready"

# ── Go ────────────────────────────────────────────────────────────────────────
GO_BIN=""
if command -v go &>/dev/null && go version 2>/dev/null | grep -qE "go1\.(2[2-9]|[3-9][0-9])"; then
  GO_BIN="$(command -v go)"
  ok "Go: $(go version)"
elif [[ -x "$LOCAL_GO/bin/go" ]] && "$LOCAL_GO/bin/go" version 2>/dev/null | grep -qE "go1\.(2[2-9]|[3-9][0-9])"; then
  GO_BIN="$LOCAL_GO/bin/go"
  ok "Go: $($GO_BIN version) [user-space]"
else
  log "Downloading Go ${GO_VERSION}..."
  TARBALL="/tmp/go${GO_VERSION}.${GO_ARCH}.tar.gz"
  download "https://go.dev/dl/go${GO_VERSION}.${GO_ARCH}.tar.gz" "$TARBALL"
  mkdir -p "$HOME/.local"
  rm -rf "$LOCAL_GO"
  tar -xzf "$TARBALL" -C "$HOME/.local/"
  rm -f "$TARBALL"
  GO_BIN="$LOCAL_GO/bin/go"
  ok "Go: $($GO_BIN version) [user-space installed]"
fi
export GOPATH="$HOME/go"
export PATH="$(dirname "$GO_BIN"):$PATH"

# ── Node.js ───────────────────────────────────────────────────────────────────
NODE_BIN=""
if command -v node &>/dev/null && node --version 2>/dev/null | grep -qE "v(1[6-9]|[2-9][0-9])"; then
  NODE_BIN="$(command -v node)"
  ok "Node: $(node --version)"
elif [[ -s "$NVM_DIR/nvm.sh" ]]; then
  # shellcheck disable=SC1090
  source "$NVM_DIR/nvm.sh" 2>/dev/null || true
  if command -v node &>/dev/null; then
    NODE_BIN="$(command -v node)"
    ok "Node: $(node --version) [nvm]"
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
  NODE_BIN="$(command -v node 2>/dev/null)" || fail "Node.js install failed."
  ok "Node: $(node --version) [nvm installed]"
fi

# ── Python packages ───────────────────────────────────────────────────────────
for pkg in pyyaml bcrypt; do
  python3 -c "import ${pkg//-/_}" 2>/dev/null || \
    python3 -m pip install --user -q "$pkg" 2>/dev/null || true
done
ok "Python packages: pyyaml, bcrypt"

# =============================================================================
section "Step 1: Clone / Update Repository"
# =============================================================================

$SUDO mkdir -p "$INSTALL_DIR"
$SUDO chown "$USER":"$USER" "$INSTALL_DIR"

REPO_DIR="$INSTALL_DIR/repo"
if [[ -d "$REPO_DIR/.git" ]]; then
  log "Repository exists — pulling latest..."
  git -C "$REPO_DIR" pull --ff-only 2>&1 | tail -1
else
  log "Cloning repository..."
  git clone --depth=1 "$REPO_URL" "$REPO_DIR"
fi
ok "Repository at $REPO_DIR"

# =============================================================================
section "Step 2: Apply Patches"
# =============================================================================

# Patch: secrets/manager.go — filepath.Dir fix
SECRETS_MGR="$REPO_DIR/daemon/internal/secrets/manager.go"
if ! grep -q '"path/filepath"' "$SECRETS_MGR" 2>/dev/null; then
  python3 - <<PATCH
with open('$SECRETS_MGR') as f: src = f.read()
src = src.replace('"fmt"\n\t"io"\n\t"os"', '"fmt"\n\t"io"\n\t"os"\n\t"path/filepath"')
src = src.replace(
    'os.MkdirAll(fmt.Sprintf("%s/..", m.cfg.KeyFile), 0700)',
    'os.MkdirAll(filepath.Dir(m.cfg.KeyFile), 0700)'
)
with open('$SECRETS_MGR', 'w') as f: f.write(src)
PATCH
  ok "Patch applied: secrets/manager.go"
else
  ok "Patch already applied: secrets/manager.go"
fi

# Patch: UI entry files
if [[ ! -f "$REPO_DIR/ui/index.html" ]]; then
  cat > "$REPO_DIR/ui/index.html" <<'HTML'
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Games Dashboard</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
HTML
  cat > "$REPO_DIR/ui/src/main.tsx" <<'TSX'
import React from 'react';
import ReactDOM from 'react-dom/client';
import { App } from './App';
import './index.css';
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode><App /></React.StrictMode>
);
TSX
  printf '@tailwind base;\n@tailwind components;\n@tailwind utilities;\n' \
    > "$REPO_DIR/ui/src/index.css"
  ok "Patch applied: UI entry files"
else
  ok "Patch already applied: UI entry files"
fi

# =============================================================================
section "Step 3: Build"
# =============================================================================

BIN_DIR="$INSTALL_DIR/bin"
mkdir -p "$BIN_DIR"

log "Building daemon..."
(cd "$REPO_DIR/daemon" && "$GO_BIN" mod tidy -e 2>/dev/null; \
  "$GO_BIN" build -o "$BIN_DIR/games-daemon" ./cmd/daemon)
ok "Daemon → $BIN_DIR/games-daemon"

log "Building CLI..."
(cd "$REPO_DIR/cli" && "$GO_BIN" mod tidy -e 2>/dev/null; \
  "$GO_BIN" build -o "$BIN_DIR/gdash" ./cmd)
$SUDO ln -sf "$BIN_DIR/gdash" /usr/local/bin/gdash 2>/dev/null || true
ok "CLI → $BIN_DIR/gdash (linked to /usr/local/bin/gdash)"

log "Building UI (VITE_DAEMON_URL=$UI_API_URL)..."
chmod +x "$REPO_DIR/ui/node_modules/.bin/"* 2>/dev/null || true
(cd "$REPO_DIR/ui" && \
  npm install --silent 2>/dev/null && \
  VITE_DAEMON_URL="$UI_API_URL" node_modules/.bin/vite build --outDir "$INSTALL_DIR/ui" 2>/dev/null)
ok "UI → $INSTALL_DIR/ui"

# =============================================================================
section "Step 4: TLS Certificate"
# =============================================================================

TLS_DIR="$INSTALL_DIR/tls"
mkdir -p "$TLS_DIR"

if [[ ! -f "$TLS_DIR/server.crt" ]]; then
  openssl req -x509 -newkey rsa:2048 \
    -keyout "$TLS_DIR/server.key" -out "$TLS_DIR/server.crt" \
    -days 3650 -nodes \
    -subj "/CN=${SERVER_IP}" \
    -addext "subjectAltName=IP:${SERVER_IP},DNS:${SERVER_IP},DNS:localhost,IP:127.0.0.1" \
    2>/dev/null
  ok "TLS cert generated (10 years, SAN: ${SERVER_IP})"
else
  ok "TLS cert already exists"
fi

# =============================================================================
section "Step 5: Daemon Configuration"
# =============================================================================

CFG_DIR="$INSTALL_DIR/config"
DATA_DIR="$INSTALL_DIR/data"
SECRETS_DIR="$INSTALL_DIR/secrets"
mkdir -p "$CFG_DIR" "$DATA_DIR" "$SECRETS_DIR"

# Generate JWT secret
JWT_SECRET=$(python3 -c "import secrets; print(secrets.token_hex(32))")

cat > "$CFG_DIR/daemon.yaml" <<YAML
bind_addr: "127.0.0.1:${DAEMON_PORT}"
log_level: "info"
data_dir: "${DATA_DIR}"
shutdown_timeout: 30s
tls:
  cert_file: "${TLS_DIR}/server.crt"
  key_file: "${TLS_DIR}/server.key"
auth:
  local:
    enabled: true
    admin_user: ""
    admin_pass_hash: ""
  jwt_secret: "${JWT_SECRET}"
  token_ttl: 8h
  mfa_required: false
secrets:
  backend: "local"
  key_file: "${SECRETS_DIR}/master.key"
storage:
  data_dir: "${DATA_DIR}"
adapters:
  dir: "${REPO_DIR}/adapters"
backup:
  default_schedule: "0 3 * * *"
  retain_days: 30
  compression: "gzip"
metrics:
  enabled: true
  path: "/metrics"
cluster:
  enabled: false
YAML
ok "Daemon config → $CFG_DIR/daemon.yaml"

# =============================================================================
section "Step 6: Systemd Service"
# =============================================================================

$SUDO tee /etc/systemd/system/gdash-daemon.service > /dev/null <<SERVICE
[Unit]
Description=Games Dashboard Daemon
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=${USER}
Group=${USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BIN_DIR}/games-daemon \\
    --config ${CFG_DIR}/daemon.yaml \\
    --tls-cert ${TLS_DIR}/server.crt \\
    --tls-key ${TLS_DIR}/server.key \\
    --bind 127.0.0.1:${DAEMON_PORT}
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gdash-daemon
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
SERVICE

$SUDO systemctl daemon-reload
$SUDO systemctl enable gdash-daemon 2>/dev/null
$SUDO systemctl restart gdash-daemon
sleep 3

if $SUDO systemctl is-active --quiet gdash-daemon; then
  ok "Daemon service running (systemd: gdash-daemon)"
else
  echo -e "${RED}Daemon failed to start. Check logs:${NC}"
  $SUDO journalctl -u gdash-daemon -n 30 --no-pager
  exit 1
fi

# =============================================================================
section "Step 7: nginx Configuration"
# =============================================================================

$SUDO tee /etc/nginx/sites-available/gdash > /dev/null <<NGINX
# Games Dashboard
server {
    listen 80;
    server_name _;
    # Redirect HTTP to HTTPS
    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl;
    server_name _;

    ssl_certificate     ${TLS_DIR}/server.crt;
    ssl_certificate_key ${TLS_DIR}/server.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    # UI static files
    root ${INSTALL_DIR}/ui;
    index index.html;

    # SPA fallback — all non-asset routes serve index.html
    location / {
        try_files \$uri \$uri/ /index.html;
    }

    # API — proxy to daemon
    location /api/ {
        proxy_pass https://127.0.0.1:${DAEMON_PORT};
        proxy_ssl_verify off;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 120s;

        # WebSocket upgrade
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Health + metrics — proxy to daemon
    location ~ ^/(healthz|metrics)$ {
        proxy_pass https://127.0.0.1:${DAEMON_PORT};
        proxy_ssl_verify off;
    }

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
}
NGINX

# Enable site, disable default
$SUDO ln -sf /etc/nginx/sites-available/gdash /etc/nginx/sites-enabled/gdash
$SUDO rm -f /etc/nginx/sites-enabled/default

# Test and reload nginx
if $SUDO nginx -t 2>/dev/null; then
  $SUDO systemctl enable nginx 2>/dev/null
  $SUDO systemctl restart nginx
  ok "nginx configured and running"
else
  $SUDO nginx -t
  fail "nginx config test failed."
fi

# =============================================================================
section "Step 8: Bootstrap Admin Account"
# =============================================================================

# Wait for daemon to be fully ready
log "Waiting for daemon API..."
READY=false
for i in $(seq 1 15); do
  sleep 1
  if python3 -c "
import urllib.request, ssl
ctx = ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
urllib.request.urlopen('https://127.0.0.1:${DAEMON_PORT}/healthz', context=ctx, timeout=2)
" 2>/dev/null; then
    READY=true; break
  fi
done
[[ "$READY" == "true" ]] || fail "Daemon did not become ready in time."

# Generate secure admin password
ADMIN_PASS=$(python3 -c "
import secrets, string
chars = string.ascii_letters + string.digits + '!@#%^&*'
print(''.join(secrets.choice(chars) for _ in range(16)))
")

BOOT_RESP=$(python3 - <<PYEOF
import urllib.request, urllib.error, ssl, json
ctx = ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
data = json.dumps({"username": "admin", "password": "${ADMIN_PASS}"}).encode()
req = urllib.request.Request(
    "https://127.0.0.1:${DAEMON_PORT}/api/v1/system/bootstrap",
    data=data, method="POST"
)
req.add_header("Content-Type", "application/json")
try:
    with urllib.request.urlopen(req, context=ctx, timeout=10) as r:
        print(r.read().decode())
except urllib.error.HTTPError as e:
    print(e.read().decode())
PYEOF
)

if echo "$BOOT_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'id' in d" 2>/dev/null; then
  ok "Admin account created"
else
  # May already be bootstrapped (re-install)
  info "Bootstrap response: $BOOT_RESP"
  info "(If 'already initialized', the existing credentials remain valid)"
fi

# Allow UFW if present
if command -v ufw &>/dev/null; then
  $SUDO ufw allow 80/tcp  >/dev/null 2>&1 || true
  $SUDO ufw allow 443/tcp >/dev/null 2>&1 || true
  $SUDO ufw allow "${DAEMON_PORT}/tcp" >/dev/null 2>&1 || true
  ok "Firewall rules added (80, 443, ${DAEMON_PORT})"
fi

# =============================================================================
section "Install Complete"
# =============================================================================

echo ""
echo -e "${GREEN}${BOLD}  ╔══════════════════════════════════════════════╗"
echo -e "  ║       Games Dashboard is ready!              ║"
echo -e "  ╚══════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${BOLD}Dashboard URL:${NC}  https://${SERVER_IP}  (port 443 via nginx)"
echo -e "  ${BOLD}Username:${NC}       admin"
echo -e "  ${BOLD}Password:${NC}       ${ADMIN_PASS}"
echo ""
echo -e "  ${YELLOW}⚠  TLS note:${NC} The certificate is self-signed."
echo -e "     Your browser will show a security warning."
echo -e "     Click \"Advanced → Proceed\" to continue."
echo ""
echo -e "  ${BOLD}Useful commands:${NC}"
echo -e "    gdash --help                    # CLI tool"
echo -e "    sudo systemctl status gdash-daemon"
echo -e "    sudo journalctl -u gdash-daemon -f"
echo -e "    sudo systemctl restart gdash-daemon"
echo ""
echo -e "  ${DIM}Config:    $CFG_DIR/daemon.yaml"
echo -e "  Data:      $DATA_DIR"
echo -e "  Logs:      journalctl -u gdash-daemon"
echo -e "  Install:   $INSTALL_DIR${NC}"
echo ""
