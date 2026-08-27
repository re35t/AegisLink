package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/index/internal/registry"
)

const maxRegistrationBodyBytes = 1024

type Readiness interface {
	Ready(context.Context) error
}

type Registry interface {
	Register(context.Context, []byte) (registry.Registration, bool, error)
}

type Dependencies struct {
	Readiness         Readiness
	Registry          Registry
	RegistrationToken string
}

type AlwaysReady struct{}

func (AlwaysReady) Ready(context.Context) error { return nil }

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewRouter(dependencies Dependencies, logger *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(requestLogger(logger), gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.Error("Index request panic recovered", "panic", recovered)
		c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse{
			Code: "internal_error", Message: "the Index could not complete the request",
		})
	}))
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		if dependencies.Readiness == nil || dependencies.Readiness.Ready(c.Request.Context()) != nil {
			c.JSON(http.StatusServiceUnavailable, errorResponse{
				Code: "not_ready", Message: "Index dependencies are not ready",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	registryRoutes := router.Group("/api/v1/registry")
	registryRoutes.POST("/agents", func(c *gin.Context) {
		registerAgentAddr(c, dependencies, logger)
	})
	return router
}

func registerAgentAddr(c *gin.Context, dependencies Dependencies, logger *slog.Logger) {
	if dependencies.Registry == nil {
		writeInternalError(c, logger, errors.New("Registry dependency is unavailable"))
		return
	}
	if !validBearerToken(c.GetHeader("Authorization"), dependencies.RegistrationToken) {
		c.Header("WWW-Authenticate", `Bearer realm="aegislink-index"`)
		c.JSON(http.StatusUnauthorized, errorResponse{
			Code: "unauthorized", Message: "valid registration credentials are required",
		})
		return
	}
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if err := registry.ValidateIdempotencyKey(idempotencyKey); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{
			Code: "invalid_request", Message: err.Error(),
		})
		return
	}

	var request map[string]json.RawMessage
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRegistrationBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&request); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{
			Code: "invalid_request", Message: "request body must be an empty JSON object",
		})
		return
	}
	if request == nil || len(request) != 0 {
		c.JSON(http.StatusBadRequest, errorResponse{
			Code: "invalid_request", Message: "request body must be an empty JSON object",
		})
		return
	}
	if err := ensureJSONEnd(decoder); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{
			Code: "invalid_request", Message: "request body must contain exactly one JSON object",
		})
		return
	}

	registration, replayed, err := dependencies.Registry.Register(
		c.Request.Context(),
		registry.HashIdempotencyKey(dependencies.RegistrationToken, idempotencyKey),
	)
	if err != nil {
		writeRegistryError(c, logger, err)
		return
	}
	if replayed {
		c.Header("Idempotency-Replayed", "true")
		c.JSON(http.StatusOK, registration)
		return
	}
	c.JSON(http.StatusCreated, registration)
}

func writeRegistryError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, registry.ErrInvalid):
		c.JSON(http.StatusBadRequest, errorResponse{Code: "invalid_request", Message: err.Error()})
	case errors.Is(err, registry.ErrIdempotencyConflict):
		c.JSON(http.StatusConflict, errorResponse{Code: "idempotency_conflict", Message: err.Error()})
	default:
		writeInternalError(c, logger, err)
	}
}

func writeInternalError(c *gin.Context, logger *slog.Logger, err error) {
	logger.Error("Index request failed", "error", err)
	c.JSON(http.StatusInternalServerError, errorResponse{
		Code: "internal_error", Message: "the Index could not complete the request",
	})
}

func validBearerToken(header, expected string) bool {
	scheme, token, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" || strings.Contains(token, " ") {
		return false
	}
	presentedDigest := sha256.Sum256([]byte(token))
	expectedDigest := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(presentedDigest[:], expectedDigest[:]) == 1
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.Info("Index request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"durationMs", time.Since(started).Milliseconds(),
		)
	}
}
