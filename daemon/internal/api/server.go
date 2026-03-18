package api

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

	s.srv = &http.Server{
		Addr:         cfg.BindAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
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
	r.POST("/api/v1/auth/login", s.login)
	r.GET("/api/v1/auth/oidc/login", s.oidcLogin)
	r.GET("/api/v1/auth/oidc/callback", s.oidcCallback)

	// First-run bootstrap (public; guarded internally when already initialised)
	r.GET("/api/v1/system/init-status", s.getInitStatus)
	r.POST("/api/v1/system/bootstrap", s.bootstrapSystem)

	// Version (public — useful for health checks without credentials)
	r.GET("/api/v1/version", s.getVersion)

	// API v1
	v1 := r.Group("/api/v1")
	v1.Use(s.authMiddleware())

	// Servers
	v1.GET("/servers", s.listServers)
	v1.POST("/servers", s.createServer)
	v1.GET("/servers/:id", s.getServer)
	v1.PUT("/servers/:id", s.updateServer)
	v1.DELETE("/servers/:id", s.deleteServer)
	v1.POST("/servers/:id/start", s.startServer)
	v1.POST("/servers/:id/stop", s.stopServer)
	v1.POST("/servers/:id/restart", s.restartServer)
	v1.POST("/servers/:id/deploy", s.deployServer)
	v1.GET("/servers/:id/status", s.getServerStatus)
	v1.GET("/servers/:id/logs", s.getServerLogs)
	v1.GET("/servers/:id/metrics", s.getServerMetrics)
	v1.GET("/servers/:id/console/stream", s.streamConsole)
	v1.POST("/servers/:id/console/command", s.sendConsoleCommand)

	// Backups
	v1.GET("/servers/:id/backups", s.listBackups)
	v1.POST("/servers/:id/backup", s.triggerBackup)
	v1.POST("/servers/:id/restore/:backupId", s.restoreBackup)

	// Ports
	v1.GET("/servers/:id/ports", s.listPorts)
	v1.PUT("/servers/:id/ports", s.updatePorts)
	v1.POST("/ports/validate", s.validatePorts)

	// Config file editor
	v1.GET("/servers/:id/config-files", s.listConfigFiles)
	v1.GET("/servers/:id/config-files/*path", s.readConfigFile)
	v1.PUT("/servers/:id/config-files/*path", s.writeConfigFile)

	// Mods
	v1.GET("/servers/:id/mods", s.listMods)
	v1.POST("/servers/:id/mods", s.installMod)
	v1.DELETE("/servers/:id/mods/:modId", s.uninstallMod)
	v1.POST("/servers/:id/mods/test", s.testMods)
	v1.POST("/servers/:id/mods/rollback", s.rollbackMods)

	// SBOM & CVE
	v1.GET("/sbom", s.getSBOM)
	v1.GET("/sbom/:component", s.getComponentSBOM)
	v1.POST("/sbom/scan", s.triggerScan)
	v1.GET("/cve-report", s.getCVEReport)

	// Auth (protected — login/oidc are registered as public routes above)
	v1.POST("/auth/logout", s.logout)
	v1.POST("/auth/totp/setup", s.setupTOTP)
	v1.POST("/auth/totp/verify", s.verifyTOTP)

	// Cluster nodes
	v1.GET("/nodes", s.listNodes)
	v1.POST("/nodes", s.registerNode)
	v1.POST("/nodes/join-token", s.issueJoinToken) // generate a one-time worker join token
	v1.GET("/nodes/:nodeId", s.getNode)
	v1.DELETE("/nodes/:nodeId", s.deregisterNode)
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

	// Self-update (admin-only)
	admin.GET("/update/status", s.getUpdateStatus)
	admin.POST("/update/apply", s.applyUpdate)
	admin.GET("/update/log", s.getUpdateLog)

	// Notifications (admin-only)
	admin.GET("/notifications", s.getNotifications)
	admin.PATCH("/notifications", s.patchNotifications)
	admin.POST("/notifications/test", s.testNotification)

	// Firewall (UFW)
	v1.GET("/firewall", s.getFirewallStatus)
	v1.POST("/firewall/rules", s.addFirewallRule)
	v1.DELETE("/firewall/rules/:num", s.deleteFirewallRule)
	v1.POST("/firewall/enabled", s.setFirewallEnabled)

	// System
	v1.GET("/status", s.getSystemStatus)
}

