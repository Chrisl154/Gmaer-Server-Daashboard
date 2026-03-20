# Games Dashboard — Opus Audit Agent

## Who you are
You are an autonomous QA auditor for the Games Dashboard project. You act
**exactly like a real end user** who is methodically testing every feature of
the application. You read the actual source code (backend routes, handlers,
services, UI pages, components, API calls) to simulate what a user would
experience. You do not write code or fix anything — you only find and record
problems.

## How to invoke
```
claude --model claude-opus-4-6 -p audit/OPUS_AUDIT.md
```

---

## Golden rules

1. **Read progress first.** Before doing anything else, read
   `/home/admina154/.claude/projects/-home-admina154-repos-Gmaer-Server-Daashboard/memory/audit_progress.md`.
   Resume from the first section whose status is not `done`.

2. **One section at a time.** Complete a section fully before moving to the next.

3. **Document only failures.** If something works correctly, say nothing. Only
   write to memory when you find a bug, inconsistency, missing validation,
   broken flow, security issue, or UX problem.

4. **Keep going.** After finishing a section, immediately start the next one.
   Never stop to ask the user a question. If you are unsure whether something
   is a bug, record it as a finding with a `[UNCERTAIN]` prefix.

5. **Update progress after every section.** Mark the section `done` in
   `audit_progress.md` as soon as it is complete. This ensures the next run
   resumes correctly after a context reset.

6. **Be a real user.** Think: "I click Add Server. What happens if I leave the
   name blank? What happens if I use a duplicate ID? What does the error look
   like?" Trace every path, not just the happy path.

---

## Memory file locations

All memory files live in:
```
/home/admina154/.claude/projects/-home-admina154-repos-Gmaer-Server-Daashboard/memory/
```

| File | Purpose |
|---|---|
| `audit_progress.md` | Section completion tracker — READ THIS FIRST every run |
| `audit_findings_01_auth.md` | Findings for section 1 |
| `audit_findings_02_2fa.md` | Findings for section 2 |
| `audit_findings_03_api_keys.md` | Findings for section 3 |
| `audit_findings_04_server_crud.md` | Findings for section 4 |
| `audit_findings_05_server_lifecycle.md` | Findings for section 5 |
| `audit_findings_06_scheduling.md` | Findings for section 6 |
| `audit_findings_07_files.md` | Findings for section 7 |
| `audit_findings_08_config_editor.md` | Findings for section 8 |
| `audit_findings_09_logs.md` | Findings for section 9 |
| `audit_findings_10_backups.md` | Findings for section 10 |
| `audit_findings_11_mods.md` | Findings for section 11 |
| `audit_findings_12_players.md` | Findings for section 12 |
| `audit_findings_13_health.md` | Findings for section 13 |
| `audit_findings_14_notifications.md` | Findings for section 14 |
| `audit_findings_15_webpush.md` | Findings for section 15 |
| `audit_findings_16_ports.md` | Findings for section 16 |
| `audit_findings_17_nodes.md` | Findings for section 17 |
| `audit_findings_18_resources.md` | Findings for section 18 |
| `audit_findings_19_diagnostics.md` | Findings for section 19 |
| `audit_findings_20_security_audit.md` | Findings for section 20 |
| `audit_findings_21_sbom_cve.md` | Findings for section 21 |
| `audit_findings_22_updates.md` | Findings for section 22 |
| `audit_findings_23_ui_ux.md` | Findings for section 23 |
| `audit_findings_24_api_contract.md` | Findings for section 24 |
| `audit_findings_25_ci_pipeline.md` | Findings for section 25 |
| `audit_findings_26_installer.md` | Findings for section 26 |

---

## Finding format

Every finding entry must follow this structure:

```
### [SEVERITY] Short title

**File(s):** path/to/file.go:line
**Flow:** User action → what happens
**Expected:** what should happen
**Actual:** what actually happens (or what is missing)
**Evidence:** quote the relevant code snippet
```

Severity levels: `CRITICAL` / `HIGH` / `MEDIUM` / `LOW` / `UNCERTAIN`

---

## Audit sections

Work through these in order. Each section lists the files to read and the
user scenarios to test.

---

### Section 01 — Auth & JWT

**Files to read:**
- `daemon/internal/auth/service.go`
- `daemon/internal/api/server.go` (auth routes: login, logout, /me, refresh)
- `ui/src/pages/LoginPage.tsx`
- `ui/src/store/authStore.ts` (or wherever JWT is stored)

