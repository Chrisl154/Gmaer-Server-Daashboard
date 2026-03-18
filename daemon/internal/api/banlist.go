package api

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/games-dashboard/daemon/internal/broker"
	"github.com/gin-gonic/gin"
)

func (s *Server) listBannedPlayers(c *gin.Context) {
	id := c.Param("id")
	players, err := s.cfg.Broker.ListBannedPlayers(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, broker.ErrBanlistNotSupported) {
			c.JSON(http.StatusOK, gin.H{"players": []string{}, "supported": false})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"players": players, "supported": true})
}

func (s *Server) banPlayer(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Player string `json:"player" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.cfg.Broker.BanPlayer(c.Request.Context(), id, req.Player, req.Reason); err != nil {
		if errors.Is(err, broker.ErrBanlistNotSupported) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "ban_player", id, true, gin.H{"player": req.Player})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) unbanPlayer(c *gin.Context) {
	id := c.Param("id")
	player, err := url.PathUnescape(c.Param("player"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid player name"})
		return
	}
	if err := s.cfg.Broker.UnbanPlayer(c.Request.Context(), id, player); err != nil {
		if errors.Is(err, broker.ErrBanlistNotSupported) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "unban_player", id, true, gin.H{"player": player})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) listWhitelistPlayers(c *gin.Context) {
	id := c.Param("id")
	players, err := s.cfg.Broker.ListWhitelistPlayers(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, broker.ErrBanlistNotSupported) {
			c.JSON(http.StatusOK, gin.H{"players": []string{}, "supported": false})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"players": players, "supported": true})
}

func (s *Server) whitelistAddPlayer(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Player string `json:"player" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.cfg.Broker.WhitelistAdd(c.Request.Context(), id, req.Player); err != nil {
		if errors.Is(err, broker.ErrBanlistNotSupported) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "whitelist_add", id, true, gin.H{"player": req.Player})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) whitelistRemovePlayer(c *gin.Context) {
	id := c.Param("id")
	player, err := url.PathUnescape(c.Param("player"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid player name"})
		return
	}
	if err := s.cfg.Broker.WhitelistRemove(c.Request.Context(), id, player); err != nil {
		if errors.Is(err, broker.ErrBanlistNotSupported) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.recordEvent(c, "whitelist_remove", id, true, gin.H{"player": player})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
