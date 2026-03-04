# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.x     | ✅        |
| < 1.0   | ❌        |

## Threat Model

### Trust Boundaries
1. **Public Internet → Daemon API**: All traffic over TLS 1.3. Authentication required for all `/api/v1/*` endpoints.
2. **Browser → UI**: HTTPS only. JWT in Authorization header (not cookies to prevent CSRF).
3. **Daemon → Game Processes**: Least-privilege service accounts. Docker socket access scoped to game containers only.
4. **Secrets at Rest**: AES-256-GCM with per-installation master key stored at `/etc/games-dashboard/secrets/master.key` (mode 0600).

### Attack Surface
- HTTP API (TLS, authenticated)
- WebSocket console (authenticated, per-server scoped)
- Daemon systemd service (runs as dedicated user `games-daemon`)
- Docker socket access (read + game containers only, not host privileged)
- SteamCMD download channel (verified checksums)

### Mitigations
| Threat | Mitigation |
|--------|-----------|
| Credential theft | TOTP 2FA required for admin; JWT expiry 24h |
| SSRF | No user-controlled URLs in daemon except backup targets (validated) |
| Container escape | `no-new-privileges`, `cap_drop: ALL`, read-only root FS where possible |
| Secret exposure | Secrets masked in API responses; re-auth required to reveal |
| CVEs in dependencies | Weekly Trivy/Grype scans in CI; SBOM maintained |
| Audit bypass | All state-changing operations logged with user, IP, timestamp |

## Hardening Checklist

- [ ] Change default admin password immediately after install
- [ ] Enable TOTP 2FA for all admin accounts
- [ ] Replace self-signed TLS certificate with a trusted CA certificate
- [ ] Restrict daemon bind to internal interface if not internet-facing
- [ ] Configure firewall to only expose required game ports
- [ ] Rotate encryption keys after initial setup
- [ ] Enable OIDC/SSO for organizational deployments
- [ ] Set up log forwarding to a SIEM
- [ ] Schedule regular CVE scans
- [ ] Review and pin mod checksums in production

## Vulnerability Disclosure

We use **responsible disclosure**. If you discover a security vulnerability:

1. **Do NOT open a public GitHub issue.**
2. Email **security@games-dashboard.dev** with:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact assessment
   - Your proposed fix (if any)
3. We will acknowledge within **48 hours** and aim to release a patch within **14 days** for critical issues.
4. We will credit you in the release notes (unless you prefer anonymity).

## Key Rotation Procedure

```bash
# Via CLI
gdash admin secrets rotate

# Via API
curl -X POST https://localhost:8443/api/v1/admin/secrets/rotate \
  -H "Authorization: Bearer $TOKEN"

# Via installer
installer.sh --rollback-to checkpoint-pre-rotation
```

## CVE Response SLA

| Severity | Patch Timeline |
|----------|----------------|
| Critical | 24–48 hours    |
| High     | 7 days         |
| Medium   | 30 days        |
| Low      | Next release   |
