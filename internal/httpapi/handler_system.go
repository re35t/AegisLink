package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (handler *handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (handler *handler) ready(c *gin.Context) {
	if err := handler.conversations.Ready(c.Request.Context()); err != nil {
		writeError(c, http.StatusServiceUnavailable, "not_ready", "the database is not ready")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
