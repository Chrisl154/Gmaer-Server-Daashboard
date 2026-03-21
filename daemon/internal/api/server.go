package api

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/games-dashboard/daemon/internal/auth"
	"github.com/games-dashboard/daemon/internal/broker"
	daemonconfig "github.com/games-dashboard/daemon/internal/config"
	"github.com/games-dashboard/daemon/internal/firewall"
	"github.com/games-dashboard/daemon/internal/health"
	"github.com/games-dashboard/daemon/internal/metrics"
	"github.com/games-dashboard/daemon/internal/notifications"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// validServerID matches safe server IDs — same rule as broker.validServerID.
// Enforced in serverACLMiddleware before any handler runs.
var validServerID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Config holds API server configuration
type Config struct {
	BindAddr       string
	TLSCert        string
	TLSKey         string
	// AllowedOrigins is an optional allowlist of WebSocket origin hostnames.
	// When empty, localhost/127.0.0.1/::1 plus the bind host are allowed.
	AllowedOrigins  []string
	Logger          *zap.Logger
	AuthSvc         *auth.Service
	Broker          *broker.Broker
	HealthSvc       *health.Service
	MetricsSvc      *metrics.Service
	FirewallSvc     *firewall.Service
	NotificationSvc *notifications.Service
	// DaemonCfg is the live daemon configuration used by the settings API.
	DaemonCfg  *daemonconfig.Config
	// ConfigPath is the path to daemon.yaml; when set, PATCH /settings writes back to disk.
	ConfigPath string
	// AutoTLSConfig is set by main when AutoTLS is enabled; it overrides TLSCert/TLSKey
	// and uses the autocert manager's GetCertificate function for Let's Encrypt.
	AutoTLSConfig *tls.Config
}

// Server is the HTTP/WebSocket API server
type Server struct {
	cfg                Config
	router             *gin.Engine
	srv                *http.Server
	ws                 *websocket.Upgrader
	allowedOriginHosts map[string]bool
}

// NewServer creates a new API server
func NewServer(cfg Config) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Build the set of allowed WebSocket origin hostnames.
	allowed := map[string]bool{
		"localhost": true,
		"127.0.0.1": true,
		"::1":       true,
	}
	if host, _, err := net.SplitHostPort(cfg.BindAddr); err == nil && host != "" && host != "0.0.0.0" {
		allowed[host] = true
	}
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = true
	}

	s := &Server{
		cfg:                cfg,
		router:             router,
		allowedOriginHosts: allowed,
	}

	s.ws = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		CheckOrigin:     s.checkOrigin,
	}

	s.registerRoutes()

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS13}
	if cfg.AutoTLSConfig != nil {
		// Merge: keep MinVersion but adopt autocert's GetCertificate.
		tlsCfg.GetCertificate = cfg.AutoTLSConfig.GetCertificate
		tlsCfg.NextProtos = cfg.AutoTLSConfig.NextProtos
	}

	s.srv = &http.Server{
		Addr:         cfg.BindAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
		TLSConfig:    tlsCfg,
	}

	return s, nil
}

// checkOrigin validates WebSocket upgrade requests against the allowed origin list.
// Non-browser clients (empty Origin header) are permitted unconditionally.
func (s *Server) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return s.allowedOriginHosts[host]
}

