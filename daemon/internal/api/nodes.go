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
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster mode is not enabled — set cluster.enabled: true in the daemon configuration to use multi-node deployments"})
		return
	}

	var req cluster.RegisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	node, err := mgr.Register(req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, node)
}

// getNode returns a single node by ID
func (s *Server) getNode(c *gin.Context) {
	mgr := s.cfg.Broker.ClusterManager()
	if mgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster mode is not enabled — set cluster.enabled: true in the daemon configuration to use multi-node deployments"})
		return
	}

	node, err := mgr.Get(c.Param("nodeId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found — it may have been removed or the ID is incorrect"})
		return
	}
	c.JSON(http.StatusOK, node)
}

// deregisterNode removes a node from the cluster
func (s *Server) deregisterNode(c *gin.Context) {
	mgr := s.cfg.Broker.ClusterManager()
	if mgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster mode is not enabled — set cluster.enabled: true in the daemon configuration to use multi-node deployments"})
		return
	}

	if err := mgr.Deregister(c.Param("nodeId")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found — it may have already been removed"})
		return
	}
	c.Status(http.StatusNoContent)
}

// issueJoinToken generates a single-use 24-hour join token that a new worker
// node must present when calling POST /nodes. This endpoint requires auth so
// only admins can hand out tokens.
func (s *Server) issueJoinToken(c *gin.Context) {
	mgr := s.cfg.Broker.ClusterManager()
	if mgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster mode is not enabled — set cluster.enabled: true in the daemon configuration to use multi-node deployments"})
		return
	}

	token, err := mgr.IssueJoinToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate join token: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":      token,
		"expires_in": "24h",
		"usage":      "gdash node add <hostname> <address> --token " + token,
	})
}

// nodeHeartbeat updates a node's resource usage stats
func (s *Server) nodeHeartbeat(c *gin.Context) {
	mgr := s.cfg.Broker.ClusterManager()
	if mgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster mode is not enabled — set cluster.enabled: true in the daemon configuration to use multi-node deployments"})
		return
	}

	var req cluster.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid heartbeat payload: " + err.Error()})
		return
	}

	if err := mgr.Heartbeat(c.Param("nodeId"), req); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found — it may have been removed; re-register this node"})
		return
	}
	c.Status(http.StatusNoContent)
}
