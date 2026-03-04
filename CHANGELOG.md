# Changelog

All notable changes to Games Dashboard will be documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

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
