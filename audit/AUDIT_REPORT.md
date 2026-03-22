# Games Dashboard — Security & QA Audit Report

**Date:** 2026-03-20
**Auditor:** Claude Opus 4.6 (autonomous QA agent)
**Scope:** 26 sections covering auth, server lifecycle, file management, notifications, cluster, UI, CI/CD, and deployment
**Total findings:** 63 (2 Critical, 13 High, 33 Medium, 13 Low)

---

## Priority Fix List (top-to-bottom)

Work through these in order. Each item includes the file(s) to change and a concrete implementation plan.

---

### P1 — CRITICAL: Default JWT secret "change-me-in-production"

**Section:** 01 — Auth & JWT
**File:** `daemon/internal/config/config.go:199`
**Risk:** Anyone who knows the default can forge admin JWTs and take over the system.

**Fix instructions:**
1. In `config.go`, remove the default value for `JWTSecret` (set it to `""`).
2. In `daemon/cmd/daemon/main.go`, after loading the config, check if `cfg.Auth.JWTSecret` is empty or equals `"change-me-in-production"`. If so, auto-generate a 32-byte random hex string using `crypto/rand`, set it on the config, and log a WARNING telling the user to set `jwt_secret` in `daemon.yaml`.
3. Alternatively, refuse to start with a fatal error if the secret is the default.

---

### P2 — CRITICAL: Default admin password "changeme" in Docker Compose

**Section:** 26 — Installer & Deployment
**File:** `installer/configs/docker-compose.yml:19`
**Risk:** Anyone who knows the default password can log in as admin.

**Fix instructions:**
1. Remove the `:-changeme` fallback from the env var: `GDASH_ADMIN_PASSWORD: ${GDASH_ADMIN_PASSWORD}`.
2. Add a comment: `# Required — set in .env file or export before running`.
3. In `install.sh`, generate a random password at install time and write it to `.env` alongside the compose file.
4. In the daemon bootstrap flow (`BootstrapAdmin`), if the password env var is empty, refuse to start with a clear error message.

---

### P3 — HIGH: Logout is non-functional (token remains valid)

**Section:** 01 — Auth & JWT
**Files:** `daemon/internal/auth/service.go:318-321,324-351`
**Risk:** Stolen tokens cannot be revoked; they remain valid for the full 24h TTL.

**Fix instructions:**
1. Add a `tokenBlocklist map[string]time.Time` field to the `Service` struct.
2. In `Logout()`, strip the "Bearer " prefix (fix the existing bug at `server.go:858`), then add the raw token to the blocklist with its expiry time.
3. In `ValidateToken()`, after parsing the JWT, check if `tokenStr` is in the blocklist. If so, return an error.
4. Add a goroutine that periodically prunes expired entries from the blocklist (every 5 minutes, remove entries whose expiry has passed).

---

### P4 — HIGH: RCON command injection via player names

**Section:** 12 — Players / Allowlist / Banlist
**File:** `daemon/internal/broker/banlist.go:109,136,176,191`
**Risk:** Attacker can execute arbitrary RCON commands by injecting into a player name.

**Fix instructions:**
1. Create a `sanitizePlayerName(name string) string` function that strips or rejects characters like `;`, `"`, `'`, `\n`, and any non-alphanumeric/underscore/hyphen characters.
2. Call it before every `fmt.Sprintf` that interpolates a player name into an RCON command.
3. Add a validation check in the API handler (`banPlayer`, `whitelistAddPlayer`) that rejects names with special characters with a 400 error.

---

### P5 — HIGH: TOTP re-setup allows account takeover

**Section:** 02 — 2FA / TOTP
**File:** `daemon/internal/auth/service.go:383-403`
**Risk:** Attacker with a stolen session can overwrite the victim's TOTP secret and take over 2FA.

**Fix instructions:**
1. In `SetupTOTP()`, add a check: if `user.TOTPEnabled` is true, return an error `"TOTP is already enabled; disable it first or use the re-enrollment flow"`.
2. Add a `DisableTOTP(ctx, claims, currentTOTPCode)` method that validates the current TOTP code before disabling.
3. Add a `DELETE /api/v1/auth/totp` route in `server.go` that requires the current TOTP code.
4. In the UI SettingsPage, add a "Disable TOTP" button that prompts for the current code.

---

### P6 — HIGH: Server ID not validated (path traversal)

