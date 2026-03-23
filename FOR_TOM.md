# FOR TOM — Handoff Notes

> Hi! I'm Claude Sonnet 4.6. The main Claude instance hit its usage limit.
> This doc gives you everything you need to pick up where we left off.
> All current work is on `main` (and mirrored to `dev`).

---

## Project: Games Server Dashboard (gdash)

A self-hosted gaming server manager. Users deploy, start, stop, and monitor
game servers (Valheim, Minecraft, Rust, etc.) from a web UI — no SSH required.

### Stack

| Layer | Tech |
|---|---|
| Daemon (REST + WebSocket API) | Go 1.22, Gin, on `:8443` TLS |
| UI | React 18 + TypeScript + Vite + Tailwind |
| CLI | Go 1.22 (`gdash` binary) |
| Game adapters | YAML manifests in `adapters/` |
| Installer | Bash TUI (`install.sh`) |

### Important paths

```
daemon/internal/broker/broker.go   ← core server lifecycle (start/stop/deploy)
daemon/internal/api/server.go      ← all HTTP route registrations
daemon/internal/api/              ← one file per feature area (firewall.go, configfiles.go, update.go…)
daemon/internal/adapters/adapter.go ← Manifest/Registry types
ui/src/pages/                      ← one page per UI section
ui/src/pages/ServerDetailPage.tsx  ← per-server tabs (overview/console/logs/config/…)
ui/src/pages/SettingsPage.tsx      ← Settings → Updates, Users, etc.
install.sh                         ← installer + embedded gdash-update.sh script
```

---

## Build & Test Commands

```bash
# ── Go toolchain (NOT the system go — it's only 1.18) ──────────────────────
~/.local/go/bin/go build ./...          # from daemon/
~/.local/go/bin/go test ./...           # from daemon/   ← run after every change

# ── Node (via NVM) ──────────────────────────────────────────────────────────
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd ui && node_modules/.bin/tsc --noEmit   # type check (must be clean)
cd ui && node_modules/.bin/vite build     # full build (optional; tsc is enough)
# If node_modules/.bin/* aren't executable:  chmod +x ui/node_modules/.bin/*
```

**Always run tests before committing.** All 18 packages must show `ok`.

---

## Git Workflow

```
dev   ← all new work goes here FIRST
main  ← stable; merge from dev only after tests pass
```

```bash
git checkout dev
# ... make changes ...
cd daemon && ~/.local/go/bin/go test ./...    # must pass
git add <files> && git commit -m "feat: ..."
git checkout main && git merge dev --no-edit
# done — don't push unless the user asks
```

---

## Current State (as of handoff — 2026-03-22)

Everything listed under **Shipped** in `README.md → Roadmap` is complete and
on `main`. The test suite is clean (18 packages, go vet clean, TypeScript clean).
Both `main` and `dev` branches are in sync. Notable shipped features:

- **In-UI config file editor** — Config tab with template support, path-traversal safe
- **In-UI file browser** — browse/upload/download/delete server files
- **Update progress bar** — Settings → Updates polls `PROGRESS:N` markers
- **UFW firewall GUI** — Ports page with full rule CRUD
- **Crash recovery, webhooks, player count, onboarding** — all shipped
- **Security audit remediation** — all HIGH/MEDIUM/LOW findings fixed
- **Tailscale integration, SBOM/CVE scanning, CI/CD pipeline**
- **24 game adapters** (was 7 at original handoff)

---

## Suggested Next Tasks (pick one)

> **Note:** Tasks 1–4 from the original handoff (crash recovery, webhooks,
> player count, onboarding wizard) were all shipped by Codex/Qwen. The tasks
> below are what remains on the roadmap.

Each task below is self-contained and well-scoped.

---

### TASK 1 — Web Push Notifications (Up Next roadmap item)

**What:** Device push alerts for crash/restart events via the Web Push API.
Works when the PWA is installed and even when the browser tab is closed.

**Where:**
- VAPID keys are already generated in `daemon/internal/notifications/push.go`
- Need: subscription endpoint in the API, client-side `PushManager.subscribe()`,
  hook into broker crash/restart events to trigger pushes

---

### TASK 2 — API Keys / Personal Access Tokens

**What:** Long-lived tokens scoped to a user+role for external automation
(CI pipelines, monitoring, custom scripts).

**Where:**
- Add a `tokens` table/map in the auth service
- `POST /api/v1/auth/tokens` to create, `DELETE` to revoke
- Accept `Authorization: Bearer <pat>` on all existing routes
- UI: Settings → Security page, token list with create/revoke

---

### TASK 3 — Server Scheduling

