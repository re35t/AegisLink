package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/conversation"
)

type handler struct {
	accounts      AccountService
	agents        AgentService
	conversations ConversationService
	auth          AuthConfig
	model         ModelInfo
	logger        *slog.Logger
}

type errorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

func (handler *handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, account.ErrInvalidRegistration):
		writeError(c, http.StatusBadRequest, "invalid_registration", "use a valid email, display name, and password of at least 10 characters")
	case errors.Is(err, account.ErrEmailTaken):
		writeError(c, http.StatusConflict, "email_already_registered", "an account already uses this email")
	case errors.Is(err, account.ErrInvalidCredentials), errors.Is(err, account.ErrUnauthenticated):
		writeError(c, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	case errors.Is(err, conversation.ErrNotFound), errors.Is(err, agent.ErrNotFound):
		writeError(c, http.StatusNotFound, "resource_not_found", "the requested resource was not found")
	case errors.Is(err, conversation.ErrActiveRun):
		writeError(c, http.StatusConflict, "active_run_exists", "the conversation already has an active run")
	case errors.Is(err, conversation.ErrRunNotActive):
		writeError(c, http.StatusConflict, "run_not_active", "the run is no longer active")
	case errors.Is(err, conversation.ErrInvalidMessage):
		writeError(c, http.StatusBadRequest, "invalid_message", "the message is empty or too long")
	default:
		handler.logger.Error("request failed", "requestId", requestIDFrom(c), "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "the server could not complete the request")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{Code: code, Message: message, RequestID: requestIDFrom(c)})
}
