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

## Supported games

| Game | Adapter |
|------|---------|
| Valheim | `valheim` |
| Minecraft | `minecraft` |
| Satisfactory | `satisfactory` |
| Palworld | `palworld` |
| Eco | `eco` |
| Enshrouded | `enshrouded` |
| Riftbreaker | `riftbreaker` |

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

Pull requests should target the **dev** branch. `main` is the stable release branch.
