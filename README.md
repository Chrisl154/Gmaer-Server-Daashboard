# Games Dashboard

[![CI](https://github.com/Chrisl154/Gmaer-Server-Daashboard/actions/workflows/ci.yml/badge.svg)](https://github.com/Chrisl154/Gmaer-Server-Daashboard/actions)
[![License: Proprietary](https://img.shields.io/badge/License-Proprietary-red.svg)](LICENSE)

**Production-grade, open-source Gaming Server Dashboard** for automated provisioning, deployment, management, backup, networking, security, and observability of game servers — supporting Docker single-host and Kubernetes cluster deployments.

## ✨ Features

| Category | Capabilities |
|---|---|
| **Servers** | 24 supported games — visual poster-card grid; start/stop/restart/delete; auto-restart on crash with configurable retries & back-off |
| **Add Server** | 2-step wizard: visual game picker filtered by deploy method → config form; first-login onboarding checklist |
| **Deploy** | SteamCMD, manual archive, Docker (19 games), Kubernetes operator |
| **Live Metrics** | Dashboard resource table: per-server CPU %, RAM %, Disk % bars updated every 15 s; live player count for 13 supported games |
| **Player Count** | Live current/max player count via RCON/WebRCON/Telnet — shown on dashboard table and server detail page; supports Minecraft, Palworld, CS2, TF2, GMod, L4D2, Dota2, Factorio, Squad, ARK, Conan Exiles, Rust, 7 Days to Die |
| **Console** | Live WebSocket console streaming + RCON/Telnet command input per server; plain-English error banners with help modal |
| **Config Editor** | In-UI config file editor — edit `server.properties`, launch scripts, etc. without SSH |
| **File Browser** | In-UI file browser — browse, upload, download, and delete files in the install directory |
| **Share** | "Share with Friends" panel — public IP + port + game join string in one click |
| **Notifications** | Discord, Slack, and custom webhook alerts for server crash, restart, disk warnings, and backup events |
| **Logs** | 4-tab Logs page: live server console tail, lifecycle Events, Security/auth events, full Audit Trail |
| **Backups** | Scheduled/manual, NFS + S3, incremental, integrity-verified restore |
| **Mods** | Steam Workshop, CurseForge, Thunderstore, Git, local; sandboxed test harness; RBAC |
| **Security** | TLS everywhere, AES-256 secrets at rest, TOTP 2FA, OIDC/SAML/OAuth2 |
| **Audit** | Signed audit trail for all operations; filterable by lifecycle events vs. auth events |
| **CVE/SBOM** | CycloneDX SBOM, Trivy/Grype scanning, OSV/NVD queries, evidence model |
| **Networking** | Port mapping UI, reachability probe, UPnP/NAT, UFW firewall management (add/delete rules, enable/disable, live status) |
| **Observability** | Prometheus metrics, Grafana dashboards, structured JSON logs |
| **Scale** | Docker Compose (single-host) or Kubernetes/k3s via Helm + CRD operator |
| **Self-Update** | One-click update from the UI (Settings → Updates) or `gdash update`; choose `main` (stable) or `dev` branch |

---

## 🚀 Quick Start

**Minimum requirement:** Ubuntu 22.04 or 24.04 with internet access and `bash`. Everything else — Go 1.22, Node.js 20 LTS, nginx, SteamCMD, Java, Python packages — is installed automatically.

### Install (Deploy & Run)

Deploys the full stack and leaves it running. Prints the dashboard URL and credentials when done.

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/install.sh | bash
```

**Want to skip the setup wizard entirely?** Pass your password and you're done — everything else uses sensible defaults:

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/install.sh | GDASH_NONINTERACTIVE=1 GDASH_ADMIN_PASS=MySecurePass123 bash
```

**Adding a worker node to a cluster?** Install in node-only mode — daemon only, no UI or nginx:

```bash
bash install.sh --mode=node
```

The installer will print a `gdash node add` command to run on the master machine to register the worker.

After install, open `https://<your-server-ip>` in a browser. Your browser will show a self-signed certificate warning — click **Advanced → Proceed**.

> **What it installs:**
> - Daemon as a systemd service (`gdash-daemon`) on `127.0.0.1:8443`
> - nginx reverse proxy on port 443 serving the UI and proxying `/api/*` to the daemon
> - `gdash` CLI available system-wide at `/usr/local/bin/gdash`
> - All files under `/opt/gdash/`
> - **Docker CE** (required — used for SteamCMD installs and Docker-native game servers)
> - Java 21 LTS (required for Minecraft and JVM-based servers)
> - k3s Kubernetes if selected during setup

#### Installer Modes

| Mode | Command | Description |
|---|---|---|
| **Full stack** (default) | `bash install.sh` | Daemon + UI + nginx + all dependencies |
| **Docker Compose** | `bash install.sh --mode=docker` | Run everything inside Docker containers |
| **Kubernetes** | `bash install.sh --mode=k8s` | Deploy via Helm + k3s |
| **Node only** | `bash install.sh --mode=node` | Daemon only (no UI/nginx) — for worker nodes in a cluster |

#### Interactive Setup Wizard

When run in a terminal the installer launches a full TUI wizard with five screens:

1. **Network & Paths** — install directory, server IP, optional hostname, daemon port, HTTPS port
2. **Admin Account** — username and password (auto-generates a secure password you can keep or replace)
3. **Storage & Backup** — data directory, backup cron schedule, retention days
4. **Container Runtimes** — Docker CE (required; installed automatically) and optional k3s
5. **Review & Confirm** — summary of all settings before anything is written to disk

> **Tip:** Requires `whiptail` for the full TUI (pre-installed on Ubuntu). Falls back to plain readline prompts if unavailable.

#### Non-Interactive / CI Install

Skip the wizard entirely using environment variables:

```bash
GDASH_NONINTERACTIVE=1 \
GDASH_ADMIN_PASS=MySecurePass123 \
GDASH_INSTALL_DOCKER=true \
  bash install.sh
```

| Variable | Default | Description |
|---|---|---|
| `GDASH_NONINTERACTIVE` | — | Set to `1` to skip all prompts |
| `GDASH_INSTALL_DIR` | `/opt/gdash` | Root install directory |
| `GDASH_HOST` | auto-detected | Server IP for TLS and URLs |
| `GDASH_HOSTNAME` | — | Optional FQDN for TLS SAN |
| `GDASH_DAEMON_PORT` | `8443` | Internal daemon port |
| `GDASH_UI_PORT` | `443` | Public HTTPS port |
| `GDASH_ADMIN_USER` | `admin` | Admin username |
| `GDASH_ADMIN_PASS` | auto-generated | Admin password |
| `GDASH_DATA_DIR` | `{INSTALL_DIR}/data` | Runtime data directory |
| `GDASH_BACKUP_SCHEDULE` | `0 3 * * *` | Default backup cron schedule |
| `GDASH_BACKUP_RETAIN_DAYS` | `30` | Days to keep old backups |
| `GDASH_INSTALL_DOCKER` | `true` | Install Docker CE (required; set `false` only if Docker is already installed) |
| `GDASH_INSTALL_K8S` | `false` | Install k3s Kubernetes (`true`/`false`) |
| `GDASH_MODE` | `docker` | Install mode: `docker`, `k8s`, or `node` |
| `GDASH_SKIP_UFW` | `false` | Set `true` to skip UFW firewall configuration |
| `GDASH_UFW_EXTRA_SSH_SUBNETS` | — | Comma-separated extra CIDRs to allow SSH from (e.g. `10.0.0.0/8,172.16.0.0/12`) |

---

### Uninstall (Complete Removal)

Removes everything — systemd service, nginx config, `/opt/gdash/`, and the `gdash` CLI symlink.

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/uninstall.sh | bash
```

After uninstalling, re-run the install command above for a clean reinstall.

---

### Test Only (No Permanent Install)

Validates the full stack end-to-end — builds, starts the daemon temporarily, runs all API and CLI tests, then tears everything down cleanly. Nothing is left running.

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/test-live.sh | bash
```

---

## 📋 What Gets Installed

| Path | Description |
|---|---|
| `/opt/gdash/repo/` | Cloned source repository |
| `/opt/gdash/bin/games-daemon` | Compiled daemon binary |
| `/opt/gdash/bin/gdash` | Compiled CLI binary |
| `/opt/gdash/ui/` | Built React UI static files |
| `/opt/gdash/config/daemon.yaml` | Daemon configuration |
| `/opt/gdash/tls/` | Self-signed TLS certificate (10 years) |
| `/opt/gdash/data/` | Runtime data (servers, backups, etc.) |
| `/opt/gdash/secrets/` | Encrypted secrets store |
| `/opt/gdash/logs/gdash-update.log` | Self-update script output (viewable in Settings → Updates) |
| `/opt/gdash/bin/gdash-update.sh` | Self-update script (called by daemon on update) |
| `/etc/systemd/system/gdash-daemon.service` | Systemd unit |
| `/etc/nginx/sites-available/gdash` | nginx site config |
| `/usr/local/bin/gdash` | CLI symlink |

---

## 🎮 Supported Games & Adapters (24)

Deploy methods: **S** = SteamCMD · **M** = Manual · **D** = Docker · **C** = Custom

| Game | Steam App ID | Deploy | Console | Mods |
|---|---|---|---|---|
| 7 Days to Die | 294420 | S, D | Telnet | Yes |
| Among Us (Impostor) | 945360 | M, C, D | stdio | Yes |
| ARK Survival Ascended | 2430930 | S, D | RCON | Yes |
| Conan Exiles | 443030 | S, D | RCON | Yes |
| Counter-Strike 2 | 730 | S, D | RCON | Yes |
| DayZ | 223350 | S, D | stdio | Yes |
| Don't Starve Together | 343050 | S, D | stdio | Yes |
| Dota 2 | 570 | S, D | RCON | Yes |
| Eco | 382310 | S, M | WebSocket | Yes |
| Enshrouded | 2278520 | S, M | stdio | — |
| Factorio | 427520 | M, S, D | RCON | Yes |
| Garry's Mod | 4020 | S, D | RCON | Yes |
| Left 4 Dead 2 | 222860 | S, D | RCON | Yes |
| Minecraft Java | — | M, C, D | RCON | Yes |
| Palworld | 2394010 | S, M | RCON | — |
| Project Zomboid | 380870 | S, D | stdio | Yes |
| Risk of Rain 2 | 1180760 | S, D | stdio | Yes |
| Rust | 252490 | S, D | WebRCON | Yes |
| Satisfactory | 1690800 | S, M | stdio | Yes |
| Squad | 403240 | S, D | RCON | Yes |
| Team Fortress 2 | 232250 | S, D | RCON | Yes |
| Terraria | — | M, D | stdio | Yes |
| The Riftbreaker | — | M, C | stdio | Yes |
| Valheim | 896660 | S, M, D | stdio | Yes (Thunderstore) |

> **Docker:** 19 of 24 games have pre-configured Docker images. Install Docker CE during setup (or set `GDASH_INSTALL_DOCKER=true`) to enable the Docker deploy method.

---

## 🏗 Architecture

```
┌─────────────────────────────────────┐
│           Web UI (React/TS)         │  HTTPS :443
└────────────────┬────────────────────┘
                 │ REST + WebSocket
┌────────────────▼────────────────────┐
│        Daemon (Go binary)           │  HTTPS :8443
│  Auth · Broker · Adapters · Mods    │
│  Backup · Networking · SBOM/CVE     │
└──┬──────────┬──────────┬────────────┘
   │          │          │
Docker    Kubernetes  SteamCMD
Compose    (k3s/k8s)
   │          │
Game      Game
Containers Pods (CRD)
```

### Components

| Component | Language | Purpose |
|---|---|---|
| `daemon/` | Go 1.22 | REST + WebSocket API server on `:8443` (TLS) |
| `ui/` | React 18 + TypeScript + Vite | Web dashboard |
| `cli/` | Go 1.22 | `gdash` command-line tool |
| `adapters/` | YAML | Game server manifest definitions (24 games) |
| `installer/` | Bash | Single-artifact interactive installer |
| `helm/` | Helm 3 | Kubernetes charts + GameInstance CRD |
| `tests/` | Bash + Go + Vitest | Unit, integration, E2E test suites |

---

## 📁 Repository Layout

```
games-dashboard/
├── daemon/           Go daemon binary + OpenAPI
├── ui/               React + TypeScript web UI
├── cli/              gdash CLI binary
├── adapters/         Game adapter manifests (24 games)
├── helm/             Helm charts + CRDs
├── docs/             Operator runbook, API reference, security docs
├── tests/            Unit, integration, E2E test suites
├── install.sh        One-liner production installer
├── uninstall.sh      One-liner complete uninstaller
├── test-live.sh      One-liner end-to-end test runner (no permanent install)
└── .github/          GitHub Actions workflows
```

---

## 🔒 Security

- TLS 1.3 everywhere (auto self-signed or provided certificates)
- AES-256-GCM secrets encryption at rest (local KMS or HashiCorp Vault)
- JWT authentication with TOTP 2FA, OIDC, SAML, OAuth2
- RBAC: `admin`, `operator`, `modder`, `viewer` roles
- Signed audit trail for all operations
- Signed release artifacts via cosign / Sigstore
- UFW firewall auto-configured by installer; SSH restricted to local subnets only

See [SECURITY.md](docs/SECURITY.md) for the threat model and vulnerability disclosure.

---

## 🔥 Firewall Management

The installer automatically configures UFW on the host machine:

- **Incoming traffic is denied by default**
- **SSH (port 22)** is allowed only from the subnets the machine is attached to (auto-detected at install time). This keeps SSH off the public internet by default.
- **Daemon API (port 8443)** is open for the dashboard
- **HTTP/HTTPS (80/443)** are opened only in `docker` and `k8s` install modes

After install, firewall rules can be managed directly from the **Ports** page in the dashboard — no SSH required:

| Action | How |
|---|---|
| View all UFW rules | Ports page → Firewall Rules panel |
| Add a rule (open a port) | Enter port, protocol (TCP/UDP), optional source CIDR, and a label |
| Delete a rule | Click the trash icon on any row |
| Enable / disable UFW | Toggle at the top of the Firewall Rules panel |

The panel polls the daemon every 15 seconds and stays live. IPv6 mirror rules are hidden by default (they're managed automatically alongside their IPv4 counterpart).

> **Note:** The `ufw` binary must be installed on the host. If UFW is not present, the panel shows an "unavailable" state instead of erroring.

### Cluster Node Join Tokens

When cluster mode is enabled, new worker nodes must present a **join token** to register. Tokens are single-use and expire after 24 hours.

```bash
# On the master — generate a join token
gdash node token

# Output:
#   Token: a3f9...
#   Expires: 24h
#   Run on the new node:
#     gdash node add <hostname> https://<address>:8443 --token a3f9...
```

Tokens survive daemon restarts and are stored in `{data_dir}/nodes.json` alongside registered node state.

---

## 📊 Observability

- Prometheus metrics at `/metrics`
- Grafana dashboard templates in `docs/grafana/`
- Structured JSON logs with configurable retention
- Per-server health endpoints at `/healthz`

---

## 🛠 Development

### Requirements

| Tool | Version | Install |
|---|---|---|
| Go | 1.22+ | `test-live.sh` installs automatically, or [go.dev](https://go.dev/dl) |
| Node.js | 20 LTS | `test-live.sh` installs via NVM automatically |
| Python 3 | 3.8+ | Ships with Ubuntu 24.04 |
| openssl | any | Ships with Ubuntu 24.04 |

### Build from Source

```bash
git clone https://github.com/Chrisl154/Gmaer-Server-Daashboard.git
cd Gmaer-Server-Daashboard

# Daemon
cd daemon && go build -o /tmp/games-daemon ./cmd/daemon

# CLI
cd cli && go build -o /tmp/gdash ./cmd

# UI (production build)
cd ui && npm install && npm run build

# UI (dev server with hot-reload)
cd ui && npm install && npm run dev
```

### Run Tests

```bash
# Full automated test run (recommended) — installs deps, builds, tests
bash test-live.sh

# Daemon unit tests only
cd daemon && go test ./...

# UI tests only
cd ui && npm test

# Integration suite (requires daemon running on :8443)
GDASH_DAEMON_URL=https://localhost:8443 \
GDASH_ADMIN_PASSWORD=changeme \
  bash tests/integration/run-tests.sh

# CLI E2E smoke tests (requires daemon + gdash binary)
GDASH_DAEMON=https://localhost:8443 \
GDASH_ADMIN_PASSWORD=changeme \
GDASH_BIN=/tmp/gdash \
  bash tests/e2e/cli-smoke.sh
```

### Test Suite Coverage

| Suite | Tests | Status |
|---|---|---|
| Go daemon unit tests | 5 packages | ✅ PASS |
| UI Vitest | 158 tests, 20 files | ✅ PASS |
| UI production build | — | ✅ Clean |
| CLI smoke tests | all commands | ✅ PASS |
| Live API tests | 54 endpoints | ✅ 50 PASS, 4 behavior-correct |
| Integration suite | 68 checks | ✅ 65 PASS, 3 SKIP (env-expected) |

### gdash CLI

```bash
# Configure
gdash config set daemon_url https://localhost:8443
gdash config set insecure true   # for self-signed certs

# Auth
gdash auth login -u admin -p changeme

# Servers
gdash server list
gdash server create my-valheim "My Valheim Server" --adapter valheim --deploy-method steamcmd
gdash server start my-valheim
gdash server logs my-valheim

# Backups
gdash backup create my-valheim --type full
gdash backup list my-valheim

# SBOM / CVE
gdash sbom show
gdash sbom cve-report

# Nodes (cluster mode)
gdash node list
gdash node add worker-01 https://worker-01:8443 --token <join-token>
gdash node token          # generate a single-use join token (24h expiry)

# Updates
gdash update                    # update to latest stable (main)
gdash update --branch dev       # switch to dev / pre-release
gdash update --check            # check for updates without applying
gdash version                   # show local + daemon version, branch, commit
```

See [CONTRIBUTING.md](docs/CONTRIBUTING.md) to get started contributing.

See [ROADMAP.md](docs/ROADMAP.md) for planned features.

---

---

## 🗺 Roadmap

What's coming next — loosely ordered by impact. See [ROADMAP.md](docs/ROADMAP.md) for the full list with details.

### Shipped
| Feature | Description |
|---|---|
| **SteamCMD via Docker** | All SteamCMD installs now run in an isolated Docker container — no host SteamCMD required |
| **Persistent server state** | JSON-backed state that survives daemon restarts; transient states reset to stopped on reload |
| **Per-server logs tab** | Logs tab on each server detail page streams lifecycle output |
| **Subsystem log filtering** | Global Logs page Events tab filters by subsystem prefix |
| **Self-update** | Settings → Updates tab + `gdash update`; choose `main` or `dev` branch; in-UI update log viewer |
| **Plain-English errors** | Human-readable error messages everywhere; red error banner on server cards with help modal |
| **Automatic crash recovery** | Auto-restart on unexpected exit; configurable retries, back-off delay, and max attempts |
| **Node-install mode** | `--mode=node` deploys daemon-only on a worker machine |
| **UFW firewall management** | Installer auto-configures UFW; full rule CRUD in the Ports page — no SSH needed |
| **Cluster join tokens** | Single-use 24 h tokens for worker node registration; `gdash node token` to generate |
| **Disk space warnings** | Color-coded disk bars on server cards; sticky dashboard banner at ≥85%; throttled console warnings at 80% and 95% |
| **Live resource table** | Dashboard overview table with CPU/RAM/Disk bars, refreshed every 15 s |
| **In-UI config file editor** | Edit `server.properties`, launch scripts, etc. without SSH |
| **In-UI file browser** | Browse, upload, download, and delete files in the install directory |
| **Share with Friends** | Public IP + port + game join string in one click |
| **Discord/Slack/webhook alerts** | Crash, restart, backup, disk-full notifications |
| **Onboarding wizard** | Getting Started checklist guides new users from first login to running server |
| **Live player count** | Current/max players via RCON/WebRCON/Telnet for 13 games; shown on dashboard table and server detail |

### Up Next
| Feature | Description |
|---|---|
| **Allowlist/banlist UI** | Manage server allowlists and banlists from the dashboard |
| **Game server auto-update** | Daily SteamCMD / Docker pull update with automatic pre-update backup |
| **Log rotation** | Configurable size/age-based rotation for server logs |
| **TLS auto-renewal** | Automatic Let's Encrypt cert renewal (when FQDN is configured) |
| **Per-user ACLs** | Grant specific users access to specific servers only |
| **Steam account auth** | Sign in with Steam for player-facing server management |

### Long-term / Stretch
Mobile PWA · Community adapter marketplace · Multi-region WireGuard cluster · Integrated DDNS · In-app guided diagnostics · 2FA enrollment UI

---

## 📝 License

This project is **proprietary source-available** software. You may run it for personal use and read the source code, but you may **not** distribute it, sell it, or use it as the basis for a competing product without explicit written permission from the author.

See [LICENSE](LICENSE) for the full terms. To request permission for any use not covered: [open an issue](https://github.com/Chrisl154/Gmaer-Server-Daashboard/issues).
