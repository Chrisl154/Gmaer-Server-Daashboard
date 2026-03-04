package api

import (
	"net/http"

	"github.com/games-dashboard/daemon/internal/cluster"
	"github.com/gin-gonic/gin"
)

// listNodes returns all registered cluster nodes
func (s *Server) listNodes(c *gin.Context) {
	mgr := s.cfg.Broker.ClusterManager()
	if mgr == nil {
		c.JSON(http.StatusOK, gin.H{"nodes": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"nodes": mgr.List()})
}

// registerNode adds a new node to the cluster
func (s *Server) registerNode(c *gin.Context) {
	mgr := s.cfg.Broker.ClusterManager()
	if mgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster is not enabled"})
		return
	}

	var req cluster.RegisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node, err := mgr.Register(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, node)
}

// getNode returns a single node by ID
func (s *Server) getNode(c *gin.Context) {
	mgr := s.cfg.Broker.ClusterManager()
	if mgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster is not enabled"})
		return
	}

	node, err := mgr.Get(c.Param("nodeId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, node)
}

// deregisterNode removes a node from the cluster
func (s *Server) deregisterNode(c *gin.Context) {
	mgr := s.cfg.Broker.ClusterManager()
	if mgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster is not enabled"})
		return
	}

	if err := mgr.Deregister(c.Param("nodeId")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// nodeHeartbeat updates a node's resource usage stats
func (s *Server) nodeHeartbeat(c *gin.Context) {
	mgr := s.cfg.Broker.ClusterManager()
	if mgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster is not enabled"})
		return
	}

	var req cluster.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := mgr.Heartbeat(c.Param("nodeId"), req); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
