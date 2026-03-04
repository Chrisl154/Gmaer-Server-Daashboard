# Contributing to Games Dashboard

We welcome contributions! This guide covers adding adapters, extending the mod manager, and running the test suite.

## Development Setup

```bash
git clone https://github.com/games-dashboard/games-dashboard.git
cd games-dashboard

# Daemon
cd daemon && go mod download && go build ./...

# UI
cd ui && npm install && npm run dev

# CLI
cd cli && go build ./cmd
```

## Adding a Game Adapter

1. Create directory: `adapters/<game-id>/`
2. Create `manifest.yaml` following the schema:

```yaml
id: mygame
name: My Game Server
engine: Custom
steam_app_id: "123456"   # or null
deploy_methods: [steamcmd, manual]
start_command: "./server -port 27015"
stop_command: "kill -TERM $(cat /tmp/server.pid)"
restart_command: "..."
console:
  type: rcon   # rcon | websocket | stdio
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

3. Create `test-harness.sh` with start/stop/backup/restore tests (see `adapters/valheim/test-harness.sh`)
4. Submit a PR

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

## Running Tests

```bash
# Unit tests
cd daemon && go test ./... -race

# UI tests
cd ui && npm test

# Adapter test harness
docker pull lloesche/valheim-server
./adapters/valheim/test-harness.sh

# Full integration suite
./tests/integration/run-tests.sh --mode docker
```

## Code Style

- Go: `gofmt`, `golangci-lint`
- TypeScript: ESLint + Prettier (config in `ui/.eslintrc`)
- Commit messages: `type(scope): description`
  - Types: `feat`, `fix`, `docs`, `chore`, `test`, `refactor`

## PR Checklist

- [ ] Tests pass (`go test ./...`, `npm test`)
- [ ] Lint passes
- [ ] Adapter manifest valid YAML
- [ ] Test harness passes on Docker
- [ ] Docs updated if API changed
- [ ] SBOM/CVE scan clean
