package api

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/games-dashboard/daemon/internal/auth"
	"github.com/games-dashboard/daemon/internal/broker"
	daemonconfig "github.com/games-dashboard/daemon/internal/config"
	"github.com/games-dashboard/daemon/internal/health"
	"github.com/games-dashboard/daemon/internal/metrics"
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
	AllowedOrigins []string
	Logger         *zap.Logger
	AuthSvc        *auth.Service
	Broker         *broker.Broker
	HealthSvc      *health.Service
	MetricsSvc     *metrics.Service
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

	// Backups
	v1.GET("/servers/:id/backups", s.listBackups)
	v1.POST("/servers/:id/backup", s.triggerBackup)
	v1.POST("/servers/:id/restore/:backupId", s.restoreBackup)

	// Ports
	v1.GET("/servers/:id/ports", s.listPorts)
	v1.PUT("/servers/:id/ports", s.updatePorts)
	v1.POST("/ports/validate", s.validatePorts)

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

	// Auth
	v1.POST("/auth/login", s.login)
	v1.POST("/auth/logout", s.logout)
	v1.POST("/auth/totp/setup", s.setupTOTP)
	v1.POST("/auth/totp/verify", s.verifyTOTP)
	v1.GET("/auth/oidc/login", s.oidcLogin)
	v1.GET("/auth/oidc/callback", s.oidcCallback)

	// Cluster nodes
	v1.GET("/nodes", s.listNodes)
	v1.POST("/nodes", s.registerNode)
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

	// System
	v1.GET("/status", s.getSystemStatus)
	v1.GET("/version", s.getVersion)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (s *Server) startServer(c *gin.Context) {
	id := c.Param("id")
	if err := s.cfg.Broker.StartServer(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "starting", "id": id})
}

func (s *Server) stopServer(c *gin.Context) {
	id := c.Param("id")
	if err := s.cfg.Broker.StopServer(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopping", "id": id})
}

func (s *Server) restartServer(c *gin.Context) {
	id := c.Param("id")
	if err := s.cfg.Broker.RestartServer(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
	
	// SECURITY FIX: Validate authentication BEFORE upgrading WebSocket
	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization"})
		return
	}
	
	// Validate token
	claims, err := s.cfg.AuthSvc.ValidateToken(c.Request.Context(), authHeader)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
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
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"stream unavailable"}`))
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, job)
}

func (s *Server) restoreBackup(c *gin.Context) {
	id := c.Param("id")
	backupID := c.Param("backupId")
	job, err := s.cfg.Broker.RestoreBackup(c.Request.Context(), id, backupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, job)
}

func (s *Server) uninstallMod(c *gin.Context) {
	id := c.Param("id")
	modID := c.Param("modId")
	if err := s.cfg.Broker.UninstallMod(c.Request.Context(), id, modID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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

func (s *Server) getVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": "1.0.0",
		"build":   "release",
	})
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
