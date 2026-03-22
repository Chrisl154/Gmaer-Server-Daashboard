#!/usr/bin/env bash
# =============================================================================
# Games Dashboard — Production Installer (TUI)
# Deploys the daemon + UI and leaves everything running.
#
# Usage (Ubuntu 22.04/24.04):
#   curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/install.sh | bash
#
#   Or locally:
#   bash install.sh
#
# Worker-node mode (no UI, no nginx, no admin account):
#   bash install.sh --mode=node
#
# Non-interactive (CI/scripted):
#   GDASH_NONINTERACTIVE=1 bash install.sh
#
# After install, open https://<your-server-ip> in a browser.
# Default login: admin / (shown at end of install)
# =============================================================================
set -euo pipefail

# ── Mode flag ─────────────────────────────────────────────────────────────────
# --mode=node  → worker-only install (daemon + Docker + SteamCMD, no UI/nginx)
INSTALL_MODE="full"
for _arg in "$@"; do
  case "$_arg" in
    --mode=node) INSTALL_MODE="node" ;;
    --mode=full) INSTALL_MODE="full" ;;
  esac
done

# ── Tool versions ─────────────────────────────────────────────────────────────
GO_VERSION="1.22.4"
GO_ARCH="linux-amd64"
NODE_VERSION="20"

# ── Runtime paths ─────────────────────────────────────────────────────────────
LOCAL_BIN="$HOME/.local/bin"
LOCAL_GO="$HOME/.local/go"
NVM_DIR="$HOME/.nvm"

# ── Colours ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'

log()     { echo -e "${CYAN}[$(date '+%H:%M:%S')]${NC} $*"; }
ok()      { echo -e "  ${GREEN}✓${NC} $*"; }
fail()    { echo -e "  ${RED}✗ ERROR:${NC} $*"; exit 1; }
warn()    { echo -e "  ${YELLOW}⚠${NC}  $*"; }
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

DETECTED_IP=$(detect_ip)

# ── Generate a secure default admin password ──────────────────────────────────
gen_password() {
  python3 -c "
import secrets, string
lower  = string.ascii_lowercase
upper  = string.ascii_uppercase
digits = string.digits
special= '!@#%^&*'
# Guarantee at least one of each required class, then pad to 16 chars total
pool = lower + upper + digits + special
pwd  = [secrets.choice(lower), secrets.choice(upper),
        secrets.choice(digits), secrets.choice(special)]
pwd += [secrets.choice(pool) for _ in range(12)]
secrets.SystemRandom().shuffle(pwd)
print(''.join(pwd))
" 2>/dev/null || openssl rand -base64 18 | tr -d '/+='
}

