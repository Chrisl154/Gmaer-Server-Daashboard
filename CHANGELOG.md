# Changelog

All notable changes to Games Dashboard will be documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0] - 2026-03-22

### Fixed
- **Self-update stale remote ref** (`install.sh`): `git fetch origin` replaced with
  `git fetch --prune origin` so stale `refs/remotes/origin/*` entries (left behind
  after a PR merge re-targets the tip commit) are pruned before the checkout step.
  `git pull --ff-only` replaced with `git reset --hard origin/<branch>` so the
  working tree always matches the remote tip even when fast-forward is not possible.

### Security — Audit Round 2 (all findings remediated)

#### MEDIUM fixes
- **M3 — Firewall From/Comment injection** (`firewall/service.go`): `From` field
  validated against a CIDR/IP regex; `Comment` restricted to safe printable ASCII
  (128 chars max) before being passed to UFW.
- **M4 — `--no-tls` on public addresses** (`cmd/daemon/main.go`): logs a WARN
  when `--no-tls` is active with a non-localhost bind address.
- **M5 — bcrypt cost too low for passwords** (`auth/service.go`): cost raised from
  `DefaultCost` (10) to 12 for all user password hashes (OWASP recommendation).
- **M6 — recovery codes hashed at `MinCost` (4)** (`auth/service.go`): cost raised
  to 12, matching user passwords.
- **M7 — VAPID keys non-atomic write** (`notifications/push.go`): write now uses
  temp file + rename, consistent with `saveUsers()` and `saveServersLocked()`.
- **M8 — no request body size limit** (`api/server.go`): global gin middleware
  applies `http.MaxBytesReader` (1 MB) to all requests before routing.

#### LOW fixes
- **L1 — Tailscale state dir permissions** (`cmd/daemon/main.go`): `os.MkdirAll`
  with mode 0700 now called before `tsnet.Server` is created.
- **L2 — SBOM/CVE report world-readable** (`sbom/service.go`): both `writeSBOM`
  and `writeReport` now use mode 0600 instead of 0644.
- **L3 — audit log scanner 64 KB limit** (`auth/persist.go`): `scanner.Buffer`
  raised to 256 KB so oversized lines don't silently truncate audit log loading.
- **L4 — `patchSettings` re-writes secrets** (`api/server.go`): config snapshot
  zeroes `JWTSecret`, `VaultToken`, `S3.SecretKey`, and `Tailscale.AuthKey`
  before marshaling to disk — secrets provided via env/encrypted files are not
  re-persisted in plaintext.
- **L6 — no rate limit on bootstrap** (`api/server.go`): `POST /system/bootstrap`
  now rate-limited at 1 req/min burst 3 (matching login endpoint precedent).
- **L5 — JWT in WebSocket URL** (`api/server.go`): documented as accepted risk;
  WebSocket clients cannot set Authorization headers; ensure access logs strip
  query strings in production.

### Security — Audit Round 2 HIGH findings

- **H1 — RCON ban reason injection** (`broker/banlist.go`): `reason` parameter in
  `BanPlayer` is now sanitized with `rconDangerousChars.Replace()` before being
  interpolated into the RCON command, stripping `\n`, `\r`, `\x00`, and `;`.
- **H2 — Race condition on `s.users` map** (`auth/service.go`, `auth/persist.go`):
  Added `usersMu sync.RWMutex` to `Service` and applied it at every `s.users`
  access site (14 call sites across Login, CreateUser, UpdateUser, DeleteUser,
  OIDCCallback, SteamCallback, push subscriptions, API key methods, and
  `saveUsers()`). Removed the narrower `subsMu` and `apiKeysMu` fields — all user
  data is now protected by a single consistent mutex, eliminating lock-ordering
  risk. `saveUsers()` now snapshots under `RLock` before marshaling.
- **H3 — Tailscale `AuthKey` JSON tag** (`config/config.go`): Changed
  `json:"auth_key"` → `json:"-"` so the Tailscale auth key cannot leak into
  any JSON API response.
- **H4 — Non-atomic `servers.json` write** (`broker/broker.go`): `saveServersLocked()`
  now writes to `servers.json.tmp` and renames atomically, matching the same
  pattern used by `saveUsers()`. A daemon crash mid-write no longer truncates
  server state.

### Added
- **Tailscale integration** (`tailscale.com/tsnet`) — daemon joins your Tailnet
  as an embedded node; TLS is automatic via `*.ts.net` certs (no cert files
  needed). Configure via `tailscale:` block in `daemon.yaml` or set
  `TAILSCALE_AUTH_KEY` in the environment. Tailscale is the default network
  transport when enabled; set `dual: true` to also expose the standard
  bind-addr listener simultaneously for LAN/WAN access.
- **`--no-tls` CLI flag** — run the daemon over plain HTTP for local testing
  (not for production use).
- **`test/smoke.sh`** — self-contained curl-based smoke test suite (18 checks):
  health, login, wrong-password rejection, RBAC, user lifecycle, API key
  create/use/revoke, persistence, and version endpoint. Configurable via
  `BASE_URL`, `ADMIN_USER`, `ADMIN_PASS`, `GDASH_DATA_DIR` env vars.

### Security — Full audit remediation (63 findings, 2 CRITICAL / 13 HIGH / 33 MEDIUM / 13 LOW)

#### Critical & High fixes
- **JWT default secret** — daemon refuses to start if `JWTSecret` is the default placeholder; random secret auto-generated when none is configured (S01)
- **Default admin password** — Docker Compose no longer has a `changeme` fallback; startup fails with a clear error if `ADMIN_PASSWORD_HASH` is unset (S26)
- **Logout token invalidation** — `ValidateToken` now checks the in-memory blocklist so `Logout` has immediate effect; hourly background sweep evicts expired entries (S01)
- **TOTP re-enrollment account takeover** — `SetupTOTP` requires the existing TOTP code before overwriting the secret; UI shows inline confirmation prompt (S02)
- **Server ID path traversal** — `serverACLMiddleware` validates all `:id` params against `^[a-zA-Z0-9_-]{1,64}$` before any handler runs (S04)
- **Start/stop state machine** — all transitional states guarded; double-start and concurrent-stop races return 409 (S05)
- **Restore without stopping** — `executeRestore` stops the server and waits before touching files (S10)
- **Mod manifest not persisted** — mods written to `servers.json` via `saveServersLocked`; remote mod source verified against allowlist (S11)
- **RCON command injection** — player names sanitized/quoted before RCON interpolation (S12)
- **`doStart` goroutine leak** — server delete cancels the running goroutine via `serverCancels` map (S13)
- **SMTP password leak** — `EmailConfig.Password` tagged `json:"-"`; SMTP errors sanitized with `sanitizeSMTPErr` (S14)
- **Node registration privilege** — `POST /nodes`, `POST /nodes/join-token`, `DELETE /nodes/:nodeId` moved to admin-only group (S17)
- **User management not audited** — create/update/delete user calls emit audit events on success and failure (S20)
- **Unsigned self-update** — `applyUpdate` runs `git verify-commit` on the tip commit; configurable via `updates.require_signed_commits` (S22)
- **CSP `unsafe-inline`** — removed from `script-src`; added `frame-ancestors 'none'`, `base-uri`, `form-action`, `Permissions-Policy`, HSTS (S26)

