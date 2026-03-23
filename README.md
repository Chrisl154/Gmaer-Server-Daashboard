# Gmaer Server Dashboard

A self-hosted web dashboard for deploying and managing game servers (Valheim, Minecraft, Satisfactory, Palworld, and more).
Single-command install on any Ubuntu 22.04 / 24.04 machine.

---

## Quick Install

### Stable (main branch)

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/install.sh | bash
```

### Dev / Preview (latest features & fixes)

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/dev/install.sh | bash
```

> Use the **dev** URL when live-testing recent changes before they are promoted to main.

---

## What the installer does

1. Installs system dependencies (Docker CE, Go, Node.js, Java, SteamCMD)
2. Clones this repository to `/opt/gdash/repo`
3. Builds the Go daemon and CLI, then the React UI
4. Generates a self-signed TLS certificate
5. Writes a systemd service (`gdash-daemon`) and an nginx reverse proxy
6. Creates the admin account and prints credentials on completion

The installer is interactive by default — whiptail TUI when available, plain readline otherwise.
Run non-interactively with:

```bash
GDASH_NONINTERACTIVE=1 curl -fsSL .../install.sh | bash
```

---

## After install

| What | Command |
|------|---------|
| Open the dashboard | `https://<your-server-ip>` |
| CLI help | `gdash --help` |
| Check daemon status | `sudo systemctl status gdash-daemon` |
| Tail daemon logs | `sudo journalctl -u gdash-daemon -f` |
| Restart daemon | `sudo systemctl restart gdash-daemon` |
| Update to latest stable | `gdash update` |
| Update to dev build | `gdash update --branch dev` |
| Check for updates only | `gdash update --check` |

> The browser will show a TLS warning (self-signed cert). Click **Advanced → Proceed** to continue.

---

## Uninstall

### Stable

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/uninstall.sh | bash
```

### Dev

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/dev/uninstall.sh | bash
```

The uninstaller:
- Stops all running game server Docker containers (`gd-*`)
- Preserves your backups and world saves to `~/gdash-backups-<timestamp>/`
- Removes the daemon service, nginx config, binaries, and `/opt/gdash`

---

## Supported games (24 adapters)

| Game | Adapter | Game | Adapter |
|------|---------|------|---------|
| 7 Days to Die | `7-days-to-die` | Among Us | `among-us` |
| ARK: Survival Ascended | `ark-survival-ascended` | Conan Exiles | `conan-exiles` |
| Counter-Strike 2 | `counter-strike-2` | DayZ | `dayz` |
| Don't Starve Together | `dont-starve-together` | Dota 2 | `dota2` |
| Eco | `eco` | Enshrouded | `enshrouded` |
| Factorio | `factorio` | Garry's Mod | `garrys-mod` |
| Left 4 Dead 2 | `left-4-dead-2` | Minecraft | `minecraft` |
| Palworld | `palworld` | Project Zomboid | `project-zomboid` |
| Riftbreaker | `riftbreaker` | Risk of Rain 2 | `risk-of-rain-2` |
| Rust | `rust` | Satisfactory | `satisfactory` |
| Squad | `squad` | Team Fortress 2 | `team-fortress-2` |
| Terraria | `terraria` | Valheim | `valheim` |

---

## Architecture

```
daemon/      Go REST + WebSocket API (:8443 TLS)
cli/         gdash CLI client
ui/          React 18 + TypeScript SPA (Vite, Tailwind, Zustand)
adapters/    Per-game Bash adapter scripts
installer/   install.sh (TUI) + uninstall.sh
```

---

## Development

```bash
git clone https://github.com/Chrisl154/Gmaer-Server-Daashboard.git
cd Gmaer-Server-Daashboard && git checkout dev

# Daemon
cd daemon && go build ./cmd/daemon

# UI dev server
cd ui && npm install && npm run dev

# Tests
cd daemon && go test ./...
cd ui && npm test
```

### Test Suite Coverage

| Suite | Tests | Status |
|---|---|---|
| Go daemon unit tests | 18 packages | ✅ PASS |
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

## 🗺 Roadmap

What's coming next — loosely ordered by impact. See [ROADMAP.md](docs/ROADMAP.md) for the full list with details.

