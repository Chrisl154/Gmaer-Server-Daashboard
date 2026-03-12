# Changelog

All notable changes to Games Dashboard will be documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

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
