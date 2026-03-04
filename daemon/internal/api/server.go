package api

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/games-dashboard/daemon/internal/auth"
	"github.com/games-dashboard/daemon/internal/broker"
	"github.com/games-dashboard/daemon/internal/health"
	"github.com/games-dashboard/daemon/internal/metrics"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Config holds API server configuration
type Config struct {
	BindAddr   string
	TLSCert    string
	TLSKey     string
	Logger     *zap.Logger
	AuthSvc    *auth.Service
	Broker     *broker.Broker
	HealthSvc  *health.Service
	MetricsSvc *metrics.Service
}

// Server is the HTTP/WebSocket API server
type Server struct {
	cfg    Config
	router *gin.Engine
	srv    *http.Server
	ws     *websocket.Upgrader
}

// NewServer creates a new API server
func NewServer(cfg Config) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	s := &Server{
		cfg:    cfg,
		router: router,
		ws: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true // TODO: restrict in production
			},
		},
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
	v1.GET("/auth/oidc/callback", s.oidcCallback)

	// Admin (requires admin role)
	admin := v1.Group("/admin")
	admin.Use(s.requireRole("admin"))
	admin.GET("/users", s.listUsers)
	admin.POST("/users", s.createUser)
	admin.PUT("/users/:userId", s.updateUser)
	admin.DELETE("/users/:userId", s.deleteUser)
	admin.GET("/audit", s.getAuditLog)
	admin.POST("/secrets/rotate", s.rotateSecrets)

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