func (s *Server) ListenAndServeTLS() error {
	return s.srv.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
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
	servers, err := s.cfg.Broker.ListServers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "delete_server", id, true, nil)
	c.JSON(http.StatusNoContent, nil)
}

func (s *Server) startServer(c *gin.Context) {
	id := c.Param("id")
	if err := s.cfg.Broker.StartServer(c.Request.Context(), id); err != nil {
		s.recordEvent(c, "start_server", id, false, gin.H{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "start_server", id, true, nil)
	c.JSON(http.StatusOK, gin.H{"status": "starting", "id": id})
}

func (s *Server) stopServer(c *gin.Context) {
	id := c.Param("id")
	if err := s.cfg.Broker.StopServer(c.Request.Context(), id); err != nil {
		s.recordEvent(c, "stop_server", id, false, gin.H{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "stop_server", id, true, nil)
	c.JSON(http.StatusOK, gin.H{"status": "stopping", "id": id})
}

func (s *Server) restartServer(c *gin.Context) {
	id := c.Param("id")
	if err := s.cfg.Broker.RestartServer(c.Request.Context(), id); err != nil {
		s.recordEvent(c, "restart_server", id, false, gin.H{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "restart_server", id, true, nil)
	c.JSON(http.StatusOK, gin.H{"status": "restarting", "id": id})
}

func (s *Server) deployServer(c *gin.Context) {
	id := c.Param("id")
	var req broker.DeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
	
	// Validate authentication BEFORE upgrading to WebSocket.
	// Browsers cannot set custom headers on WebSocket upgrade requests, so
	// accept the JWT from either the Authorization header (CLI) or the
	// ?token= query parameter (browser console tab).
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		if t := c.Query("token"); t != "" {
			authHeader = "Bearer " + t
		}
	}
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no auth token — log in first and include the token as Authorization: Bearer <token> or ?token= in the URL"})
		return
	}

	// Validate token
	claims, err := s.cfg.AuthSvc.ValidateToken(c.Request.Context(), authHeader)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired or invalid — log in again to get a fresh token"})
		return
	}
	
	// TODO: Add RBAC check to verify user has access to this server
	// For now, any authenticated user can access. Implement server access control:
	// if !s.cfg.AuthSvc.CanAccessServer(c.Request.Context(), claims.UserID, id) {
	//     c.JSON(http.StatusForbidden, gin.H{"error": "access denied to this server"})
	//     return
	// }
	_ = claims // Use for future permission checks
	
	// Now safe to upgrade WebSocket with authenticated user
	conn, err := s.ws.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.cfg.Logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	stream, err := s.cfg.Broker.GetConsoleStream(ctx, id)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","msg":"Console stream not available — make sure the server has been deployed and is running"}`)) //nolint:errcheck
		return
	}

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	setup, err := s.cfg.AuthSvc.SetupTOTP(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	if err := s.cfg.AuthSvc.VerifyTOTP(c.Request.Context(), user, req.Code); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid TOTP code"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"verified": true})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (s *Server) deleteUser(c *gin.Context) {
	userID := c.Param("userId")
	if err := s.cfg.AuthSvc.DeleteUser(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (s *Server) getAuditLog(c *gin.Context) {
	logs, err := s.cfg.AuthSvc.GetAuditLog(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"audit_log": logs})
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
	c.JSON(http.StatusOK, gin.H{
		"webhook_url":    cfg.WebhookURL,
		"webhook_format": cfg.WebhookFormat,
		"events":         cfg.Events,
	})
}

func (s *Server) patchNotifications(c *gin.Context) {
	var body struct {
		WebhookURL    *string  `json:"webhook_url"`
		WebhookFormat *string  `json:"webhook_format"`
		Events        []string `json:"events"`
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
	s.cfg.NotificationSvc.UpdateConfig(cur)
	// Also persist to daemon.yaml if ConfigPath is set
	if s.cfg.DaemonCfg != nil {
		s.cfg.DaemonCfg.Notifications.WebhookURL = cur.WebhookURL
		s.cfg.DaemonCfg.Notifications.WebhookFormat = cur.WebhookFormat
		s.cfg.DaemonCfg.Notifications.Events = cur.Events
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

// recordEvent is a convenience wrapper that writes a structured event to the
// audit log using the authenticated user extracted from the gin context.
func (s *Server) recordEvent(c *gin.Context, action, resource string, success bool, details any) {
	claims := s.getUser(c)
	if claims == nil {
		return
	}
	s.cfg.AuthSvc.RecordEvent(claims.UserID, claims.Username, action, resource, c.ClientIP(), success, details)
}