# ── Validate password complexity ───────────────────────────────────────────────
# Returns 0 (valid) or prints an error message and returns 1.
# Rules: ≥12 chars, ≥1 uppercase, ≥1 lowercase, ≥1 digit, ≥1 special char.
validate_password() {
  local pw="$1"
  if [[ ${#pw} -lt 12 ]]; then
    echo "  Password must be at least 12 characters."; return 1
  fi
  if ! [[ "$pw" =~ [A-Z] ]]; then
    echo "  Password must contain at least one uppercase letter."; return 1
  fi
  if ! [[ "$pw" =~ [a-z] ]]; then
    echo "  Password must contain at least one lowercase letter."; return 1
  fi
  if ! [[ "$pw" =~ [0-9] ]]; then
    echo "  Password must contain at least one digit."; return 1
  fi
  if ! [[ "$pw" =~ [^A-Za-z0-9] ]]; then
    echo "  Password must contain at least one special character (!@#%^&* etc)."; return 1
  fi
  return 0
}

# =============================================================================
# TUI configuration — whiptail-based, with readline fallback
# =============================================================================

# Check whether we can show a TUI (terminal attached, whiptail available)
USE_TUI=false
# Check if we have a real terminal to interact with.
# When piped (curl | bash) stdin is the pipe, but /dev/tty still connects to the user's keyboard.
HAVE_TTY=false
[[ -r /dev/tty ]] && HAVE_TTY=true

if [[ -z "${GDASH_NONINTERACTIVE:-}" ]] && $HAVE_TTY && command -v whiptail &>/dev/null; then
  USE_TUI=true
elif [[ -z "${GDASH_NONINTERACTIVE:-}" ]] && $HAVE_TTY && command -v dialog &>/dev/null; then
  # use dialog as whiptail-compatible replacement
  whiptail() { dialog "$@"; }
  USE_TUI=true
fi

# ─── Whiptail helpers ─────────────────────────────────────────────────────────
# wt_input VAR_NAME TITLE PROMPT DEFAULT
wt_input() {
  local _var="$1" _title="$2" _prompt="$3" _default="$4"
  local _result
  _result=$(whiptail --title "$_title" \
    --inputbox "$_prompt" 10 70 "$_default" 3>&1 1>&2 2>&3) || true
  # If user hit Cancel, keep the default
  printf -v "$_var" '%s' "${_result:-$_default}"
}

# wt_password VAR_NAME TITLE PROMPT
wt_password() {
  local _var="$1" _title="$2" _prompt="$3"
  local _result _confirm _err
  while true; do
    _result=$(whiptail --title "$_title" \
      --passwordbox "$_prompt" 10 60 "" 3>&1 1>&2 2>&3) || true
    [[ -z "$_result" ]] && break  # empty = keep auto-generated
    # Complexity check
    _err=$(validate_password "$_result" 2>&1) || {
      whiptail --title "$_title" --msgbox "Weak password:\n$_err\n\nRequirements: ≥12 chars, uppercase, lowercase, digit, special character." 12 60
      continue
    }
    _confirm=$(whiptail --title "$_title" \
      --passwordbox "Confirm password:" 10 60 "" 3>&1 1>&2 2>&3) || true
    if [[ "$_result" == "$_confirm" ]]; then
      break
    fi
    whiptail --title "$_title" --msgbox "Passwords do not match. Please try again." 8 50
  done
  printf -v "$_var" '%s' "${_result}"
}

# wt_yesno TITLE PROMPT  →  returns 0 for yes, 1 for no
wt_yesno() {
  whiptail --title "$1" --yesno "$2" 10 60 3>&1 1>&2 2>&3
}

# ─── Readline fallback helpers ────────────────────────────────────────────────
# All reads use /dev/tty so they work even when stdin is a pipe (curl | bash).
rl_input() {
  local _var="$1" _prompt="$2" _default="$3"
  local _result=""
  IFS= read -r -p "  $_prompt [${_default}]: " _result </dev/tty 2>/dev/null || true
  printf -v "$_var" '%s' "${_result:-$_default}"
}

rl_password() {
  local _var="$1" _prompt="$2"
  local _result="" _confirm="" _err
  while true; do
    IFS= read -r -s -p "  $_prompt (blank = auto-generate): " _result </dev/tty 2>/dev/null || true
    echo
    [[ -z "$_result" ]] && break
    # Complexity check
    if ! _err=$(validate_password "$_result" 2>&1); then
      echo "  $_err"
      echo "  Requirements: ≥12 chars, uppercase, lowercase, digit, special character."
      continue
    fi
    IFS= read -r -s -p "  Confirm password: " _confirm </dev/tty 2>/dev/null || true
    echo
    [[ "$_result" == "$_confirm" ]] && break
    echo "  Passwords do not match. Please try again."
  done
  printf -v "$_var" '%s' "${_result}"
}

# =============================================================================
# Main configuration collection
# =============================================================================

collect_config_tui() {
  local _auto_pass
  _auto_pass=$(gen_password)

  # ── Welcome ────────────────────────────────────────────────────────────────
  whiptail --title "Games Dashboard Installer" \
    --msgbox "\nWelcome to the Games Dashboard installer!\n\nThe next screens will let you configure all settings.\nPress Enter / OK to accept a default, or type a new value.\n\nTip: run with GDASH_NONINTERACTIVE=1 to skip this wizard." \
    16 68

  # ── Page 1: Network ────────────────────────────────────────────────────────
  wt_input INSTALL_DIR \
    "Network & Paths (1/4)" \
    "Install directory (daemon, UI, certs, data will all live here):" \
    "/opt/gdash"

  wt_input SERVER_IP \
    "Network & Paths (1/4)" \
    "Server IP address (used for TLS SAN and API URL):" \
    "$DETECTED_IP"

  wt_input SERVER_HOSTNAME \
    "Network & Paths (1/4)" \
    "Optional hostname / FQDN for TLS (e.g. dashboard.example.com)\nLeave blank to use the IP address only:" \
    ""

  wt_input DAEMON_PORT \
    "Network & Paths (1/4)" \
    "Daemon port (daemon listens here; nginx proxies to it internally):" \
    "8443"

  wt_input UI_PORT \
    "Network & Paths (1/4)" \
    "HTTPS port that nginx will serve the dashboard on:" \
    "443"

  # ── Page 2: Admin credentials ──────────────────────────────────────────────
  wt_input ADMIN_USER \
    "Admin Account (2/4)" \
    "Admin username:" \
    "admin"

  # Show the auto-generated password; let user replace it or leave blank to keep it
  whiptail --title "Admin Account (2/4)" \
    --msgbox "An auto-generated password has been prepared:\n\n  $_auto_pass\n\nOn the next screen you may type your own password,\nor leave it blank to use the one shown above." \
    12 62

  local _custom_pass=""
  wt_password _custom_pass "Admin Account (2/4)" "New admin password (blank = keep auto-generated):"
  ADMIN_PASS="${_custom_pass:-$_auto_pass}"

  # ── Page 3: Storage & Backup ───────────────────────────────────────────────
  local _default_data="${INSTALL_DIR:-/opt/gdash}/data"
  wt_input DATA_DIR \
    "Storage & Backup (3/5)" \
    "Data directory (server files, world saves, logs):" \
    "$_default_data"

  wt_input BACKUP_SCHEDULE \
    "Storage & Backup (3/5)" \
    "Default backup schedule (cron syntax):" \
    "0 3 * * *"

  wt_input BACKUP_RETAIN_DAYS \
    "Storage & Backup (3/5)" \
    "Backup retention (days before old backups are deleted):" \
    "30"

  # ── Page 4: Container runtimes ────────────────────────────────────────────
  INSTALL_DOCKER=true   # Docker CE is required — always installed
  INSTALL_K8S=false

  if wt_yesno "Container Runtimes (4/5)" \
      "Install Kubernetes (k3s — lightweight single-node K8s)?\n\nOptional. Only needed if you want to run game servers as\nKubernetes workloads or plan to scale across multiple nodes.\n\nDocker CE will be installed automatically (required).\n\nMost users should say No here."; then
    INSTALL_K8S=true
  fi

  # ── Page 5: Review & Confirm ──────────────────────────────────────────────
  local _hostname_line="${SERVER_HOSTNAME:-  (none — using IP only)}"
  local _k8s_line="No"
  $INSTALL_K8S && _k8s_line="Yes — k3s (lightweight Kubernetes)"
  local _summary
  _summary=$(printf '%s\n' \
    "" \
    "  Install dir   : ${INSTALL_DIR}" \
    "  Data dir      : ${DATA_DIR}" \
    "" \
    "  Server IP     : ${SERVER_IP}" \
    "  Hostname      : ${_hostname_line}" \
    "  Daemon port   : ${DAEMON_PORT}  (internal, nginx proxies to it)" \
    "  HTTPS port    : ${UI_PORT}" \
    "" \
    "  Admin user    : ${ADMIN_USER}" \
    "  Admin pass    : ${ADMIN_PASS}" \
    "" \
    "  Backup cron   : ${BACKUP_SCHEDULE}" \
    "  Retain days   : ${BACKUP_RETAIN_DAYS}" \
    "" \
    "  Docker CE     : Yes (required)" \
    "  Kubernetes    : ${_k8s_line}" \
    "")

  if ! whiptail --title "Review & Confirm (5/5)" \
      --yesno "$_summary\n\nProceed with installation?" \
      32 74; then
    echo ""
    fail "Installation cancelled by user."
  fi
}

collect_config_readline() {
  local _auto_pass
  _auto_pass=$(gen_password)

  echo ""
  echo -e "${BOLD}=================================================="
  echo -e "  Games Dashboard -- Configuration"
  echo -e "==================================================${NC}"
  echo -e "  Press Enter to accept each default shown in [brackets]."
  echo ""

  echo -e "  ${BOLD}-- Network & Paths ------------------------------${NC}"
  rl_input INSTALL_DIR   "Install directory"    "/opt/gdash"
  rl_input SERVER_IP     "Server IP address"    "$DETECTED_IP"
  rl_input SERVER_HOSTNAME "Hostname / FQDN (blank = use IP only)" ""
  rl_input DAEMON_PORT   "Daemon port"          "8443"
  rl_input UI_PORT       "HTTPS (nginx) port"   "443"

  echo ""
  echo -e "  ${BOLD}-- Admin Account --------------------------------${NC}"
  rl_input ADMIN_USER "Admin username" "admin"
  echo -e "  Auto-generated password: ${BOLD}${_auto_pass}${NC}"
  rl_password _custom_pass "Admin password"
  ADMIN_PASS="${_custom_pass:-$_auto_pass}"

  echo ""
  echo -e "  ${BOLD}-- Storage & Backup -----------------------------${NC}"
  local _default_data="${INSTALL_DIR}/data"
  rl_input DATA_DIR          "Data directory"          "$_default_data"
  rl_input BACKUP_SCHEDULE   "Backup cron schedule"    "0 3 * * *"
  rl_input BACKUP_RETAIN_DAYS "Backup retention (days)" "30"

  echo ""
  echo -e "  ${BOLD}-- Container Runtimes ---------------------------${NC}"
  echo -e "  Docker CE is required and will be installed automatically."
  INSTALL_DOCKER=true   # always
  INSTALL_K8S=false
  local _k8s_yn=""
  IFS= read -r -p "  Install Kubernetes / k3s? [y/N]: " _k8s_yn </dev/tty 2>/dev/null || true
  case "${_k8s_yn,,}" in
    y|yes) INSTALL_K8S=true ;;
  esac

  echo ""
  echo -e "  ${BOLD}-- Summary --------------------------------------${NC}"
  echo -e "  Install dir   : ${INSTALL_DIR}"
  echo -e "  Data dir      : ${DATA_DIR}"
  echo -e "  Server IP     : ${SERVER_IP}"
  echo -e "  Hostname      : ${SERVER_HOSTNAME:-(none - using IP only)}"
  echo -e "  Daemon port   : ${DAEMON_PORT}"
  echo -e "  HTTPS port    : ${UI_PORT}"
  echo -e "  Admin user    : ${ADMIN_USER}"
  echo -e "  Admin pass    : ${ADMIN_PASS}"
  echo -e "  Backup cron   : ${BACKUP_SCHEDULE}"
  echo -e "  Retain days   : ${BACKUP_RETAIN_DAYS}"
  echo -e "  Docker CE     : Yes (required)"
  echo -e "  Kubernetes    : $($INSTALL_K8S && echo 'Yes (k3s)' || echo 'No')"
  echo ""
  local _confirm=""
  IFS= read -r -p "  Proceed with installation? [Y/n]: " _confirm </dev/tty 2>/dev/null || true
  case "${_confirm,,}" in
    n|no) fail "Installation cancelled by user." ;;
  esac
}

collect_config_noninteractive() {
  INSTALL_DIR="${GDASH_INSTALL_DIR:-/opt/gdash}"
  SERVER_IP="${GDASH_HOST:-$DETECTED_IP}"
  SERVER_HOSTNAME="${GDASH_HOSTNAME:-}"
  DAEMON_PORT="${GDASH_DAEMON_PORT:-8443}"
  UI_PORT="${GDASH_UI_PORT:-443}"
  ADMIN_USER="${GDASH_ADMIN_USER:-admin}"
  ADMIN_PASS="${GDASH_ADMIN_PASS:-$(gen_password)}"
  DATA_DIR="${GDASH_DATA_DIR:-${INSTALL_DIR}/data}"
  BACKUP_SCHEDULE="${GDASH_BACKUP_SCHEDULE:-0 3 * * *}"
  BACKUP_RETAIN_DAYS="${GDASH_BACKUP_RETAIN_DAYS:-30}"
  INSTALL_DOCKER="${GDASH_INSTALL_DOCKER:-true}"
  INSTALL_K8S="${GDASH_INSTALL_K8S:-false}"

  echo ""
  echo -e "${BOLD}  Non-interactive mode — using defaults / environment overrides${NC}"
  echo -e "  Set GDASH_INSTALL_DIR, GDASH_HOST, GDASH_HOSTNAME, GDASH_ADMIN_PASS, etc. to customise."
  echo ""
}

# ── Node-mode config collection (minimal — no UI/nginx/admin) ─────────────────
collect_config_node() {
  INSTALL_DIR="${GDASH_INSTALL_DIR:-/opt/gdash}"
  SERVER_IP="${GDASH_HOST:-$DETECTED_IP}"
  SERVER_HOSTNAME="${GDASH_HOSTNAME:-}"
  DAEMON_PORT="${GDASH_DAEMON_PORT:-8443}"
  UI_PORT="443"           # unused in node mode but keeps derived-values section happy
  ADMIN_USER="admin"      # unused
  ADMIN_PASS=""           # unused
  BACKUP_SCHEDULE="0 3 * * *"   # unused
  BACKUP_RETAIN_DAYS="30"       # unused
  DATA_DIR="${GDASH_DATA_DIR:-${INSTALL_DIR}/data}"
  INSTALL_DOCKER=true
  INSTALL_K8S=false

  if $HAVE_TTY && [[ -z "${GDASH_NONINTERACTIVE:-}" ]]; then
    echo ""
    echo -e "${BOLD}=================================================="
    echo -e "  Games Dashboard — Node-Mode Configuration"
    echo -e "==================================================${NC}"
    echo -e "  This machine will be a ${BOLD}worker node${NC} (no UI, no nginx)."
    echo -e "  Press Enter to accept defaults."
    echo ""
    rl_input INSTALL_DIR  "Install directory"   "/opt/gdash"
    rl_input SERVER_IP    "This node's IP"      "$DETECTED_IP"
    rl_input DAEMON_PORT  "Daemon port"         "8443"
    echo ""
    local _confirm=""
    IFS= read -r -p "  Proceed? [Y/n]: " _confirm </dev/tty 2>/dev/null || true
    case "${_confirm,,}" in
      n|no) fail "Installation cancelled by user." ;;
    esac
  else
    echo ""
    echo -e "${BOLD}  Node mode — using defaults / GDASH_* environment overrides${NC}"
    echo ""
  fi
}

