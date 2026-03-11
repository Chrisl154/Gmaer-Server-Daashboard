# Contributing to Games Dashboard

We welcome contributions! This guide covers setting up a dev environment, adding adapters, extending the mod manager, and running the test suite.

---

## Development Setup

### Prerequisites

| Tool | Minimum Version | Notes |
|---|---|---|
| Go | 1.22 | The one-liner installs this automatically |
| Node.js | 20 LTS | The one-liner installs this automatically via NVM |
| Python 3 | 3.8+ | Ships with Ubuntu 24.04 |
| git, openssl | any | Ships with Ubuntu 24.04 |

### Option A — One-Liner (Recommended)

Clone, build, start the daemon, and run all tests automatically on any fresh Linux box:

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/test-live.sh | bash
```

Everything is installed in userspace — no root required beyond standard system packages.

### Option B — Manual Setup

```bash
git clone https://github.com/Chrisl154/Gmaer-Server-Daashboard.git
cd Gmaer-Server-Daashboard

# Install Go 1.22 (if not present)
mkdir -p ~/.local
curl -fsSL https://go.dev/dl/go1.22.4.linux-amd64.tar.gz | tar -xz -C ~/.local/
export PATH="$HOME/.local/go/bin:$PATH"

# Install Node.js 20 via NVM (if not present)
curl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash
source ~/.nvm/nvm.sh
nvm install 20 && nvm use 20

# Build daemon
cd daemon && go mod tidy && go build -o /tmp/games-daemon ./cmd/daemon

# Build CLI
cd ../cli && go mod tidy && go build -o /tmp/gdash ./cmd

# Build UI (production)
cd ../ui && npm install && npm run build

# UI dev server (hot-reload)
cd ../ui && npm run dev
```

> **Note:** After a fresh `git checkout`, run `chmod +x ui/node_modules/.bin/*` if any UI scripts fail with permission errors.

### Starting a Local Daemon for Development

```bash
# Generate self-signed TLS cert (one-time)
mkdir -p /tmp/dev-daemon/{tls,secrets,data,config}
openssl req -x509 -newkey rsa:2048 \
  -keyout /tmp/dev-daemon/tls/server.key \
  -out /tmp/dev-daemon/tls/server.crt \
  -days 365 -nodes -subj "/CN=localhost" \
  -addext "subjectAltName=IP:127.0.0.1,DNS:localhost"

# Generate admin password hash
python3 -c "import bcrypt; print(bcrypt.hashpw(b'DevPass123!', bcrypt.gensalt()).decode())"

# Write config
cat > /tmp/dev-daemon/config/daemon.yaml << EOF
bind_addr: ":8443"
log_level: "debug"
tls:
  cert_file: "/tmp/dev-daemon/tls/server.crt"
  key_file:  "/tmp/dev-daemon/tls/server.key"
auth:
  local:
    enabled: true
    admin_user: "admin"
    admin_pass_hash: "<paste hash here>"
  jwt_secret: "dev-secret"
  token_ttl: 24h
secrets:
  backend: "local"
  key_file: "/tmp/dev-daemon/secrets/master.key"
storage:
  data_dir: "/tmp/dev-daemon/data"
adapters:
  dir: "./adapters"
metrics:
  enabled: true
  path: "/metrics"
cluster:
  enabled: false
EOF

# Start daemon
/tmp/games-daemon --config /tmp/dev-daemon/config/daemon.yaml \
  --tls-cert /tmp/dev-daemon/tls/server.crt \
  --tls-key /tmp/dev-daemon/tls/server.key \
  --bind :8443

# In another terminal — verify it's up
curl -fsk https://localhost:8443/healthz
curl -fsk https://localhost:8443/api/v1/version
```

---

## Running Tests

### Full Automated Suite (Recommended)

```bash
bash test-live.sh
```

Runs everything: build, daemon start, API tests, CLI smoke tests, Go unit tests.

### Individual Suites

```bash
# Go unit tests (daemon packages)
cd daemon && go test ./... -race

# UI unit tests (Vitest)
cd ui && npm test

# UI TypeScript type-check
cd ui && npm run build   # fails on type errors

# Integration suite (requires daemon on :8443)
GDASH_DAEMON_URL=https://localhost:8443 \
GDASH_ADMIN_PASSWORD=DevPass123! \
  bash tests/integration/run-tests.sh

# CLI E2E smoke tests (requires daemon + gdash binary)
GDASH_DAEMON=https://localhost:8443 \
GDASH_ADMIN_PASSWORD=DevPass123! \
GDASH_BIN=/tmp/gdash \
  bash tests/e2e/cli-smoke.sh
```

### Test Results (current baseline)

| Suite | Tests | Result |
|---|---|---|
| Go unit tests | 5 packages | PASS |
| UI Vitest | 27 tests | 27/27 PASS |
| UI TypeScript | — | Clean |
| CLI smoke | all commands | PASS |
| Integration | 68 checks | 65 PASS, 3 SKIP (CVE scanner, TLS path) |

---

## Adding a Game Adapter

1. Create a directory: `adapters/<game-id>/`
2. Create `manifest.yaml` following this schema:

```yaml
id: mygame
name: My Game Server
engine: Custom
steam_app_id: "123456"   # or null if not on Steam
deploy_methods: [steamcmd, manual]
start_command: "./server -port 27015"
stop_command: "kill -TERM $(cat /tmp/server.pid)"
restart_command: "..."
console:
  type: rcon   # rcon | websocket | stdio | telnet | webrcon
  attach_command: "..."
  rcon_enabled: true
  rcon_port: 27015
backup_paths:
  - /opt/mygame/saves
config_templates:
  - path: /opt/mygame/server.cfg
    description: "Main config"
    sample: |
      port=27015
      maxplayers=16
ports:
  - internal: 27015
    default_external: 27015
    protocol: udp
health_checks:
  - type: tcp
    host: localhost
    port: 27015
    timeout_seconds: 5
mod_support: false
recommended_resources:
  cpu_cores: 2
  ram_gb: 4
  disk_gb: 5
notes: "Additional notes"
```

3. Create `test-harness.sh` with start/stop/backup/restore tests (see `adapters/valheim/test-harness.sh` as a reference)
4. Add the adapter to the table in `README.md`
5. Submit a PR

---

## Adding a Mod Source

Implement the `ModSource` interface in `daemon/internal/modmanager/`:

```go
type ModSource interface {
    FetchMod(ctx context.Context, id, version string) (*Mod, error)
    ListVersions(ctx context.Context, id string) ([]string, error)
    Checksum(ctx context.Context, id, version string) (string, error)
}
```

Register in `daemon/internal/modmanager/manager.go`.

---

## Code Style

- **Go:** `gofmt`, `golangci-lint run`
- **TypeScript:** ESLint + Prettier (config in `ui/.eslintrc`)
- **Commit messages:** `type(scope): description`
  - Types: `feat`, `fix`, `docs`, `chore`, `test`, `refactor`
  - Example: `fix(api): make /version endpoint public`

---

## PR Checklist

- [ ] `bash test-live.sh` passes end-to-end
- [ ] `cd daemon && go test ./... -race` passes
- [ ] `cd ui && npm test` passes
- [ ] Lint passes (`gofmt`, `golangci-lint`)
- [ ] For new adapters: `manifest.yaml` is valid YAML, test harness added
- [ ] For API changes: `docs/api-reference.yaml` updated, integration test updated
- [ ] SBOM/CVE scan clean (trivy/grype if available)
- [ ] `README.md` updated if new games, features, or flags added
