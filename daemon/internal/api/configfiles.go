package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// listConfigFiles returns the adapter-defined config file list with existence status.
// GET /api/v1/servers/:id/config/files
func (s *Server) listConfigFiles(c *gin.Context) {
	id := c.Param("id")
	files, err := s.cfg.Broker.ListConfigFiles(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// readConfigFile returns the content of a config file for editing.
// GET /api/v1/servers/:id/config/files/*path
func (s *Server) readConfigFile(c *gin.Context) {
	id := c.Param("id")
	path := strings.TrimPrefix(c.Param("path"), "/")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	content, err := s.cfg.Broker.ReadConfigFile(c.Request.Context(), id, path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path, "content": content})
}

// writeConfigFile saves updated content back to a config file.
// PUT /api/v1/servers/:id/config/files/*path
func (s *Server) writeConfigFile(c *gin.Context) {
	id := c.Param("id")
	path := strings.TrimPrefix(c.Param("path"), "/")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.cfg.Broker.WriteConfigFile(c.Request.Context(), id, path, req.Content); err != nil {
		s.cfg.Logger.Error("Config file write failed",
			zap.String("server_id", id), zap.String("path", path), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "edit_config_file", id, true, gin.H{"path": path})
	c.JSON(http.StatusOK, gin.H{"status": "saved", "path": path})
}

// getAdapterTemplates returns the config file templates declared in an adapter manifest.
// GET /api/v1/adapters/:adapterId/config-templates
func (s *Server) getAdapterTemplates(c *gin.Context) {
	adapterID := c.Param("adapterId")
	templates := s.cfg.Broker.GetAdapterConfigTemplates(adapterID)
	c.JSON(http.StatusOK, gin.H{"adapter": adapterID, "templates": templates})
}