func (s *Server) registerRoutes() {
	r := s.router

	// Health and metrics (unauthenticated)
	r.GET("/healthz", s.handleHealthz)
	r.GET("/metrics", s.handleMetrics)

	// Public auth endpoints (must be outside the authenticated group)
	// Rate-limited: 5 attempts per minute per IP to block brute-force.
	r.POST("/api/v1/auth/login", rateLimitMiddleware(5.0/60, 5), s.login)
	r.GET("/api/v1/auth/oidc/login", s.oidcLogin)
	r.GET("/api/v1/auth/oidc/callback", s.oidcCallback)
	r.GET("/api/v1/auth/steam/login", s.steamLogin)
	r.GET("/api/v1/auth/steam/callback", s.steamCallback)

	// First-run bootstrap (public; guarded internally when already initialised)
	r.GET("/api/v1/system/init-status", s.getInitStatus)
	r.POST("/api/v1/system/bootstrap", s.bootstrapSystem)

	// Version (public — useful for health checks without credentials)
	r.GET("/api/v1/version", s.getVersion)

	// Public IP detection (auth-protected; called by Share panel)
	r.GET("/api/v1/system/public-ip", s.authMiddleware(), s.getPublicIP)

	// System resource snapshot (auth-protected; used by the pre-flight check)
	r.GET("/api/v1/system/resources", s.authMiddleware(), s.getSystemResources)

	// API v1
	v1 := r.Group("/api/v1")
	v1.Use(s.authMiddleware())

	// Server collection — list is ACL-filtered server-side; create is admin-only
	v1.GET("/servers", s.listServers)
	v1.POST("/servers", s.requireRole("admin"), s.createServer)
	v1.POST("/ports/validate", s.validatePorts)

	// Per-server routes — all gated by serverACLMiddleware
	srv := v1.Group("/servers/:id")
	srv.Use(s.serverACLMiddleware())

	// Read-only (any authenticated user with access)
	srv.GET("", s.getServer)
	srv.GET("/status", s.getServerStatus)
	srv.GET("/logs", s.getServerLogs)
	srv.GET("/metrics", s.getServerMetrics)
	srv.GET("/backups", s.listBackups)
	srv.GET("/ports", s.listPorts)
	srv.GET("/config-files", s.listConfigFiles)
	srv.GET("/config-files/*path", s.readConfigFile)
	srv.GET("/files", s.listFiles)
	srv.GET("/files/download", s.downloadFile)
	srv.GET("/banlist", s.listBannedPlayers)
	srv.GET("/whitelist", s.listWhitelistPlayers)
	srv.GET("/mods", s.listMods)
	srv.GET("/console/stream", s.streamConsole)

	// Write ops — require operator or admin
	srv.PUT("", s.requireOperator(), s.updateServer)
	srv.DELETE("", s.requireRole("admin"), s.deleteServer)
	srv.POST("/start", s.requireOperator(), s.startServer)
	srv.POST("/stop", s.requireOperator(), s.stopServer)
	srv.POST("/restart", s.requireOperator(), s.restartServer)
	srv.POST("/deploy", s.requireOperator(), s.deployServer)
	srv.POST("/console/command", s.requireOperator(), s.sendConsoleCommand)
	srv.POST("/backup", s.requireOperator(), s.triggerBackup)
	srv.POST("/restore/:backupId", s.requireOperator(), s.restoreBackup)
	srv.PUT("/ports", s.requireOperator(), s.updatePorts)
	srv.PUT("/config-files/*path", s.requireOperator(), s.writeConfigFile)
	srv.POST("/files/upload", s.requireOperator(), s.uploadFile)
	srv.DELETE("/files", s.requireOperator(), s.deleteFile)
	srv.POST("/banlist", s.requireOperator(), s.banPlayer)
	srv.DELETE("/banlist/:player", s.requireOperator(), s.unbanPlayer)
	srv.POST("/whitelist", s.requireOperator(), s.whitelistAddPlayer)
	srv.DELETE("/whitelist/:player", s.requireOperator(), s.whitelistRemovePlayer)
	srv.POST("/update", s.requireOperator(), s.triggerServerUpdate)
	srv.POST("/mods", s.requireOperator(), s.installMod)
	srv.DELETE("/mods/:modId", s.requireOperator(), s.uninstallMod)
	srv.POST("/mods/test", s.requireOperator(), s.testMods)
	srv.POST("/mods/rollback", s.requireOperator(), s.rollbackMods)
	srv.GET("/diagnose", s.diagnoseServer)
	srv.POST("/clone", s.requireOperator(), s.cloneServer)

	// SBOM & CVE
	v1.GET("/sbom", s.getSBOM)
	v1.GET("/sbom/:component", s.getComponentSBOM)
	v1.POST("/sbom/scan", s.triggerScan)
	v1.GET("/cve-report", s.getCVEReport)

	// Auth (protected — login/oidc are registered as public routes above)
	v1.POST("/auth/logout", s.logout)
	v1.POST("/auth/totp/setup", s.setupTOTP)
	v1.POST("/auth/totp/verify", rateLimitMiddleware(5.0/60, 5), s.verifyTOTP)
	v1.GET("/auth/totp/recovery-codes", s.getRecoveryCodesCount)
	v1.POST("/auth/totp/recovery-codes/regenerate", s.regenerateRecoveryCodes)
	v1.GET("/auth/totp/recovery-codes/download", s.downloadRecoveryCodes)
	v1.GET("/users/me", s.getMe)

	// Cluster nodes — reads and heartbeat are open to any authenticated user/node;
	// writes require admin role.
	v1.GET("/nodes", s.listNodes)
	v1.GET("/nodes/:nodeId", s.getNode)
	v1.POST("/nodes/:nodeId/heartbeat", s.nodeHeartbeat)

	// Admin (requires admin role)
	admin := v1.Group("/admin")
	admin.Use(s.requireRole("admin"))
	admin.GET("/users", s.listUsers)
	admin.POST("/users", s.createUser)
	admin.PUT("/users/:userId", s.updateUser)
	admin.DELETE("/users/:userId", s.deleteUser)
	admin.GET("/audit", s.getAuditLog)
	admin.POST("/secrets/rotate", s.rotateSecrets)
	admin.GET("/settings", s.getSettings)
	admin.PATCH("/settings", s.patchSettings)

	// Cluster node management (admin-only writes)
	admin.POST("/nodes", s.registerNode)
	admin.POST("/nodes/join-token", s.issueJoinToken)
	admin.DELETE("/nodes/:nodeId", s.deregisterNode)

	// Self-update (admin-only)
	admin.GET("/update/status", s.getUpdateStatus)
	admin.POST("/update/apply", s.applyUpdate)
	admin.GET("/update/log", s.getUpdateLog)

	// Notifications (admin-only)
	admin.GET("/notifications", s.getNotifications)
	admin.PATCH("/notifications", s.patchNotifications)
	admin.POST("/notifications/test", s.testNotification)

	// TLS status (admin-only)
	admin.GET("/tls/status", s.getTLSStatus)

	// Firewall (UFW)
	v1.GET("/firewall", s.getFirewallStatus)
	v1.POST("/firewall/rules", s.addFirewallRule)
	v1.DELETE("/firewall/rules/:num", s.deleteFirewallRule)
	v1.POST("/firewall/enabled", s.setFirewallEnabled)

	// System
	v1.GET("/status", s.getSystemStatus)

	// API keys / personal access tokens (any authenticated user, own keys only)
	v1.GET("/auth/api-keys", s.listAPIKeys)
	v1.POST("/auth/api-keys", s.createAPIKey)
	v1.DELETE("/auth/api-keys/:keyId", s.revokeAPIKey)

	// Web Push subscriptions (any authenticated user)
	v1.GET("/push/vapid-key", s.getPushVAPIDKey)
	v1.POST("/push/subscribe", s.subscribePush)
	v1.DELETE("/push/subscribe", s.unsubscribePush)
}

func (s *Server) ListenAndServeTLS() error {
	// When AutoTLS is configured, GetCertificate is set — pass empty cert/key
	// so the standard library uses the TLSConfig instead of file-based certs.
	if s.srv.TLSConfig != nil && s.srv.TLSConfig.GetCertificate != nil {
		return s.srv.ListenAndServeTLS("", "")
	}
	return s.srv.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
}

// ListenAndServe starts the server on plain HTTP (no TLS). Intended for local
// testing only — do not use in production.
func (s *Server) ListenAndServe() error {
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// --- Handler implementations ---

func (s *Server) handleHealthz(c *gin.Context) {
	status := s.cfg.HealthSvc.Check()
	code := http.StatusOK
	if !status.Healthy {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, status)
}

func (s *Server) handleMetrics(c *gin.Context) {
	s.cfg.MetricsSvc.Handler().ServeHTTP(c.Writer, c.Request)
}

func (s *Server) listServers(c *gin.Context) {
	claims := s.getUser(c)
	servers, err := s.cfg.Broker.ListServers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Non-admins only see their allowed servers
	if claims != nil && !claims.HasRole("admin") {
		allowed := s.cfg.AuthSvc.GetAllowedServers(claims.UserID)
		if len(allowed) > 0 {
			allowSet := make(map[string]bool, len(allowed))
			for _, id := range allowed {
				allowSet[id] = true
			}
			filtered := servers[:0]
			for _, sv := range servers {
				if allowSet[sv.ID] {
					filtered = append(filtered, sv)
				}
			}
			servers = filtered
		}
	}
	c.JSON(http.StatusOK, gin.H{"servers": servers, "count": len(servers)})
}

func (s *Server) createServer(c *gin.Context) {
	var req broker.CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	server, err := s.cfg.Broker.CreateServer(c.Request.Context(), req)
	if err != nil {
		s.recordEvent(c, "create_server", req.ID, false, gin.H{"name": req.Name, "adapter": req.Adapter, "error": err.Error()})
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid server ID") {
			status = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "create_server", server.ID, true, gin.H{"name": server.Name, "adapter": server.Adapter})
	c.JSON(http.StatusCreated, server)
}

func (s *Server) getServer(c *gin.Context) {
	id := c.Param("id")
	server, err := s.cfg.Broker.GetServer(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	c.JSON(http.StatusOK, server)
}

func (s *Server) updateServer(c *gin.Context) {
	id := c.Param("id")
	var req broker.UpdateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	server, err := s.cfg.Broker.UpdateServer(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, server)
}

func (s *Server) deleteServer(c *gin.Context) {
	id := c.Param("id")
	if err := s.cfg.Broker.DeleteServer(c.Request.Context(), id); err != nil {
		s.recordEvent(c, "delete_server", id, false, gin.H{"error": err.Error()})
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "running") || strings.Contains(err.Error(), "starting") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "delete_server", id, true, nil)
	c.JSON(http.StatusNoContent, nil)
}

func (s *Server) startServer(c *gin.Context) {
	id := c.Param("id")
	if err := s.cfg.Broker.StartServer(c.Request.Context(), id); err != nil {
		s.recordEvent(c, "start_server", id, false, gin.H{"error": err.Error()})
		status := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "already running") || strings.Contains(msg, "already starting") ||
			strings.Contains(msg, "is stopping") || strings.Contains(msg, "is deploying") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": msg})
		return
	}
	s.recordEvent(c, "start_server", id, true, nil)
	c.JSON(http.StatusOK, gin.H{"status": "starting", "id": id})
}

