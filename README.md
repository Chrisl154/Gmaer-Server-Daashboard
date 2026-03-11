# Games Dashboard

[![CI](https://github.com/Chrisl154/Gmaer-Server-Daashboard/actions/workflows/ci.yml/badge.svg)](https://github.com/Chrisl154/Gmaer-Server-Daashboard/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Production-grade, open-source Gaming Server Dashboard** for automated provisioning, deployment, management, backup, networking, security, and observability of game servers — supporting Docker single-host and Kubernetes cluster deployments.

## ✨ Features

| Category | Capabilities |
|---|---|
| **Servers** | 24 supported games (Valheim, Minecraft, Rust, CS2, ARK, Palworld, and more) |
| **Deploy** | SteamCMD, manual archive, Docker, Kubernetes operator |
| **Console** | Live WebSocket console streaming per server |
| **Backups** | Scheduled/manual, NFS + S3, incremental, integrity-verified restore |
| **Mods** | Steam Workshop, CurseForge, Thunderstore, Git, local; sandboxed test harness; RBAC |
| **Security** | TLS everywhere, AES-256 secrets at rest, TOTP 2FA, OIDC/SAML/OAuth2 |
| **CVE/SBOM** | CycloneDX SBOM, Trivy/Grype scanning, OSV/NVD queries, evidence model |
| **Networking** | Port mapping UI, reachability probe, UPnP/NAT, firewall automation |
| **Observability** | Prometheus metrics, Grafana dashboards, structured JSON logs |
| **Scale** | Docker Compose (single-host) or Kubernetes/k3s via Helm + CRD operator |

---

## 🚀 Quick Start

### One-Liner Test & Install (Ubuntu 24.04+)

The fastest way to validate the full stack on any fresh Linux box — installs all dependencies, builds from source, starts the daemon, and runs all tests automatically:

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/test-live.sh | bash
```

**Minimum requirement:** Ubuntu 24.04 (or any modern Linux) with internet access and `bash`. Everything else — Go 1.22, Node.js 20 LTS, Trivy CVE scanner, Python packages — is installed automatically in userspace (no root required beyond standard system packages).

> Run locally if you already have the repo cloned:
> ```bash
> bash test-live.sh
> ```

### Interactive Install (Docker)

```bash
curl -fsSL https://github.com/Chrisl154/Gmaer-Server-Daashboard/releases/latest/download/installer.sh \
  -o installer.sh && chmod +x installer.sh
./installer.sh --mode docker
```

### Headless Install

```bash
./installer.sh --headless --config config.json --mode docker --accept-licenses
```

### Kubernetes (k3s)

```bash
./installer.sh --mode k8s --k8s-distribution k3s --install-helm --install-metalb
# or via Helm:
helm upgrade --install games-dashboard ./helm/charts/games-dashboard \
  --namespace games-dashboard --create-namespace
```

### Dry Run

```bash
./installer.sh --mode docker --dry-run
```

---

## 📋 Installer Flags

```
--mode docker|k8s               Deployment mode (required)
--install-dir /path             Installation directory
--headless                      Non-interactive
--config /path/config.json      Headless config file
--reuse-existing                Reuse existing installation
--offline-bundle /path          Offline bundle path
--accept-licenses               Accept all licenses
--min-hardware-profile small|medium|large
--probe-remote-validator <url>  Remote port probe endpoint
--k8s-distribution k3s|kubeadm|managed
--container-runtime docker|containerd|podman
--enable-mod-manager
--log-level debug|info|warn|error
--skip-preflight
--force
--no-reboot
--tls-cert /path/cert.pem
--tls-key /path/key.pem
--vault-endpoint <url>
--vault-token <token>
--install-helm
--install-metalb
--install-csi-nfs
--accept-defaults
--dry-run
--rollback-to <checkpoint-id>
--output-audit /path/audit.json
```

---

## 🎮 Supported Games & Adapters (24)

| Game | Steam App ID | Deploy Methods | Console | Mods |
|---|---|---|---|---|
| 7 Days to Die | 294420 | SteamCMD | Telnet | Yes |
| Among Us (Impostor) | 945360 | Manual/Custom | stdio | Yes |
| ARK Survival Ascended | 2430930 | SteamCMD | RCON | Yes |
| Conan Exiles | 443030 | SteamCMD | RCON | Yes |
| Counter-Strike 2 | 730 | SteamCMD | RCON | Yes |
| DayZ | 223350 | SteamCMD | stdio | Yes |
| Don't Starve Together | 343050 | SteamCMD | stdio | Yes |
| Dota 2 | 570 | SteamCMD | RCON | Yes |
| Eco | 382310 | SteamCMD/Manual | WebSocket | Yes |
| Enshrouded | 2278520 | SteamCMD/Manual | stdio | — |
| Factorio | 427520 | Manual/SteamCMD | RCON | Yes |
| Garry's Mod | 4020 | SteamCMD | RCON | Yes |
| Left 4 Dead 2 | 222860 | SteamCMD | RCON | Yes |
| Minecraft Java | — | Manual/Custom | RCON | Yes |
| Palworld | 2394010 | SteamCMD/Manual | RCON | — |
| Project Zomboid | 380870 | SteamCMD | stdio | Yes |
| Risk of Rain 2 | 1180760 | SteamCMD | stdio | Yes |
| Rust | 252490 | SteamCMD | WebRCON | Yes |
| Satisfactory | 1690800 | SteamCMD/Manual | stdio | Yes |
| Squad | 403240 | SteamCMD | RCON | Yes |
| Team Fortress 2 | 232250 | SteamCMD | RCON | Yes |
| Terraria | — | Manual | stdio | Yes |
| The Riftbreaker | — | Manual/Custom | stdio | Yes |
| Valheim | 896660 | SteamCMD/Manual | stdio | Yes (Thunderstore) |

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
├── installer/        install.sh single-artifact installer
├── helm/             Helm charts + CRDs
├── docs/             Operator runbook, API reference, security docs
├── tests/            Unit, integration, E2E test suites
├── ci/               CI helper scripts
├── test-live.sh      One-liner end-to-end test runner
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

See [SECURITY.md](docs/SECURITY.md) for the threat model and vulnerability disclosure.

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
| UI Vitest | 27 tests | ✅ PASS |
| UI TypeScript check | — | ✅ Clean |
| UI production build | — | ✅ 813 KB bundle |
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
gdash node add worker-01 https://worker-01:8443
```

See [CONTRIBUTING.md](docs/CONTRIBUTING.md) to get started contributing.

---

## 📝 License

[MIT](LICENSE)