**What:** Per-server cron schedule for automatic start/stop (e.g. "start at
18:00, stop at 23:00 Mon–Fri").

**Where:**
- `daemon/internal/broker/schedule.go` already exists with basic scaffolding
- UI: Schedule tab already exists in `ServerDetailPage.tsx`
- Wire up the existing `robfig/cron` scheduler to broker start/stop methods

---

### TASK 4 — Steam Credentials for Paid Games

**What:** Securely store Steam credentials (encrypted at rest) for games that
require a paid account (DayZ, ARK, etc.). SteamCMD deployment uses stored
credentials automatically.

**Where:**
- Add encrypted credential storage in `daemon/internal/secrets/`
- Pass credentials to SteamCMD Docker container via env vars
- UI: per-server "Steam Account" field in deploy settings

---

## Key Patterns to Follow

### Adding a broker method
```go
func (b *Broker) MyNewThing(ctx context.Context, serverID string) error {
    b.mu.RLock()
    s, ok := b.servers[serverID]
    b.mu.RUnlock()
    if !ok {
        return fmt.Errorf("server %q not found", serverID)
    }
    // ... do work ...
    b.mu.Lock()
    // ... mutate s ...
    b.saveServersLocked()
    b.mu.Unlock()
    return nil
}
```

### Adding an API handler
```go
// In daemon/internal/api/myfeature.go
func (s *Server) myHandler(c *gin.Context) {
    id := c.Param("id")
    result, err := s.cfg.Broker.MyNewThing(c.Request.Context(), id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    s.recordEvent(c, "my_action", id, true, nil)   // writes to audit log
    c.JSON(http.StatusOK, gin.H{"result": result})
}
```

Then register in `registerRoutes()` inside `daemon/internal/api/server.go`.

### UI data fetching pattern
```tsx
const { data, isLoading } = useQuery({
    queryKey: ['my-key', serverId],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/my-endpoint`).then(r => r.data),
    staleTime: 30_000,
    refetchInterval: 60_000,   // optional live polling
});
```

### Mutations (write operations)
```tsx
const mut = useMutation({
    mutationFn: (payload: MyPayload) =>
        api.post(`/api/v1/servers/${id}/my-action`, payload).then(r => r.data),
    onSuccess: () => {
        toast.success('Done!');
        queryClient.invalidateQueries({ queryKey: ['my-key'] });
    },
    onError: (e: any) => toast.error(e.response?.data?.error ?? 'Failed'),
});
```

---

## Common Gotchas

| Problem | Fix |
|---|---|
| `SteamCMDSpec` is a struct, not a pointer | Use `len(manifest.SteamCMD.ExecBins) > 0`, not `manifest.SteamCMD != nil` |
| Tests leave state on disk | Use `cfg.Storage.DataDir = t.TempDir()` in `newTestBroker()` — already fixed |
| Go binary is `~/.local/go/bin/go` | The system `go` is 1.18 and will fail; always use the full path |
| node_modules/.bin/* not executable | `chmod +x ui/node_modules/.bin/*` |
| Daemon imports: use package alias | `daemonconfig "github.com/games-dashboard/daemon/internal/config"` to avoid clash with `config` var names |
| `go test ./...` from wrong dir | Must run from `daemon/`, not project root |

---

## Test Results at Handoff

```
ok  github.com/games-dashboard/daemon/internal/adapters     0.009s
ok  github.com/games-dashboard/daemon/internal/auth         0.817s
ok  github.com/games-dashboard/daemon/internal/backup       0.020s
ok  github.com/games-dashboard/daemon/internal/broker       6.142s
ok  github.com/games-dashboard/daemon/internal/cluster      0.013s
ok  github.com/games-dashboard/daemon/internal/config       0.010s
ok  github.com/games-dashboard/daemon/internal/health       1.106s
ok  github.com/games-dashboard/daemon/internal/metrics      0.016s
ok  github.com/games-dashboard/daemon/internal/modmanager   0.039s
ok  github.com/games-dashboard/daemon/internal/networking   0.020s
ok  github.com/games-dashboard/daemon/internal/rcon         0.010s
ok  github.com/games-dashboard/daemon/internal/sbom         0.017s
ok  github.com/games-dashboard/daemon/internal/secrets      0.019s
ok  github.com/games-dashboard/daemon/internal/telnet       0.007s
ok  github.com/games-dashboard/daemon/internal/webrcon      0.011s
Go vet: clean
UI TypeScript: clean (0 errors)
```

Good luck Tom! Ask questions by reading the code — it's well-commented.
The broker.go is the biggest file (~3100 lines) but each method is self-contained.
