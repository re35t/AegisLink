package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/index/internal/discovery"
	"github.com/re35t/AegisLink/index/internal/registry"
)

const maxDiscoveryBodyBytes = 2 << 20

func replaceRepresentation(c *gin.Context, dependencies Dependencies, logger *slog.Logger) {
	if dependencies.Discovery == nil {
		writeInternalError(c, logger, errors.New("Discovery dependency is unavailable"))
		return
	}
	if !authorize(c, dependencies.RegistrationToken, "valid publication credentials are required") {
		return
	}
	var snapshot discovery.Snapshot
	if err := decodeJSON(c, &snapshot, maxDiscoveryBodyBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, errorResponse{
				Code: "request_too_large", Message: "representation body must not exceed 2 MiB",
			})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, errorResponse{
			Code: "invalid_vector_snapshot", Message: "request body must be one valid representation snapshot",
		})
		return
	}
	err := dependencies.Discovery.Publish(
		c.Request.Context(), registry.AgentAddr(c.Param("agentAddr")), snapshot,
	)
	if err == nil {
		c.Status(http.StatusNoContent)
		return
	}
	writePublicationError(c, logger, err)
}

func searchRepresentations(c *gin.Context, dependencies Dependencies, logger *slog.Logger) {
	if dependencies.Discovery == nil {
		writeSearchUnavailable(c, logger, errors.New("Discovery dependency is unavailable"))
		return
	}
	if !authorize(c, dependencies.QueryToken, "valid query credentials are required") {
		return
	}
	var query discovery.Query
	if err := decodeJSON(c, &query, maxDiscoveryBodyBytes); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errorResponse{
			Code: "invalid_query_vector", Message: "request body must be one valid discovery query",
		})
		return
	}
	result, err := dependencies.Discovery.Search(c.Request.Context(), query)
	if err == nil {
		c.JSON(http.StatusOK, result)
		return
	}
	switch {
	case errors.Is(err, discovery.ErrUnsupportedProfile):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Code: "encoder_profile_unsupported", Message: err.Error()})
	case errors.Is(err, discovery.ErrInvalidQuery):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Code: "invalid_query_vector", Message: err.Error()})
	default:
		writeSearchUnavailable(c, logger, err)
	}
}

func writePublicationError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, discovery.ErrAgentNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Code: "agent_not_found", Message: err.Error()})
	case errors.Is(err, discovery.ErrUnsupportedProfile):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Code: "encoder_profile_unsupported", Message: err.Error()})
	case errors.Is(err, discovery.ErrInvalidSnapshot):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Code: "invalid_vector_snapshot", Message: err.Error()})
	case errors.Is(err, discovery.ErrStaleRevision):
		c.JSON(http.StatusConflict, errorResponse{Code: "stale_representation_revision", Message: err.Error()})
	default:
		writeInternalError(c, logger, err)
	}
}

func authorize(c *gin.Context, token, message string) bool {
	if validBearerToken(c.GetHeader("Authorization"), token) {
		return true
	}
	c.Header("WWW-Authenticate", `Bearer realm="aegislink-index"`)
	c.JSON(http.StatusUnauthorized, errorResponse{Code: "unauthorized", Message: message})
	return false
}

func decodeJSON(c *gin.Context, destination any, limit int64) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	return ensureJSONEnd(decoder)
}

func writeSearchUnavailable(c *gin.Context, logger *slog.Logger, err error) {
	logger.Error("Index search failed", "error", err)
	c.JSON(http.StatusServiceUnavailable, errorResponse{
		Code: "index_unavailable", Message: "Index search is temporarily unavailable",
	})
}