# ── Run config collection ─────────────────────────────────────────────────────
if [[ "$INSTALL_MODE" == "node" ]]; then
  collect_config_node
elif [[ -n "${GDASH_NONINTERACTIVE:-}" ]]; then
  collect_config_noninteractive
elif $USE_TUI; then
  collect_config_tui
elif $HAVE_TTY; then
  collect_config_readline
else
  # No terminal available (e.g. curl | bash without a controlling tty).
  # Fall back to non-interactive defaults and let the user know.
  warn "No interactive terminal detected. Using defaults."
  warn "Re-run directly in a terminal for the interactive setup wizard, or set"
  warn "GDASH_NONINTERACTIVE=1 and GDASH_* env vars to customise (see README)."
  collect_config_noninteractive
fi

# ── Derived values ─────────────────────────────────────────────────────────────
REPO_URL="https://github.com/Chrisl154/Gmaer-Server-Daashboard.git"

if [[ -n "$SERVER_HOSTNAME" ]]; then
  TLS_CN="$SERVER_HOSTNAME"
  TLS_SAN="IP:${SERVER_IP},DNS:${SERVER_HOSTNAME},DNS:localhost,IP:127.0.0.1"
  UI_API_URL="https://${SERVER_HOSTNAME}"
  log "Using hostname: $SERVER_HOSTNAME (IP: $SERVER_IP)"
