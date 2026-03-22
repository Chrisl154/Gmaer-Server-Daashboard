# Roadmap

Planned features and improvements. The target audience is people who are comfortable spinning up a Linux VM but are not otherwise technical — the dashboard should handle everything so they never need to SSH in or edit config files.

Items within each section are loosely ordered by impact. All dates are targets, not commitments.

---

## Shipped

### SteamCMD via Docker (zero host dependencies)
Docker is a hard requirement for the dashboard. SteamCMD deployments run transparently inside `cm2network/steamcmd` — users never need to install SteamCMD, its 32-bit libraries, or any Steam tooling on the host. The container is pulled automatically on first deploy, game files land in the configured install directory owned by the daemon's user, and the container is removed when the download finishes.

---

## Near-term

### ~~Node-install mode (worker-only installer)~~ ✅ SHIPPED
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

### ~~In-UI file browser~~ ✅ SHIPPED
Browse, download, upload, and delete files in the server install directory from the browser — important for mod assets, world saves, and config files.

- Directory tree with breadcrumb navigation — click folders to descend, `..` row to go up
- File table showing name, size, modified date with icons for files vs folders
- Download individual files (raw bytes served via `GET /files/download`)
- Upload one or more files to the current directory via multipart form
- Delete files with a confirmation modal (directory delete blocked)
- New **Files** tab on the Server Detail page (tab #8)

### ~~"Share with Friends" connection info panel~~ ✅ SHIPPED
One click to get everything a friend needs to join.

- Displays the public IP, port, and game-specific join string (e.g. `steam://connect/1.2.3.4:2456` for Valheim)
- Optional QR code for mobile games
- Copy-to-clipboard button
- Shows whether the port is reachable from the internet (uses the existing reachability probe)

### ~~Player count & active player list~~ ✅ SHIPPED
Show live player counts on server cards and in the overview dashboard.

- Query the game server via its native protocol (Source query, Minecraft status ping, etc.) on a 60-second interval
- Display current / max players on the server card
- Player list with join time visible in the server detail overview tab

### ~~Allowlist / banlist management UI~~ ✅ SHIPPED
Manage who can join without editing text files.

- Adapter manifests declare the path and format of the whitelist/banlist files
- UI shows a searchable table: add players by name/Steam ID, remove with one click
- Changes write the file and optionally send a live RCON reload command

### ~~Discord / webhook notifications~~ ✅ SHIPPED
Alert the server owner when something important happens — no polling required.

- Configurable webhook URL in Settings (Discord, Slack, generic HTTP POST)
- Events: server crashed, server restarted, disk >80% full
- Per-event toggles so noisy events don't spam
- One-click "Send Test" button to verify the webhook works

### ~~Disk space and resource warning banners~~ ✅ SHIPPED
Non-technical users won't notice a full disk until everything breaks.

- Dashboard shows a banner when any server's install directory host partition is >85% full
- Server cards show a color-coded disk bar (green → yellow → red)
- Daemon emits a console warning event at 80% and 95% thresholds
- Live resource table on dashboard: CPU, RAM, Disk bars + allocated resources per server row

### ~~Game server auto-update~~ ✅ SHIPPED
Keep game servers up to date without manual deploys.

- Per-server setting: check for updates on a schedule (default: daily at 4 AM)
- For SteamCMD games: re-run `app_update <id> validate`; for Docker games: pull the latest image tag and recreate the container
- Automatic backup before every update
- Show "Update available" badge on server cards when a newer version is detected

### ~~Onboarding wizard (in-UI first-run)~~ ✅ SHIPPED
After the install the user lands on a blank servers page with no guidance.

- A dismissible "Getting Started" checklist on the Dashboard (Deploy a server · Take a backup · Set up notifications · Invite a user)
- Auto-marks the "Add server" step done when the first server is created
- Collapsible, persisted via localStorage, with per-step toggle
- Hides completely once dismissed

### ~~Persistent server state across daemon restarts~~ ✅ SHIPPED
- JSON-backed state under `data/servers.json`; containerIDs, deploy methods, and config all survive restarts

### ~~Log rotation for gdash-events.log~~ ✅ SHIPPED
- Custom `rotatingWriter` with configurable size/age-based rotation, backup shifting, and gzip compression
- Configurable via `daemon.yaml` (`max_size_mb`, `max_backups`, `max_age_days`, `compress`)

### ~~TLS auto-renewal (Let's Encrypt / ACME)~~ ✅ SHIPPED
- `autocert.Manager` HTTP-01 challenge server when `auto_tls: true` in config
- `ACMEDomain`, `ACMEEmail`, `ACMECacheDir`, `ACMEStaging` options in `TLSConfig`

### ~~Per-user server ACLs~~ ✅ SHIPPED
- `AllowedServers []string` per user; role-based route enforcement
- ACL management UI in the admin panel

### ~~Sign in with Steam~~ ✅ SHIPPED
- OpenID 2.0 "Sign in with Steam" for user authentication (`GET /auth/steam/login` → callback → JWT)
- Replay nonce protection; `GetSteamDisplayName` for profile display
- "Sign in with Steam" button on login page; `loginWithToken` action for token-in-URL flow

### Steam credentials for paid games (SteamCMD)
- Securely store Steam credentials (encrypted at rest) for games that require a paid account (DayZ, etc.)
- SteamCMD deployment uses stored credentials automatically

---

## Long-term / Stretch

### ~~Mobile-friendly PWA — installable app~~ ✅ SHIPPED (base)
- Installable progressive web app with Web App Manifest and Workbox service worker
- Offline shell: service worker precaches all static assets; API requests use NetworkFirst strategy
- Apple Web App meta tags for iOS Add-to-Home-Screen
- Push notifications via Web Push for crash/restart events — **pending** (see Up Next)

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

### ~~In-app guided diagnostics~~ ✅ SHIPPED
- `GET /api/v1/servers/:id/diagnose` runs 7 heuristic checks and returns `[]DiagnosticFinding`
- `DiagnosticsModal` in the UI shows severity-coloured cards; "Diagnose" CTA on the error banner

### ~~Bandwidth / network usage monitoring~~ ✅ SHIPPED
- `NetInKbps` / `NetOutKbps` per server from Docker stats, updated every 15 s
- Dashboard resource table "Network" column; server detail metrics tab with net I/O line chart

### ~~System requirements pre-flight check~~ ✅ SHIPPED
- `GET /api/v1/system/resources` returns CPU cores, total/free RAM, total/free disk
- `ResourceWarning` component in the Add Server wizard: amber banner if requirements aren't met (not a blocker)

### Two-factor authentication enrollment QR-code flow
- Current TOTP 2FA requires manual secret entry; add a QR-code enrollment flow in the Security settings page
- Recovery codes already generated at enrollment (10 codes, downloadable); this item tracks the QR-code scan UI

---

## Newly Identified — Up Next

### Web Push notifications
- Device push alerts (crash, restart, disk full) via the Web Push API
- Requires VAPID key pair on the daemon and a push subscription stored per user
- Works when the PWA is installed and even when the browser tab is closed

### API keys / personal access tokens
- Long-lived tokens scoped to a user+role, managed from the Settings → Security page
- Enables external automation: CI/CD pipelines, monitoring integrations, custom scripts
- `Authorization: Bearer <pat>` accepted on all existing API routes; separate token table in state

### Server scheduling
- Per-server cron schedule for automatic start and stop (e.g. "start at 18:00, stop at 23:00 Mon–Fri")
- Configured in the server detail Overview tab alongside auto-restart settings
- Uses the existing `robfig/cron` scheduler already in the broker

### Integrated DDNS
- When the host's public IP changes (residential ISP), automatically update a free DDNS provider (DuckDNS, Cloudflare)
- Configured in Settings; daemon polls public IP every 5 minutes and pushes updates only on change
- No more "what's the IP today?" for friends trying to connect

### Community adapter marketplace
- Pull additional game adapter manifests from a curated GitHub registry (separate repo)
- One-click install of community-contributed adapters directly from the dashboard UI
- Adapter manifest signing/verification via cosign so users know what they're running

### Server templates
- Save a server's full configuration (adapter, env vars, ports, deploy method) as a named template
- "New from template" in the Add Server wizard — pre-fills the form with template values
- Templates stored in `data/templates.json`; shareable as JSON export/import

### Multi-region cluster with WireGuard overlay
- Worker nodes on different networks / clouds connected via automatic WireGuard mesh
- Game traffic routes across the overlay; latency-aware placement prefers closest node
- Cost display per node (manual entry or cloud API)

### Helm chart for the master stack
- Run the full master stack (daemon + UI + nginx) as a Kubernetes Deployment + Service + Ingress
- Useful for users who already have a k8s cluster (home lab Proxmox + k3s)
- Values: image tags, TLS secret, persistence claim, ingress hostname

### Backup cross-server migration
- Restore a backup to a different server (e.g. migrate a Minecraft world from one server to another)
- Handles adapter differences: warns when source and target adapters don't match
- Available as both a UI action and `gdash backup restore --to <target-id>`

### Kubernetes operator status UI
- Show pod health, events, and restart counts for game servers deployed as Kubernetes GameInstance CRDs
- Live status card in the server detail page when the server's deploy method is `k8s`

### Audit log export
- Export the full audit trail as CSV or JSON from the Logs → Audit Trail tab
- Filterable by date range, user, and event type before export
- Useful for compliance and incident post-mortems
