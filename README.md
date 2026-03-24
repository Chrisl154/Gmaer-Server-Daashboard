# Gmaer Server Dashboard

A self-hosted web dashboard for deploying and managing game servers — no SSH required.
Deploy Valheim, Minecraft, Rust, Palworld, and 20+ more games from your browser in minutes.

> **Single-command install on Ubuntu 22.04 / 24.04.**

---

## Screenshots

> *(Screenshots coming soon — dashboard, server detail, deploy wizard)*

---

## Features

- **One-click deploy** — SteamCMD (via Docker) or manual install; no host SteamCMD required
- **Full server lifecycle** — start, stop, restart, and monitor all your servers from one place
- **Live console** — real-time stdout/stderr streamed directly in the browser
- **Config file editor** — edit `server.properties`, launch scripts, and config files in-browser; no SSH needed
- **File browser** — browse, upload, download, and delete files in any server's install directory
- **Auto crash recovery** — configurable restart on crash with back-off and max-retry limits
- **Self-update** — pull the latest release from the dashboard; choose Production, Beta, or a custom feature branch
- **Live metrics** — CPU, RAM, and disk usage refreshed every 15 seconds
- **Live player count** — current/max players via RCON, WebRCON, or Telnet for 13 games
- **Backup system** — manual and scheduled backups with retention policy
- **Firewall manager** — add, remove, and toggle UFW rules from the Ports page
- **Discord / webhook alerts** — crash, restart, backup, and disk-full notifications
- **Multi-user with roles** — admin and viewer roles; per-user server ACLs
- **2FA (TOTP)** — time-based one-time passwords with recovery codes
- **Mod manager** — install mods from Nexus, Thunderstore, or local uploads
- **Steam sign-in** — OpenID 2.0 Steam auth for player-facing management
- **Cluster mode** — add worker nodes with a single join token
- **Installable PWA** — install the dashboard as an app on any device

---

## Quick Install

### Stable (Production)

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/install.sh | bash
```

### Beta (Latest features & fixes)

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/dev/install.sh | bash
```

The installer is interactive by default. Run non-interactively with:

```bash
GDASH_NONINTERACTIVE=1 curl -fsSL .../install.sh | bash
```

> **TLS note:** The browser will show a security warning for the self-signed certificate. Click **Advanced → Proceed** to continue. You can configure a real domain and Let's Encrypt cert after install.

---

## After Install

| What | How |
|------|-----|
| Open the dashboard | `https://<your-server-ip>` |
| Check daemon status | `sudo systemctl status gdash-daemon` |
| CLI help | `gdash --help` |
| Update to latest stable | `gdash update` |
| Update to beta build | `gdash update --branch dev` |

---

## Supported Games (24 adapters)

| Game | Game | Game | Game |
|------|------|------|------|
| 7 Days to Die | Among Us | ARK: Survival Ascended | Conan Exiles |
| Counter-Strike 2 | DayZ | Don't Starve Together | Dota 2 |
| Eco | Enshrouded | Factorio | Garry's Mod |
| Left 4 Dead 2 | Minecraft | Palworld | Project Zomboid |
| Riftbreaker | Risk of Rain 2 | Rust | Satisfactory |
| Squad | Team Fortress 2 | Terraria | Valheim |

---

## Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/uninstall.sh | bash
```

Preserves backups and world saves to `~/gdash-backups-<timestamp>/` before removing the service, nginx config, and `/opt/gdash`.

---

## Roadmap

### Recently Shipped
| Feature | |
|---|---|
| SteamCMD via Docker | Isolated container deploys — no host SteamCMD required |
| Same-origin API | UI works from any IP or DNS hostname without reconfiguration |
| Custom branch updates | Deploy any feature branch directly from the Settings page |
| Persistent server state | Survives daemon restarts; transient states reset cleanly |
| Self-update system | In-UI update with progress bar, log viewer, and auto-restart |
| Pre-flight write check | Detects permission issues before starting a multi-GB download |
| Auto crash recovery | Configurable restart with crash counter and back-off |
| In-browser config editor | Path-traversal safe, 1 MiB cap, audit-logged |
| Disk space warnings | Color-coded bars; sticky banner at ≥85% |
| Security audit | All HIGH/MEDIUM/LOW audit findings remediated |

### Coming Next
| Feature | |
|---|---|
| Web Push notifications | Device alerts for crash/restart events |
| Server scheduling | Cron-based auto start/stop per server |
| API keys | Long-lived tokens for automation and CI pipelines |
| Integrated DDNS | Auto-update DuckDNS/Cloudflare on IP change |
| Community adapter marketplace | Pull game manifests from a curated registry |

See [docs/ROADMAP.md](docs/ROADMAP.md) for the full list.

---

## Open Source Credits

This project is built on the shoulders of excellent open source work. Thank you to:

**Backend**
| Project | Use |
|---|---|
| [Gin](https://github.com/gin-gonic/gin) | HTTP web framework |
| [Uber Zap](https://github.com/uber-go/zap) | Structured logging |
| [golang-jwt/jwt](https://github.com/golang-jwt/jwt) | JWT authentication |
| [robfig/cron](https://github.com/robfig/cron) | Scheduled tasks |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | bcrypt password hashing |
| [Tailscale tsnet](https://tailscale.com) | Embedded Tailscale networking |

**Frontend**
| Project | Use |
|---|---|
| [React](https://react.dev) | UI library |
| [Vite](https://vitejs.dev) | Build tooling |
| [Tailwind CSS](https://tailwindcss.com) | Utility-first styling |
| [TanStack Query](https://tanstack.com/query) | Data fetching and caching |
| [Zustand](https://github.com/pmndrs/zustand) | Client state management |
| [Axios](https://axios-http.com) | HTTP client |
| [Lucide](https://lucide.dev) | Icon set |
| [Radix UI](https://www.radix-ui.com) | Accessible UI primitives |
| [Recharts](https://recharts.org) | Charts and metrics graphs |
| [vite-plugin-pwa](https://vite-pwa-org.netlify.app) | Progressive web app support |

**Infrastructure & Tooling**
| Project | Use |
|---|---|
| [cm2network/steamcmd](https://hub.docker.com/r/cm2network/steamcmd) | SteamCMD Docker image for game deploys |
| [Docker CE](https://www.docker.com) | Container runtime |
| [nginx](https://nginx.org) | Reverse proxy and static file serving |
| [NVM](https://github.com/nvm-sh/nvm) | Node.js version management in installer |

---

## License

This project is **proprietary source-available** software. You may run it for personal use and read the source code, but you may **not** distribute it, sell it, or use it as the basis for a competing product without explicit written permission from the author.

See [LICENSE](LICENSE) for the full terms. To request permission: [open an issue](https://github.com/Chrisl154/Gmaer-Server-Daashboard/issues).

Pull requests are welcome — target the **dev** branch. See [CONTRIBUTING.md](docs/CONTRIBUTING.md) for setup instructions.
