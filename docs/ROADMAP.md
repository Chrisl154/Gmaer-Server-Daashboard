# Roadmap

Planned features and improvements. The target audience is people who are comfortable spinning up a Linux VM but are not otherwise technical — the dashboard should handle everything so they never need to SSH in or edit config files.

Items within each section are loosely ordered by impact. All dates are targets, not commitments.

---

## Shipped

### SteamCMD via Docker (zero host dependencies)
Docker is a hard requirement for the dashboard. SteamCMD deployments run transparently inside `cm2network/steamcmd` — users never need to install SteamCMD, its 32-bit libraries, or any Steam tooling on the host. The container is pulled automatically on first deploy, game files land in the configured install directory owned by the daemon's user, and the container is removed when the download finishes.

---

## Near-term

### Node-install mode (worker-only installer)
Add a `--mode=node` flag (or a TUI screen) to `install.sh` that installs a machine as a **pure worker node** — daemon only, no UI, no nginx, no admin account.

- Generates a TLS cert + short-lived join token on the worker
- Prints a ready-to-paste `gdash node add <host> https://<ip>:8443 --token <token>` command
- On first connection the node registers itself (CPUs / RAM / disk) and waits for workloads
- Worker node card in the dashboard shows connected nodes, per-node resource bars, and running server list
- `gdash node remove <id>` drain + deregister flow
- What is already built: `cluster.Manager`, BestFit placement, `gdash node list/add` CLI

### ~~Persistent server state~~ ✅ SHIPPED
Server records are now written to `{data_dir}/servers.json` on every create/update/delete and re-hydrated on startup. Transient states (starting/running/stopping) are reset to `stopped` on load. Container IDs and all configuration are preserved across restarts.

### ~~Automatic crash recovery~~ ✅ SHIPPED
If a game server process exits unexpectedly, the broker should auto-restart it.

- Configurable per server: `auto_restart: true`, max retries (default 3), back-off delay
- After max retries mark state `error` and send a notification
- Show restart count and last-crash time on the server card

### ~~Self-update mechanism~~ ✅ SHIPPED
Settings → Updates tab + `gdash update` CLI command. Fetches latest from git, rebuilds daemon + CLI + UI, restarts the systemd service. Branch selector: `main` (stable) or `dev` (pre-release). The update script runs detached so the daemon restart doesn't kill it mid-flight. Sudoers entry installed automatically so no password prompt is required.

### ~~Plain-English error messages~~ ✅ SHIPPED
Raw Go error strings are meaningless to non-technical users.

- Every `StateError` transition writes a human-readable explanation to the console stream (e.g. "SteamCMD could not find the game files — your disk may be full, or the Steam servers may be down. Try deploying again.")
- The UI renders the last error message prominently on the server card instead of just showing the `error` badge
- A "What does this mean?" help link next to each error opens a short explanation modal

---

## Medium-term

### ~~In-UI config file editor~~ ✅ SHIPPED
Users should be able to tweak server settings without SSH.

- Each adapter manifest declares a list of "well-known config files" (e.g. `server.properties`, `valheim/start_server.sh`, `BepInEx/config/`)
- The server detail page **Config Files** tab lists manifest-declared files in a sidebar; clicking any opens a full textarea editor
- Saves write directly to the install directory with path-traversal protection
- Falls back to the manifest sample content when the file doesn't exist yet

### In-UI file browser
Browse, download, upload, and delete files in the server install directory from the browser — important for mod assets, world saves, and config files.

- Tree view with drag-and-drop upload
- Download individual files or zip an entire folder
- Delete with confirmation

### "Share with Friends" connection info panel
One click to get everything a friend needs to join.

- Displays the public IP, port, and game-specific join string (e.g. `steam://connect/1.2.3.4:2456` for Valheim)
- Optional QR code for mobile games
- Copy-to-clipboard button
- Shows whether the port is reachable from the internet (uses the existing reachability probe)

### Player count & active player list
Show live player counts on server cards and in the overview dashboard.

- Query the game server via its native protocol (Source query, Minecraft status ping, etc.) on a 60-second interval
- Display current / max players on the server card
- Player list with join time visible in the server detail overview tab

### Allowlist / banlist management UI
Manage who can join without editing text files.

