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

**Always run tests before committing.** All 5 packages must show `ok`.

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

## Current State (as of handoff)

Everything listed under **Shipped** in `README.md → Roadmap` is complete and
on `main`. The test suite is clean. Notable recent work:

- **In-UI config file editor** (Config tab on server detail page)
- **Update progress bar** (Settings → Updates; polls log for `PROGRESS:N` markers)
- **UFW firewall GUI** (Ports page)
- **Self-update branch selection + log viewer** (Settings → Updates)
- **Valheim SteamCMD start-command fix**

---

## Suggested Next Tasks (pick one)

Each task below is self-contained and well-scoped.

---

### TASK 1 — Automatic Crash Recovery ⭐ (Near-term roadmap item)

**What:** When a server process dies unexpectedly (not via Stop), auto-restart
it up to N times with exponential back-off.

**Where the crash happens** — `daemon/internal/broker/broker.go` lines 851–870:

```go
// Wait for the process to exit, then update state.
if err := cmd.Wait(); err != nil { ... }

b.mu.Lock()
delete(b.processes, id)
if sv, ok2 := b.servers[id]; ok2 && sv.State != StateStopping {
    sv.State = StateStopped   // ← unexpected exit lands here
    ...
}
b.mu.Unlock()
```

**What to add:**

1. New struct on `Server` (broker.go, around line 50):
```go
CrashRecovery *CrashRecoveryConfig `json:"crash_recovery,omitempty"`
```

2. New type (same file):
```go
type CrashRecoveryConfig struct {
    Enabled    bool `json:"enabled"`
    MaxRetries int  `json:"max_retries"`   // 0 = unlimited
    BackoffSec int  `json:"backoff_sec"`   // base delay; doubles each attempt
}
```

3. Track state on Server:
```go
CrashCount    int        `json:"crash_count"`
LastCrashAt   *time.Time `json:"last_crash_at,omitempty"`
```

4. In the crash-detection block (after `sv.State = StateStopped`), if
   `sv.CrashRecovery != nil && sv.CrashRecovery.Enabled` and state was
   `StateRunning` (not `StateStopping`), increment crash count and schedule
   a restart goroutine with `time.AfterFunc(backoff, func() { b.StartServer(...) })`.
   Back-off formula: `backoff = base * 2^(crashCount-1)`, capped at 5 min.
   Stop retrying when `MaxRetries > 0 && crashCount >= MaxRetries`.

5. Reset `CrashCount` to 0 when the server runs stably for > 10 min
   (check `time.Since(*sv.LastStarted) > 10*time.Minute` at start time).

6. Add `crash_recovery` to `UpdateServerRequest` so the UI can configure it.

7. **UI** — in `ServerDetailPage.tsx`, add a small "Crash Recovery" section
   to the Overview tab or a settings panel. Just a toggle + max-retries input.
   Uses `PUT /api/v1/servers/:id` with `{ crash_recovery: {...} }`.

8. Add `b.saveServersLocked()` after updating crash state (it's already called
   in other mutation paths — just follow the same pattern).

**Tests** — Add a test in `broker_test.go`:
```go
func TestCrashRecovery(t *testing.T) { ... }
```
The test broker uses a temp dir, so no persistent state leaks.

---

### TASK 2 — Discord / Webhook Alerts (Medium-term roadmap)

**What:** POST a JSON payload to a user-configured webhook URL when:
- Server crashes / auto-restarts
- Server start / stop
- Backup completes or fails
- Disk usage > 90%

**Where:**

1. Add `WebhookURL string` to `Server.Config` (already `map[string]any`) —
   or add a dedicated `Notifications *NotificationConfig` struct to `Server`.

2. Create `daemon/internal/notify/webhook.go`:
```go
package notify

type WebhookPayload struct {
    Event     string    `json:"event"`       // "server_crash", "server_start", etc.
    ServerID  string    `json:"server_id"`
    ServerName string   `json:"server_name"`
    Message   string    `json:"message"`
    Timestamp time.Time `json:"timestamp"`
}

func Send(ctx context.Context, url string, payload WebhookPayload) error {
    // json.Marshal + http.Post
}
```

3. Call `notify.Send(...)` from the relevant broker methods.

4. **UI** — Add a "Notifications" section to the server Overview or Settings tab.
   A text input for webhook URL + checkboxes for which events to notify on.

**No new API routes needed** — the webhook URL is just stored in server config
and the daemon calls it internally.

---

### TASK 3 — Player Count Display (Medium-term roadmap)

**What:** Show a live player count on the server overview card by querying the
game server using the Source Query Protocol (used by Valve games: CS2, TF2,
Rust, etc.) and Minecraft's server list ping protocol.

**Where:**

1. Create `daemon/internal/query/source.go` — implements
   [Source Query Protocol](https://developer.valvesoftware.com/wiki/Server_queries#A2S_INFO)
   (UDP, port usually 27015). Returns `{ players_online, max_players, map_name }`.

2. Create `daemon/internal/query/minecraft.go` — implements Minecraft's
   server list ping (TCP, port 25565). Returns `{ players_online, max_players }`.

3. Add `GET /api/v1/servers/:id/players` route in `server.go` and handler
   that calls the right query method based on the adapter ID.

4. **UI** — In `ServerDetailPage.tsx` Overview tab, add a "Players" row that
   fetches `/api/v1/servers/:id/players` every 60 s while server is `running`.

**Adapters that use Source protocol:** `rust`, `counter-strike-2`, `team-fortress-2`,
`garrys-mod`, `left-4-dead-2`, `7-days-to-die`, `squad`, `dayz`, `conan-exiles`

**Adapters that use Minecraft protocol:** `minecraft`

---

### TASK 4 — Onboarding Wizard (Medium-term roadmap)

**What:** First-login modal (3 steps): pick a game → name the server → deploy.
Show this when the user has 0 servers.

**Where:** `ui/src/pages/DashboardPage.tsx` (or wherever the server list is).

This is a pure UI task — no backend changes needed. Use the existing
`POST /api/v1/servers` and `POST /api/v1/servers/:id/deploy` endpoints.

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
ok  github.com/games-dashboard/daemon/internal/adapters   0.008s
ok  github.com/games-dashboard/daemon/internal/auth       (cached)
ok  github.com/games-dashboard/daemon/internal/broker     6.175s  (17/17 pass)
ok  github.com/games-dashboard/daemon/internal/cluster    (cached)
ok  github.com/games-dashboard/daemon/internal/networking (cached)
UI TypeScript: clean (0 errors)
```

Good luck Tom! Ask questions by reading the code — it's well-commented.
The broker.go is the biggest file (~2200 lines) but each method is self-contained.