#### Medium fixes (P17–P35)
- Per-IP token-bucket rate limiter on login and TOTP verify endpoints (P17)
- MFA enforcement gate in `Login()` when `mfa_required: true` (P18)
- 409 Conflict for duplicate server ID and running-server delete (P19)
- Old backup archives deleted from disk on prune, not just from records (P20)
- Backup records persisted to JSON and reloaded at startup (P21)
- Cron expressions validated with `cron.ParseStandard` before storing (P22)
- Console fan-out broadcaster — each WebSocket client gets its own channel (P23)
- Symlink escape prevention in `configFilePath` via `filepath.EvalSymlinks` (P24)
- Upload body limited to 100 MB with `http.MaxBytesReader` (P25)
- Config file backup (`.bak`) before every write (P26)
- `RestartServer` returns a `*Job` immediately; polling goroutine runs in background (P27)
- Notification throttle — 5-minute cooldown per server+event to prevent spam loops (P28)
- `GetAuditLog` paginated with offset+limit, newest-first, mutex-protected (P29)
- Backup restore verifies SHA-256 checksum before overwriting server files (P30)
- Nginx CSP tightened: `img-src`, `font-src` restricted; HSTS added (P31)
- Dockerfile creates data directories and sets ownership before switching to nonroot (P32)
- CI integration test step no longer gated by `|| true` (P34)
- CVE scan wrapped in 5-minute context timeout (P35)
- GPG commit verification in self-update flow (P36/S22 overlap)
- OpenAPI spec updated with all missing routes, schemas, and tags (P33)

#### Low fixes (P37–P55)
- Recovery codes hashed with bcrypt at rest; comparison uses bcrypt (timing-safe) (P37/P38)
- Hourly background goroutine evicts expired blocklist entries (P39)
- `CloneServer` deep-copies config via JSON round-trip to handle nested values (P40)
- File browser delete now supports directories via `os.RemoveAll` (P43)
- Config file writes are atomic: temp file + `os.Rename` (P44)
- Log lines capped at 10,000 to prevent memory exhaustion (P45)
- Mod install blocked while server is running or starting (P46)
- File-backed ban list for games without RCON support; persisted to `{data_dir}/banlists/` (P47)
- Crash notifications suppressed during the 90-second startup grace window (P48)
- Webhook URL redacted from error log messages (P49)
- Dead push subscriptions auto-removed on 410 Gone response (P50)
- `GET /sbom` returns 404 instead of empty placeholder before first scan (P51)
- Current daemon binary backed up to `.bak` before self-update runs (P52)
- Setup wizard skip buttons disabled while async operations are in-flight (P53)
- Dashboard disk warning uses explicit `!= null` check on `disk_pct` (P54)
- `diagnoseServer` properly cancels timeout contexts — no goroutine leak (P55)

### Added

#### Auth persistence — users, API keys, TOTP, push subscriptions, audit log (`daemon/internal/auth/persist.go`, `auth/service.go`, `cmd/daemon/main.go`)
- All user state now survives daemon restarts. Nothing is lost on reboot.
- New `auth/persist.go` introduces a `storedUser` struct (with explicit json tags for fields marked `json:"-"` on the API-facing `User` struct) so `PasswordHash`, `TOTPSecret`, `RecoveryCodes`, and `APIKey.Hash` are written to disk without ever appearing in API responses.
- `saveUsers()` atomically writes all users to `{data_dir}/users.json` (temp file + `os.Rename`). Called asynchronously after every mutation: login, create/update/delete user, TOTP enroll/regenerate, API key create/revoke/use, push subscription add/remove, OIDC/Steam first-login user creation.
- `loadUsers()` reads `users.json` at startup, populating the in-memory map before any requests are served.
- `mergeAdminFromConfig()` ensures the admin from `daemon.yaml` is always present with the latest password hash — `daemon.yaml` remains authoritative for the admin account.
- Audit log entries appended to `{data_dir}/audit.log` as newline-delimited JSON after each event (non-blocking goroutine). Up to 10,000 entries loaded into memory on startup; on-disk file retains full history.
- `auth.Config.DataDir` wired from `cfg.Storage.DataDir` in `cmd/daemon/main.go`.



#### Server scheduling (`daemon/internal/broker/schedule.go`, `broker.go`, `ui/src/pages/ServerDetailPage.tsx`)
- `StartSchedule` and `StopSchedule` cron fields added to the `Server` struct and
  `UpdateServerRequest`; persisted in servers.json and survives daemon restarts.
- New `broker/schedule.go` mirrors the existing auto-update cron pattern — dedicated
  `schedCron` instance with `schedEntries map[string][2]cron.EntryID` and `schedMu` mutex to
  avoid lock-ordering issues with `b.mu`.
- `initScheduler` re-registers all existing server schedules at startup.
- `scheduleStartStop` / `unscheduleStartStop` called from `UpdateServer` and `DeleteServer`.
- New **Schedule** tab in Server Detail page:
  - Separate Auto-start and Auto-stop cards, each with a monospace cron input, live
    human-readable preview (e.g. "Every weekday at 18:00"), and quick-pick preset buttons.
  - Save wires to `PUT /servers/:id` with the two schedule fields.

#### API keys / personal access tokens (`daemon/internal/auth/service.go`, `daemon/internal/api/server.go`, `ui/src/pages/SettingsPage.tsx`)
- New `APIKey` struct stored per-user on `auth.User`; each key carries a name, 12-char display
  prefix, SHA-256 hash (raw token never stored), scoped roles, created-at, optional expiry,
  and a last-used timestamp updated on every authenticated request.
- Token format: `gdash_<24-byte base64url>` (32 chars after prefix) — clearly identifiable as a
  Games Dashboard token and safe to search for in logs.
- `ValidateToken` now branches on the `gdash_` prefix — API keys are validated via SHA-256 hash
  lookup across all users; JWTs follow the existing path unchanged.