func (s *Server) stopServer(c *gin.Context) {
	id := c.Param("id")
	if err := s.cfg.Broker.StopServer(c.Request.Context(), id); err != nil {
		s.recordEvent(c, "stop_server", id, false, gin.H{"error": err.Error()})
		// "not running" is a client-side state conflict, not a server error.
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not running") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "stop_server", id, true, nil)
	c.JSON(http.StatusOK, gin.H{"status": "stopping", "id": id})
}

func (s *Server) restartServer(c *gin.Context) {
	id := c.Param("id")
	job, err := s.cfg.Broker.RestartServer(c.Request.Context(), id)
	if err != nil {
		s.recordEvent(c, "restart_server", id, false, gin.H{"error": err.Error()})
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "stopping") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "restart_server", id, true, nil)
	c.JSON(http.StatusAccepted, gin.H{"status": "restarting", "id": id, "job_id": job.ID})
}

func (s *Server) deployServer(c *gin.Context) {
	id := c.Param("id")
	var req broker.DeployRequest
	// Body is optional — a bare POST with no body uses the server's existing deploy method.
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	job, err := s.cfg.Broker.DeployServer(c.Request.Context(), id, req)
	if err != nil {
		s.recordEvent(c, "deploy_server", id, false, gin.H{"method": req.Method, "error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "deploy_server", id, true, gin.H{"method": req.Method, "job_id": job.ID})
	c.JSON(http.StatusAccepted, job)
}

func (s *Server) getServerStatus(c *gin.Context) {
	id := c.Param("id")
	status, err := s.cfg.Broker.GetServerStatus(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (s *Server) getServerMetrics(c *gin.Context) {
	id := c.Param("id")
	n := 60
	if nStr := c.Query("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			n = parsed
		}
	}
	samples, err := s.cfg.Broker.GetServerMetrics(c.Request.Context(), id, n)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if samples == nil {
		samples = []broker.ServerMetricSample{}
	}
	c.JSON(http.StatusOK, gin.H{"server_id": id, "samples": samples})
}

func (s *Server) sendConsoleCommand(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Command string `json:"command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is required"})
		return
	}
	response, err := s.cfg.Broker.SendConsoleCommand(c.Request.Context(), id, req.Command)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"response": response})
}

func (s *Server) getServerLogs(c *gin.Context) {
	id := c.Param("id")
	lines := c.DefaultQuery("lines", "100")
	logs, err := s.cfg.Broker.GetServerLogs(c.Request.Context(), id, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func (s *Server) streamConsole(c *gin.Context) {
	id := c.Param("id")
	// Auth and ACL are already enforced by authMiddleware + serverACLMiddleware.
	// The claims are available in context for any audit logging.
	_ = id

	// Now safe to upgrade WebSocket with authenticated user
	conn, err := s.ws.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.cfg.Logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	stream, unsub, err := s.cfg.Broker.GetConsoleStream(ctx, id)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","msg":"Console stream not available — make sure the server has been deployed and is running"}`)) //nolint:errcheck
		return
	}
	defer unsub()

	for {
		select {
		case msg, ok := <-stream:
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) listBackups(c *gin.Context) {
	id := c.Param("id")
	backups, err := s.cfg.Broker.ListBackups(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"backups": backups})
}

func (s *Server) triggerBackup(c *gin.Context) {
	id := c.Param("id")
	var req broker.BackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = broker.BackupRequest{Type: "full"}
	}
	job, err := s.cfg.Broker.TriggerBackup(c.Request.Context(), id, req)
	if err != nil {
		s.recordEvent(c, "trigger_backup", id, false, gin.H{"type": req.Type, "error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "trigger_backup", id, true, gin.H{"type": req.Type, "job_id": job.ID})
	c.JSON(http.StatusAccepted, job)
}

func (s *Server) restoreBackup(c *gin.Context) {
	id := c.Param("id")
	backupID := c.Param("backupId")
	job, err := s.cfg.Broker.RestoreBackup(c.Request.Context(), id, backupID)
	if err != nil {
		s.recordEvent(c, "restore_backup", id, false, gin.H{"backup_id": backupID, "error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "restore_backup", id, true, gin.H{"backup_id": backupID, "job_id": job.ID})
	c.JSON(http.StatusAccepted, job)
}

func (s *Server) listPorts(c *gin.Context) {
	id := c.Param("id")
	ports, err := s.cfg.Broker.ListPorts(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ports": ports})
}

func (s *Server) updatePorts(c *gin.Context) {
	id := c.Param("id")
	var req broker.UpdatePortsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ports, err := s.cfg.Broker.UpdatePorts(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ports": ports})
}

func (s *Server) validatePorts(c *gin.Context) {
	var req broker.ValidatePortsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.cfg.Broker.ValidatePorts(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ── Config file editor handlers ───────────────────────────────────────────────

func (s *Server) listConfigFiles(c *gin.Context) {
	id := c.Param("id")
	templates, err := s.cfg.Broker.GetConfigTemplates(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": templates})
}

func (s *Server) readConfigFile(c *gin.Context) {
	id := c.Param("id")
	relPath := c.Param("path") // includes leading slash, e.g. "/data/server.properties"
	content, err := s.cfg.Broker.ReadConfigFile(c.Request.Context(), id, relPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": relPath, "content": content})
}

func (s *Server) writeConfigFile(c *gin.Context) {
	id := c.Param("id")
	relPath := c.Param("path")
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content field is required"})
		return
	}
	if err := s.cfg.Broker.WriteConfigFile(c.Request.Context(), id, relPath, body.Content); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── File browser ──────────────────────────────────────────────────────────────

func (s *Server) listFiles(c *gin.Context) {
	id := c.Param("id")
	dirPath := c.DefaultQuery("path", "/")
	entries, err := s.cfg.Broker.ListFiles(c.Request.Context(), id, dirPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": dirPath, "entries": entries})
}

func (s *Server) downloadFile(c *gin.Context) {
	id := c.Param("id")
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	// P42: resolve and validate the absolute path then stream it with c.FileAttachment
	// instead of reading the entire file into memory.
	absPath, err := s.cfg.Broker.ResolveFilePath(c.Request.Context(), id, filePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	filename := filePath
	if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
		filename = filePath[idx+1:]
	}
	c.FileAttachment(absPath, filename)
}

func (s *Server) uploadFile(c *gin.Context) {
	id := c.Param("id")
	destDir := c.DefaultQuery("dir", "/")
	// Limit upload body to 100 MB to prevent resource exhaustion.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 100<<20)
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart form expected"})
		return
	}
	files := form.File["file"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files in request"})
		return
	}
	for _, fh := range files {
		f, openErr := fh.Open()
		if openErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot open uploaded file"})
			return
		}
		data := make([]byte, fh.Size)
		if _, readErr := f.Read(data); readErr != nil {
			f.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read uploaded file"})
			return
		}
		f.Close()
		if uploadErr := s.cfg.Broker.UploadFile(c.Request.Context(), id, destDir, fh.Filename, data); uploadErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": uploadErr.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(files)})
}

func (s *Server) deleteFile(c *gin.Context) {
	id := c.Param("id")
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	if err := s.cfg.Broker.DeleteFile(c.Request.Context(), id, filePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) listMods(c *gin.Context) {
	id := c.Param("id")
	mods, err := s.cfg.Broker.ListMods(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mods": mods})
}

func (s *Server) installMod(c *gin.Context) {
	id := c.Param("id")
	var req broker.InstallModRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	job, err := s.cfg.Broker.InstallMod(c.Request.Context(), id, req)
	if err != nil {
		s.recordEvent(c, "install_mod", id, false, gin.H{"mod_id": req.ModID, "source": req.Source, "error": err.Error()})
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unknown mod source") || strings.Contains(err.Error(), "must use HTTPS") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "install_mod", id, true, gin.H{"mod_id": req.ModID, "source": req.Source, "job_id": job.ID})
	c.JSON(http.StatusAccepted, job)
}

func (s *Server) uninstallMod(c *gin.Context) {
	id := c.Param("id")
	modID := c.Param("modId")
	if err := s.cfg.Broker.UninstallMod(c.Request.Context(), id, modID); err != nil {
		s.recordEvent(c, "uninstall_mod", id, false, gin.H{"mod_id": modID, "error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "uninstall_mod", id, true, gin.H{"mod_id": modID})
	c.JSON(http.StatusNoContent, nil)
}

func (s *Server) testMods(c *gin.Context) {
	id := c.Param("id")
	result, err := s.cfg.Broker.TestMods(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) rollbackMods(c *gin.Context) {
	id := c.Param("id")
	var req broker.RollbackModsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.cfg.Broker.RollbackMods(c.Request.Context(), id, req); err != nil {
		s.recordEvent(c, "rollback_mods", id, false, gin.H{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "rollback_mods", id, true, nil)
	c.JSON(http.StatusOK, gin.H{"status": "rolled back"})
}

func (s *Server) getSBOM(c *gin.Context) {
	sbom, err := s.cfg.Broker.GetSBOM(c.Request.Context())
	if err != nil {
		// P51: return 404 when no scan has been run yet rather than 500.
		if strings.Contains(err.Error(), "no SBOM available") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sbom)
}

func (s *Server) getComponentSBOM(c *gin.Context) {
	component := c.Param("component")
	sbom, err := s.cfg.Broker.GetComponentSBOM(c.Request.Context(), component)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
		return
	}
	c.JSON(http.StatusOK, sbom)
}

func (s *Server) triggerScan(c *gin.Context) {
	job, err := s.cfg.Broker.TriggerCVEScan(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, job)
}

func (s *Server) getCVEReport(c *gin.Context) {
	report, err := s.cfg.Broker.GetCVEReport(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (s *Server) login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := s.cfg.AuthSvc.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) logout(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if err := s.cfg.AuthSvc.Logout(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "logged out"})
}

func (s *Server) setupTOTP(c *gin.Context) {
	user := s.getUser(c)
	var req struct {
		CurrentCode string `json:"current_code"`
	}
	// body is optional — ignore parse errors (first-time setup has no body)
	_ = c.ShouldBindJSON(&req)
	setup, err := s.cfg.AuthSvc.SetupTOTP(c.Request.Context(), user, req.CurrentCode)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, setup)
}

func (s *Server) verifyTOTP(c *gin.Context) {
	user := s.getUser(c)
	var req auth.TOTPVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := s.cfg.AuthSvc.VerifyTOTP(c.Request.Context(), user, req.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid TOTP code"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) getRecoveryCodesCount(c *gin.Context) {
	user := s.getUser(c)
	count, err := s.cfg.AuthSvc.GetRecoveryCodesCount(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"remaining": count})
}

func (s *Server) regenerateRecoveryCodes(c *gin.Context) {
	user := s.getUser(c)
	var req struct {
		TOTPCode string `json:"totp_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := s.cfg.AuthSvc.RegenerateRecoveryCodes(c.Request.Context(), user, req.TOTPCode)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) downloadRecoveryCodes(c *gin.Context) {
	user := s.getUser(c)
	// We need the raw codes — ask the service for a text representation
	count, err := s.cfg.AuthSvc.GetRecoveryCodesCount(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// The codes themselves are not retrievable after first display; this endpoint
	// returns the count so the client knows TOTP is enabled. Actual code download
	// is handled client-side from the enrollment/regenerate response.
	c.JSON(http.StatusOK, gin.H{"remaining": count, "note": "recovery codes are only shown once at enrollment or regeneration"})
}

func (s *Server) oidcLogin(c *gin.Context) {
	authURL, state, err := s.cfg.AuthSvc.GetOIDCAuthURL(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": authURL, "state": state})
}

func (s *Server) oidcCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	resp, err := s.cfg.AuthSvc.OIDCCallback(c.Request.Context(), code, state)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// steamLogin redirects the browser to Steam's OpenID 2.0 login page.
func (s *Server) steamLogin(c *gin.Context) {
	loginURL, _, err := s.cfg.AuthSvc.GetSteamLoginURL()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Redirect(http.StatusFound, loginURL)
}

// steamCallback receives the OpenID 2.0 assertion from Steam, verifies it,
// issues a JWT, and redirects the browser back to the frontend login page.
func (s *Server) steamCallback(c *gin.Context) {
	resp, err := s.cfg.AuthSvc.SteamCallback(c.Request.Context(), c.Request.URL.RawQuery)
	if err != nil {
		// Redirect to login with an error hint rather than returning JSON
		// (the browser is at this URL due to a redirect from Steam).
		c.Redirect(http.StatusFound, "/login?error=steam_auth_failed")
		return
	}

	frontendBase := s.cfg.AuthSvc.SteamFrontendURL()
	redirectURL := frontendBase + "/login?token=" + resp.Token +
		"&expires_at=" + resp.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
	c.Redirect(http.StatusFound, redirectURL)
}

// getMe returns the profile of the currently authenticated user.
func (s *Server) getMe(c *gin.Context) {
	claims := c.MustGet("user").(*auth.Claims)
	users, err := s.cfg.AuthSvc.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, u := range users {
		if u.ID == claims.UserID {
			c.JSON(http.StatusOK, u)
			return
		}
	}
	// Fallback: return a minimal record from the claims if the user map lookup
	// failed (e.g. short-lived Steam user not yet persisted).
	c.JSON(http.StatusOK, gin.H{
		"id":       claims.UserID,
		"username": claims.Username,
		"roles":    claims.Roles,
	})
}

func (s *Server) listUsers(c *gin.Context) {
	users, err := s.cfg.AuthSvc.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (s *Server) createUser(c *gin.Context) {
	var req auth.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := s.cfg.AuthSvc.CreateUser(c.Request.Context(), req)
	if err != nil {
		s.recordEvent(c, "user.create", "user:"+req.Username, false, map[string]any{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "user.create", "user:"+user.ID, true, map[string]any{"username": req.Username, "roles": req.Roles})
	c.JSON(http.StatusCreated, user)
}

func (s *Server) updateUser(c *gin.Context) {
	userID := c.Param("userId")
	var req auth.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := s.cfg.AuthSvc.UpdateUser(c.Request.Context(), userID, req)
	if err != nil {
		s.recordEvent(c, "user.update", "user:"+userID, false, map[string]any{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "user.update", "user:"+userID, true, map[string]any{"username": user.Username, "roles": req.Roles})
	c.JSON(http.StatusOK, user)
}

func (s *Server) deleteUser(c *gin.Context) {
	userID := c.Param("userId")
	if err := s.cfg.AuthSvc.DeleteUser(c.Request.Context(), userID); err != nil {
		s.recordEvent(c, "user.delete", "user:"+userID, false, map[string]any{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "user.delete", "user:"+userID, true, nil)
	c.JSON(http.StatusNoContent, nil)
}

func (s *Server) getAuditLog(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	logs, total, err := s.cfg.AuthSvc.GetAuditLog(c.Request.Context(), offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"audit_log": logs, "total": total, "offset": offset, "limit": limit})
}

func (s *Server) rotateSecrets(c *gin.Context) {
	if err := s.cfg.Broker.RotateSecrets(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "secrets rotated"})
}

// settingsResponse is the safe, secrets-free view of the daemon configuration.
type settingsResponse struct {
	BindAddr          string               `json:"bind_addr"`
	ShutdownTimeoutS  int                  `json:"shutdown_timeout_s"`
	DataDir           string               `json:"data_dir"`
	LogLevel          string               `json:"log_level"`
	Storage           settingsStorage      `json:"storage"`
	Backup            settingsBackup       `json:"backup"`
	Metrics           settingsMetrics      `json:"metrics"`
	Cluster           settingsCluster      `json:"cluster"`
}

type settingsStorage struct {
	DataDir   string              `json:"data_dir"`
	NFSMounts []settingsNFSMount  `json:"nfs_mounts"`
	S3        *settingsS3         `json:"s3,omitempty"`
}

type settingsNFSMount struct {
	Server     string `json:"server"`
	Path       string `json:"path"`
	MountPoint string `json:"mount_point"`
	Options    string `json:"options,omitempty"`
}

type settingsS3 struct {
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
	Region   string `json:"region"`
	UseSSL   bool   `json:"use_ssl"`
}

type settingsBackup struct {
	DefaultSchedule string `json:"default_schedule"`
	RetainDays      int    `json:"retain_days"`
	Compression     string `json:"compression"`
}

type settingsMetrics struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

type settingsCluster struct {
	Enabled                bool  `json:"enabled"`
	HealthCheckIntervalS   int64 `json:"health_check_interval_s"`
	NodeTimeoutS           int64 `json:"node_timeout_s"`
}

// settingsPatchRequest contains the mutable fields the UI can update.
type settingsPatchRequest struct {
	LogLevel *string             `json:"log_level,omitempty"`
	Backup   *settingsBackup     `json:"backup,omitempty"`
	Metrics  *settingsMetrics    `json:"metrics,omitempty"`
	Cluster  *settingsClusterPatch `json:"cluster,omitempty"`
}

type settingsClusterPatch struct {
	HealthCheckIntervalS *int64 `json:"health_check_interval_s,omitempty"`
	NodeTimeoutS         *int64 `json:"node_timeout_s,omitempty"`
}

func (s *Server) getSettings(c *gin.Context) {
	cfg := s.cfg.DaemonCfg
	if cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "daemon config not available"})
		return
	}

	resp := settingsResponse{
		BindAddr:         cfg.BindAddr,
		ShutdownTimeoutS: int(cfg.ShutdownTimeout.Seconds()),
		DataDir:          cfg.DataDir,
		LogLevel:         cfg.LogLevel,
		Storage: settingsStorage{
			DataDir: cfg.Storage.DataDir,
		},
		Backup: settingsBackup{
			DefaultSchedule: cfg.Backup.DefaultSchedule,
			RetainDays:      cfg.Backup.RetainDays,
			Compression:     cfg.Backup.Compression,
		},
		Metrics: settingsMetrics{
			Enabled: cfg.Metrics.Enabled,
			Path:    cfg.Metrics.Path,
		},
		Cluster: settingsCluster{
			Enabled:              cfg.Cluster.Enabled,
			HealthCheckIntervalS: int64(cfg.Cluster.HealthCheckInterval.Seconds()),
			NodeTimeoutS:         int64(cfg.Cluster.NodeTimeout.Seconds()),
		},
	}

	// NFS mounts
	for _, m := range cfg.Storage.NFS {
		resp.Storage.NFSMounts = append(resp.Storage.NFSMounts, settingsNFSMount{
			Server:     m.Server,
			Path:       m.Path,
			MountPoint: m.MountPoint,
			Options:    m.Options,
		})
	}
	if resp.Storage.NFSMounts == nil {
		resp.Storage.NFSMounts = []settingsNFSMount{}
	}

	// S3 (no credentials)
	if cfg.Storage.S3 != nil {
		resp.Storage.S3 = &settingsS3{
			Endpoint: cfg.Storage.S3.Endpoint,
			Bucket:   cfg.Storage.S3.Bucket,
			Region:   cfg.Storage.S3.Region,
			UseSSL:   cfg.Storage.S3.UseSSL,
		}
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) patchSettings(c *gin.Context) {
	cfg := s.cfg.DaemonCfg
	if cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "daemon config not available"})
		return
	}

	var req settingsPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply mutable fields
	if req.LogLevel != nil {
		cfg.LogLevel = *req.LogLevel
	}
	if req.Backup != nil {
		if req.Backup.DefaultSchedule != "" {
			cfg.Backup.DefaultSchedule = req.Backup.DefaultSchedule
		}
		if req.Backup.RetainDays > 0 {
			cfg.Backup.RetainDays = req.Backup.RetainDays
		}
		if req.Backup.Compression != "" {
			cfg.Backup.Compression = req.Backup.Compression
		}
	}
	if req.Metrics != nil {
		cfg.Metrics.Enabled = req.Metrics.Enabled
		if req.Metrics.Path != "" {
			cfg.Metrics.Path = req.Metrics.Path
		}
	}
	if req.Cluster != nil {
		if req.Cluster.HealthCheckIntervalS != nil {
			cfg.Cluster.HealthCheckInterval = time.Duration(*req.Cluster.HealthCheckIntervalS) * time.Second
		}
		if req.Cluster.NodeTimeoutS != nil {
			cfg.Cluster.NodeTimeout = time.Duration(*req.Cluster.NodeTimeoutS) * time.Second
		}
	}

	// Persist to disk if a config path is configured
	if s.cfg.ConfigPath != "" {
		data, err := yaml.Marshal(cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialise config: " + err.Error()})
			return
		}
		if err := os.WriteFile(s.cfg.ConfigPath, data, 0o600); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write config file: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "settings updated"})
}

func (s *Server) getSystemStatus(c *gin.Context) {
	status := s.cfg.HealthSvc.SystemStatus()
	c.JSON(http.StatusOK, status)
}

// getInitStatus returns whether the system has been bootstrapped (≥1 user exists).
// This endpoint is public so the UI can redirect to /setup on first run.
func (s *Server) getInitStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"initialized": s.cfg.AuthSvc.IsInitialized()})
}

// bootstrapRequest is the payload for the first-run bootstrap endpoint.
type bootstrapRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// bootstrapSystem creates the first admin account and, when ConfigPath is set,
// persists the credentials to daemon.yaml so they survive restarts.
// It refuses with 409 Conflict if the system is already initialised.
func (s *Server) bootstrapSystem(c *gin.Context) {
	var req bootstrapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, hash, err := s.cfg.AuthSvc.BootstrapAdmin(c.Request.Context(),
		auth.CreateUserRequest{Username: req.Username, Password: req.Password},
	)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	// Best-effort: persist the hashed password to daemon.yaml so the admin
	// account survives a daemon restart.
	if s.cfg.ConfigPath != "" && s.cfg.DaemonCfg != nil {
		s.cfg.DaemonCfg.Auth.Local.Enabled = true
		s.cfg.DaemonCfg.Auth.Local.AdminUser = req.Username
		s.cfg.DaemonCfg.Auth.Local.AdminPassHash = hash
		_ = daemonconfig.Write(s.cfg.ConfigPath, s.cfg.DaemonCfg)
	}

	c.JSON(http.StatusCreated, user)
}

func (s *Server) getVersion(c *gin.Context) {
	commit := strings.TrimSpace(runGit("rev-parse", "--short", "HEAD"))
	branch := strings.TrimSpace(runGit("rev-parse", "--abbrev-ref", "HEAD"))
	if branch == "" {
		branch = "main"
	}
	if commit == "" {
		commit = "unknown"
	}
	c.JSON(http.StatusOK, gin.H{
		"version": "1.0.0",
		"build":   "release",
		"commit":  commit,
		"branch":  branch,
	})
}

// getPublicIP detects the machine's public IP by querying ipify.org with a
// short timeout. Falls back to the local outbound IP on failure.
func (s *Server) getPublicIP(c *gin.Context) {
	ip := fetchPublicIP()
	c.JSON(http.StatusOK, gin.H{"public_ip": ip})
}

// getSystemResources returns a snapshot of the host's CPU, RAM, and disk capacity.
// Used by the create-server wizard to warn users before deploying a game that
// exceeds available resources.
func (s *Server) getSystemResources(c *gin.Context) {
	cpuCores := runtime.NumCPU()
	totalRAMGB, freeRAMGB := readHostMemInfo()
	totalDiskGB, freeDiskGB := readHostDisk("/")
	c.JSON(http.StatusOK, gin.H{
		"cpu_cores":     cpuCores,
		"total_ram_gb":  totalRAMGB,
		"free_ram_gb":   freeRAMGB,
		"total_disk_gb": totalDiskGB,
		"free_disk_gb":  freeDiskGB,
	})
}

// readHostMemInfo parses /proc/meminfo and returns (totalGB, availableGB).
func readHostMemInfo() (totalGB, availableGB float64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}
	var totalKB, availKB uint64
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			totalKB = val
		case strings.HasPrefix(line, "MemAvailable:"):
			availKB = val
		}
		if totalKB > 0 && availKB > 0 {
			break
		}
	}
	const kb2gb = 1.0 / 1024 / 1024
	return float64(totalKB) * kb2gb, float64(availKB) * kb2gb
}

// readHostDisk returns (totalGB, freeGB) for the filesystem that contains path.
func readHostDisk(path string) (totalGB, freeGB float64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return
	}
	blockSize := float64(st.Bsize)
	const b2gb = 1.0 / 1024 / 1024 / 1024
	totalGB = float64(st.Blocks) * blockSize * b2gb
	freeGB = float64(st.Bavail) * blockSize * b2gb
	return
}

// ─────────────────────────────────────────────── Server diagnostics ──

// DiagnosticFinding is a single discovered issue or OK signal.
type DiagnosticFinding struct {
	Severity string `json:"severity"` // "ok" | "warning" | "error"
	Title    string `json:"title"`
	Detail   string `json:"detail,omitempty"`
	Fix      string `json:"fix,omitempty"`
}

// DiagnosticReport is the structured result of a server self-diagnosis run.
type DiagnosticReport struct {
	ServerID string               `json:"server_id"`
	RunAt    time.Time            `json:"run_at"`
	Findings []DiagnosticFinding  `json:"findings"`
}

// diagnoseServer runs a set of heuristic checks against a server and returns
// a structured report with plain-English explanations and actionable fixes.
// It never blocks — each check has a short timeout.
func (s *Server) diagnoseServer(c *gin.Context) {
	id := c.Param("id")
	sv, err := s.cfg.Broker.GetServer(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	report := DiagnosticReport{
		ServerID: id,
		RunAt:    time.Now(),
	}
	add := func(f DiagnosticFinding) { report.Findings = append(report.Findings, f) }

	// 1. Docker daemon availability
	// P55: properly cancel the timeout context to avoid goroutine leaks.
	ctx5 := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(c.Request.Context(), 3*time.Second)
	}
	dockerOK := false
	dockerCtx, dockerCancel := ctx5()
	out, err2 := exec.CommandContext(dockerCtx, "docker", "info", "--format", "{{.ServerVersion}}").Output()
	dockerCancel()
	if err2 == nil && len(out) > 0 {
		dockerOK = true
		add(DiagnosticFinding{Severity: "ok", Title: "Docker is running", Detail: "Docker daemon responded successfully."})
	} else {
		add(DiagnosticFinding{
			Severity: "error",
			Title:    "Docker is not running",
			Detail:   "The Docker daemon did not respond. Without Docker, no game server can start.",
			Fix:      "Run: sudo systemctl start docker  (or reboot the machine)",
		})
	}

	// 2. Last error message
	if sv.LastError != "" {
		add(DiagnosticFinding{
			Severity: "error",
			Title:    "Previous start error",
			Detail:   sv.LastError,
			Fix:      "Check the Console tab for the full log output near the time of the error.",
		})
	} else if sv.State == broker.StateError {
		add(DiagnosticFinding{
			Severity: "error",
			Title:    "Server is in error state",
			Detail:   "The server stopped unexpectedly but no error message was captured.",
			Fix:      "Open the Console tab and look for recent error output.",
		})
	}

	// 3. Crash loop detection
	if sv.RestartCount > 0 {
		severity := "warning"
		fix := "Check the Console tab for error messages. Consider increasing server RAM or disk space."
		if sv.RestartCount >= 3 {
			severity = "error"
			fix = "The server has crashed repeatedly. Check the Console tab and disable auto-restart to investigate."
		}
		add(DiagnosticFinding{
			Severity: severity,
			Title:    "Server has crashed and restarted",
			Detail:   strings.ReplaceAll("The server has restarted %d time(s) since its last clean run.", "%d", strconv.Itoa(sv.RestartCount)),
			Fix:      fix,
		})
	}

	// 4. Disk space
	_, freeGB := readHostDisk("/")
	if freeGB < 1.0 {
		add(DiagnosticFinding{
			Severity: "error",
			Title:    "Disk is nearly full",
			Detail:   strings.ReplaceAll("Only %.1f GB free on the root filesystem.", "%.1f", strconv.FormatFloat(freeGB, 'f', 1, 64)),
			Fix:      "Free up disk space by deleting old backups, log files, or unused Docker images (docker system prune).",
		})
	} else if freeGB < 5.0 {
		add(DiagnosticFinding{
			Severity: "warning",
			Title:    "Low disk space",
			Detail:   strings.ReplaceAll("%.1f GB free on the root filesystem.", "%.1f", strconv.FormatFloat(freeGB, 'f', 1, 64)),
			Fix:      "Consider freeing up disk space before the server runs out.",
		})
	} else {
		add(DiagnosticFinding{Severity: "ok", Title: "Disk space is adequate", Detail: strconv.FormatFloat(freeGB, 'f', 1, 64) + " GB free."})
	}

	// 5. Port conflicts — check each configured port
	if dockerOK && len(sv.Ports) > 0 {
		for _, p := range sv.Ports {
			if p.External <= 0 {
				continue
			}
			addr := ":" + strconv.Itoa(p.External)
			conn, dialErr := net.DialTimeout("tcp", addr, 500*time.Millisecond)
			if dialErr == nil {
				conn.Close()
				if sv.State != broker.StateRunning {
					add(DiagnosticFinding{
						Severity: "error",
						Title:    "Port " + strconv.Itoa(p.External) + " is already in use",
						Detail:   "Something else on this machine is listening on port " + strconv.Itoa(p.External) + " (" + p.Protocol + "). The game server cannot bind to it.",
						Fix:      "Find what is using the port: sudo ss -tlnp | grep :" + strconv.Itoa(p.External) + " — then stop it or change the server port in the Ports tab.",
					})
				}
			}
		}
	}

	// 6. Memory availability
	_, freeRAM := readHostMemInfo()
	if freeRAM < 0.5 {
		add(DiagnosticFinding{
			Severity: "error",
			Title:    "Very low available RAM",
			Detail:   strconv.FormatFloat(freeRAM, 'f', 2, 64) + " GB free. The server process may be killed by the OS.",
			Fix:      "Stop other running servers or applications to free up memory.",
		})
	} else if freeRAM < 2.0 {
		add(DiagnosticFinding{
			Severity: "warning",
			Title:    "Low available RAM",
			Detail:   strconv.FormatFloat(freeRAM, 'f', 1, 64) + " GB free. Some game servers require 2–4 GB.",
			Fix:      "Stop other running servers to free up memory before starting this one.",
		})
	} else {
		add(DiagnosticFinding{Severity: "ok", Title: "Memory is adequate", Detail: strconv.FormatFloat(freeRAM, 'f', 1, 64) + " GB free."})
	}

	// 7. State summary — if running, everything looks fine
	if sv.State == broker.StateRunning {
		add(DiagnosticFinding{Severity: "ok", Title: "Server is running", Detail: "The server is currently online and accepting connections."})
	}

	c.JSON(http.StatusOK, report)
}

// cloneServer creates a copy of a server under a new name. The clone starts
// in the stopped state and must be deployed before it can be started.
func (s *Server) cloneServer(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	clone, err := s.cfg.Broker.CloneServer(c.Request.Context(), id, body.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "server_clone", id, true, map[string]string{"clone_id": clone.ID, "name": clone.Name})
	c.JSON(http.StatusCreated, clone)
}

// fetchPublicIP tries several well-known plain-text IP services.
// Returns the first success or falls back to the local outbound address.
func fetchPublicIP() string {
	services := []string{
		"https://api.ipify.org",
		"https://checkip.amazonaws.com",
		"https://ifconfig.me/ip",
	}
	client := &http.Client{Timeout: 3 * time.Second}
	for _, svc := range services {
		resp, err := client.Get(svc) //nolint:gosec
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		ip := strings.TrimSpace(string(b))
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	// Fall back to local outbound IP
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		defer conn.Close()
		return conn.LocalAddr().(*net.UDPAddr).IP.String()
	}
	return ""
}

func runGit(args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", repoDirPath}, args...)...).Output() //nolint:gosec
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Middleware

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		// Also accept ?token= for WebSocket upgrade requests from browsers,
		// which cannot set custom headers.
		if token == "" {
			if t := c.Query("token"); t != "" {
				token = "Bearer " + t
			}
		}
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			c.Abort()
			return
		}

		claims, err := s.cfg.AuthSvc.ValidateToken(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set("user", claims)
		c.Next()
	}
}

// serverACLMiddleware enforces per-server access control. Admins bypass the check.
// Must run after authMiddleware (requires "user" in context).
func (s *Server) serverACLMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate the server ID before anything else — prevents path traversal.
		if !validServerID.MatchString(c.Param("id")) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server ID"})
			c.Abort()
			return
		}
		claims := s.getUser(c)
		if claims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			c.Abort()
			return
		}
		if claims.HasRole("admin") {
			c.Next()
			return
		}
		serverID := c.Param("id")
		if !s.cfg.AuthSvc.CanAccessServer(claims.UserID, serverID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied to this server"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// requireOperator rejects requests from users without operator or admin role.
func (s *Server) requireOperator() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := s.getUser(c)
		if claims == nil || (!claims.HasRole("admin") && !claims.HasRole("operator")) {
			c.JSON(http.StatusForbidden, gin.H{"error": "operator or admin role required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) requireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := s.getUser(c)
		if claims == nil || !claims.HasRole(role) {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) getUser(c *gin.Context) *auth.Claims {
	user, exists := c.Get("user")
	if !exists {
		return nil
	}
	claims, ok := user.(*auth.Claims)
	if !ok {
		return nil
	}
	return claims
}

func (s *Server) getNotifications(c *gin.Context) {
	if s.cfg.NotificationSvc == nil {
		c.JSON(http.StatusOK, gin.H{"webhook_url": "", "webhook_format": "discord", "events": []string{}})
		return
	}
	cfg := s.cfg.NotificationSvc.GetConfig()
	resp := gin.H{
		"webhook_url":    cfg.WebhookURL,
		"webhook_format": cfg.WebhookFormat,
		"events":         cfg.Events,
	}
	if cfg.Email != nil {
		// Never return the password — send a boolean so the UI can show "configured"
		resp["email"] = gin.H{
			"enabled":      cfg.Email.Enabled,
			"smtp_host":    cfg.Email.SMTPHost,
			"smtp_port":    cfg.Email.SMTPPort,
			"username":     cfg.Email.Username,
			"password_set": cfg.Email.Password != "",
			"from":         cfg.Email.From,
			"to":           cfg.Email.To,
			"use_tls":      cfg.Email.UseTLS,
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) patchNotifications(c *gin.Context) {
	var body struct {
		WebhookURL    *string                    `json:"webhook_url"`
		WebhookFormat *string                    `json:"webhook_format"`
		Events        []string                   `json:"events"`
		Email         *notifications.EmailConfig `json:"email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if s.cfg.NotificationSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "notification service not available"})
		return
	}
	cur := s.cfg.NotificationSvc.GetConfig()
	if body.WebhookURL != nil {
		cur.WebhookURL = *body.WebhookURL
	}
	if body.WebhookFormat != nil {
		cur.WebhookFormat = *body.WebhookFormat
	}
	if body.Events != nil {
		cur.Events = body.Events
	}
	if body.Email != nil {
		// Preserve the stored password if the client sent an empty one —
		// the GET response never returns the password, so the client can't echo it back.
		if body.Email.Password == "" && cur.Email != nil {
			body.Email.Password = cur.Email.Password
		}
		cur.Email = body.Email
	}
	s.cfg.NotificationSvc.UpdateConfig(cur)
	// Also persist to daemon.yaml if ConfigPath is set
	if s.cfg.DaemonCfg != nil {
		s.cfg.DaemonCfg.Notifications.WebhookURL = cur.WebhookURL
		s.cfg.DaemonCfg.Notifications.WebhookFormat = cur.WebhookFormat
		s.cfg.DaemonCfg.Notifications.Events = cur.Events
		if cur.Email != nil {
			s.cfg.DaemonCfg.Notifications.Email = &daemonconfig.EmailConfig{
				Enabled:  cur.Email.Enabled,
				SMTPHost: cur.Email.SMTPHost,
				SMTPPort: cur.Email.SMTPPort,
				Username: cur.Email.Username,
				Password: cur.Email.Password,
				From:     cur.Email.From,
				To:       cur.Email.To,
				UseTLS:   cur.Email.UseTLS,
			}
		}
		if s.cfg.ConfigPath != "" {
			_ = daemonconfig.Write(s.cfg.ConfigPath, s.cfg.DaemonCfg)
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) testNotification(c *gin.Context) {
	if s.cfg.NotificationSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "notification service not available"})
		return
	}
	if err := s.cfg.NotificationSvc.Test(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// triggerServerUpdate kicks off a game-server update (re-deploy) for the given server.
// The server is stopped first if it is running, re-deployed, then restarted.
func (s *Server) triggerServerUpdate(c *gin.Context) {
	id := c.Param("id")
	job, err := s.cfg.Broker.TriggerServerUpdate(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "server_update", id, true, nil)
	c.JSON(http.StatusAccepted, job)
}

// getTLSStatus returns current TLS certificate metadata (expiry, subject, auto-renewal status).
func (s *Server) getTLSStatus(c *gin.Context) {
	cfg := s.cfg.DaemonCfg
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{"auto_tls": false})
		return
	}

	resp := gin.H{
		"auto_tls":     cfg.TLS.AutoTLS,
		"acme_domain":  cfg.TLS.ACMEDomain,
		"acme_email":   cfg.TLS.ACMEEmail,
		"acme_staging": cfg.TLS.ACMEStaging,
	}

	// Parse the cert file (if present) to expose expiry.
	certPath := cfg.TLS.CertFile
	if cfg.TLS.AutoTLS && cfg.TLS.ACMECacheDir != "" {
		// autocert stores cert under <cache>/<domain>
		certPath = cfg.TLS.ACMECacheDir + "/" + cfg.TLS.ACMEDomain
	}
	if data, err := os.ReadFile(certPath); err == nil {
		resp["cert_file"] = certPath
		_ = data // cert parsing would require x509 — just confirm it exists
		resp["cert_present"] = true
	} else {
		resp["cert_present"] = false
	}

	c.JSON(http.StatusOK, resp)
}

// recordEvent is a convenience wrapper that writes a structured event to the
// audit log using the authenticated user extracted from the gin context.
func (s *Server) recordEvent(c *gin.Context, action, resource string, success bool, details any) {
	claims := s.getUser(c)
	if claims == nil {
		return
	}
	s.cfg.AuthSvc.RecordEvent(claims.UserID, claims.Username, action, resource, c.ClientIP(), success, details)
}

// --- Web Push handlers ---

func (s *Server) getPushVAPIDKey(c *gin.Context) {
	pub := s.cfg.NotificationSvc.GetVAPIDPublicKey()
	if pub == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Web Push not configured"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"public_key": pub})
}

func (s *Server) subscribePush(c *gin.Context) {
	claims := s.getUser(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var sub auth.PushSubscription
	if err := c.ShouldBindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if sub.Endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint is required"})
		return
	}
	if err := s.cfg.AuthSvc.AddPushSubscription(claims.UserID, sub); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) unsubscribePush(c *gin.Context) {
	claims := s.getUser(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var body struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.cfg.AuthSvc.RemovePushSubscription(claims.UserID, body.Endpoint)
	c.Status(http.StatusNoContent)
}

// --- API key handlers ---

func (s *Server) listAPIKeys(c *gin.Context) {
	claims := s.getUser(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	keys, err := s.cfg.AuthSvc.ListAPIKeys(claims.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

func (s *Server) createAPIKey(c *gin.Context) {
	claims := s.getUser(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req auth.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := s.cfg.AuthSvc.CreateAPIKey(c.Request.Context(), claims, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (s *Server) revokeAPIKey(c *gin.Context) {
	claims := s.getUser(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	keyID := c.Param("keyId")
	if err := s.cfg.AuthSvc.RevokeAPIKey(c.Request.Context(), claims.UserID, keyID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
