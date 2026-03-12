# Roadmap

Planned features and improvements. Items are loosely ordered by priority but are subject to change.

---

## Near-term

### Node-install mode (worker-only installer)
Add a `--mode=node` flag (or a TUI option) to `install.sh` that installs a machine as a **pure worker node** instead of a full master stack.

What the node install would do:
- Install the `games-daemon` binary and systemd unit only (no nginx, no UI, no admin account seed)
- Generate a TLS certificate and a join token
- Print a one-liner `gdash node add <hostname> https://<ip>:8443 --token <token>` to run on the master
- On first connection from the master the node registers itself, reports available CPUs/RAM/disk, and waits for workloads

What the master already supports (design is in place):
- `cluster.Manager` in `daemon/internal/cluster/` handles node registration, health-checking, and BestFit placement
- `gdash node list` / `gdash node add` CLI commands are implemented
- `CreateServer` already auto-places servers onto the best-fit node when `cluster.enabled: true`

What still needs to be built:
- Worker-only install path in `install.sh` (skip nginx, UI build, admin bootstrap)
- Join-token generation and validation on first node → master handshake
- Forwarding of start/stop/deploy API calls from master to worker over mTLS
- Worker node UI card in the dashboard (connected nodes, per-node CPU/RAM, workload list)
- `gdash node remove <id>` drain + deregister flow

---

## Medium-term

- **Persistent server state across daemon restarts** — currently all server records live in memory; a SQLite or JSON-file store would survive restarts
- **Log rotation for gdash-events.log** — cap per-server event logs at a configurable size (default 50 MB) and rotate to `.1`, `.2`, etc.
- **TLS auto-renewal** — integrate with ACME/Let's Encrypt for public-facing installs
- **Per-user server ACLs** — extend RBAC so operators can be scoped to specific servers rather than the whole cluster
- **Steam auth for game servers** — pass a stored Steam login to SteamCMD for games that require a paid account (e.g., DayZ)

---

## Long-term / Stretch

- **Helm chart for the dashboard itself** — run the full master stack as a Kubernetes Deployment
- **Multi-region cluster** — WireGuard overlay so worker nodes can be on different networks/clouds
- **Marketplace for community adapter manifests** — pull additional game manifests from a curated registry URL
- **Mobile-friendly PWA** — installable progressive web app with push notifications for server state changes
