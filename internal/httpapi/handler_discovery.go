package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/discovery"
)

func (handler *handler) getAgentPublication(c *gin.Context) {
	item, err := handler.discovery.Get(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(200, item)
}
func (handler *handler) updateAgentPublication(c *gin.Context) {
	var body discovery.SettingsUpdate
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, 400, "invalid_request", "provide valid publication settings")
		return
	}
	item, err := handler.discovery.Update(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), body)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(200, item)
}
func (handler *handler) verifyAgentPublicationHostname(c *gin.Context) {
	item, err := handler.discovery.VerifyHostname(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(200, item)
}
func (handler *handler) rotateAgentPublicationKey(c *gin.Context) {
	item, err := handler.discovery.RotateKey(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(200, item)
}
func (handler *handler) listAgentAccessTokens(c *gin.Context) {
	item, err := handler.discovery.Get(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(200, map[string]any{"tokens": item.Tokens})
}
func (handler *handler) createAgentAccessToken(c *gin.Context) {
	var body discovery.TokenRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, 400, "invalid_request", "provide a label and audience")
		return
	}
	item, err := handler.discovery.CreateToken(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), body)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (handler *handler) revokeAgentAccessToken(c *gin.Context) {
	if err := handler.discovery.RevokeToken(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), c.Param("tokenId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(204)
}
func (handler *handler) getAgentCardPreview(c *gin.Context) {
	item, err := handler.agentCards.Preview(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(200, item)
}

func requestHost(c *gin.Context) string { return c.Request.Host }
func (handler *handler) getPublicAgentFacts(c *gin.Context) {
	document, etag, ttl, err := handler.discovery.PublicDocument(c.Request.Context(), requestHost(c))
	if err != nil {
		if errors.Is(err, discovery.ErrUnavailable) {
			writeError(c, http.StatusServiceUnavailable, "publication_unavailable", "AgentFacts publication is temporarily unavailable")
			return
		}
		handler.handleError(c, err)
		return
	}
	if c.GetHeader("If-None-Match") == `"`+etag+`"` {
		c.Status(304)
		return
	}
	c.Header("ETag", `"`+etag+`"`)
	c.Header("Cache-Control", "public, max-age="+formatInt(ttl))
	c.JSON(200, document)
}
func (handler *handler) getAgentFactsJWKS(c *gin.Context) {
	document, err := handler.discovery.JWKS(c.Request.Context(), requestHost(c))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(200, document)
}
func (handler *handler) getAgentFactsRevocations(c *gin.Context) {
	document, err := handler.discovery.Revocations(c.Request.Context(), requestHost(c))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, document)
}
func (handler *handler) queryAgentFacts(c *gin.Context) {
	var body discovery.QueryRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, 400, "invalid_request", "provide a valid AgentFacts query")
		return
	}
	bearer := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	document, err := handler.discovery.Query(c.Request.Context(), requestHost(c), bearer, body)
	if err != nil {
		if errors.Is(err, discovery.ErrUnavailable) {
			writeError(c, http.StatusServiceUnavailable, "publication_unavailable", "AgentFacts publication is temporarily unavailable")
			return
		}
		handler.handleError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, document)
}
func formatInt(value int64) string {
	if value == 0 {
		return "0"
	}
	result := ""
	for value > 0 {
		result = string(rune('0'+value%10)) + result
		value /= 10
	}
	return result
}
