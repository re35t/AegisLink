package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/agent"
)

func (handler *handler) getAgentProfile(c *gin.Context) {
	profile, err := handler.profiles.Get(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (handler *handler) updateAgentProfile(c *gin.Context) {
	var update agent.ProfileUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_agent_profile", "provide an expectedVersion and at least one identity field")
		return
	}
	profile, err := handler.profiles.Update(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), update)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (handler *handler) updateAgentProfileDisclosurePolicies(c *gin.Context) {
	var body struct {
		ExpectedVersion int64                `json:"expectedVersion"`
		Changes         []agent.PolicyChange `json:"changes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_agent_profile", "provide an expectedVersion and disclosure policy changes")
		return
	}
	profile, err := handler.profiles.UpdatePolicies(
		c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), body.ExpectedVersion, body.Changes,
	)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}