- New service methods: `CreateAPIKey`, `ListAPIKeys`, `RevokeAPIKey`; role scoping prevents
  privilege escalation (key roles are capped to the creator's roles).
- New API routes (any authenticated user, own keys only):
  - `GET    /api/v1/auth/api-keys`          — list my keys (hash omitted)
  - `POST   /api/v1/auth/api-keys`          — create key (raw token returned once)
  - `DELETE /api/v1/auth/api-keys/:keyId`   — revoke key
- Settings → Users & Auth → **API Keys** card: table of existing keys (name, prefix, roles,
  created/last-used/expires), "New key" button opens a modal (name + optional expiry date),
  and the raw token is shown in a green one-time reveal banner with copy button.

#### 2FA enrollment QR-code flow (`ui/src/pages/SettingsPage.tsx`)
- TOTP setup step now renders an actual scannable QR code (`QRCodeSVG` from `qrcode.react`)
  instead of the raw `otpauth://` URI string.
- Manual-entry secret displayed below the QR code with 4-character groups and a one-click
  copy button for users who cannot scan.
- After `verifyTOTP` succeeds, a **Recovery Codes Modal** is shown with the 10 one-time codes:
  - 2-column code grid in monospace with highlighted styling.
  - "Copy all" button writes a formatted block (header + date + codes) to the clipboard.
  - "Download .txt" saves `games-dashboard-recovery-codes.txt` — numbered list with instructions.
  - "I've saved my recovery codes" button dismisses and refreshes the remaining-codes count.
- Two-Factor Authentication card shows **remaining recovery code count** (red when ≤ 2).
- **Regenerate recovery codes** flow: confirm with current TOTP code → inline form → generates
  new codes → shows the same Recovery Codes Modal with fresh codes.

#### Web Push notifications (`daemon/internal/notifications/push.go`, `daemon/internal/auth/service.go`, `daemon/internal/api/server.go`, `ui/src/sw.ts`, `ui/src/pages/SettingsPage.tsx`)
- VAPID key pair generated automatically on first run and persisted to `{data_dir}/vapid_keys.json`.
- `notifications.Service.SetPush(vapidKeys, getSubscriptions)` wires push sending into the existing
  `Send(event, serverName, message)` flow — every `server.crash`, `server.restart`, and `disk.warning`
  event fans out to all registered device subscriptions alongside webhook and email.
- Per-user `PushSubscriptions []PushSubscription` stored on `auth.User`; subscriptions survive
  daemon restarts as part of the user state.
- New API routes:
  - `GET  /api/v1/push/vapid-key`       — returns the VAPID public key (base64url) for browser subscribe calls
  - `POST /api/v1/push/subscribe`       — registers a push subscription for the authenticated user
  - `DELETE /api/v1/push/subscribe`     — unregisters a subscription by endpoint
- Service worker (`ui/src/sw.ts`) switched to `injectManifest` strategy with a custom TypeScript
  service worker that handles `push` events (shows a notification) and `notificationclick` events
  (focuses an existing tab or opens a new window at the notification URL).
- Settings → Notifications → **Push Notifications** card: shows permission state; "Enable" button
  requests browser permission, fetches the VAPID key, subscribes via `PushManager`, and saves the
  subscription to the daemon; "Disable" button unsubscribes and removes the server-side record.
- Added `github.com/SherClockHolmes/webpush-go v1.3.0` dependency.

### Changed
- **`daemon/Dockerfile`** — switched runtime base from `gcr.io/distroless/static-debian12:nonroot`
  to `debian:bookworm-slim` and added a `docker:27-cli` build stage so the daemon can shell out
  to the `docker` binary for container lifecycle management. The `nonroot` user is added to the
  `docker` group (GID 999) and the entrypoint uses `USER nonroot` (not `USER nonroot:nonroot`) so
  supplemental group membership is inherited by the process.

### Fixed
- **Health check killed servers during slow startup** — game servers such as Minecraft JVM can take
  60–90 s to bind their ports. `checkServerHealth` now skips TCP/UDP probes for the first 90 seconds
  after `LastStarted` is set, preventing false-positive `error` transitions on slow-starting servers.
  (`daemon/internal/broker/broker.go`)
- **`POST /servers/:id/deploy` with no body returned `{"error":"EOF"}`** — `deployServer` now
  treats an absent request body as "use the server's configured `deploy_method`", so a bare
  `POST /deploy` works without a JSON body. (`daemon/internal/api/server.go`,
  `daemon/internal/broker/broker.go`)
- **`POST /servers/:id/stop` returned HTTP 500 for a non-running server** — "server not running" is a
  client-side state conflict; the handler now returns HTTP 409 Conflict in that case instead of 500.
  (`daemon/internal/api/server.go`)
- **CLI: `gdash node token` returned 404** — `cli/cmd/main.go` was calling `POST /api/v1/nodes/token`
  but the API registers the route as `POST /api/v1/nodes/join-token`. The `gdash node token` command
  now calls the correct path.
- **MFA login: no way to use a recovery code** — the login page only showed a TOTP code input when
  MFA was required. Added a "Use a recovery code instead" toggle in `LoginPage.tsx` that switches to
  a free-text recovery code input and sends `recovery_code` to the backend. `authStore.ts` updated
  to forward the field.
- **Auth: unused `mfaRequired` variable** — `auth/service.go` computed
  `mfaRequired := user.TOTPEnabled && s.cfg.MFARequired` and then discarded it with
  `_ = mfaRequired`. Both the declaration and the dead-code line have been removed.
- **Broker: `GetServer` data race** — `GetServer` returned a pointer directly into the live server
  map, allowing callers to mutate broker state without holding the lock. It now returns a shallow
  copy so external code cannot alias the live record.

#### PWA — installable web app (`ui/vite.config.ts`, `ui/index.html`, `ui/public/icons/`)
- Integrated `vite-plugin-pwa` with `workbox` service-worker generation.
- Web App Manifest declares `name`, `short_name`, `theme_color`, `display: standalone`, and 192×512 icon set.
- Service worker uses `NetworkFirst` for all `/api/` requests (10 s timeout, 5 min cache) and precaches all static assets for offline shell.
- `index.html` updated with `theme-color`, Apple Web App meta tags, and touch icon link for iOS Add-to-Home-Screen support.
- App icons added at `ui/public/icons/icon-192.png` and `icon-512.png`.

#### CI/CD pipeline (`.github/workflows/ci.yml` — full rewrite)
- Fixed branch triggers: `develop` → `dev`; added `dev` to pull-request triggers.
- Fixed `npm run test -- --run` for non-interactive CI execution.
- Upgraded `golangci-lint-action` v4 → v6.
- Upgraded `azure/setup-helm` v3 → v4.
- Removed broken Docker-in-Docker service block from `integration-tests`; GitHub Actions runners include Docker natively.
- Added separate `docker/metadata-action` step for the UI image (was missing; only the daemon had one).
- Added macOS arm64 CLI build (`gdash-darwin-arm64`).
- Added QEMU setup for multi-arch Docker builds (linux/amd64 + linux/arm64).
- Replaced CycloneDX CLI with `cyclonedx-gomod` for Go-native SBOM generation.
- Fixed `cache-dependency-path` to point at `go.sum` files instead of `go.mod`.
- Updated GitHub Release artifact list to include `daemon-linux-arm64` and `gdash-darwin-arm64`.

### Added

#### Email notifications (`daemon/internal/notifications/service.go`, `config/config.go`, `ui/src/pages/SettingsPage.tsx`)
- `EmailConfig` in config: SMTP host/port, username, password, from address, to list, implicit-TLS flag.
- `notifications.Send()` fires email in a separate goroutine alongside webhooks; `Test()` checks both.
- Two SMTP paths: STARTTLS (`smtp.SendMail`, port 587) and implicit TLS (`tls.DialWithDialer` +
  `smtp.NewClient`, port 465).
- Settings page: full SMTP card with show/hide password and comma-separated recipient list; test
  button enabled when either webhook URL or email is configured.

#### Network I/O monitoring (`daemon/internal/broker/metrics.go`, `broker.go`, `ui/src/pages/DashboardPage.tsx`, `ServerDetailPage.tsx`, `ui/src/types/index.ts`)
- `NetInKbps` / `NetOutKbps float64` on `Server` and `ServerMetricSample`.
- Docker stats format extended with `{{.NetInput}}`/`{{.NetOutput}}`; `parseDockerBytes` handles `B`,
  `kB`, `MB`, `GB` suffixes.
- Cumulative byte totals tracked in `prevNetBytes` / `prevNetTime`; kbps rate computed each 15 s cycle.
- Dashboard resource table: "Network" column with auto-scaled ↓/↑ Kbps/Mbps display.
- Server detail metrics tab: second `LineChart` for net in/out in Mbps (hidden until data arrives).

#### Server cloning (`daemon/internal/broker/broker.go`, `daemon/internal/api/server.go`, `ui/src/pages/ServerDetailPage.tsx`)
- `POST /api/v1/servers/:id/clone` body `{"name":"…"}` — deep-copies config, ports, and resources
  into a new server with a `crypto/rand` 6-byte hex ID; clone starts in `stopped` state.
- UI: "Clone" button in server header; `CloneModal` navigates to the new server on success.

#### In-app guided diagnostics (`daemon/internal/api/server.go`, `ui/src/pages/ServerDetailPage.tsx`)
- `GET /api/v1/servers/:id/diagnose` — 7 heuristic checks: Docker reachable, last error present,
  crash-loop detection, disk ≥ 90 %, port conflicts, RAM headroom, running-state verification.
  Returns `[]DiagnosticFinding{Severity, Title, Detail, Fix}`.
- UI: "Diagnose" button in Overview tab; `DiagnosticsModal` with severity-coloured finding cards and
  a re-run button. Red error banner when state = `error` includes a "Diagnose" CTA.

#### TOTP recovery codes (`daemon/internal/auth/service.go`, `daemon/internal/api/server.go`, `ui/src/pages/SecurityPage.tsx`, `ui/src/store/authStore.ts`)
- 10 single-use codes generated at enrollment; `consumeRecoveryCode` burns a code atomically.
- `Login` accepts `recovery_code` as an alternative to `totp_code`.
- `RegenerateRecoveryCodes` requires TOTP re-auth before issuing a fresh set.
- API: `GET /auth/totp/recovery-codes` (returns count), `POST /auth/totp/recovery-codes/regenerate`.
- UI: `RecoveryCodesModal` with downloadable `.txt`; TOTP card shows remaining count + regenerate flow.

#### System resource pre-flight check (`daemon/internal/api/server.go`, `ui/src/components/CreateServerWizard.tsx`)
- `GET /api/v1/system/resources` — reads `/proc/meminfo` (RAM) and `syscall.Statfs` (disk); returns
  `cpu_cores`, `total_ram_gb`, `free_ram_gb`, `total_disk_gb`, `free_disk_gb`.
- `ResourceWarning` amber banner in the Create Server wizard when free RAM/disk/CPU falls below the
  selected game's minimum requirements (non-blocking advisory only).

#### Per-user ACLs (`daemon/internal/broker/broker.go`, `daemon/internal/api/server.go`, `ui/src/pages/SettingsPage.tsx`)
- `AllowedServers []string` per user; empty = unrestricted (admin behaviour).
- Role-based route enforcement: non-admin requests to server routes are filtered by `AllowedServers`.
- ACL management UI in Settings → Users.

#### Steam account authentication (`daemon/internal/auth/steam.go`, `service.go`, `daemon/internal/api/server.go`, `ui/src/pages/LoginPage.tsx`, `ui/src/store/authStore.ts`)
- OpenID 2.0 `check_authentication` verify; replay nonce map (`steamStates sync.Map`).
- API: `GET /auth/steam/login` → redirect; `GET /auth/steam/callback` → JWT + redirect to frontend.
- Login page: "Sign in with Steam" button; `loginWithToken` authStore action for token-in-URL flow.
- 18 unit tests; all pass without network access.

#### Node-install mode (`install.sh`, `cli/cmd/main.go`)
- `install.sh --mode=node`: worker-only install — Docker + SteamCMD + daemon; no Node.js, UI, nginx,
  or admin wizard. Daemon binds `0.0.0.0:<port>` for master reachability.
- `gdash node token`: calls `POST /api/v1/nodes/join-token`, prints token + usage hint.
- `gdash node add --token <tok>`: sends `join_token` in body for cluster validation.

#### Automatic Crash Recovery
- **`auto_restart` per-server flag** — When enabled, the broker automatically restarts a game server process if it exits unexpectedly. Configurable: `auto_restart` (bool), `max_restarts` (default 3), `restart_delay_secs` (default 10).
- **Exponential back-off loop** — `doStart` now runs as a restart loop. After each unexpected exit it increments `restart_count`, waits `restart_delay_secs`, and re-launches. After `max_restarts` attempts it marks the server `error`.
- **Crash counter reset** — If the server runs cleanly for more than 60 seconds the restart counter resets, so isolated crashes don't accumulate toward the limit over time.
- **`restart_count` and `last_crash_at`** — Both fields are visible on the server object (API + UI) so users can see how many times a server has restarted.
- **API fields** — `CreateServerRequest` and `UpdateServerRequest` both expose the auto-restart fields.

#### Plain-English Error Messages
- **`last_error` field on `Server`** — Set whenever a server transitions to `StateError`; cleared automatically when it returns to `StateRunning`.
- **`setServerError()` helper** — All 9 error-transition sites in `broker.go` now call this helper with a human-readable explanation (e.g. "SteamCMD could not find the game files — your disk may be full…").
- **Error banner on `ServerCard`** — A red inline banner appears below the status badge when `state === 'error'` and `last_error` is set, showing the first two lines of the message.
- **"What does this mean?" modal** — A `HelpCircle` button next to the error banner opens `ErrorHelpModal` with the full error text plus generic next-steps. Dismissible via button or backdrop click.

#### Disk Space Warning Banners
- **`diskUsagePct(path)`** — New function in `metrics.go` using `syscall.Statfs` to compute disk usage for any path on Linux.
- **`disk_pct` on `Server`** — Sampled every 15 seconds for all servers (running or stopped) and included in both the server list API response and `ServerMetricSample`.
- **Daemon console warnings** — `checkDiskWarning()` emits a console warning event at 80% and again at 95% full; throttled to once per hour per server via `diskWarnedAt` map.
- **Color-coded disk bar on `ServerCard`** — Green below 70%, yellow 70–84%, orange 85–94%, red ≥ 95%.
- **Dashboard-level sticky banner** — `DashboardPage` shows a warning/critical banner listing every server at ≥ 85% full with their current percentage. Banner is orange at warning level and red when any server hits ≥ 95%.

#### In-UI File Browser
- **`GET /api/v1/servers/:id/files`** — lists directory entries (name, is_dir, size, modified, path) at an arbitrary path inside the server's install dir. Path-traversal-safe via the existing `configFilePath` helper.
- **`GET /api/v1/servers/:id/files/download`** — serves a file as a raw download attachment. Uses the existing `ReadConfigFile` broker method.
- **`POST /api/v1/servers/:id/files/upload`** — accepts a `multipart/form-data` upload of one or more files (`field name: file`) and writes them to the destination directory (`?dir=` query param).
- **`DELETE /api/v1/servers/:id/files`** — deletes a single file (directories blocked). Path-traversal-safe.
- **Broker methods** — `ListFiles`, `UploadFile`, `DeleteFile` added to `broker.go`, all using path-traversal protection consistent with the config file editor.
- **FilesTab component** — new 8th tab on the Server Detail page. Shows a breadcrumb navigator, sortable file table (dirs first, then files alphabetically), size formatting (`formatBytes`), upload button, per-file download and delete buttons, and a delete-confirmation modal.

#### Getting Started Checklist (Onboarding)
- **`GettingStartedChecklist` component** — A dismissible panel on the Dashboard with four steps: Add your first server, Take a backup, Set up crash notifications, Invite a user.
- **Auto-progress** — The "Add server" step is automatically marked done when `serverCount > 0` (server list updates every 15 s).
- **Per-step toggle** — Each step has a checkbox button; clicking marks it done / undone. State persisted to `localStorage` (`gdash_checklist_steps`).
- **Collapsible header** — Clicking the "Getting Started" header collapses/expands the step list. A `n/4` progress badge is always visible.
- **Dismiss** — The × button permanently hides the checklist (persisted via `localStorage` key `gdash_checklist_dismissed`).
- **Action links** — Each step has a "Go to …" button linking to the relevant page.

#### Live Resource Overview Table (Dashboard)
- **`cpu_pct` / `ram_pct` mirrored onto `Server`** — The metrics collector now copies the latest CPU and RAM percentages from the ring buffer directly onto the `Server` struct each cycle, so the servers-list endpoint carries live metrics without extra API calls.
- **`ResourceTable` component** — Replaces the old server card grid on the dashboard. Each server is a row with columns: Server (icon + name + game) | Status | CPU bar | RAM bar | Disk bar | Allocated (cores / RAM / disk).
- **`MiniBar` component** — Inline mini progress bar (thin colored fill + numeric label) used in the resource table cells.
- **Stopped-server handling** — CPU and RAM show `—` for non-running servers; Disk is always shown.
- **Live badge + 15 s refresh** — Table header shows "Updates every 15 s" and a "Live" badge; driven by the existing 15-second `react-query` refetch interval.
- **Click-to-navigate** — Clicking any row navigates to the server detail page.

#### Installer — Interactive TUI Wizard
- **Full whiptail TUI** — The installer now launches a 5-screen interactive wizard when run in a terminal (including via `curl | bash`). Screens: Network & Paths → Admin Account → Storage & Backup → Container Runtimes → Review & Confirm. Falls back to plain readline prompts if `whiptail`/`dialog` is unavailable.
- **Docker CE installation** — New "Container Runtimes" screen asks whether to install Docker CE (recommended — enables the Docker deploy method for 19 of 24 games) and optionally k3s (lightweight Kubernetes). Both can also be set via `GDASH_INSTALL_DOCKER=true` / `GDASH_INSTALL_K8S=true` for non-interactive installs.
- **Non-interactive mode** — All wizard settings are overridable via environment variables (`GDASH_INSTALL_DIR`, `GDASH_HOST`, `GDASH_HOSTNAME`, `GDASH_ADMIN_PASS`, `GDASH_INSTALL_DOCKER`, `GDASH_INSTALL_K8S`, etc.) when `GDASH_NONINTERACTIVE=1` is set. Documented in README.
- **SteamCMD** — Now installed automatically by the installer (required for Valheim, CS2, Rust, ARK, and all other Steam-based servers).
- **Java 21 LTS** — Now installed automatically (required for Minecraft and other JVM-based game servers).

#### Servers Page — UI Improvements
- **Visual game picker** — "New Server" now opens a 2-step modal. Step 1 shows all 24 games as colour-themed poster cards with filter tabs (All / SteamCMD / Manual / Docker). Clicking a game advances to step 2.
- **Deploy method filtering** — The game grid filters live as you click the deploy method tabs; only games that support the selected method are shown. The deploy method in step 2 is pre-populated to match your filter choice.
- **Delete server** — Hovering a server poster now shows a trash icon in the top-right corner of the hover overlay. Clicking it opens a confirmation modal before calling `DELETE /api/v1/servers/:id`.

#### Logs Page
- **New Logs page** (`/logs`) — Four tabs:
  - **Server Logs** — server picker sidebar + live console tail polling `GET /api/v1/servers/:id/logs` every 5 s, with line-count selector (100/200/400/800) and manual refresh.
  - **Events** — lifecycle events (start, stop, deploy, backup) from the audit log.
  - **Security** — authentication events only (logins, failures) from the audit log.
  - **Audit Trail** — full audit log in reverse-chronological order, auto-refreshing every 20 s.
- **Persistent event log** — Broker writes `system`, `error`, and `deploy` console messages to `{data_dir}/servers/{id}/logs/gdash-events.log` so they survive restarts and appear in the Server Logs tab even before a game process starts.
- **Audit recording** — `RecordEvent()` wired into all major API handlers: create/delete server, start/stop/restart, deploy, backup/restore, mod install/uninstall/rollback.

#### Adapter Fixes
- **Valheim** — Fixed `start_command` to export `LD_LIBRARY_PATH=./linux64` and `SteamAppId=892970` before launching, resolving server startup failure.
- **"Not deployed" guard** — `doStart()` now checks for the install directory before executing and emits a clear error message instead of a cryptic `chdir` failure.
- **Docker deploy method** — Added `docker` to `deploy_methods` in all 19 adapter manifests that have a configured Docker image (Valheim, Minecraft, CS2, Rust, TF2, GMod, ARK, DayZ, DST, Project Zomboid, Terraria, Factorio, L4D2, Risk of Rain 2, Squad, 7DTD, Among Us, Dota 2, Conan Exiles).
- **All 24 adapters audited** — Fixed broken `kill -SIGTERM $(cat /tmp/*.pid)` stop/restart commands across all manifests; replaced with `pkill -SIGTERM -f <binary> || true`.
- **Eco** — Fixed `start_command` (was incorrect `mono EcoServer.exe`; now `./EcoServer`).
- **DayZ** — Added `LD_LIBRARY_PATH` and explicit paths to `start_command`.
- **Minecraft** — Auto-accepts EULA (`echo 'eula=true' > eula.txt`) before starting.
- **Risk of Rain 2** — Added `wine` prefix to `start_command`.

### Fixed
- **Installer TUI broken under `curl | bash`** — When piped, bash reads the script from stdin; `read` calls were consuming lines of the script itself as "user input". Fixed by redirecting all `read` commands to `/dev/tty`. Added `HAVE_TTY` detection (`[[ -r /dev/tty ]]`) so the readline path is only used when a keyboard is actually available, and a clear no-tty fallback message otherwise.
- **Installer unicode rendering** — Replaced `━` box-drawing characters with plain ASCII `=`/`-` so the header renders on all terminals regardless of locale.
- **`tailFile` ring-buffer returning empty strings** — When a log file had fewer lines than `maxLines`, `start := idx % n` pointed to the uninitialized portion of the ring buffer, returning empty strings. Fixed to `start := 0` when `total <= n`.

---

### Added
- **`test-live.sh`** — One-liner end-to-end test runner. Installs all dependencies (Go 1.22, Node.js 20 LTS, Trivy, Python packages) in userspace, clones the repo, builds both binaries, generates TLS certs, starts the daemon, and runs the full API + CLI + unit test suite. Minimum requirement: Ubuntu 24.04 with internet access and `bash`.
- **24 game adapters** — Expanded from 7 to 24: added 7 Days to Die, Among Us (Impostor), ARK Survival Ascended, Conan Exiles, Counter-Strike 2, DayZ, Don't Starve Together, Dota 2, Factorio, Garry's Mod, Left 4 Dead 2, Project Zomboid, Risk of Rain 2, Rust, Squad, Team Fortress 2, and Terraria — alongside the existing Eco, Enshrouded, Minecraft, Palworld, Riftbreaker, Satisfactory, and Valheim.

### Fixed
- **`GET /api/v1/version` now public** — The version endpoint was registered inside the authenticated `v1` middleware group, causing integration tests and unauthenticated health checks to receive `401 Unauthorized`. Moved to the public route section alongside `/healthz` and `/metrics`.
- **`secrets/manager.go` `saveKey()` directory bug** — `os.MkdirAll(fmt.Sprintf("%s/..", m.cfg.KeyFile), 0700)` caused Go to create `master.key` as a directory (before resolving `..`). Fixed to use `filepath.Dir(m.cfg.KeyFile)`.
- **UI missing entry files** — `ui/index.html`, `ui/src/main.tsx`, and `ui/src/index.css` were absent from the repository, preventing Vite from building. Standard boilerplate files added.

---

### Added

#### Settings Page — Storage, Networking, Monitoring
- **`GET /api/v1/admin/settings`** — New admin endpoint that returns a secrets-free snapshot of the live daemon config: bind address, shutdown timeout, data dir, log level, storage (data dir, NFS mounts list, S3 endpoint/bucket/region), backup (schedule, retention, compression), metrics (enabled, path), and cluster (enabled, intervals).
- **`PATCH /api/v1/admin/settings`** — Allows updating the safe mutable subset (log level, backup schedule/retention/compression, metrics enabled/path, cluster health-check interval and node timeout). Writes the updated config back to `daemon.yaml` when `ConfigPath` is set; in-memory update succeeds regardless.
- **`api.Config`** extended with `DaemonCfg *config.Config` and `ConfigPath string`; both wired in `daemon/cmd/daemon/main.go`.
- **Settings → Storage section** (`ui/src/pages/SettingsPage.tsx`) — Displays live data directory path, NFS mount list (server/path/mount-point/options), optional S3 config (no credentials). Backup subsection is fully editable: cron schedule, retention days, compression algorithm; saves via PATCH.
- **Settings → Networking section** — Displays bind address and shutdown timeout from live config; read-only with a note to edit `daemon.yaml` and restart.
- **Settings → Monitoring section** — Log level select (debug/info/warn/error), Prometheus metrics toggle + path input, and cluster health-check interval / node timeout editors (shown only when cluster is enabled). All fields save via PATCH.
- **Loading skeleton** (`SectionSkeleton`) shown while settings are fetching.
- **Bug fix** — `UsersSection` and `CreateUserModal` were calling `/api/v1/auth/users` instead of the correct `/api/v1/admin/users` route.
- **TypeScript types** — Added `DaemonSettings`, `DaemonSettingsNFSMount`, `DaemonSettingsS3`, and `SettingsPatch` to `ui/src/types/index.ts`.

#### Test Coverage
- **Cluster manager unit tests** (`daemon/internal/cluster/manager_test.go`) — 26 tests covering `Register` (new + duplicate address re-registration), `Deregister`, `Heartbeat` (fields + last_seen update), `List` (snapshot isolation), `Get`, `BestFit` (best CPU, excludes offline/draining/insufficient-capacity nodes, empty-cluster case), `AllocateOnNode`/`ReleaseFromNode` (increment, decrement, floor-at-zero), `Node.Available`, `Node.CanFit` (per-resource failure cases), and `checkNodes` timeout behaviour. Removed phantom `encrypt-io/encrypt` dependency from `go.mod` and generated `go.sum`.
- **Networking service unit tests** (`daemon/internal/networking/service_test.go`) — 17 tests covering `ReservePort`/`ReleaseServer` conflict detection and selective cleanup, `isPortAvailable` for TCP/UDP free and in-use ports and unsupported protocols, `probeReachability` with no URL / reachable true/false / server error / bad JSON, `ValidatePorts` integration (free port + reservation priority), and `FindFreePort` for TCP and UDP.
- **UI type-shape tests** (`ui/src/types/index.test.ts`) — Extended with 4 new test suites for `NodeCapacity`, `Node` (required fields, all valid status values, optional fields absent), and `RegisterNodeRequest`. Total UI type tests: 10.
- **Auth store tests** (`ui/src/store/authStore.test.ts`) — 13 Vitest tests using `vi.mock` for the `api` module, covering initial state, `login` success / MFA-required / header injection / error-rejection, `logout` state clearing / header removal / no-op when unauthenticated, `checkAuth` header restore / absent when no token, `verifyTOTP` MFA flag reset / endpoint call, and `setupTOTP` return value.

#### Real Process & Deployment Execution
- **Game server process management** (`daemon/internal/broker/broker.go`) — `doStart` now executes the adapter's `StartCommand` via `sh -c` in the server's `InstallDir`. Stdout/stderr are piped line-by-line into the server's console channel as JSON-encoded messages. The PID is stored on the `Server` record. `doStop` sends `SIGTERM` to the running process, waits up to 15 seconds, then `SIGKILL`s if it hasn't exited. `RestartServer` polls until the process has fully stopped before re-launching. `Broker.processes` map tracks live `*exec.Cmd` handles per server ID.
- **Command template expansion** — `StartCommand` strings from adapter manifests support `{name}`, `{id}`, `{port}`, `{install_dir}`, `{adapter}` placeholders, expanded before execution.
- **Real SteamCMD deployment** (`deploySteamCMD`) — Locates `steamcmd` via `exec.LookPath`; runs with `+login anonymous +force_install_dir {dir} +app_update {appID} validate +quit`. Supports beta branch and beta password. Streams stdout line-by-line into the job's progress messages.
- **Real manual deployment** (`deployManual`) — Downloads the `archive_url` via HTTP with a 30-minute timeout. Computes SHA-256 while streaming to a temp file; verifies against `checksum` if provided. Extracts `.tar.gz` to `InstallDir` with path-traversal protection.
- **Real remote reachability probe** (`daemon/internal/networking/service.go`) — `probeReachability` now calls `GET {ValidatorURL}/probe?proto=<proto>&port=<port>` and parses `{"reachable": bool, "latency_ms": n}` JSON from the response. Returns `(false, 0)` immediately when `ValidatorURL` is not configured (avoids spurious "not reachable" results).

#### Broker Improvements
- **Cluster-aware server placement** (`daemon/internal/broker/broker.go`) — `CreateServer` now calls `clusterMgr.BestFit` when no explicit `node_id` is requested, automatically selecting the online node with the most available capacity. `DeleteServer` calls `ReleaseFromNode` to return resources to the correct node. Gracefully falls back to local-host placement when no node can satisfy the request.
- **Real backup execution** — `ListBackups`, `TriggerBackup`, and `RestoreBackup` in the broker now delegate to the `backup.Service` instead of using in-memory placeholders. Backup paths are derived from `Server.BackupConfig.Paths` (with `InstallDir` as fallback); real SHA-256 checksums and byte counts are stored. The backup service cron scheduler is started as a background goroutine at daemon startup.
- **Real server log reading** — `GetServerLogs` reads actual log files from the server's `install_dir`: checks `logs/latest.log`, `server.log`, `output.log`, and any `logs/*.log` in preference order. Uses a circular-buffer tail algorithm to return the last N lines (default 100) without loading entire files into memory.

#### Cluster / Multi-Node Support
- **Cluster manager** (`daemon/internal/cluster/manager.go`) — Node registry with `Register`, `Deregister`, `Heartbeat`, `List`, `Get`, `BestFit` (best-available-capacity placement), `AllocateOnNode`, `ReleaseFromNode`, and background health-check loop. Nodes mark themselves offline after a configurable timeout (`node_timeout`) with no heartbeat; a periodic HTTP ping of each node's `/healthz` restores `online` status automatically.
- **Node API** (`daemon/internal/api/nodes.go`) — Five REST endpoints: `GET /api/v1/nodes`, `POST /api/v1/nodes` (register), `GET /api/v1/nodes/:nodeId`, `DELETE /api/v1/nodes/:nodeId` (deregister), `POST /api/v1/nodes/:nodeId/heartbeat`.
- **Config** (`daemon/internal/config/config.go`) — New `ClusterConfig` struct (`enabled`, `health_check_interval`, `node_timeout`) wired into top-level `Config.Cluster`.
- **Broker** (`daemon/internal/broker/broker.go`) — `clusterMgr *cluster.Manager` field; `ClusterManager()` accessor; `Server.NodeID` and `CreateServerRequest.NodeID` fields for node-aware placement.
- **Main** (`daemon/cmd/daemon/main.go`) — Cluster manager started as background goroutine when `cluster.enabled: true`.
- **Nodes UI** (`ui/src/pages/NodesPage.tsx`) — Cluster node management page with per-node CPU/RAM/disk utilisation bars, status badges, cluster-wide summary tiles, and an "Add Node" modal.
- **UI types** (`ui/src/types/index.ts`) — `Node`, `NodeCapacity`, `NodeStatus`, `NodesResponse`, `RegisterNodeRequest`, `HeartbeatRequest` types added; `CreateServerRequest.node_id` field added.
- **Navigation** (`ui/src/components/shared/Layout.tsx`, `ui/src/App.tsx`) — "Nodes" nav item (Layers icon) and `/nodes` route wired up.
- **CLI node commands** (`cli/cmd/main.go`) — `gdash node list`, `gdash node add`, `gdash node remove`, `gdash node status` with human-readable table output and JSON mode.

#### Daemon (`daemon/`)
- **OIDC authentication** (`internal/auth/service.go`) — Full OAuth2 authorization code flow via `coreos/go-oidc`; lazy provider discovery with `sync.Once`; CSRF-protected state nonces (5-minute TTL); auto-provisions local user records from OIDC identity (`email` → `preferred_username` → `sub`); issues Games Dashboard JWT on successful ID token verification. New `GET /api/v1/auth/oidc/login` endpoint returns the authorization URL for browser redirect.
- **Vault secrets backend** (`internal/secrets/manager.go`) — `vault` backend now initializes a real Vault client (`hashicorp/vault/api`); reads/writes the 256-bit AES master key from the configured KV path (supports both KV v1 and v2); generates and stores a new key if the secret doesn't exist yet; gracefully falls back to local key file on Vault errors. `Rotate` persists rotated key to whichever backend is active.
- **Adapter Registry** (`internal/adapters/adapter.go`) — YAML manifest loader for all 7 game adapters; falls back to built-in defaults when no manifests directory is configured.
- **Backup Service** (`internal/backup/service.go`) — Cron-based backup scheduler (robfig/cron); supports full/incremental backups, SHA-256 checksums, retention pruning.
- **Mod Manager** (`internal/modmanager/manager.go`) — Install/uninstall/rollback/test harness for mods from Steam, CurseForge, Modrinth, Thunderstore, Git, and local sources.
- **Networking Service** (`internal/networking/service.go`) — OS-level port availability checker; tracks reserved ports per server; supports optional remote reachability probe.
- **SBOM Service** (`internal/sbom/service.go`) — CycloneDX 1.5 SBOM generation + CVE report with Trivy/Grype integration.
- **Broker integration** — Adapter registry wired into broker for real health checks (TCP/UDP probes) and auto-populated default ports/resources on server creation.
- **Broker services wired** — `sbom.Service` and `networking.Service` injected into broker; `GetSBOM`, `GetCVEReport`, `ValidatePorts`, `TriggerCVEScan` now use real service implementations.
- **Dockerfiles** — Multi-stage `daemon/Dockerfile` (distroless runtime) and `ui/Dockerfile` (nginx:1.25-alpine with SPA routing + security headers).

#### CLI (`cli/`)
- **Real HTTP client** (`cli/cmd/main.go`) — Replaced stub `doRequest` with a full `net/http` implementation supporting TLS, `--insecure` flag, Bearer token auth, JSON body/response, and API error extraction.
- **Config persistence** (`cli/internal/config/`) — `~/.gdash/config.yaml` stores daemon URL, token, and output format; loaded automatically on startup.
- **Table output** (`cli/internal/table/`) — ASCII table printer for formatted `text` output on list commands.
- **CLI module** (`cli/go.mod`) — Standalone Go module (`github.com/games-dashboard/cli`) with cobra, viper, websocket dependencies.

#### UI (`ui/`)
- **TypeScript types** (`ui/src/types/index.ts`) — Full type definitions for all API resources.
- **TanStack Query hooks** (`ui/src/hooks/useServers.ts`) — ~30 hooks covering all API endpoints with cache invalidation.
- **BackupsPage** — Per-server expandable backup history with trigger and restore actions.
- **ModsPage** — Mod install modal (6 sources), test harness, rollback, uninstall per mod.
- **SettingsPage** — Users/Auth section (CRUD + TOTP QR setup), TLS, system status panel.
- **SBOMPage fix** — CVE severity counts now read from top-level `critical/high/medium/low` fields (not a nested `summary` object).
- **Vitest setup** (`ui/vite.config.ts`, `ui/src/test/setup.ts`) — jsdom environment with coverage.
- **UI tests** — `adapters.test.ts` (ADAPTER_NAMES/COLORS coverage), `types/index.test.ts` (runtime shape checks).

#### Helm (`helm/`)
- **games-dashboard chart templates** — Added `configmap.yaml`, `service.yaml`, `serviceaccount.yaml` (with ClusterRole/Binding), `pvc.yaml`, `ingress.yaml`, `hpa.yaml`.
- **game-instance chart** — New standalone chart for single game server deployment as a Kubernetes StatefulSet; includes headless + LoadBalancer services, optional backup sidecar, configurable probes, PDB.

#### Installer (`installer/`)
- **Config templates** — `installer/configs/daemon.yaml` (full example daemon config), `installer/configs/docker-compose.yml` (deployment template).
- **Helper scripts** — `generate-tls.sh` (self-signed cert), `health-check.sh` (post-install poller with retries), `uninstall.sh` (clean removal with optional `--purge`).
- **Offline bundle** — `installer/offline/README.md` documents the bundle structure and install workflow; `bundle-manifest.json` is a template with the full artifact schema.
- **build-offline-bundle.sh** — Pulls Docker images, cross-compiles `gdash` CLI binaries, packages Helm charts, bundles SteamCMD and adapter manifests; generates `manifest.json` with per-artifact SHA-256 checksums; produces a `.tar.gz` + `.sha256` sidecar (optional GPG `.asc` signature).
- **verify-bundle.sh** — Verifies a bundle before installation: outer archive SHA-256, optional GPG signature, and every artifact's SHA-256 against `manifest.json`; exits non-zero on any mismatch.

#### Tests
- **Go unit tests** — `adapter_test.go`, `auth/service_test.go`, `broker/broker_test.go`.
- **Integration test suite** (`tests/integration/run-tests.sh`) — 7 suites: preflight, installer dry-run, runtime API, adapter manifests, SBOM/CVE, documentation, security hardening.
- **E2E tests** (`tests/e2e/`) — CLI smoke tests against a live daemon.

### Changed
- **CI** (`.github/workflows/ci.yml`) — `integration-tests` job now runs on both `push` and `pull_request` events so PRs are fully validated before merge.
- **daemon `go.mod`** — `golang.org/x/oauth2 v0.18.0` added as a direct dependency (required by OIDC flow); direct deps sorted alphabetically.

### Fixed
- Broker `checkServerHealth` compilation bug: `pushConsoleMsg` renamed to `sendConsoleMessage`.
- `GetCVEReport` now returns `critical`, `high`, `medium`, `low` at the top level to match the UI `CVEReport` TypeScript type.
- **WebSocket `CheckOrigin`** (`api/server.go`) — replaced unconditional `return true` with an explicit hostname allowlist (localhost, 127.0.0.1, ::1, daemon bind host); extended via `api.Config.AllowedOrigins`.
- **Backup archival** (`backup/service.go`) — `archivePath` creates a real `tar.gz` using stdlib `archive/tar` + `compress/gzip` with SHA-256 of the archive file; `restorePath` extracts with path-traversal protection; `Config.DataDir` added (default `/var/lib/games-dashboard/backups`).
- **CVE scanning** (`sbom/service.go`) — `TriggerScan` shells out to `trivy fs` or `grype` (whichever is in PATH), parses their JSON output into `[]CVEFinding`, and falls back to disk-cached report or clean placeholder when no scanner is available.

### Changed
- Broker `GetSBOM` delegates to `sbom.Service.GetSBOM` (real CycloneDX BOM).
- Broker `GetCVEReport` delegates to `sbom.Service.GetReport` (marshalled to map via JSON).
- Broker `ValidatePorts` delegates to `networking.Service.ValidatePorts` (OS-level checks).
- Broker `TriggerCVEScan` delegates to `sbom.Service.TriggerScan`.

---

## [1.0.0] — Initial Development

### Added
- Project scaffolding: daemon, CLI, UI, adapters, installer, Helm charts, CI/CD pipeline.
- Core daemon packages: API server (Gin), auth (JWT/TOTP/OIDC), broker, config, secrets (AES-256-GCM), health, metrics (Prometheus).
- YAML adapter manifests for 7 games: Valheim, Minecraft, Satisfactory, Palworld, Eco, Enshrouded, The Riftbreaker.
- CI/CD pipeline: unit tests, lint, build (daemon/CLI/UI), SBOM generation, CVE scan (Trivy), Helm packaging, cosign signing, GitHub Release.
- CRD: `GameInstance` custom resource definition.
