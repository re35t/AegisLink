package httpapi

import (
	"net/http"
	"strconv"
	"strings"

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

func (handler *handler) listMCPLibrary(c *gin.Context) {
	actor := actorFrom(c)
	items, err := handler.mcp.ListLibrary(c.Request.Context(), actor.User.ID)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"servers": items})
}

func (handler *handler) createMCPServer(c *gin.Context) {
	var body struct {
		AgentID  string `json:"agentId" binding:"required"`
		Name     string `json:"name" binding:"required"`
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_mcp_server", "agentId, name and endpoint are required")
		return
	}
	actor := actorFrom(c)
	item, err := handler.mcp.Create(c.Request.Context(), actor.User.ID, body.AgentID, body.Name, body.Endpoint)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (handler *handler) updateMCPServer(c *gin.Context) {
	var body struct {
		Name     string `json:"name" binding:"required"`
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_mcp_server", "name and endpoint are required")
		return
	}
	actor := actorFrom(c)
	item, err := handler.mcp.Update(c.Request.Context(), actor.User.ID, c.Param("serverId"), body.Name, body.Endpoint)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (handler *handler) deleteMCPServer(c *gin.Context) {
	actor := actorFrom(c)
	if err := handler.mcp.Delete(c.Request.Context(), actor.User.ID, c.Param("serverId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *handler) refreshMCPServer(c *gin.Context) {
	actor := actorFrom(c)
	item, err := handler.mcp.Refresh(c.Request.Context(), actor.User.ID, c.Param("serverId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (handler *handler) updateMCPToolRisk(c *gin.Context) {
	var body struct {
		RiskLevel mcp.RiskLevel `json:"riskLevel" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_mcp_tool", "riskLevel is required")
		return
	}
	actor := actorFrom(c)
	item, err := handler.mcp.UpdateToolRisk(c.Request.Context(), actor.User.ID, c.Param("serverId"), c.Param("toolId"), body.RiskLevel)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (handler *handler) bindMCPServer(c *gin.Context) {
	actor := actorFrom(c)
	item, err := handler.mcp.BindServer(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("serverId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (handler *handler) unbindMCPServer(c *gin.Context) {
	actor := actorFrom(c)
	if err := handler.mcp.UnbindServer(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("serverId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *handler) bindMCPTool(c *gin.Context) {
	actor := actorFrom(c)
	item, err := handler.mcp.BindTool(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("toolId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (handler *handler) unbindMCPTool(c *gin.Context) {
	actor := actorFrom(c)
	if err := handler.mcp.UnbindTool(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("toolId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *handler) listMentions(c *gin.Context) {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(c, http.StatusBadRequest, "invalid_mentions_query", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	kinds := []string{"mcp-tool"}
	if raw := strings.TrimSpace(c.Query("kinds")); raw != "" {
		kinds = strings.Split(raw, ",")
	}
	actor := actorFrom(c)
	page, err := handler.mcp.Mentions(c.Request.Context(), actor.User.ID, c.Param("agentId"), mcp.MentionQuery{
		Query: c.Query("query"), Kinds: kinds, Cursor: c.Query("cursor"), Limit: limit,
	})
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}
