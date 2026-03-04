# Operator Runbook

## Table of Contents
1. [Initial Installation](#installation)
2. [Upgrade Procedure](#upgrade)
3. [Rollback Procedure](#rollback)
4. [Emergency Recovery](#emergency-recovery)
5. [Key Rotation](#key-rotation)
6. [Adding a Game Server](#adding-a-game-server)
7. [Backup & Restore](#backup--restore)
8. [Scaling (Kubernetes)](#scaling)
9. [Troubleshooting](#troubleshooting)

---

## Installation

### Docker Mode (Single Host)

```bash
# 1. Download installer
curl -fsSL https://github.com/games-dashboard/games-dashboard/releases/latest/download/installer.sh \
  -o installer.sh && chmod +x installer.sh

# 2. Run interactive install
./installer.sh --mode docker --min-hardware-profile small

# 3. Verify
curl -fsk https://localhost:8443/healthz
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

# 2. Check config
cat /opt/games-dashboard/config/daemon.yaml

# 3. Check TLS
openssl x509 -in /opt/games-dashboard/tls/server.crt -noout -dates

# 4. Reset to factory defaults (DATA LOSS)
./installer.sh --mode docker --force --reuse-existing
```

### Cannot Authenticate

```bash
# Reset admin password via daemon config
# 1. Stop daemon
docker stop games-dashboard-daemon

# 2. Generate new bcrypt hash
python3 -c "import bcrypt; print(bcrypt.hashpw(b'newpassword', bcrypt.gensalt()).decode())"

# 3. Update config
vim /opt/games-dashboard/config/daemon.yaml
# Change: auth.local.admin_pass_hash

# 4. Restart
docker start games-dashboard-daemon
```

### Corrupt Backup

```bash
# Verify backup integrity
sha256sum /mnt/games/backups/valheim/*.tar.gz
# Compare against checksums stored in install-audit.json

# Restore from a different backup
gdash backup restore valheim-1 <previous-backup-id>
```

---

## Key Rotation

```bash
# 1. Ensure all secrets are backed up
gdash backup create --all-servers --type full

# 2. Rotate encryption keys
gdash admin secrets rotate
# or via API:
curl -X POST https://localhost:8443/api/v1/admin/secrets/rotate \
  -H "Authorization: Bearer $GDASH_TOKEN"

# 3. Verify services still healthy
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

# 2. Deploy
gdash server deploy valheim-prod --method steamcmd --app-id 896660

# 3. Start
gdash server start valheim-prod

# 4. Check status
gdash server get valheim-prod
```

### Via Web UI

1. Navigate to **Servers** → **Add Server**
2. Fill in Name, Adapter, Deploy Method
3. Click **Create**, then **Deploy**
4. Click **Start** once deployed

---

## Backup & Restore

### Manual Backup

```bash
gdash backup create valheim-1 --type full
```

### Restore

```bash
# List available backups
gdash backup list valheim-1

# Restore (will stop server first)
gdash backup restore valheim-1 <backup-id>
```

### NFS Backup Target

Configure in `daemon.yaml`:
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
    access_key: AKIA...
    secret_key: SECRET
    use_ssl: true
```

---

## Scaling

### Add Kubernetes Node

```bash
# On new node, get k3s join token from master:
cat /var/lib/rancher/k3s/server/node-token

# On new node:
curl -sfL https://get.k3s.io | K3S_URL=https://MASTER_IP:6443 K3S_TOKEN=TOKEN sh -
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
|---------|-------|-----|
| UI not loading | `curl -fsk https://localhost:443` | Check UI container logs |
| Daemon unhealthy | `curl -fsk https://localhost:8443/healthz` | Check daemon logs, TLS certs |
| Game server won't start | `gdash server logs <id>` | Check adapter manifest, install dir |
| SteamCMD fails | Network to steamcmd endpoints | Use offline bundle |
| Backup fails | Target mount, S3 credentials | Check storage config |
| CVE scan hangs | Trivy/Grype installed? | Install scanner or skip |
| WebSocket disconnects | Proxy timeout | Increase proxy read timeout to 300s |

### Collect Diagnostic Bundle

```bash
./installer.sh --output-audit /tmp/diag-bundle.json
# Includes: preflight-report, install-audit, daemon logs, CVE report
```
