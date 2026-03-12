package api

import (
	"net/http"
	"strconv"

	"github.com/games-dashboard/daemon/internal/firewall"
	"github.com/gin-gonic/gin"
)

// getFirewallStatus returns UFW availability, enabled state, and the current
// numbered rule list. Always returns 200 — the "available" field tells the UI
// whether ufw is installed.
func (s *Server) getFirewallStatus(c *gin.Context) {
	status, err := s.cfg.FirewallSvc.GetStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read firewall status: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// addFirewallRule opens a port in UFW.
// Body: { "port": 25565, "proto": "tcp", "from": "", "comment": "Minecraft" }
func (s *Server) addFirewallRule(c *gin.Context) {
	var req firewall.AddRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := s.cfg.FirewallSvc.AddRule(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// deleteFirewallRule removes a rule by its UFW rule number.
// The rule numbers come from GET /firewall (ufw status numbered).
func (s *Server) deleteFirewallRule(c *gin.Context) {
	num, err := strconv.Atoi(c.Param("num"))
	if err != nil || num < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule number must be a positive integer"})
		return
	}

	if err := s.cfg.FirewallSvc.DeleteRule(c.Request.Context(), num); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// setFirewallEnabled enables or disables UFW.
// Body: { "enabled": true }
func (s *Server) setFirewallEnabled(c *gin.Context) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := s.cfg.FirewallSvc.SetEnabled(c.Request.Context(), body.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