**Section:** 04 — Server CRUD
**Files:** `daemon/internal/broker/broker.go:762-850`, `daemon/internal/api/server.go:337-351`
**Risk:** A crafted ID like `../../etc` can cause path traversal when used in filepath operations.

**Fix instructions:**
1. Create a `validateServerID(id string) error` function in `broker.go` that checks: only alphanumeric, hyphens, underscores allowed; max 64 chars; no `.`, `/`, `\`, or spaces.
2. Call it at the top of `CreateServer()` before the duplicate check.
3. Also validate in the `createServer` API handler and return 400 if invalid.

---

### P7 — HIGH: StartServer incomplete state validation (double-start)

**Section:** 05 — Server Lifecycle
**File:** `daemon/internal/broker/broker.go:1041-1057`
**Risk:** Two concurrent start requests launch two server processes simultaneously.

**Fix instructions:**
1. Change the guard in `StartServer()` from `if s.State == StateRunning` to:
   ```go
   if s.State != StateStopped && s.State != StateIdle && s.State != StateError {
       return fmt.Errorf("server is %s; cannot start", s.State)
   }
   ```
2. This blocks starting from `StateStarting`, `StateStopping`, and `StateDeploying`.

---

### P8 — HIGH: Restore does not stop server first

**Section:** 10 — Backups
**File:** `daemon/internal/backup/service.go:275-301`
**Risk:** Restoring files while the server is running corrupts live game data.

**Fix instructions:**
1. The `Restore` method in backup service doesn't have access to the broker. Instead, add a state check in the `restoreBackup` API handler in `server.go`.
2. Before calling `backupSvc.Restore()`, check `broker.GetServer(id)` and verify `server.State != StateRunning`. Return 409 if running.
3. Alternatively, auto-stop the server before restore and auto-start after.

---

### P9 — HIGH: Mods not persisted to disk

**Section:** 11 — Mods
**File:** `daemon/internal/modmanager/manager.go`
**Risk:** All installed mods vanish on daemon restart.

**Fix instructions:**
1. In the `Manager` struct, add a `dataDir string` field.
2. After every mod install/uninstall, serialize `m.mods` to `{dataDir}/mods.json` using `json.MarshalIndent` + `os.WriteFile`.
3. In `NewManager()`, load `mods.json` if it exists to restore state.
4. Same pattern as `broker.saveServersLocked()`.

---

### P10 — HIGH: Remote mods installed without verification

**Section:** 11 — Mods
**File:** `daemon/internal/modmanager/manager.go:326-366`
**Risk:** Malicious mod content accepted without integrity check.

**Fix instructions:**
1. When downloading from a known source (Thunderstore, Modrinth), fetch the expected checksum from the source API.
2. After download, compare the computed SHA-256 against the expected value.
3. If no expected checksum is available, log a WARNING and set the mod's `Checksum` field to the computed value (but mark it as unverified).
4. Never store `sha256:unverified` — always compute the real hash.

---

### P11 — HIGH: doStart goroutine not cancelled on server delete

**Section:** 13 — Health Checks
**File:** `daemon/internal/broker/broker.go:1055,990-1038`
**Risk:** Goroutine leak on repeated create/delete cycles.

**Fix instructions:**
1. Add a `contexts map[string]context.CancelFunc` field to the `Broker` struct.
2. In `StartServer()`, create a cancellable context: `ctx, cancel := context.WithCancel(context.Background())`, store `cancel` in `b.contexts[id]`.
3. Pass this `ctx` to `go b.doStart(ctx, id)`.
4. In `DeleteServer()`, call `b.contexts[id]()` to cancel the goroutine before deleting from the map.
5. Also call it in `StopServer()` to properly cancel the restart loop.

---

### P12 — HIGH: SMTP password may leak in error logs

**Section:** 14 — Notifications
**File:** `daemon/internal/notifications/service.go:217`
**Risk:** SMTP credentials visible in daemon logs.

**Fix instructions:**
1. Wrap the `smtp.SendMail` call in a function that catches the error and redacts any occurrence of the password from the error message string before logging.
2. Use `strings.ReplaceAll(err.Error(), cfg.Password, "[REDACTED]")` before passing to `zap.Error()`.

---

### P13 — HIGH: Node registration accessible without admin role

**Section:** 17 — Nodes / Cluster
**File:** `daemon/internal/api/server.go:231`
**Risk:** Any authenticated user (even viewer) can register fake nodes.

**Fix instructions:**
1. Change line 231 from `v1.POST("/nodes", s.registerNode)` to `v1.POST("/nodes", s.requireRole("admin"), s.registerNode)`.
2. Also in `cluster/manager.go`, enforce that a join token is ALWAYS required (remove the `if len(m.joinTokens) > 0` bypass).

---

### P14 — HIGH: User management operations not audited

**Section:** 20 — Security & Audit Trail
**File:** `daemon/internal/api/server.go:1010-1047`
**Risk:** User creation/modification/deletion invisible to security review.

**Fix instructions:**
1. In `createUser`, after successful creation, add: `s.recordEvent(c, "create_user", user.ID, true, gin.H{"username": req.Username, "roles": req.Roles})`.
2. In `updateUser`, add: `s.recordEvent(c, "update_user", userID, true, gin.H{"roles": req.Roles})`.
3. In `deleteUser`, add: `s.recordEvent(c, "delete_user", userID, true, nil)`.

---

### P15 — HIGH: No signature verification on self-update

**Section:** 22 — Self-Update
**File:** `daemon/internal/api/update.go:112-129`
**Risk:** Compromised update server can deliver malicious binaries.

**Fix instructions:**
1. Before executing the update script, download a `.sha256` or `.sig` file from the release.
2. Verify the checksum or GPG signature against a bundled public key.
3. Only proceed with `cmd.Start()` if verification passes.
4. As a minimum: compute SHA-256 of the downloaded script and log it so operators can audit.

---

### P16 — HIGH: CSP allows unsafe-inline (XSS exploitable)

**Section:** 26 — Installer & Deployment
**File:** `ui/Dockerfile:73`
**Risk:** Any XSS vulnerability can execute arbitrary JS.

**Fix instructions:**
1. Remove `'unsafe-inline'` from `script-src`. If inline scripts are needed for Vite/React, use nonce-based CSP or `'strict-dynamic'`.
2. For `style-src`, `'unsafe-inline'` is often required for CSS-in-JS — this is lower risk but consider using nonces too.
3. Test the UI thoroughly after the change to ensure nothing breaks.

---

### P17 — MEDIUM: No rate limiting on login / TOTP endpoints

**Section:** 01, 02 — Auth & JWT, 2FA
**Files:** `daemon/internal/api/server.go:140,222`
**Risk:** Brute-force attacks on passwords and 6-digit TOTP codes.

**Fix instructions:**
1. Add a rate-limiting middleware. Use `golang.org/x/time/rate` or a per-IP token bucket.
2. Apply to `POST /api/v1/auth/login` (e.g., 5 attempts/minute per IP).
3. Apply to `POST /api/v1/auth/totp/verify` (e.g., 5 attempts/minute per user).
4. Return 429 Too Many Requests when rate exceeded.

---

### P18 — MEDIUM: MFARequired config field never enforced

**Section:** 01 — Auth & JWT
**File:** `daemon/internal/auth/service.go:35,250-279`

**Fix instructions:**
1. In `Login()`, after the password check, add:
   ```go
   if s.cfg.MFARequired && !user.TOTPEnabled {
       return nil, fmt.Errorf("MFA is required but not set up for this account")
   }
   ```
2. This forces users to enroll in TOTP before they can log in when `mfa_required: true`.

---

### P19 — MEDIUM: HTTP 500 for operational errors (should be 409)

**Section:** 04, 05 — Server CRUD, Lifecycle
**Files:** `daemon/internal/api/server.go:344,380,392`

**Fix instructions:**
1. In `createServer`, check if `err.Error()` contains "already exists" and return 409 instead of 500.
2. In `deleteServer`, check for "cannot delete running server" and return 409.
3. In `startServer`/`stopServer`, check for "already running"/"not running" and return 409.
4. Better approach: define sentinel errors in `broker.go` (e.g., `var ErrAlreadyExists = errors.New(...)`) and check with `errors.Is()`.

---

### P20 — MEDIUM: Backup pruning doesn't delete archive files

**Section:** 10 — Backups
**File:** `daemon/internal/backup/service.go:444-472`

**Fix instructions:**
1. In the prune loop, before incrementing `pruned`, call:
   ```go
   archiveDir := s.localArchiveDir(r.Target, r.ID)
   os.RemoveAll(archiveDir)
   ```
2. Log the deletion for traceability.

---

### P21 — MEDIUM: Backup records in-memory only

**Section:** 10 — Backups
**File:** `daemon/internal/backup/service.go:80,105`

**Fix instructions:**
1. Add a `persistRecords()` method that writes `s.records` to `{DataDir}/backup-records.json`.
2. Call it after every backup completion and prune.
3. In `NewService()`, load the file if it exists.

---

### P22 — MEDIUM: Invalid cron expressions silently accepted

**Section:** 06 — Scheduling
**File:** `daemon/internal/broker/broker.go:920-982`

**Fix instructions:**
1. In `UpdateServer()`, before storing schedule expressions, validate them with `cron.Parse()`.
2. If invalid, return an error that the API handler can map to 400.

---

### P23 — MEDIUM: Console stream shared channel (multiple viewers steal messages)

**Section:** 09 — Logs
**File:** `daemon/internal/broker/broker.go:2098-2106`

**Fix instructions:**
1. Replace the single `chan string` per server with a broadcast/fan-out pattern.
2. Create a `consoleBroadcaster` type that maintains a set of subscriber channels.
3. `sendConsoleMessage` sends to the broadcaster; each WebSocket client gets its own subscriber channel.
4. When a client disconnects, remove its channel from the subscriber set.

---

### P24 — MEDIUM: Symlink escape in file browser

**Section:** 07 — File Browser
**File:** `daemon/internal/broker/broker.go:2644-2657`

**Fix instructions:**
1. In `configFilePath()`, after computing `abs`, call `filepath.EvalSymlinks(abs)`.
2. Re-check that the resolved path still starts with `filepath.Clean(installDir)`.
3. If not, return the path-outside-directory error.

---

### P25 — MEDIUM: No file size limit on upload

**Section:** 07 — File Browser
**File:** `daemon/internal/api/server.go:693-725`

**Fix instructions:**
1. At the top of `uploadFile()`, set a max body size:
   ```go
   c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 100<<20) // 100MB
   ```
2. Return 413 if exceeded.

---

### P26 — MEDIUM: No backup before config file overwrite

**Section:** 08 — Config Editor
**File:** `daemon/internal/broker/broker.go:2695-2713`

**Fix instructions:**
1. Before `os.WriteFile`, check if the file exists. If so, copy it to `abs + ".bak"`.
2. Keep only the last backup (overwrite previous `.bak`).

---

### P27 — MEDIUM: RestartServer timeout race + blocking HTTP

**Section:** 05 — Server Lifecycle
**File:** `daemon/internal/broker/broker.go:1371-1386`

**Fix instructions:**
1. Make `RestartServer` async like Start/Stop: return immediately, spawn a goroutine.
2. In the goroutine, poll until stopped or timeout, then start. If timeout, set error state instead of starting anyway.

---

### P28 — MEDIUM: Notification spam during crash loops

**Section:** 14 — Notifications
**File:** `daemon/internal/broker/broker.go:1286`

**Fix instructions:**
1. In the notification service, add a per-event rate limiter: at most 1 notification per event type per server per 5 minutes.
2. Use a map like `lastNotified map[string]time.Time` keyed by `"serverID:eventType"`.
3. Skip notification if the last one for the same key was < 5 minutes ago.

---

### P29 — MEDIUM: Audit log no pagination + not persisted

**Section:** 20 — Security & Audit Trail
**Files:** `daemon/internal/auth/service.go:696-698`, `daemon/internal/api/server.go:1049-1056`

**Fix instructions:**
1. Add `offset` and `limit` query params to `getAuditLog` handler. Default limit: 100, max: 1000.
2. For persistence: write audit entries to a JSONL (JSON Lines) file, one entry per line. Append-only.
3. Load the file on startup. Cap at a configurable max (e.g., 100k entries; rotate old ones).

---

### P30 — MEDIUM: No integrity check before backup restore

**Section:** 10 — Backups
**File:** `daemon/internal/backup/service.go:275-301`

**Fix instructions:**
1. Before calling `restorePath()`, compute SHA-256 of the archive file.
2. Compare against `record.Checksum`.
3. If mismatch, fail the restore with a clear error.

---

### P31 — MEDIUM: Missing HSTS header

**Section:** 26 — Installer
**File:** `install.sh:856-905`

**Fix instructions:**
1. Add to the nginx server block:
   ```nginx
   add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
   ```

---

### P32 — MEDIUM: Docker nonroot user can't write VOLUME dirs

**Section:** 26 — Installer
**File:** `daemon/Dockerfile:37,46`

**Fix instructions:**
1. Add before the `USER nonroot` line:
   ```dockerfile
   RUN chown -R nonroot:nonroot /etc/games-dashboard /var/lib/games-dashboard
   ```

---

### P33 — MEDIUM: OpenAPI spec missing many routes

**Section:** 24 — API Contract
**File:** `docs/api-reference.yaml`

**Fix instructions:**
1. Add all missing route groups to the spec: admin, auth/api-keys, push, firewall, nodes, system, clone, mods/test, mods/rollback.
2. Use the existing spec format. Document request/response schemas and status codes.

---

### P34 — MEDIUM: CI pipeline `|| true` masks test + SAST failures

**Section:** 25 — CI/CD
**File:** `.github/workflows/ci.yml:103,118,131,140,440`

**Fix instructions:**
1. Remove `|| true` from the integration test step (line 440).
2. For SAST, either remove `|| true` to make findings blocking, or keep `|| true` but add a `continue-on-error: true` at the job level so failures are visible as warnings in the PR.

---

### P35 — MEDIUM: CVE scan no timeout

**Section:** 21 — SBOM & CVE
**File:** `daemon/internal/broker/broker.go:2492`

**Fix instructions:**
1. Replace `context.Background()` with `context.WithTimeout(context.Background(), 5*time.Minute)`.

---

### P36 — MEDIUM: No timeout on self-update process

**Section:** 22 — Self-Update
**File:** `daemon/internal/api/update.go:123`

**Fix instructions:**
1. Use `exec.CommandContext(ctx, ...)` with a 10-minute timeout context instead of bare `exec.Command`.

---

### P37-P63 — LOW priority items

These are lower-risk quality improvements. Fix them as time permits:

- **P37** Recovery codes stored in plaintext (S01) — hash them with bcrypt
- **P38** Recovery codes use non-constant-time comparison (S02) — use `subtle.ConstantTimeCompare`
- **P39** tokenCache memory leak (S01) — add periodic eviction of expired entries
- **P40** Clone server shallow config copy (S04) — use `encoding/json` round-trip for deep copy
- **P41** Start/stop errors return 500 not 409 (S05) — same fix as P19
- **P42** Download reads entire file into memory (S07) — use `c.File()` for streaming
- **P43** Cannot delete directories in file browser (S07) — add recursive delete or remove misleading error
- **P44** Config write not atomic (S08) — write to temp file + `os.Rename`
- **P45** No upper bound on log lines param (S09) — cap at 10,000
- **P46** Mod install while server running (S11) — add state check
- **P47** Ban not persisted for RCON-less games (S12) — fallback file storage
- **P48** Health check grace period edge case (S13) — suppress crash notifications during grace
- **P49** Webhook URL in error logs (S14) — redact URL
- **P50** Dead push subscriptions not cleaned (S15) — call RemovePushSubscription on 410
- **P51** SBOM returns empty placeholder (S21) — return 404 before first generation
- **P52** No backup before daemon self-update (S22) — copy current binary first
- **P53** Setup wizard Skip button not disabled (S23) — tie to loading state
- **P54** DashboardPage disk warning null check (S23) — explicit check
- **P55** Diagnostics context cancel leak (S19) — defer cancel()
- **P56** All auth state in-memory (S01) — persist users to JSON file (large effort)

---

## Clean sections (no findings)

| # | Section |
|---|---------|
| 03 | API Keys |
| 16 | Ports / UFW |
| 18 | System Resources |

---

## Recurring themes for Sonnet to watch for

1. **In-memory-only storage** — Users, audit, backups, mods all need persistence. Use the `saveServersLocked()` pattern: JSON file, load on boot, save after mutation.
2. **Missing input validation** — Always validate user-supplied IDs, names, cron expressions, paths.
3. **HTTP status code misuse** — Use sentinel errors + `errors.Is()` to map broker errors to correct HTTP codes.
4. **No rate limiting** — Add a middleware; apply to auth endpoints.
5. **Hardcoded insecure defaults** — Remove all default secrets/passwords.
