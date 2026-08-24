package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/mcp"
)

func (handler *handler) listMCPServers(c *gin.Context) {
	actor := actorFrom(c)
	items, err := handler.mcp.List(c.Request.Context(), actor.User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"servers": items})
}

func (handler *handler) createMCPServer(c *gin.Context) {
	var body struct {
		Name     string `json:"name" binding:"required"`
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_mcp_server", "name and endpoint are required")
		return
	}
	actor := actorFrom(c)
	item, err := handler.mcp.Create(c.Request.Context(), actor.User.ID, c.Param("agentId"), body.Name, body.Endpoint)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (handler *handler) updateMCPServer(c *gin.Context) {
	var body struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Enabled == nil {
		writeError(c, http.StatusBadRequest, "invalid_mcp_server", "enabled is required")
		return
	}
	actor := actorFrom(c)
	item, err := handler.mcp.SetEnabled(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("serverId"), *body.Enabled)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (handler *handler) deleteMCPServer(c *gin.Context) {
	actor := actorFrom(c)
	if err := handler.mcp.Delete(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("serverId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *handler) refreshMCPServer(c *gin.Context) {
	actor := actorFrom(c)
	item, err := handler.mcp.Refresh(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("serverId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (handler *handler) updateMCPTool(c *gin.Context) {
	var body struct {
		Enabled   *bool         `json:"enabled" binding:"required"`
		RiskLevel mcp.RiskLevel `json:"riskLevel" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Enabled == nil {
		writeError(c, http.StatusBadRequest, "invalid_mcp_server", "enabled and riskLevel are required")
		return
	}
	actor := actorFrom(c)
	item, err := handler.mcp.UpdateTool(
		c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("serverId"), c.Param("toolName"), *body.Enabled, body.RiskLevel,
	)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
