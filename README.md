# Games Dashboard

[![CI](https://github.com/games-dashboard/games-dashboard/actions/workflows/ci.yml/badge.svg)](https://github.com/games-dashboard/games-dashboard/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Production-grade, open-source Gaming Server Dashboard** for automated provisioning, deployment, management, backup, networking, security, and observability of game servers — supporting Docker single-host and Kubernetes cluster deployments.

## ✨ Features

| Category | Capabilities |
|---|---|
| **Servers** | Valheim, Minecraft, Satisfactory, Palworld, Eco, Enshrouded, The Riftbreaker |
| **Deploy** | SteamCMD, manual archive, Docker, Kubernetes operator |
| **Console** | Live WebSocket console streaming per server |
| **Backups** | Scheduled/manual, NFS + S3, incremental, integrity-verified restore |
| **Mods** | Steam Workshop, CurseForge, Git, local; sandboxed test harness; RBAC |
| **Security** | TLS everywhere, AES-256 secrets at rest, TOTP 2FA, OIDC/SAML/OAuth2 |
| **CVE/SBOM** | CycloneDX SBOM, Trivy/Grype scanning, OSV/NVD queries, evidence model |
| **Networking** | Port mapping UI, reachability probe, UPnP/NAT, firewall automation |
| **Observability** | Prometheus metrics, Grafana dashboards, structured JSON logs |
| **Scale** | Docker Compose (single-host) or Kubernetes/k3s via Helm + CRD operator |

## 🚀 Quick Start

### Interactive Install (Docker)

```bash
curl -fsSL https://github.com/games-dashboard/games-dashboard/releases/latest/download/installer.sh \
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

## 🎮 Supported Games & Adapters

| Game | Steam App ID | Deploy Method | Console | Mods |
|---|---|---|---|---|
| Valheim | 896660 | SteamCMD | stdio | Thunderstore |
| Minecraft | — | Manual/Docker | RCON | CurseForge/Modrinth |
| Satisfactory | 1690800 | SteamCMD | stdio | ficsit.app |
| Palworld | 2394010 | SteamCMD | RCON | — |
| Eco | 382310 | SteamCMD | WebSocket | Eco Mod Kit |
| Enshrouded | 2278520 | SteamCMD | stdio | — |
| The Riftbreaker | — | Manual | stdio | Steam Workshop |

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

## 📁 Repository Layout

```
games-dashboard/
├── daemon/           Go daemon binary + OpenAPI
├── ui/               React + TypeScript web UI
├── cli/              gdash CLI binary
├── adapters/         Game adapter manifests + test harnesses
├── installer/        install.sh single-artifact installer
├── helm/             Helm charts + CRDs
├── docs/             README, API reference, runbook, security
├── tests/            Unit, integration, e2e test suites
├── ci/               CI helper scripts
└── .github/          GitHub Actions workflows
```

## 🔒 Security

- TLS 1.3 everywhere (auto self-signed or provided certificates)
- AES-256-GCM secrets encryption at rest (local KMS or HashiCorp Vault)
- JWT authentication with TOTP 2FA, OIDC, SAML, OAuth2
- RBAC: `admin`, `operator`, `modder`, `viewer` roles
- Signed audit trail for all operations
- Signed release artifacts via cosign / Sigstore

See [SECURITY.md](docs/SECURITY.md) for the threat model and vulnerability disclosure.

## 📊 Observability

- Prometheus metrics at `/metrics`
- Grafana dashboard templates in `docs/grafana/`
- Structured JSON logs with configurable retention
- Per-server health endpoints

## 🛠 Development

```bash
# Daemon
cd daemon && go run ./cmd/daemon --log-level debug

# UI (dev server)
cd ui && npm install && npm run dev

# CLI
cd cli && go run ./cmd version

# Run tests
cd daemon && go test ./...
cd ui && npm test
```

See [CONTRIBUTING.md](docs/CONTRIBUTING.md) to get started.

## 📝 License

[MIT](LICENSE)