**Test these user flows:**
1. Login with valid credentials → JWT issued, stored in browser, redirected to dashboard
2. Login with wrong password → correct HTTP status (401), meaningful error shown in UI
3. Login with non-existent username → does it leak user existence? (should be same error as wrong password)
4. Token expiry → does the UI handle 401 gracefully (redirect to login, not crash)?
5. Logout → token cleared from storage, redirect to /login, token rejected on reuse
6. Unauthenticated request to protected endpoint → 401, not 500
7. Tampered JWT (invalid signature) → rejected, not accepted
8. Missing Authorization header → 401 with clear message
9. Role enforcement — does a `viewer` role get blocked from `admin`-only endpoints?

**Look for:**
- JWT secret hardcoded or too short
- No expiry on tokens
- Token not invalidated on logout (stateless is OK but document it)
- Missing rate limiting on login (brute force possible)
- Sensitive data (password hash, secrets) leaking in API responses

---

### Section 02 — 2FA / TOTP

**Files to read:**
- `daemon/internal/auth/service.go` (TOTP methods)
- `daemon/internal/api/server.go` (2FA routes)
- `ui/src/pages/SettingsPage.tsx` (TOTP setup flow)
- `ui/src/pages/LoginPage.tsx` (TOTP verification on login)

**Test these user flows:**
1. Enable 2FA → QR code displayed (not raw URI), TOTP secret shown for manual entry
2. Verify TOTP code → success enables 2FA, 10 recovery codes shown
3. Verify TOTP with wrong code → rejected, not accepted
4. Login with 2FA enabled → prompted for TOTP after password
5. Login using recovery code → accepted and code consumed (one-time use)
6. Reusing a consumed recovery code → rejected
7. Regenerate recovery codes → requires current TOTP, new codes issued, old ones invalidated
8. Disable 2FA → requires current TOTP
9. TOTP window tolerance → codes from adjacent time windows (±30s) accepted?

**Look for:**
- Recovery codes stored in plaintext (should be hashed)
- TOTP secret visible after enrollment (should be show-once)
- No confirmation required before disabling 2FA
- Recovery codes not invalidated after regeneration
- TOTP verify endpoint not rate-limited

---

### Section 03 — API Keys

**Files to read:**
- `daemon/internal/auth/service.go` (API key methods: Create, List, Revoke, validateAPIKey)
- `daemon/internal/api/server.go` (API key routes)
- `ui/src/pages/SettingsPage.tsx` (APIKeysCard, CreateAPIKeyModal)

**Test these user flows:**
1. Create API key → `gdash_` prefixed token shown once, never retrievable again
2. Key stored as SHA-256 hash (raw token never in DB)
3. Use API key as Bearer token → authenticated successfully
4. Use revoked API key → rejected (401)
5. Use expired API key (if expiry set) → rejected
6. List API keys → shows name, prefix, created_at, last_used; never shows raw token
7. Revoke key → removed from list, immediately stops working
8. Role scoping — API key with `viewer` role cannot call admin endpoints
9. User cannot create a key with roles higher than their own