else
  TLS_CN="$SERVER_IP"
  TLS_SAN="IP:${SERVER_IP},DNS:localhost,IP:127.0.0.1"
  UI_API_URL="https://${SERVER_IP}"
  log "Using IP: $SERVER_IP (no hostname)"
fi

DAEMON_URL="https://${SERVER_IP}:${DAEMON_PORT}"  # internal only

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

for pkg in git openssl python3 curl nginx lsof lib32gcc-s1; do
  if ! command -v "$pkg" &>/dev/null && ! dpkg -l "$pkg" 2>/dev/null | grep -q "^ii"; then
    log "Installing $pkg..."
    pkg_install "$pkg"
  fi
done
ok "System packages ready"

# ── SteamCMD ──────────────────────────────────────────────────────────────────
if ! command -v steamcmd &>/dev/null; then
  log "Installing SteamCMD..."
  $SUDO dpkg --add-architecture i386 >/dev/null 2>&1
  $SUDO apt-get update -qq >/dev/null 2>&1
  echo steam steam/question select "I AGREE" | $SUDO debconf-set-selections 2>/dev/null || true
  echo steam steam/license note ''           | $SUDO debconf-set-selections 2>/dev/null || true
  DEBIAN_FRONTEND=noninteractive $SUDO apt-get install -y -qq steamcmd >/dev/null 2>&1 || {
    warn "apt steamcmd failed — installing manually to /usr/local/bin/steamcmd"
    mkdir -p /tmp/steamcmd-setup
    download "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz" /tmp/steamcmd-setup/steamcmd.tar.gz
    tar -xzf /tmp/steamcmd-setup/steamcmd.tar.gz -C /tmp/steamcmd-setup/
    $SUDO mv /tmp/steamcmd-setup/steamcmd.sh /usr/local/bin/steamcmd
    $SUDO chmod +x /usr/local/bin/steamcmd
    rm -rf /tmp/steamcmd-setup
  }
  ok "SteamCMD installed"
