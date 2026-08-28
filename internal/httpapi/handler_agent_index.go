package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/agentindex"
)

func (handler *handler) configureAgent(c *gin.Context) {
	if handler.setup == nil {
		handler.handleError(c, agentindex.ErrUnavailable)
		return
	}
	var body struct {
		Name         string `json:"name"`
		PrimaryFocus string `json:"primaryFocus"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.PrimaryFocus) == "" {
		writeError(c, http.StatusBadRequest, "invalid_agent_profile", "provide an Agent name and primary focus")
		return
	}
	profile, err := handler.setup.Configure(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), body.Name, body.PrimaryFocus)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (handler *handler) syncAgentDiscovery(c *gin.Context) {
	if handler.agentIndex == nil {
		handler.handleError(c, agentindex.ErrUnavailable)
		return
	}
	if err := handler.agentIndex.Sync(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *handler) searchAgentDiscovery(c *gin.Context) {
	var body struct {
		Query string `json:"query"`
		TopK  int    `json:"topK"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Query) == "" {
		writeError(c, http.StatusBadRequest, "invalid_discovery_query", "provide non-empty input and topK between 1 and 50")
		return
	}
	if body.TopK == 0 {
		body.TopK = 5
	}
	if handler.agentIndex == nil {
		handler.handleError(c, agentindex.ErrUnavailable)
		return
	}
	candidates, err := handler.agentIndex.Search(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), body.Query, body.TopK)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"candidates": candidates})
}
