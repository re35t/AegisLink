package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (handler *handler) listConversations(c *gin.Context) {
	conversations, err := handler.conversations.ListConversations(c.Request.Context(), actorFrom(c).User.ID)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

func (handler *handler) createConversation(c *gin.Context) {
	var body struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	created, err := handler.conversations.CreateConversation(c.Request.Context(), actorFrom(c).User.ID, body.Title)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"conversation": created})
}

func (handler *handler) getConversation(c *gin.Context) {
	detail, err := handler.conversations.GetConversation(c.Request.Context(), actorFrom(c).User.ID, c.Param("conversationId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (handler *handler) sendMessage(c *gin.Context) {
	var body struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_message", "content is required")
		return
	}
	message, run, err := handler.conversations.SendMessage(
		c.Request.Context(), actorFrom(c).User.ID, c.Param("conversationId"), body.Content,
	)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": message, "run": run})
}