else
  ok "SteamCMD: $(command -v steamcmd)"
fi

# ── Docker CE ─────────────────────────────────────────────────────────────────
if [[ "${INSTALL_DOCKER}" == "true" ]]; then
  if command -v docker &>/dev/null; then
    ok "Docker: $(docker --version 2>/dev/null | head -1)"
  else
    log "Installing Docker CE..."
    # Official Docker install script (handles all Ubuntu/Debian variants)
    download "https://get.docker.com" /tmp/get-docker.sh
    $SUDO sh /tmp/get-docker.sh >/dev/null 2>&1 || {
      # Fallback: manual apt-based install
      pkg_install apt-transport-https ca-certificates gnupg lsb-release
      download "https://download.docker.com/linux/ubuntu/gpg" /tmp/docker.gpg
      $SUDO mkdir -p /etc/apt/keyrings
      cat /tmp/docker.gpg | $SUDO gpg --dearmor -o /etc/apt/keyrings/docker.gpg 2>/dev/null
      echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
        | $SUDO tee /etc/apt/sources.list.d/docker.list >/dev/null
      $SUDO apt-get update -qq >/dev/null 2>&1
      pkg_install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    }
    rm -f /tmp/get-docker.sh /tmp/docker.gpg
    $SUDO systemctl enable docker >/dev/null 2>&1
    $SUDO systemctl start docker >/dev/null 2>&1
    # Add current user to docker group so the daemon can run containers
    $SUDO usermod -aG docker "$USER" 2>/dev/null || true
    ok "Docker CE installed ($(docker --version 2>/dev/null | head -1))"
    info "You may need to log out and back in for the docker group to take effect."
  fi
fi

