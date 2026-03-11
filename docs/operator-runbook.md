# Operator Runbook

## Table of Contents
1. [Quick Dev/Test Install](#quick-devtest-install)
2. [Initial Installation](#installation)
3. [Upgrade Procedure](#upgrade)
4. [Rollback Procedure](#rollback)
5. [Emergency Recovery](#emergency-recovery)
6. [Key Rotation](#key-rotation)
7. [Adding a Game Server](#adding-a-game-server)
8. [Backup & Restore](#backup--restore)
9. [Scaling (Kubernetes)](#scaling)
10. [Troubleshooting](#troubleshooting)

---

## Quick Dev/Test Install

The fastest way to validate the full stack on a fresh machine — one command, no pre-installed dependencies required beyond `bash` and internet access.

**Minimum:** Ubuntu 24.04 (or any modern Linux x86-64)

```bash
curl -fsSL https://raw.githubusercontent.com/Chrisl154/Gmaer-Server-Daashboard/main/test-live.sh | bash
```

Or if the repo is already cloned locally:

```bash
bash test-live.sh
```

**What it installs automatically (all in userspace, no root needed):**

| Tool | How |
|---|---|
| Go 1.22 | Downloaded to `~/.local/go` |
| Node.js 20 LTS | Installed via NVM to `~/.nvm` |
| Trivy (CVE scanner) | Downloaded to `~/.local/bin` |
| `pyyaml`, `bcrypt` | `pip install --user` |
| `git`, `openssl`, `python3`, `curl` | `apt-get` (already present on Ubuntu 24.04) |

**What it tests:**

| Step | Description |
|---|---|
| Step 0 | Detect OS, install all requirements |
| Step 1 | Clone / pull the repository |
| Step 2 | Build daemon binary (`games-daemon`) |
| Step 3 | Build CLI binary (`gdash`) |
| Step 4 | Generate self-signed TLS certificate |
| Step 5 | Write config, start daemon |
| Step 6 | API tests (54 endpoints: health, auth, server CRUD, backup, mods, SBOM, admin) |
| Step 7 | CLI smoke tests (all `gdash` commands) |
| Step 8 | Go unit tests (all daemon packages) |

Expected output on a clean Ubuntu 24.04 instance:

```
══ Results ══
  ALL TESTS PASSED: 54/54
```

---

## Installation

### Docker Mode (Single Host)

```bash
# 1. Download installer
curl -fsSL https://github.com/Chrisl154/Gmaer-Server-Daashboard/releases/latest/download/installer.sh \
  -o installer.sh && chmod +x installer.sh

# 2. Run interactive install
./installer.sh --mode docker --min-hardware-profile small

# 3. Verify
curl -fsk https://localhost:8443/healthz
curl -fsk https://localhost:8443/api/v1/version
```

### Headless Install

```bash
# Create config
cat > config.json << 'EOF'
{
  "mode": "docker",
  "install_dir": "/opt/games-dashboard",
  "auth": {
    "enable_local": true,
    "local_admin": { "username": "admin", "password_hash": "<bcrypt>" }
  }
}
EOF

./installer.sh --headless --config config.json --accept-licenses --log-level info
```

### Kubernetes (k3s)

```bash
./installer.sh --mode k8s --k8s-distribution k3s \
  --install-helm --install-metalb --install-csi-nfs \
  --min-hardware-profile medium
```

Post-install verification:
```bash
kubectl get pods -n games-dashboard
kubectl get gameinstances -A
curl -fsk https://$(kubectl get svc -n games-dashboard games-dashboard-ui -o jsonpath='{.status.loadBalancer.ingress[0].ip}')/healthz
```

### Manual (Build from Source)

For development or environments without Docker:

```bash
git clone https://github.com/Chrisl154/Gmaer-Server-Daashboard.git
cd Gmaer-Server-Daashboard

# Build binaries (Go 1.22+ required)
cd daemon && go build -o /usr/local/bin/games-daemon ./cmd/daemon
cd ../cli  && go build -o /usr/local/bin/gdash ./cmd

# Generate TLS certificate
bash installer/scripts/generate-tls.sh /etc/games-dashboard/tls

# Write config (copy and edit the example)
mkdir -p /etc/games-dashboard
cp installer/configs/daemon.yaml /etc/games-dashboard/daemon.yaml

# Generate admin password hash
python3 -c "import bcrypt; print(bcrypt.hashpw(b'YourPassword', bcrypt.gensalt()).decode())"
# Paste the hash into daemon.yaml → auth.local.admin_pass_hash

# Start daemon
games-daemon --config /etc/games-dashboard/daemon.yaml
```

---

## Upgrade

### Docker Mode

```bash
# 1. Pull new images
docker compose -f /opt/games-dashboard/docker-compose.yml pull

# 2. Rolling restart
docker compose -f /opt/games-dashboard/docker-compose.yml up -d

# 3. Verify
docker compose -f /opt/games-dashboard/docker-compose.yml ps
curl -fsk https://localhost:8443/api/v1/version
```

### Kubernetes (Helm)

```bash
# 1. Update Helm repo
helm repo update

# 2. Upgrade
helm upgrade games-dashboard games-dashboard/games-dashboard \
  --namespace games-dashboard \
  --version 1.1.0 \
  --reuse-values \
  --wait --timeout 300s

# 3. Verify
kubectl rollout status deployment/games-dashboard-daemon -n games-dashboard
```

---

## Rollback

### To Installer Checkpoint

```bash
# List checkpoints
ls /opt/games-dashboard/checkpoints/

# Rollback to specific checkpoint
./installer.sh --rollback-to checkpoint-3
```

### Docker Compose (image rollback)

```bash
# Edit docker-compose.yml to pin previous image tag
# then:
docker compose up -d
```

### Helm Rollback

```bash
helm history games-dashboard -n games-dashboard
helm rollback games-dashboard 2 -n games-dashboard --wait
```

---

## Emergency Recovery

### Daemon Won't Start

```bash
# 1. Check logs
docker logs games-dashboard-daemon --tail 100
# or: journalctl -u games-dashboard-daemon -n 100
# or (bare metal): cat /var/log/games-dashboard/daemon.log

# 2. Verify config is valid YAML
python3 -c "import yaml; yaml.safe_load(open('/etc/games-dashboard/daemon.yaml'))" && echo OK

# 3. Check TLS certificate validity
openssl x509 -in /etc/games-dashboard/tls/server.crt -noout -dates

# 4. Regenerate self-signed cert if expired
bash installer/scripts/generate-tls.sh /etc/games-dashboard/tls

# 5. Last resort reset (DATA LOSS)
./installer.sh --mode docker --force --reuse-existing
```

### Cannot Authenticate

```bash
# Reset admin password:

# 1. Stop daemon
docker stop games-dashboard-daemon

# 2. Generate new bcrypt hash
python3 -c "import bcrypt; print(bcrypt.hashpw(b'NewPassword123!', bcrypt.gensalt()).decode())"

# 3. Update config
# Edit /etc/games-dashboard/daemon.yaml → auth.local.admin_pass_hash

# 4. Restart
docker start games-dashboard-daemon

# 5. Verify
gdash auth login -u admin -p NewPassword123!
```

### Corrupt Backup

```bash
# Verify backup integrity
sha256sum /var/lib/games-dashboard/backups/valheim/*.tar.gz

# Restore from a previous backup
gdash backup list valheim-1
gdash backup restore valheim-1 <previous-backup-id>
```

---

## Key Rotation

```bash
# 1. Ensure all servers are backed up first
gdash backup create --all-servers --type full

# 2. Rotate encryption keys (via CLI)
gdash admin secrets rotate

# 3. Or via API
curl -X POST https://localhost:8443/api/v1/admin/secrets/rotate \
  -H "Authorization: Bearer $GDASH_TOKEN"

# 4. Verify services still healthy
curl -fsk https://localhost:8443/healthz
```

---

## Adding a Game Server

### Via CLI

```bash
# 1. Create server record
gdash server create valheim-prod "Valheim Production" \
  --adapter valheim \
  --deploy-method steamcmd \
  --install-dir /mnt/games/valheim-prod

# 2. Deploy (downloads game files via SteamCMD)
gdash server deploy valheim-prod --method steamcmd --app-id 896660

# 3. Start
gdash server start valheim-prod

# 4. Check status
gdash server get valheim-prod
gdash server logs valheim-prod
```

### Via Web UI

1. Navigate to **Servers** → **Add Server**
2. Fill in Name, Adapter (choose from 24 games), Deploy Method
3. Click **Create**, then **Deploy**
4. Click **Start** once deployed

### Via API

```bash
TOKEN=$(curl -sk -X POST https://localhost:8443/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

curl -sk -X POST https://localhost:8443/api/v1/servers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id":"valheim-prod","name":"Valheim Production","adapter":"valheim","deploy_method":"steamcmd"}'
```

---

## Backup & Restore

### Manual Backup

```bash
gdash backup create valheim-1 --type full
```

### Scheduled Backup

Configure in `daemon.yaml`:
```yaml
backup:
  default_schedule: "0 3 * * *"   # 3am daily
  retain_days: 30
  compression: "gzip"
```

### Restore

```bash
# List available backups
gdash backup list valheim-1

# Restore (will stop server first, then restart after restore)
gdash backup restore valheim-1 <backup-id>
```

### NFS Backup Target

```yaml
storage:
  nfs_mounts:
    - server: 10.0.0.5
      path: /exports/games
      mount_point: /mnt/games
      options: rw
```

### S3 Backup Target

```yaml
storage:
  s3:
    endpoint: https://s3.example.com
    bucket: game-backups
    access_key: AKIA...      # or use secrets backend
    secret_key: SECRET       # or use secrets backend
    use_ssl: true
```

---

## Scaling

### Add Kubernetes Node

```bash
# On master — get join token
cat /var/lib/rancher/k3s/server/node-token

# On new worker node
curl -sfL https://get.k3s.io | K3S_URL=https://MASTER_IP:6443 K3S_TOKEN=TOKEN sh -

# Register node with Games Dashboard
gdash node add worker-01 https://worker-01:8443
```

### Scale Operator Replicas

```bash
helm upgrade games-dashboard games-dashboard/games-dashboard \
  --set replicaCount=2 \
  -n games-dashboard
```

---

## Troubleshooting

| Symptom | Check | Fix |
|---|---|---|
| UI not loading | `curl -fsk https://localhost:443` | Check UI container logs |
| Daemon unhealthy | `curl -fsk https://localhost:8443/healthz` | Check daemon logs, TLS certs |
| `api/v1/version` 401 | Check daemon version (pre-fix) | Upgrade to latest — version endpoint is now public |
| Game server won't start | `gdash server logs <id>` | Check adapter manifest, install dir, SteamCMD path |
| SteamCMD fails | Network to Steam endpoints | Use offline bundle or set `install_dir` |
| Backup fails | Target mount, S3 credentials | Check storage config in `daemon.yaml` |
| CVE scan shows "no scanner" | Trivy/Grype not in PATH | `bash test-live.sh` installs Trivy automatically |
| WebSocket disconnects | Proxy timeout | Increase proxy read timeout to 300s |
| Port conflict on :8443 | `lsof -ti:8443` | Change `bind_addr` in `daemon.yaml` |
| `gdash: command not found` | CLI not built or not in PATH | `cd cli && go build -o ~/.local/bin/gdash ./cmd` |

### Collect Diagnostic Bundle

```bash
./installer.sh --output-audit /tmp/diag-bundle.json
# Includes: preflight-report, install-audit, daemon logs, CVE report
```

### Useful One-Liners

```bash
# Check daemon is responding
curl -fsk https://localhost:8443/healthz | python3 -m json.tool

# Get daemon version (no auth required)
curl -fsk https://localhost:8443/api/v1/version

# Tail daemon logs
docker logs -f games-dashboard-daemon

# List all servers via API
curl -fsk https://localhost:8443/api/v1/servers \
  -H "Authorization: Bearer $GDASH_TOKEN" | python3 -m json.tool

# Run the full test suite against a running daemon
GDASH_DAEMON_URL=https://localhost:8443 \
GDASH_ADMIN_PASSWORD=changeme \
  bash tests/integration/run-tests.sh
```
