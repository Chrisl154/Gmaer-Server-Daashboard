# Current Logged Bugs

Test run: 2026-03-18 — dev branch (commit 863710c7)
Baseline: 42/42 PASS (test-live.sh against main)
Dev-branch targeted tests: 13 PASS / 4 FAIL (real bugs) / 3 env/test issues

---

## BUG-001 — GET /banlist and GET /whitelist return HTTP 400 when server is stopped

**Endpoints:** `GET /api/v1/servers/:id/banlist`, `GET /api/v1/servers/:id/whitelist`

**Observed:** HTTP 400 `{"error":"server must be running to query player data"}`

**Expected:** HTTP 200 `{"supported": true, "online": false, "players": []}` so the UI tab loads cleanly and shows a "start the server to manage this list" message instead of a query error.

**Impact:** Medium — Players tab on a stopped Minecraft server shows a red error banner ("Could not load ban list — server must be running with RCON enabled") instead of a structured empty state. Functional but poor UX.

**Root cause:** `listBannedPlayers` / `listWhitelistPlayers` handlers fall through to `c.JSON(http.StatusBadRequest, ...)` for any non-`ErrBanlistNotSupported` error, including "server not running".

**Fix:** In `daemon/internal/api/banlist.go`, detect "not running" errors and return HTTP 200 with `{"supported": true, "online": false, "players": []}`.

---

## BUG-002 — GET /files returns HTTP 400 when server install directory does not exist

**Endpoint:** `GET /api/v1/servers/:id/files`

**Observed:** HTTP 400 `{"error":"cannot list directory: open /opt/gdash/data/servers: no such file or directory"}`

**Expected:** HTTP 200 `{"entries": [], "path": "/"}` (empty listing) or a structured "not deployed" response — not a 400.

**Impact:** Medium — File browser tab crashes with an error on any server that hasn't been deployed yet. Common case for newly-created servers.

**Root cause:** File browser handler returns the raw `os.ReadDir` error to the client as a 400 when the directory simply doesn't exist yet.

**Fix:** In the `listFiles` handler, check for `os.IsNotExist(err)` and return an empty `{"entries": []}` instead of propagating the error.

---

## BUG-003 — GET /firewall returns HTTP 500 when ufw is not accessible

**Endpoint:** `GET /api/v1/firewall`

**Observed:** HTTP 500 `{"error":"could not read firewall status: ufw status: exit status 1"}`

**Expected:** HTTP 200 `{"available": false, "enabled": false, "rules": []}` — the UI should display a "UFW not available on this system" notice rather than crashing the firewall page.

**Impact:** Low-Medium — Firewall tab shows a server error instead of a graceful "not available" state. Affects any deployment without ufw (non-Ubuntu, or no sudo access).

**Root cause:** Firewall service propagates the raw ufw exec error up to the API handler which returns 500.

**Fix:** In the firewall service / handler, catch `exec` errors and return a structured not-available response with HTTP 200.

---

## Non-bugs / Dismissed

- **PATCH /admin/notifications returns `{"ok":true}`** — by design; test assertion was incorrect. Handler intentionally returns ok confirmation, not the full config echo.
- **GET /files 400 in test env** — overlaps with BUG-002 above; root cause is missing install_dir on the test server.
- **GET /firewall 500 in test env** — overlaps with BUG-003 above; ufw requires elevated permissions not present in test environment.