# ── k3s (lightweight Kubernetes) ──────────────────────────────────────────────
if [[ "${INSTALL_K8S}" == "true" ]]; then
  if command -v k3s &>/dev/null; then
    ok "k3s: $(k3s --version 2>/dev/null | head -1)"
  else
    log "Installing k3s (lightweight Kubernetes)..."
    download "https://get.k3s.io" /tmp/k3s-install.sh
    INSTALL_K3S_EXEC="server --disable traefik" sh /tmp/k3s-install.sh >/dev/null 2>&1 || \
      warn "k3s install encountered an error — check logs with: journalctl -u k3s"
    rm -f /tmp/k3s-install.sh
    command -v k3s &>/dev/null && ok "k3s installed: $(k3s --version 2>/dev/null | head -1)" \
      || warn "k3s not found after install"
  fi
fi

# ── Java (required for Minecraft and other JVM-based game servers) ────────────
if command -v java &>/dev/null && java -version 2>&1 | grep -qE "version \"(17|1[89]|2[0-9])"; then
  ok "Java: $(java -version 2>&1 | head -1)"
else
  log "Installing Java 21 LTS (required for Minecraft and other game servers)..."
  pkg_install openjdk-21-jre-headless 2>/dev/null || pkg_install openjdk-17-jre-headless 2>/dev/null || \
    warn "Could not install Java via apt — Minecraft servers will need Java installed manually."
  command -v java &>/dev/null && ok "Java: $(java -version 2>&1 | head -1)" || warn "Java not available"
fi

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

# ── Node.js (not needed in worker-node mode) ──────────────────────────────────
NODE_BIN=""
if [[ "$INSTALL_MODE" != "node" ]]; then
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
else
  ok "Node.js: skipped (worker-node mode)"
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

$SUDO mkdir -p "$INSTALL_DIR" "$INSTALL_DIR/logs"
$SUDO chown "$USER":"$USER" "$INSTALL_DIR" "$INSTALL_DIR/logs"

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

if [[ "$INSTALL_MODE" != "node" ]]; then
  log "Building UI (VITE_DAEMON_URL=$UI_API_URL)..."
  chmod +x "$REPO_DIR/ui/node_modules/.bin/"* 2>/dev/null || true
  (cd "$REPO_DIR/ui" && \
    npm install --silent 2>/dev/null && \
    VITE_DAEMON_URL="$UI_API_URL" node_modules/.bin/vite build --outDir "$INSTALL_DIR/ui" 2>/dev/null)
  ok "UI → $INSTALL_DIR/ui"
else
  ok "UI build: skipped (worker-node mode)"
fi

# =============================================================================
section "Step 4: TLS Certificate"
# =============================================================================

TLS_DIR="$INSTALL_DIR/tls"
mkdir -p "$TLS_DIR"

if [[ ! -f "$TLS_DIR/server.crt" ]]; then
  openssl req -x509 -newkey rsa:2048 \
    -keyout "$TLS_DIR/server.key" -out "$TLS_DIR/server.crt" \
    -days 3650 -nodes \
    -subj "/CN=${TLS_CN}" \
    -addext "subjectAltName=${TLS_SAN}" \
    2>/dev/null
  ok "TLS cert generated (10 years, CN=${TLS_CN}, SAN: ${TLS_SAN})"
else
  ok "TLS cert already exists — delete $TLS_DIR/server.crt to regenerate"
fi

# =============================================================================
section "Step 5: Daemon Configuration"
# =============================================================================

CFG_DIR="$INSTALL_DIR/config"
SECRETS_DIR="$INSTALL_DIR/secrets"
mkdir -p "$CFG_DIR" "$DATA_DIR" "$SECRETS_DIR"

# Generate JWT secret
JWT_SECRET=$(python3 -c "import secrets; print(secrets.token_hex(32))")

if [[ "$INSTALL_MODE" == "node" ]]; then
  # Worker node — binds on all interfaces so the master can reach it;
  # no local auth (nodes are trusted via the join token on the master).
  cat > "$CFG_DIR/daemon.yaml" <<YAML
bind_addr: "0.0.0.0:${DAEMON_PORT}"
log_level: "info"
data_dir: "${DATA_DIR}"
shutdown_timeout: 30s
tls:
  cert_file: "${TLS_DIR}/server.crt"
  key_file: "${TLS_DIR}/server.key"
auth:
  local:
    enabled: false
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
metrics:
  enabled: true
  path: "/metrics"
cluster:
  enabled: false
YAML
else
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
  default_schedule: "${BACKUP_SCHEDULE}"
  retain_days: ${BACKUP_RETAIN_DAYS}
  compression: "gzip"
metrics:
  enabled: true
  path: "/metrics"
cluster:
  enabled: false
YAML
fi
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

if [[ "$INSTALL_MODE" == "node" ]]; then
  ok "nginx: skipped (worker-node mode — daemon is directly accessible on port ${DAEMON_PORT})"
else

$SUDO tee /etc/nginx/sites-available/gdash > /dev/null <<NGINX
# Games Dashboard
server {
    listen 80;
    server_name _;
    return 301 https://\$host\$request_uri;
}

