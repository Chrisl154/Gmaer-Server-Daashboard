# Security Audit Report
**Date**: March 4, 2026  
**Project**: Games Dashboard  
**Scope**: File structure, dependencies, modules, configuration, and code security

---

## Executive Summary

The Games Dashboard project demonstrates **good security practices** overall with an established security policy, TLS implementation, TOTP 2FA support, and proper SBOM/CVE scanning. However, **one CRITICAL security vulnerability** was identified that requires immediate remediation.

**Overall Security Level**: 🟡 **MEDIUM** (with critical issue)

---

## 🔴 CRITICAL SECURITY ISSUES

### 1. **WebSocket Token Exposure - Unauthenticated Console Access**
**Severity**: CRITICAL  
**Location**: 
- [daemon/internal/api/server.go](daemon/internal/api/server.go#L335-L370) (`streamConsole` handler)  
- [ui/src/utils/api.ts](ui/src/utils/api.ts#L36-L48) (WebSocket URL builder)

**Issue**:
The WebSocket console streaming endpoint (`/api/v1/servers/:id/console/stream`) has a critical authentication bypass vulnerability:

1. **Token in Query String**: The UI passes JWT tokens in the URL query parameter:
   ```typescript
   return `${base}${path}${token ? `?token=${token}` : ''}`;
   ```

2. **No Token Validation**: The `streamConsole()` handler upgrades the WebSocket connection WITHOUT validating the token:
   ```go
   func (s *Server) streamConsole(c *gin.Context) {
       id := c.Param("id")
       conn, err := s.ws.Upgrade(c.Writer, c.Request, nil)  // ← NO AUTH CHECK
       // ...
   }
   ```

3. **Bypassed Auth Middleware**: While the route is in the `v1` group with `authMiddleware()`, the middleware validates the `Authorization` header, not query parameters. WebSocket clients can connect without proper authentication.

**Impact**:
- Unauthenticated users can stream live console output from any game server
- Complete disclosure of server logs, commands, and real-time server state
- Potential credential exposure if secrets are logged
- Violates RBAC permission model

**Remediation**:
```go
func (s *Server) streamConsole(c *gin.Context) {
    id := c.Param("id")
    
    // Validate token from query parameter (WebSocket compatibility)
    token := c.Query("token")
    if token == "" {
        token = c.GetHeader("Authorization")
    }
    
    claims, err := s.cfg.AuthSvc.ValidateToken(c.Request.Context(), token)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
        return
    }
    
    // Verify server access permission
    user := claims
    // TODO: Check if user has read access to server `id`
    
    conn, err := s.ws.Upgrade(c.Writer, c.Request, nil)
    // ... rest of implementation
}
```

**Alternative (Better)**: Use Authorization header instead of query parameters:
```typescript
// ui/src/utils/api.ts
export function getWsUrl(path: string): string {
  const base = DAEMON_URL.replace(/^http/, 'ws').replace(/^https/, 'wss');
  return `${base}${path}`;
}

// Then pass token in Authorization header-compatible way for WebSocket
```

---

## 🟡 HIGH PRIORITY ISSUES

### 2. **Inconsistent Security Headers in UI Configuration**
**Severity**: HIGH  
**Location**: [ui/Dockerfile](ui/Dockerfile#L42)

**Issue**:
Content-Security-Policy includes `'unsafe-inline'` for both script and style execution:
```nginx
Content-Security-Policy "default-src 'self'; connect-src 'self' https: wss:; 
    script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'"
```

**Impact**:
- Weakens XSS protection
- Allows inline script execution, increasing vulnerability to DOM-based XSS attacks
- Should use nonces or hash-based CSP instead

**Remediation**:
```nginx
# Use external stylesheets and scripts with:
add_header Content-Security-Policy "default-src 'self'; connect-src 'self' https: wss:; script-src 'self'; style-src 'self' data:; img-src 'self' data: https:; font-src 'self'";
```

---

### 3. **Docker Image Tag Flexibility Risk**
**Severity**: HIGH  
**Location**: 
- [installer/configs/docker-compose.yml](installer/configs/docker-compose.yml#L7-L8)
- [helm/charts/games-dashboard/values.yaml](helm/charts/games-dashboard/values.yaml#L10-L15)

**Issue**:
Image tags can be overridden via environment variables without pinning:
```yaml
image: ghcr.io/games-dashboard/daemon:${VERSION:-1.0.0}
```

**Impact**:
- Allows accidental/deliberate deployment of unvetted image versions
- No immutable deployment guarantee
- Could be exploited in supply chain attacks

**Remediation**:
- Pin exact image SHAs in production: `ghcr.io/games-dashboard/daemon:@sha256:abc123...`
- Use image signature verification
- Implement image pull policy: `imagePullPolicy: IfNotPresent`

---

## 🟠 MEDIUM SEVERITY ISSUES

### 4. **Default Credentials in Docker Compose**
**Severity**: MEDIUM  
**Location**: [installer/configs/docker-compose.yml](installer/configs/docker-compose.yml#L18)

**Issue**:
```yaml
environment:
  GDASH_ADMIN_PASSWORD: ${GDASH_ADMIN_PASSWORD:-changeme}
```

**Impact**:
- Default password `changeme` may be used if env var not set
- Increases initial exposure window
- Documentation mentions "change immediately" but not enforced

**Remediation**:
- Remove default, require explicit environment variable:
  ```yaml
  GDASH_ADMIN_PASSWORD: ${GDASH_ADMIN_PASSWORD:?GDASH_ADMIN_PASSWORD must be set}
  ```
- Add pre-deployment validation script

---

### 5. **Helm Chart Security Context Gaps**
**Severity**: MEDIUM  
**Location**: [helm/charts/game-instance/values.yaml](helm/charts/game-instance/values.yaml#L85-L86)

**Issue**:
Security context is empty:
```yaml
securityContext: {}
```

**Impact**:
- No privilege dropping, read-only filesystem, or capability restrictions
- Containers run with default permissive settings

**Remediation**:
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

---

## 🟢 POSITIVE SECURITY FINDINGS

### ✅ **Excellent Practices**

1. **TLS Enforcement**
   - [daemon/Dockerfile](daemon/Dockerfile#L23): Uses `distroless/static-debian12:nonroot` base image ✓
   - Self-signed TLS with auto-generation support ✓
   - HTTPS/WSS enforced in configuration

2. **Authentication & Authorization**
   - JWT tokens with 24h expiry ✓
   - TOTP 2FA support ✓
   - OIDC/SSO integration ready ✓
   - RBAC with admin role checks ([daemon/internal/api/server.go](daemon/internal/api/server.go#L173-L180)) ✓

3. **Secrets Management**
   - AES-256-GCM encryption at rest ✓
   - Vault integration support ✓
   - Master key with 0600 permissions ✓
   - Secrets masked in API responses ✓

4. **Web Security Headers**
   - X-Frame-Options: DENY ✓
   - X-Content-Type-Options: nosniff ✓
   - Referrer-Policy configured ✓

5. **Container Security**
   - Multi-stage builds ✓
   - Distroless runtime images ✓
   - Non-root user execution ✓
   - [daemon/Dockerfile](daemon/Dockerfile#L33): `USER nonroot:nonroot` ✓

6. **Dependency Management**
   - SBOM generation with CycloneDX ✓
   - CVE scanning (Trivy/Grype) ✓
   - Go modules with explicit versioning ✓
   - package-lock.json for npm lock ✓

7. **Security Policy**
   - [docs/SECURITY.md](docs/SECURITY.md): Clear vulnerability disclosure process ✓
   - 48-hour acknowledgment SLA ✓
   - CVE response SLAs defined ✓

---

## 📊 DEPENDENCY ANALYSIS

### Go Dependencies (Backend)
**Module**: `github.com/games-dashboard/daemon`  
**Go Version**: 1.22 ✓

**Key Dependencies**:
| Package | Version | Status |
|---------|---------|--------|
| gin-gonic/gin | v1.9.1 | ✓ Recent |
| coreos/go-oidc | v3.9.0 | ✓ Recent |
| golang-jwt/jwt | v5.2.0 | ✓ Recent |
| hashicorp/vault | v1.12.0 | ✓ Recent |
| prometheus/client_golang | v1.18.0 | ✓ Recent |
| golang.org/x/crypto | v0.21.0 | ✓ Recent |
| golang.org/x/oauth2 | v0.18.0 | ✓ Recent |

**Recommendation**: Run `go audit` regularly: `go list -json -m all | nancy sleuth`

### NPM Dependencies (Frontend)
**Module**: `games-dashboard-ui`  
**Node Version**: 20-alpine (Dockerfile)

**Key Dependencies**:
| Package | Version | Status |
|---------|---------|--------|
| react | ^18.2.0 | ✓ LTS |
| react-router-dom | ^6.22.0 | ✓ Recent |
| axios | ^1.6.7 | ✓ Recent |
| @tanstack/react-query | ^5.20.0 | ✓ Recent |
| xterm | ^5.3.0 | ✓ Recent |

**Recommendation**: 
```bash
npm audit
npm audit fix --force  # Only in dev environments
```

**Note**: `package-lock.json` exists but may need updates. Run `npm ci --audit` in CI/CD.

---

## 🔍 FILE STRUCTURE VERIFICATION

✅ **Well-organized structure**:
```
✓ /daemon          - Backend server
✓ /ui              - React frontend
✓ /cli             - Command-line interface
✓ /helm            - Kubernetes deployment
✓ /installer       - Installation scripts
✓ /docs            - Documentation including SECURITY.md
✓ /tests           - Unit, integration, e2e tests
✓ /adapters        - Game-specific adapters
```

**Version Files Present**:
- ✓ [cli/go.mod](cli/go.mod) with version constraints
- ✓ [daemon/go.mod](daemon/go.mod) with version constraints  
- ✓ [ui/package.json](ui/package.json) with version ranges
- ✓ [ui/package-lock.json](ui/package-lock.json) - Lock file present

---

## ⚠️ CONFIGURATION ISSUES

### Default Configuration Review
**File**: [installer/configs/daemon.yaml](installer/configs/daemon.yaml)

| Setting | Current | Recommendation |
|---------|---------|-----------------|
| `bind_addr` | `:8443` | ⚠️ Bind to specific interface in production |
| `log_level` | `info` | ✓ Appropriate for production |
| `tls.auto_generate` | `true` | ⚠️ Disable in production, use cert-manager |
| `auth.local.enabled` | `true` | ✓ Good fallback |
| `auth.mfa_required` | `false` | ⚠️ Should be `true` for production |
| `secrets.backend` | `local` | ⚠️ Consider Vault for production |
| `backup.retain_days` | `30` | ✓ Reasonable default |

---

## 🧪 TESTING & CI/CD

**Tests Found**:
- ✓ Unit tests: `*_test.go` files present
- ✓ Integration tests: [tests/integration/run-tests.sh](tests/integration/run-tests.sh)
- ✓ E2E tests: [tests/e2e/cli-smoke.sh](tests/e2e/cli-smoke.sh)

**CI/CD Status**:
- ⚠️ CI folder is empty ([ci/](ci/))
- ⚠️ No GitHub Actions workflows found
- ⚠️ No automated security scanning (SAST/DAST) detected

**Recommendation**: Add `.github/workflows/security.yml`:
```yaml
- Run: go vet ./...
- Run: staticcheck ./...
- Run: gosec ./...
- Run: trivy scan --exit-code 1 --severity HIGH .
- Run: npm audit in ui/
```

---

## 🛠️ HARDENING RECOMMENDATIONS

### Immediate (CRITICAL)
1. **[REQUIRED]** Fix WebSocket authentication bypass (Issue #1)

### Short-term (HIGH)
2. **[REQUIRED]** Harden CSP headers, remove `unsafe-inline`
3. **[REQUIRED]** Pin Docker image SHAs
4. Remove default password from docker-compose template
5. Add security scanning to CI/CD

### Medium-term (MEDIUM)
6. Implement Helm security context properly
7. Add SAST tools (gosec, Semgrep)
8. Implement DAST testing in CI/CD
9. Add API rate limiting and request throttling
10. Enable audit logging webhook forwarding

### Long-term (LOW)
11. Implement mTLS for daemon-UI communication
12. Add network policies for Kubernetes deployments
13. Regular penetration testing (quarterly)
14. Security incident response plan

---

## 📋 CHECKLIST FOR DEPLOYMENT

Before deploying to production:

- [ ] **CRITICAL**: Apply WebSocket authentication fix
- [ ] **HIGH**: Update CSP headers
- [ ] **HIGH**: Pin all Docker image SHAs
- [ ] Change default admin password via environment variable
- [ ] Set `auth.mfa_required: true` in daemon.yaml  
- [ ] Bind daemon to private interface only
- [ ] Enable Vault secrets backend for production
- [ ] Install TLS certificate from trusted CA (not self-signed)
- [ ] Configure OIDC/SSO for production auth
- [ ] Enable audit logging
- [ ] Set up SIEM integration for logs
- [ ] Implement network segmentation
- [ ] Run full security test suite
- [ ] Conduct penetration testing
- [ ] Document security incident procedures
- [ ] Set up vulnerability monitoring (Trivy, Grype)

---

## 📝 COMPLIANCE NOTES

- **CWE Coverage**: 
  - CWE-306 (Missing Auth Check) - ❌ **FOUND** (WebSocket issue)
  - CWE-79 (XSS) - ⚠️ Partially mitigated (CSP has unsafe-inline)
  - CWE-89 (SQL Injection) - ✓ Not applicable (no direct SQL)
  - CWE-434 (Unrestricted Upload) - ✓ Not implemented

- **OWASP Top 10**:
  - A01:2021 Broken Access Control - ❌ WebSocket endpoint
  - A02:2021 Cryptographic Failures - ✓ Good
  - A03:2021 Injection - ✓ No direct vectors found
  - A04:2021 Insecure Design - ⚠️ Default credentials
  - A05:2021 Broken Access Control - ✓ RBAC implemented
  - A07:2021 XSS - ⚠️ CSP allows unsafe-inline

---

## 🔄 NEXT STEPS

1. **Immediate**: Create GitHub issue for WebSocket authentication fix with priority `P0`
2. **Week 1**: Apply CRITICAL fixes and test thoroughly
3. **Week 2**: Review CSP and image tagging, update CI/CD
4. **Week 4**: Full security audit with external reviewer
5. **Month 2**: Implement long-term hardening recommendations

---

**Report Generated**: March 4, 2026  
**Reviewed By**: Security Audit Tool  
**Status**: 🔴 **REQUIRES ACTION** - Critical issue identified before production deployment
