package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/collaboration"
)

func (handler *handler) getCollaborationPolicy(c *gin.Context) {
	policy, err := handler.collaboration.GetPolicy(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (handler *handler) updateCollaborationPolicy(c *gin.Context) {
	var body struct {
		ExpectedRevision     int64 `json:"expectedRevision" binding:"required"`
		Enabled              bool  `json:"enabled"`
		MaxSessionTTLSeconds int64 `json:"maxSessionTtlSeconds" binding:"required"`
		MaxRequestsPerHour   int   `json:"maxRequestsPerHour" binding:"required"`
		MaxActiveSessions    int   `json:"maxActiveSessions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_collaboration", "provide the complete collaboration policy and expected revision")
		return
	}
	policy, err := handler.collaboration.UpdatePolicy(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), collaboration.PolicyUpdate{
		ExpectedRevision: body.ExpectedRevision, Enabled: body.Enabled, MaxSessionTTLSeconds: body.MaxSessionTTLSeconds,
		MaxRequestsPerHour: body.MaxRequestsPerHour, MaxActiveSessions: body.MaxActiveSessions,
	})
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (handler *handler) getCollaborations(c *gin.Context) {
	overview, err := handler.collaboration.Overview(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, overview)
}

func (handler *handler) revokeCollaborationSession(c *gin.Context) {
	if err := handler.collaboration.RevokeSession(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), c.Param("sessionId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *handler) getCollaborationAgentCard(c *gin.Context) {
	card, err := handler.collaboration.AgentCard(c.Request.Context(), c.Param("agentAddr"), handler.a2aEndpoint)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, card)
}