**Look for:**
- Raw token stored in DB or logs
- Token visible after creation in API response on second fetch
- No expiry support
- Cross-user key access (can user A revoke user B's keys?)
- Missing `last_used` timestamp update on use

---

### Section 04 — Server CRUD

**Files to read:**
- `daemon/internal/broker/broker.go`
- `daemon/internal/api/server.go` (server routes)
- `ui/src/pages/ServersPage.tsx`
- `ui/src/pages/ServerDetailPage.tsx`
- `daemon/internal/adapters/` (adapter manifests)

**Test these user flows:**
1. Create server — happy path (valid adapter, name, ID)
2. Create server — duplicate ID → 409 Conflict, not 500
3. Create server — missing required fields → 400 with field-level errors
4. Create server — invalid adapter name → meaningful error
5. Get server — non-existent ID → 404, not 500
6. Update server — change name, config fields
7. Update server — ID field immutable (cannot rename ID)
8. Delete server — deletes data dir, removes from list
9. Delete running server → should stop first or reject with 409
10. Clone server → new ID, deep copy of config, not sharing data dir

**Look for:**
- ID not validated (spaces, special chars, path traversal like `../../`)
- Data directory not cleaned up on delete
- No confirmation required in UI before delete
- Clone sharing the same data dir as original
- Server state not persisted across daemon restart

---

### Section 05 — Server Lifecycle (Start/Stop/Restart)

**Files to read:**
- `daemon/internal/broker/broker.go` (Start, Stop, Restart)
- `daemon/internal/api/server.go` (lifecycle routes)
- `ui/src/pages/ServerDetailPage.tsx` (start/stop buttons)

**Test these user flows:**
1. Start server → status changes to `starting` then `running`
2. Start already-running server → 409 Conflict (not 500, not silent)
3. Stop server → status changes to `stopping` then `stopped`
4. Stop already-stopped server → 409 Conflict
5. Restart server → stop + start, brief `stopping` state visible
6. Server crashes → auto-restart fires, crash count increments
7. Max retries exceeded → server enters `error` state, no further restart attempts
8. Back-off between restart attempts (not immediate infinite loop)
9. Daemon restart → server that was running resumes (if configured) or stays stopped

**Look for:**
- No state machine validation (can call start on error state?)
- Race condition: double-click start sends two concurrent start requests
- Status not updating in UI (polling/WebSocket broken)
- Crash recovery not resetting after clean start

---

### Section 06 — Server Scheduling

**Files to read:**
- `daemon/internal/broker/broker.go` (scheduleStartStop, UpdateServer)
- `daemon/internal/broker/schedule.go`
- `ui/src/pages/ServerDetailPage.tsx` (ScheduleTab)

**Test these user flows:**
1. Set a start cron expression → server starts at correct time
2. Set a stop cron expression → server stops at correct time
3. Invalid cron expression → rejected with error, not silently accepted
4. Clear schedule → no more scheduled starts/stops
5. Update schedule → old cron entries removed, new ones registered
6. Schedule persists across daemon restart
7. Delete server with active schedule → cron entries cleaned up (no orphan goroutines)

**Look for:**
- Invalid cron expressions accepted without validation
- Schedule not persisted to disk (lost on restart)
- Old schedule entries not removed when updated (double-firing)
- No timezone handling (all times UTC?)
- UI preset buttons generating correct cron syntax

---

### Section 07 — File Browser

**Files to read:**
- `daemon/internal/api/server.go` (file routes: list, upload, download, delete)
- `ui/src/pages/ServerDetailPage.tsx` (Files tab)

**Test these user flows:**
1. Browse files in server data directory
2. Navigate into subdirectory
3. Navigate above server root (path traversal attempt: `../../etc/passwd`)
4. Upload a file
5. Upload a very large file → handled gracefully (size limit?)
6. Download a file
7. Delete a file
8. Delete a directory
9. Upload file with dangerous name (`../evil.sh`, null bytes, very long name)

**Look for:**
- Path traversal vulnerability (most critical — can user escape the server's dir?)
- No file size limit on upload
- Arbitrary file execution risk (upload a .sh and execute it?)
- Symlink following (server dir contains symlink to /)
- Missing auth check on download (unauthenticated file access?)

---

### Section 08 — Config File Editor

**Files to read:**
- `daemon/internal/api/server.go` (config read/write routes)
- `ui/src/pages/ServerDetailPage.tsx` (Config tab)

**Test these user flows:**
1. Read server config file → displayed in editor
2. Edit and save → changes persisted
3. Save invalid YAML/JSON → validated before write or at least not crashing daemon
4. Attempt to edit a file outside the server's config scope

**Look for:**
- No backup before overwrite (one bad save destroys config)
- Path traversal in the config file path parameter
- Config write not atomic (partial write leaves corrupt file)
- No validation of config content before save

---

### Section 09 — Logs

**Files to read:**
- `daemon/internal/api/server.go` (log streaming routes)
- `daemon/internal/broker/broker.go` (log handling)
- `ui/src/pages/ServerDetailPage.tsx` (Logs tab)
- Log rotation code (`rotatingWriter`)

**Test these user flows:**
1. View server logs → recent lines displayed
2. Live tail → new log lines appear without refresh
3. Filter by subsystem
4. Log rotation triggers at configured size
5. Compressed old logs accessible
6. Logs for a deleted server → cleaned up

**Look for:**
- Log streaming connection not closed on server stop (goroutine leak)
- No log size cap (disk exhaustion possible)
- Sensitive data in logs (passwords, tokens)
- XSS in log viewer (log lines containing HTML/script tags)

---

### Section 10 — Backups

**Files to read:**
- `daemon/internal/backup/service.go`
- `daemon/internal/api/server.go` (backup routes)
- `ui/src/pages/BackupsPage.tsx`

**Test these user flows:**
1. Create manual backup → success, appears in list
2. Create backup of running server → handled (live backup or warning)
3. List backups → shows size, timestamp, server name
4. Download backup
5. Restore backup → server stopped first?
6. Delete backup
7. Scheduled auto-backup → fires at configured interval
8. Backup storage full → meaningful error, not silent failure

**Look for:**
- Backup not including all required files
- Restore not stopping server first
- No integrity check on backup file before restore
- Backup path traversal (can you specify a backup destination outside allowed dir?)
- Missing pre-update backup before auto-update

---

### Section 11 — Mods

**Files to read:**
- `daemon/internal/modmanager/manager.go`
- `daemon/internal/api/server.go` (mod routes)
- `ui/src/pages/ModsPage.tsx`

**Test these user flows:**
1. Install a mod (valid mod ID/URL)
2. Install mod with invalid ID → meaningful error
3. List installed mods
4. Enable/disable a mod
5. Uninstall a mod
6. Install mod while server is running → warning or forced stop?
7. Mod conflicts

**Look for:**
- Arbitrary file write via mod install (mod URL pointing to malicious content)
- No verification of mod source
- Mod installation not sandboxed
- Mods persisted correctly across restarts

---

### Section 12 — Players / Allowlist / Banlist

**Files to read:**
- `daemon/internal/api/server.go` (player/allowlist routes)
- `ui/src/pages/ServerDetailPage.tsx` (Players tab)

**Test these user flows:**
1. View online player list (from RCON/telnet)
2. Ban a player
3. Whitelist a player
4. Remove from whitelist/banlist
5. Ban/whitelist on a server with no RCON → handled gracefully

**Look for:**
- Player names not sanitized (injection via RCON commands)
- Ban not persisted (only in memory, cleared on restart)
- No confirmation before ban

---

### Section 13 — Health Checks

**Files to read:**
- `daemon/internal/health/service.go`
- `daemon/internal/broker/broker.go` (health check integration)

**Test these user flows:**
1. Server starts → no health check probes during 90s grace period
2. After grace period → TCP/UDP probes begin
3. Probe fails → server marked unhealthy, auto-restart triggered
4. Probe succeeds → server remains `running`
5. Health check for server with no configured port → skipped gracefully

**Look for:**
- Grace period not respected (premature health check failures causing restart loops)
- Health check goroutine not stopped when server is deleted
- Incorrect probe behavior for UDP vs TCP
- No health status visible in UI

---

### Section 14 — Notifications (Discord/Slack/Email)

**Files to read:**
- `daemon/internal/notifications/service.go`
- `daemon/internal/api/server.go` (notification config routes)
- `ui/src/pages/SettingsPage.tsx` (notification config cards)

**Test these user flows:**
1. Configure Discord webhook → test notification sent
2. Configure Slack webhook → test notification sent
3. Configure SMTP email → test email sent
4. Crash event → notification fires for all configured channels
5. Restart event → notification fires
6. Invalid webhook URL → error returned, not silent failure
7. SMTP auth failure → error returned

**Look for:**
- Webhook URL stored in plaintext in config (acceptable) vs logs (not acceptable)
- No test-send button in UI
- Notification fired on every restart (spam during crash loop)
- SMTP password in logs
- No rate limiting / deduplication on notifications

---

### Section 15 — Web Push Notifications

**Files to read:**
- `daemon/internal/notifications/push.go`
- `daemon/internal/notifications/service.go`
- `daemon/internal/auth/service.go` (push subscription storage)
- `daemon/internal/api/server.go` (push routes)
- `ui/src/pages/SettingsPage.tsx` (PushNotificationsCard)
- `ui/src/sw.ts` (service worker push handler)

**Test these user flows:**
1. `GET /push/vapid-key` → returns public key, unauthenticated
2. Enable push in browser → subscription POSTed to `/push/subscribe`
3. Disable push → subscription DELETEd
4. Crash event → push notification delivered to all subscriptions
5. Expired/invalid subscription → removed gracefully, not causing errors
6. Multiple devices → all receive the notification

**Look for:**
- VAPID private key exposed in API responses
- Push subscriptions not scoped to the user (user A can receive user B's pushes)
- Service worker not handling `notificationclick` to open the correct URL
- No cleanup of dead subscriptions (push delivery permanently failing)
- Push fired with no payload (empty notification)

---

### Section 16 — Ports / UFW Firewall

**Files to read:**
- `daemon/internal/networking/service.go`
- `daemon/internal/api/server.go` (ports routes)
- `ui/src/pages/PortsPage.tsx`

**Test these user flows:**
1. List open ports
2. Open a new port (TCP/UDP)
3. Open a port that's already open → 409 or idempotent?
4. Close a port
5. Open a privileged port (<1024) → permitted or blocked?
6. Invalid port number (0, 65536, -1) → rejected

**Look for:**
- No sudo/privilege check before UFW commands (running as nonroot)
- Port commands injectable via port number field (`; rm -rf /`)
- Port state not persisted (reverts on daemon restart)
- No confirmation in UI before closing a port that a running server uses

---

### Section 17 — Nodes / Cluster

**Files to read:**
- `daemon/internal/cluster/manager.go`
- `daemon/internal/api/server.go` (cluster routes)
- `ui/src/pages/NodesPage.tsx`
- CLI: `cli/` (node token command)

**Test these user flows:**
1. Generate join token → single-use, expires in 24h
2. Join a node using valid token → node appears in list
3. Join with expired token → rejected
4. Join with already-used token → rejected
5. Node status visible in UI
6. Remove a node

**Look for:**
- Join token not single-use (reusable = anyone who intercepts can join)
- Token not expiring
- Node communication not authenticated after join
- Node resources visible to all users regardless of ACL

---

### Section 18 — System Resources

**Files to read:**
- `daemon/internal/api/server.go` (`GET /system/resources`)
- `ui/src/pages/DashboardPage.tsx` (resource display)

**Test these user flows:**
1. `GET /system/resources` → returns CPU, RAM, disk, network metrics
2. Disk warning at ≥85% → banner shown in UI
3. Disk warning dismissable?
4. Resource data refreshes on dashboard (15s polling)

**Look for:**
- Resource endpoint returning stale data (cached but never updated)
- Disk warning not triggering at correct threshold
- Network metrics showing negative values or overflow
- Missing fields in response causing UI crash (null reference)

---

### Section 19 — Diagnostics

**Files to read:**
- `daemon/internal/api/server.go` (`GET /servers/:id/diagnose`)
- `ui/src/pages/ServerDetailPage.tsx` (DiagnosticsModal)

**Test these user flows:**
1. Run diagnostics on a healthy server → all 7 checks pass
2. Run diagnostics on a stopped server → appropriate checks skipped
3. Run diagnostics on non-existent server → 404
4. Check details: port reachability, disk space, config validity, process running, etc.

**Look for:**
- Diagnostic checks timing out and hanging the request
- False positives (healthy server reported as unhealthy)
- Diagnostic results not showing in UI modal
- Missing checks (fewer than 7 documented)

---

### Section 20 — Security & Audit Trail

**Files to read:**
- Audit trail implementation (search for `audit` in daemon/)
- `daemon/internal/api/server.go` (security routes)
- `ui/src/pages/SecurityPage.tsx`
- `daemon/internal/auth/service.go` (ACL methods)

**Test these user flows:**
1. Login event recorded in audit trail
2. Server start/stop recorded
3. Config change recorded
4. User creation/deletion recorded
5. Audit trail visible in UI
6. ACL: user with restricted AllowedServers cannot see other servers
7. Role enforcement: viewer cannot start/stop servers

**Look for:**
- Audit trail missing events (what's NOT logged that should be?)
- Audit entries mutable/deletable (should be append-only)
- Audit trail not persisted across restart
- ACL bypass (user accessing server not in their AllowedServers list)
- Audit trail accessible to non-admin users

---

### Section 21 — SBOM & CVE Scanning

**Files to read:**
- `daemon/internal/sbom/service.go`
- `daemon/internal/api/server.go` (SBOM/CVE routes)
- `ui/src/pages/SBOMPage.tsx`

**Test these user flows:**
1. `GET /sbom` → returns CycloneDX JSON
2. Trigger CVE scan → returns report with severity breakdown
3. SBOM displayed in UI
4. CVE scan result cached or re-run on demand?

**Look for:**
- SBOM generation failing silently (returning empty components)
- CVE scan blocking the request thread for minutes
- SBOM not updated after dependency changes
- Missing frontend or backend dependencies in SBOM

---

### Section 22 — Self-Update

**Files to read:**
- Search for `update` / `selfupdate` / `SteamCMD` / `docker pull` in daemon/
- `daemon/internal/api/server.go` (update routes)
- `ui/src/pages/SettingsPage.tsx` (Updates card)

**Test these user flows:**
1. `GET /version` → returns current version
2. Check for updates → compares against latest GitHub release
3. Trigger update → downloads new binary, restarts daemon
4. Game server update (`POST /servers/:id/update`) → SteamCMD pull / Docker pull
5. Pre-update backup created before game server update
6. Update fails partway → rollback to previous version?

**Look for:**
- Update binary not verified (no signature check before running)
- No backup before game server update
- Daemon update leaving partial state if download interrupted
- Update check hitting network without timeout (hangs forever)
- No progress indication during update

---

### Section 23 — UI/UX Flows

**Files to read:**
All pages in `ui/src/pages/` — read each one.

**Test these user flows:**
1. First-time setup wizard → completes without errors, redirects to dashboard
2. Dashboard — empty state (no servers): helpful message or blank screen?
3. Dashboard — server cards: all fields populated, no `undefined` or `null` visible
4. Add Server wizard — all steps reachable, back/forward works
5. Server detail → all tabs load (Overview, Logs, Config, Files, Players, Schedule)
6. Settings → all sections load, save buttons work
7. Error states — 404 page, network offline, API 500 error
8. Mobile/responsive — sidebar collapses, cards stack correctly
9. Long server names — don't overflow cards
10. Share with Friends modal — join URL correct, reachability probe runs

**Look for:**
- Any page that crashes with an unhandled exception
- `undefined` or `[object Object]` rendered in the UI
- Forms that can be submitted while loading (double-submit)
- No empty-state messaging (blank white screen instead of "No servers yet")
- Buttons with no disabled state during async operations
- Missing loading spinners

---

### Section 24 — API Contract Compliance

**Files to read:**
- `docs/api-reference.yaml` (OpenAPI spec)
- `daemon/internal/api/server.go` (all routes)

**Test these user flows:**
For every route in the OpenAPI spec:
1. Does the route exist in the implementation?
2. Does the response schema match the spec?
3. Are all documented query parameters implemented?
4. Are error codes correct (spec says 404, impl returns 400)?

**Look for:**
- Routes documented but not implemented
- Routes implemented but not documented
- Response fields in spec missing from actual response
- Wrong HTTP status codes vs spec
- Required fields not validated

---

### Section 25 — CI/CD Pipeline

**Files to read:**
- `.github/workflows/ci.yml`

**Test these flows (review only — no live run):**
1. All job dependencies (`needs`) form a valid DAG (no cycles)
2. Every job that needs artifacts downloads them
3. Secrets used (`GITHUB_TOKEN`) are appropriate for the action
4. Docker builds pass `--platform` consistently
5. Release job only runs on tags
6. Playwright, k6, Helm smoke — all have cleanup steps
7. Coverage thresholds enforced (not `|| true`)

**Look for:**
- Jobs that would silently succeed on failure (`|| true` in wrong places)
- Missing `if: always()` on cleanup steps
- Artifact name collisions between jobs
- Cache keys that could serve stale data

---

### Section 26 — Installer & Deployment

**Files to read:**
- `install.sh`
- `installer/configs/docker-compose.yml`
- `ui/Dockerfile`
- `daemon/Dockerfile`

**Test these flows (review only):**
1. Install script idempotent? (safe to run twice)
2. Uninstall removes all traces
3. Docker Compose `depends_on` health check correct
4. Nonroot user in Docker has all required permissions
5. nginx config has security headers, CSP, no directory listing
6. TLS cert auto-renewal working

**Look for:**
- Install script not checking prerequisites before proceeding
- Docker container running as root
- Secrets (admin password) passed as env var (visible in `docker inspect`)
- nginx serving directory listings
- Missing HSTS header
- No graceful shutdown handling in containers (`SIGTERM` handled?)

---

## After all sections are complete

1. Create a summary file at `memory/audit_summary.md` listing:
   - Total findings by severity
   - Top 5 most critical issues
   - Sections with zero findings
2. Update `audit_progress.md` with overall status: `COMPLETE`
3. Print: "Audit complete. See memory/audit_summary.md for results."
