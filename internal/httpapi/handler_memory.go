package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/memory"
)

func (handler *handler) listMemories(c *gin.Context) {
	actor := actorFrom(c)
	items, err := handler.memories.List(c.Request.Context(), actor.User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"memories": items})
}

func (handler *handler) createMemory(c *gin.Context) {
	var body struct {
		Kind       memory.Kind `json:"kind" binding:"required"`
		Content    string      `json:"content" binding:"required"`
		SourceURI  string      `json:"sourceUri"`
		Confidence *float64    `json:"confidence"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_memory", "kind and content are required")
		return
	}
	confidence := 1.0
	if body.Confidence != nil {
		confidence = *body.Confidence
	}
	actor := actorFrom(c)
	item, err := handler.memories.Create(
		c.Request.Context(), actor.User.ID, c.Param("agentId"), body.Kind, body.Content, body.SourceURI, confidence,
	)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (handler *handler) updateMemory(c *gin.Context) {
	var body struct {
		Kind       *memory.Kind `json:"kind"`
		Content    *string      `json:"content"`
		Confidence *float64     `json:"confidence"`
		Confirmed  bool         `json:"confirmed"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_memory", "the memory update is invalid")
		return
	}
	actor := actorFrom(c)
	item, err := handler.memories.Update(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("memoryId"), memory.Update{
		Kind: body.Kind, Content: body.Content, Confidence: body.Confidence, Confirmed: body.Confirmed,
	})
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (handler *handler) forgetMemory(c *gin.Context) {
	actor := actorFrom(c)
	if err := handler.memories.Forget(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("memoryId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
