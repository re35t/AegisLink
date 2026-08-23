package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (handler *handler) bootstrap(c *gin.Context) {
	actor := actorFrom(c)
	agentRecord, err := handler.agents.Bootstrap(c.Request.Context(), actor.User.ID)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user":  actor.User,
		"agent": agentRecord,
		"model": handler.model,
	})
}

func (handler *handler) listAgents(c *gin.Context) {
	agents, err := handler.agents.List(c.Request.Context(), actorFrom(c).User.ID)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}
