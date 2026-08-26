package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/impression"
)

func (handler *handler) listAgentImpressions(c *gin.Context) {
	items, err := handler.impressions.List(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), c.Query("status"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, map[string]any{"impressions": items})
}
func (handler *handler) updateAgentImpression(c *gin.Context) {
	var update impression.Update
	if err := c.ShouldBindJSON(&update); err != nil {
		writeError(c, 400, "invalid_request", "provide a valid Impression update")
		return
	}
	item, err := handler.impressions.Update(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), c.Param("impressionId"), update)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (handler *handler) listAgentFactCandidates(c *gin.Context) {
	items, err := handler.impressions.ListCandidates(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), c.Query("status"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, map[string]any{"candidates": items})
}
func (handler *handler) confirmAgentFactCandidate(c *gin.Context) {
	var body struct {
		ExpectedVersion          int64              `json:"expectedVersion"`
		ExpectedCandidateVersion int64              `json:"expectedCandidateVersion"`
		Subject                  *agent.FactSubject `json:"subject,omitempty"`
		Namespace                *string            `json:"namespace,omitempty"`
		Key                      *string            `json:"key,omitempty"`
		Value                    map[string]any     `json:"value,omitempty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, 400, "invalid_request", "provide valid Fact confirmation versions")
		return
	}
	profile, err := handler.profiles.ConfirmCandidate(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), c.Param("candidateId"), body.ExpectedVersion, body.ExpectedCandidateVersion, agent.ConfirmFactUpdate{Subject: body.Subject, Namespace: body.Namespace, Key: body.Key, Value: body.Value})
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}
func (handler *handler) rejectAgentFactCandidate(c *gin.Context) {
	var body struct {
		ExpectedCandidateVersion int64 `json:"expectedCandidateVersion"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, 400, "invalid_request", "provide the candidate version")
		return
	}
	if err := handler.impressions.RejectCandidate(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), c.Param("candidateId"), body.ExpectedCandidateVersion); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (handler *handler) revokeAgentConfirmedFact(c *gin.Context) {
	var body struct {
		ExpectedVersion int64 `json:"expectedVersion"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, 400, "invalid_request", "provide the Profile version")
		return
	}
	profile, err := handler.profiles.RevokeFact(c.Request.Context(), actorFrom(c).User.ID, c.Param("agentId"), c.Param("factId"), body.ExpectedVersion)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}