### Shipped
| Feature | Description |
|---|---|
| **SteamCMD via Docker** | All SteamCMD installs now run in an isolated Docker container — no host SteamCMD required |
| **Persistent server state** | JSON-backed state that survives daemon restarts; transient states reset to stopped on reload |
| **Per-server logs tab** | Logs tab on each server detail page streams lifecycle output |
| **Subsystem log filtering** | Global Logs page Events tab filters by subsystem prefix |
| **Self-update** | Settings → Updates tab + `gdash update`; choose `main` or `dev` branch; in-UI update log viewer with progress bar |
| **Plain-English errors** | Human-readable error messages everywhere; red error banner on server cards with help modal |
| **Automatic crash recovery** | Auto-restart on unexpected exit; configurable retries, back-off delay, and max attempts |
| **Node-install mode** | `--mode=node` deploys daemon-only on a worker machine |
| **UFW firewall management** | Installer auto-configures UFW; full rule CRUD in the Ports page — no SSH needed |
| **GUI firewall rule editor** | Ports page → Firewall Rules panel: view rules, add (port/proto/CIDR/comment), delete, enable/disable UFW — no SSH needed |
| **Cluster join tokens** | Single-use 24 h tokens for worker node registration; `gdash node token` to generate |
| **Valheim binary fix** | Post-deploy `exec_bins` chmod step + pre-start binary verification — fixes "not found" on SteamCMD-installed game binaries |
| **Disk space warnings** | Color-coded disk bars on server cards; sticky dashboard banner at ≥85%; throttled console warnings at 80% and 95% |
| **Live resource table** | Dashboard overview table with CPU/RAM/Disk bars, refreshed every 15 s |
| **In-UI config file editor** | Edit `server.properties`, `cfg/*.cfg`, launch scripts, etc. from the Config tab — no SSH needed; path-traversal safe, 1 MiB cap, audit-logged |
| **In-UI file browser** | Browse, upload, download, and delete files in the install directory |
| **Share with Friends** | Public IP + port + game join string in one click |
| **Discord/Slack/webhook alerts** | Crash, restart, backup, disk-full notifications |
| **Onboarding wizard** | Getting Started checklist guides new users from first login to running server |
| **Live player count** | Current/max players via RCON/WebRCON/Telnet for 13 games; shown on dashboard table and server detail |
| **Allowlist/banlist UI** | Manage server allowlists and banlists from the dashboard |
| **Game server auto-update** | Daily SteamCMD / Docker pull update with automatic pre-update backup |
| **Log rotation** | Configurable size/age-based rotation for server logs |
| **TLS auto-renewal** | Automatic Let's Encrypt cert renewal (when FQDN is configured) |
| **Per-user ACLs** | Grant specific users access to specific servers only |
| **Sign in with Steam** | OpenID 2.0 Steam authentication for player-facing server management |
| **Email notifications** | SMTP alerts (STARTTLS + implicit TLS) alongside existing Discord/webhook support |
| **Network I/O monitoring** | Per-server TX/RX kbps on the metrics tab and dashboard resource table |
| **Server cloning** | One-click deep-copy of a server config with a new name |
| **TOTP recovery codes** | 10 one-use recovery codes generated at 2FA enrollment; regenerate from Security settings |
| **In-app guided diagnostics** | 7-check "Diagnose" modal per server — finds port conflicts, missing deploys, disk issues, and more |
| **System pre-flight check** | Before deploying a game, warns if free RAM/disk/CPU is below requirements |
| **PWA — installable web app** | Installable progressive web app with offline shell and API caching via service worker |
| **CI/CD pipeline** | GitHub Actions: unit tests, lint, multi-arch binaries, Docker images to GHCR, Trivy CVE scan, CycloneDX SBOM, Helm packaging, cosign signing, GitHub Release |

### Up Next
| Feature | Description |
|---|---|
| **Web Push notifications** | Device push alerts for crash/restart events (complement to PWA install) |
| **2FA enrollment QR-code flow** | Full in-UI TOTP enrollment with QR code scan and downloadable recovery PDF |
| **API keys / personal access tokens** | Long-lived tokens for external automation, CI pipelines, and monitoring integrations |
| **Server scheduling** | Cron-based automatic start/stop per server — e.g. start at 6 PM, stop at midnight |
| **Integrated DDNS** | Auto-update DuckDNS or Cloudflare when the host's public IP changes |
| **Community adapter marketplace** | Pull additional game manifests from a curated registry; one-click install from the UI |

### Long-term / Stretch
Multi-region WireGuard cluster · Helm chart for master stack · Server templates · Backup cross-server migration · Kubernetes operator status UI · Audit log export (CSV/JSON)

---

## 📝 License

This project is **proprietary source-available** software. You may run it for personal use and read the source code, but you may **not** distribute it, sell it, or use it as the basis for a competing product without explicit written permission from the author.

See [LICENSE](LICENSE) for the full terms. To request permission for any use not covered: [open an issue](https://github.com/Chrisl154/Gmaer-Server-Daashboard/issues).
Pull requests should target the **dev** branch. `main` is the stable release branch.
