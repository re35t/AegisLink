package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (handler *handler) listSkills(c *gin.Context) {
	actor := actorFrom(c)
	items, err := handler.skills.List(c.Request.Context(), actor.User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": items})
}

func (handler *handler) installSkill(c *gin.Context) {
	var body struct {
		Content string `json:"content" binding:"required"`
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_skill", "SKILL.md content is required")
		return
	}
	actor := actorFrom(c)
	item, err := handler.skills.Install(c.Request.Context(), actor.User.ID, c.Param("agentId"), body.Content, body.Version)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (handler *handler) updateSkill(c *gin.Context) {
	var body struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Enabled == nil {
		writeError(c, http.StatusBadRequest, "invalid_skill", "enabled is required")
		return
	}
	actor := actorFrom(c)
	item, err := handler.skills.SetEnabled(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("skillId"), *body.Enabled)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (handler *handler) uninstallSkill(c *gin.Context) {
	actor := actorFrom(c)
	if err := handler.skills.Uninstall(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("skillId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
