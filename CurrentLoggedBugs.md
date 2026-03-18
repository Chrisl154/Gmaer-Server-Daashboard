# Current Logged Bugs

Test run: 2026-03-18 — dev branch

---

## BUG-001 — Banlist/whitelist returns HTTP 400 when server is stopped

**Endpoints:** `GET /api/v1/servers/:id/banlist`, `GET /api/v1/servers/:id/whitelist`

**Observed:** HTTP 400 `{"error":"server must be running to query player data"}`

**Expected:** HTTP 200 `{"supported": true, "online": false, "players": []}` so the UI tab loads cleanly.

**Root cause:** `listBannedPlayers` / `listWhitelistPlayers` handlers returned 400 for any non-`ErrBanlistNotSupported` error.

**Fix:** In `daemon/internal/api/banlist.go`, detect "not running" errors and return HTTP 200 with `{"supported": true, "online": false, "players": []}`.

**Status:** FIXED

---

## BUG-002 — GET /files returns HTTP 400 when server install directory does not exist

**Endpoint:** `GET /api/v1/servers/:id/files`

**Observed:** HTTP 400 `{"error":"cannot list directory: open /opt/gdash/data/servers: no such file or directory"}`

**Expected:** HTTP 200 `{"entries": [], "path": "/"}` for a not-yet-deployed server.

**Root cause:** File browser handler returned the raw `os.ReadDir` error as 400 when the directory did not exist.

**Fix:** In `broker.ListFiles`, check `os.IsNotExist(err)` and return an empty slice instead of propagating the error.

**Status:** FIXED

---

## BUG-003 — GET /firewall returns HTTP 500 when ufw is not accessible

**Endpoint:** `GET /api/v1/firewall`

**Observed:** HTTP 500 `{"error":"could not read firewall status: ufw status: exit status 1"}`

**Expected:** HTTP 200 `{"available": false, "enabled": false, "rules": []}`.

**Root cause:** Firewall handler propagated the raw ufw exec error as 500.

**Fix:** In `daemon/internal/api/firewall.go`, return `{Available: false}` with HTTP 200 on any error.

**Status:** FIXED

---

## BUG-004 — IsInitialized() false-positive blocks bootstrap when admin has no password hash

**File:** `daemon/internal/auth/service.go`

**Symptom:** With `admin_user: admin` in config but no `admin_pass_hash`, `IsInitialized()` returns `true`. Bootstrap returns 409, but login fails 401 permanently.

**Root cause:** `NewService` seeded the admin user even when `PasswordHash == ""`. `IsInitialized()` checks `len(s.users) > 0`, not whether any user has a usable hash.

**Fix:** Only seed admin user into `s.users` when `PasswordHash != ""`.

**Status:** FIXED

---

## Non-bugs / Dismissed

- **PATCH /admin/notifications returns `{"ok":true}`** — by design; handler intentionally confirms success without echoing the full config back.
