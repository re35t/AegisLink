package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/agent"
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

func (handler *handler) getAgentInstructions(c *gin.Context) {
	instructions, err := handler.agents.GetInstructions(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, instructions)
}

func (handler *handler) updateAgentInstructions(c *gin.Context) {
	var body struct {
		ExpectedVersion int64   `json:"expectedVersion"`
		SystemPrompt    *string `json:"systemPrompt"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.SystemPrompt == nil {
		writeError(c, http.StatusBadRequest, "invalid_agent_instructions", "provide an expectedVersion and systemPrompt")
		return
	}
	instructions, err := handler.agents.UpdateInstructions(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), agent.InstructionsUpdate{
		ExpectedVersion: body.ExpectedVersion,
		SystemPrompt:    *body.SystemPrompt,
	})
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, instructions)
}