- Adapter manifests declare the path and format of the whitelist/banlist files
- UI shows a searchable table: add players by name/Steam ID, remove with one click
- Changes write the file and optionally send a live RCON reload command

### Discord / webhook notifications
Alert the server owner when something important happens — no polling required.

- Configurable webhook URL in Settings (Discord, Slack, generic HTTP POST)
- Events: server crashed, server restarted, backup completed/failed, disk >80% full, player count hits 0 after being non-zero (server may be empty/stuck)
- Per-server toggles so noisy servers don't spam

### ~~Disk space and resource warning banners~~ ✅ SHIPPED
Non-technical users won't notice a full disk until everything breaks.

- Dashboard shows a banner when any server's install directory host partition is >85% full
- Server cards show a color-coded disk bar (green → yellow → red)
- Daemon emits a console warning event at 80% and 95% thresholds
- Live resource table on dashboard: CPU, RAM, Disk bars + allocated resources per server row

### Game server auto-update
Keep game servers up to date without manual deploys.

- Per-server setting: check for updates on a schedule (default: daily at 4 AM)
- For SteamCMD games: re-run `app_update <id> validate`; for Docker games: pull the latest image tag and recreate the container
- Automatic backup before every update
- Show "Update available" badge on server cards when a newer version is detected

### Onboarding wizard (in-UI first-run)
After the install the user lands on a blank servers page with no guidance.

- On first login (no servers exist) launch a 3-step modal: pick a game → name your server → deploy
- Contextual tooltips on every form field explaining what the value does
- A dismissible "Getting started" checklist in the sidebar (Deploy a server · Take a backup · Invite a user)

### Persistent server state across daemon restarts
- SQLite store under `data/state.db`
- Re-hydrate broker on startup; containerIDs, deploy methods, and config all survive restarts

### Log rotation for gdash-events.log
- Cap per-server event log at 50 MB; rotate to `.1` / `.2` / `.3` (keep 3 generations)
- Configurable via `daemon.yaml`

### TLS auto-renewal (Let's Encrypt / ACME)
- When the user provides an FQDN during install, opt into ACME automatically
- Daemon watches cert expiry and renews 30 days before expiry using the ACME HTTP-01 challenge

### Per-user server ACLs
- Extend RBAC so an `operator` role can be scoped to specific servers, not the whole cluster
- Useful for friend groups where each person manages their own game

### Steam account auth for paid games
- Securely store Steam credentials (encrypted at rest) for games that require a paid account (DayZ, etc.)
- SteamCMD deployment uses stored credentials automatically

---

## Long-term / Stretch

### Mobile-friendly PWA
- Installable progressive web app
- Push notifications via Web Push for crash/restart events
- Swipe gestures for start/stop on server cards

### Marketplace for community adapters
- Pull additional game manifests from a curated GitHub registry
- One-click install of community-contributed adapters directly from the dashboard UI
- Adapter signing/verification so users know what they're running

### Helm chart for the dashboard itself
- Run the full master stack as a Kubernetes Deployment + Service
- Useful for users who already have a k8s cluster (e.g. home lab Proxmox + k3s)

### Multi-region cluster with WireGuard overlay
- Worker nodes can be on different networks / clouds
- Automatic WireGuard mesh so game traffic routes efficiently
- Cost display per node (manual entry or cloud API integration)

### Integrated DDNS
- When the host's public IP changes (residential ISP), automatically update a free DDNS record (DuckDNS, Cloudflare)
- No more "what's the IP today?" for friends trying to connect

### In-app guided diagnostics
- "My server won't start" wizard that walks through the most common failure modes: not deployed → wrong adapter → port conflict → disk full → missing dependency
- Each step is automated: the wizard checks the condition and tells the user exactly what to fix

### Bandwidth / network usage monitoring
- Per-server TX/RX bytes graphed on the metrics tab
- Monthly usage summary useful for users on metered cloud VMs

### System requirements pre-flight check
- Before deploying a game, check if the host has enough free RAM, disk, and CPU
- Warn with a yellow banner (not a blocker) if requirements aren't met, with a plain-English explanation

### Two-factor authentication enrollment UI
- Current TOTP 2FA requires manual setup; add a QR-code enrollment flow in the Security settings page
- Recovery codes generated at enrollment time, downloadable as a PDF