server {
    listen ${UI_PORT} ssl;
    server_name ${SERVER_IP}${SERVER_HOSTNAME:+ $SERVER_HOSTNAME} _;

    ssl_certificate     ${TLS_DIR}/server.crt;
    ssl_certificate_key ${TLS_DIR}/server.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    root ${INSTALL_DIR}/ui;
    index index.html;

    location / {
        try_files \$uri \$uri/ /index.html;
    }

    location /api/ {
        proxy_pass https://127.0.0.1:${DAEMON_PORT};
        proxy_ssl_verify       off;
        proxy_ssl_protocols    TLSv1.3;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 120s;

        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    location ~ ^/(healthz|metrics)$ {
        proxy_pass          https://127.0.0.1:${DAEMON_PORT};
        proxy_ssl_verify    off;
        proxy_ssl_protocols TLSv1.3;
    }

    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
}
NGINX

$SUDO ln -sf /etc/nginx/sites-available/gdash /etc/nginx/sites-enabled/gdash
$SUDO rm -f /etc/nginx/sites-enabled/default

if $SUDO nginx -t 2>/dev/null; then
  $SUDO systemctl enable nginx 2>/dev/null
  $SUDO systemctl restart nginx
  ok "nginx configured and running"
else
  $SUDO nginx -t
  fail "nginx config test failed."
fi

fi  # end of [[ "$INSTALL_MODE" != "node" ]] block for nginx

# =============================================================================
section "Step 8: Bootstrap Admin Account"
# =============================================================================

if [[ "$INSTALL_MODE" == "node" ]]; then
  ok "Admin bootstrap: skipped (worker-node mode)"
else

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

BOOT_RESP=$(python3 - <<PYEOF
import urllib.request, urllib.error, ssl, json
ctx = ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
data = json.dumps({"username": "${ADMIN_USER}", "password": "${ADMIN_PASS}"}).encode()
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
  ok "Admin account created (username: ${ADMIN_USER})"
else
  info "Bootstrap response: $BOOT_RESP"
  info "(If 'already initialized', the existing credentials remain valid)"
fi

fi  # end of [[ "$INSTALL_MODE" != "node" ]] block for admin bootstrap

# =============================================================================
section "Step 9: Self-Update Script"
# =============================================================================

# Write the update script that the dashboard and gdash CLI invoke.
# It pulls the latest code, rebuilds the daemon + UI, and restarts the service.
cat > "${BIN_DIR}/gdash-update.sh" <<'UPDATESCRIPT'
#!/bin/bash
# gdash-update.sh — called by the Games Dashboard self-update feature.
# Usage: gdash-update.sh [branch]   (default: main)
set -euo pipefail

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

exec >> "$LOG" 2>&1
echo ""
echo "=== $(date '+%Y-%m-%d %H:%M:%S') === gdash self-update to branch: $BRANCH ==="

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
export NVM_DIR="$HOME/.nvm"
[[ -s "$NVM_DIR/nvm.sh" ]] && source "$NVM_DIR/nvm.sh"
if ! command -v node &>/dev/null; then
  echo "ERROR: node not found. Install Node 20 LTS and retry." >&2
  exit 1
fi
echo "Using Node: $(node --version)"

# ── Pull latest code ─────────────────────────────────────────────────────────
echo "Updating repository (branch: $BRANCH)..."
git -C "$REPO_DIR" fetch --prune origin
git -C "$REPO_DIR" checkout "$BRANCH"
git -C "$REPO_DIR" reset --hard "origin/$BRANCH"
echo "Repository updated to: $(git -C "$REPO_DIR" rev-parse --short HEAD)"

# ── Rebuild daemon ───────────────────────────────────────────────────────────
echo "Building daemon..."
$GO_BIN build -o "${BIN_DIR}/games-daemon.new" "${REPO_DIR}/daemon/cmd/daemon"
mv "${BIN_DIR}/games-daemon.new" "${BIN_DIR}/games-daemon"
echo "Daemon binary updated."

# ── Rebuild CLI ──────────────────────────────────────────────────────────────
echo "Building CLI..."
$GO_BIN build -o "${BIN_DIR}/gdash.new" "${REPO_DIR}/cli/cmd"
mv "${BIN_DIR}/gdash.new" "${BIN_DIR}/gdash"
echo "CLI binary updated."

# ── Rebuild UI ───────────────────────────────────────────────────────────────
echo "Building UI..."
cd "$UI_SRC"
npm install --silent 2>/dev/null
chmod +x node_modules/.bin/* 2>/dev/null || true
node_modules/.bin/vite build --outDir "${UI_DST}" --emptyOutDir 2>/dev/null
echo "UI rebuilt."

# ── Restart service ──────────────────────────────────────────────────────────
echo "Restarting gdash-daemon..."
sleep 2
sudo systemctl restart gdash-daemon
echo "=== Update complete ==="
UPDATESCRIPT

chmod +x "${BIN_DIR}/gdash-update.sh"
ok "Update script installed at ${BIN_DIR}/gdash-update.sh"

# Sudoers entry: allow the daemon user to restart only the gdash service
SUDOERS_FILE="/etc/sudoers.d/gdash-update"
$SUDO tee "$SUDOERS_FILE" > /dev/null <<SUDOERS
# Allow the gdash daemon user to restart the gdash service (required for self-update)
${USER} ALL=(ALL) NOPASSWD: /bin/systemctl restart gdash-daemon, /usr/bin/systemctl restart gdash-daemon
SUDOERS
$SUDO chmod 440 "$SUDOERS_FILE"
ok "Sudoers entry added for self-update ($SUDOERS_FILE)"

# Allow UFW if present
if command -v ufw &>/dev/null; then
  $SUDO ufw allow 80/tcp          >/dev/null 2>&1 || true
  $SUDO ufw allow "${UI_PORT}/tcp" >/dev/null 2>&1 || true
  $SUDO ufw allow "${DAEMON_PORT}/tcp" >/dev/null 2>&1 || true
  ok "Firewall rules added (80, ${UI_PORT}, ${DAEMON_PORT})"
fi

# =============================================================================
section "Install Complete"
# =============================================================================

if [[ "$INSTALL_MODE" == "node" ]]; then
  # ── Worker node summary ──────────────────────────────────────────────────
  NODE_ADDR="https://${SERVER_IP}:${DAEMON_PORT}"
  NODE_HOST="${SERVER_HOSTNAME:-$(hostname -s 2>/dev/null || echo "$(hostname)")}"

  echo ""
  echo -e "${GREEN}${BOLD}  ╔══════════════════════════════════════════════╗"
  echo -e "  ║     Games Dashboard Node is ready!           ║"
  echo -e "  ╚══════════════════════════════════════════════╝${NC}"
  echo ""
  echo -e "  This machine is now a ${BOLD}worker node${NC}."
  echo -e "  To add it to your master dashboard, run on the ${BOLD}master${NC}:"
  echo ""
  echo -e "  ${CYAN}  # Step 1 — generate a join token (on the master):${NC}"
  echo -e "    gdash node token"
  echo ""
  echo -e "  ${CYAN}  # Step 2 — register this node (on the master):${NC}"
  echo -e "    gdash node add ${NODE_HOST} \\"
  echo -e "      --address ${SERVER_IP}:${DAEMON_PORT} \\"
  echo -e "      --token <token-from-step-1>"
  echo ""
  echo -e "  ${YELLOW}⚠  TLS note:${NC} The certificate is self-signed."
  echo -e "     The master's gdash CLI must be run with --insecure (or trust the cert) to reach this node."
  echo ""
  echo -e "  ${BOLD}Useful commands:${NC}"
  echo -e "    sudo systemctl status gdash-daemon"
  echo -e "    sudo journalctl -u gdash-daemon -f"
  echo -e "    sudo systemctl restart gdash-daemon"
  echo ""
  echo -e "  ${DIM}Config:  $CFG_DIR/daemon.yaml"
  echo -e "  Data:    $DATA_DIR"
  echo -e "  Install: $INSTALL_DIR${NC}"
  echo ""
else
  # ── Full install summary ─────────────────────────────────────────────────
  echo ""
  echo -e "${GREEN}${BOLD}  ╔══════════════════════════════════════════════╗"
  echo -e "  ║       Games Dashboard is ready!              ║"
  echo -e "  ╚══════════════════════════════════════════════╝${NC}"
  echo ""
  echo -e "  ${BOLD}Dashboard URL:${NC}  ${UI_API_URL}  (port ${UI_PORT} via nginx)"
  echo -e "  ${BOLD}Username:${NC}       ${ADMIN_USER}"
  echo -e "  ${BOLD}Password:${NC}       ${ADMIN_PASS}"
  echo ""
  echo -e "  ${YELLOW}⚠  TLS note:${NC} The certificate is self-signed."
  echo -e "     Your browser will show a security warning."
  echo -e "     Click \"Advanced → Proceed\" to continue."
  echo ""
  echo -e "  ${BOLD}Useful commands:${NC}"
  echo -e "    gdash --help                    # CLI tool"
  echo -e "    gdash update                    # Update to latest (main branch)"
  echo -e "    gdash update --branch dev       # Update to dev branch"
  echo -e "    gdash update --check            # Check for updates without applying"
  echo -e "    sudo systemctl status gdash-daemon"
  echo -e "    sudo journalctl -u gdash-daemon -f"
  echo -e "    sudo systemctl restart gdash-daemon"
  echo ""
  echo -e "  ${DIM}Config:    $CFG_DIR/daemon.yaml"
  echo -e "  Data:      $DATA_DIR"
  echo -e "  Logs:      journalctl -u gdash-daemon"
  echo -e "  Install:   $INSTALL_DIR${NC}"
  echo ""
fi
