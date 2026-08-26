package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/catalog"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/discovery"
	"github.com/re35t/AegisLink/internal/impression"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/memory"
	"github.com/re35t/AegisLink/internal/skills"
)

type handler struct {
	accounts      AccountService
	agents        AgentService
	profiles      AgentProfileService
	impressions   ImpressionService
	discovery     DiscoveryService
	agentCards    AgentCardService
	conversations ConversationService
	memories      MemoryService
	skills        SkillService
	mcp           MCPService
	catalog       CatalogService
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
	case errors.Is(err, account.ErrInvalidSettings):
		writeError(c, http.StatusBadRequest, "invalid_account_settings", "display name, language, or theme is invalid")
	case errors.Is(err, account.ErrInvalidPassword):
		writeError(c, http.StatusBadRequest, "invalid_new_password", "use a different new password of at least 10 characters")
	case errors.Is(err, account.ErrCurrentPassword):
		writeError(c, http.StatusBadRequest, "current_password_incorrect", "the current password is incorrect")
	case errors.Is(err, account.ErrInvalidCredentials), errors.Is(err, account.ErrUnauthenticated):
		writeError(c, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	case errors.Is(err, conversation.ErrNotFound), errors.Is(err, agent.ErrNotFound):
		writeError(c, http.StatusNotFound, "resource_not_found", "the requested resource was not found")
	case errors.Is(err, agent.ErrProfileSubject):
		writeError(c, http.StatusNotFound, "profile_subject_not_found", "the requested profile item was not found")
	case errors.Is(err, agent.ErrProfileConflict):
		writeError(c, http.StatusConflict, "profile_version_conflict", "the Agent Profile changed; reload it before saving again")
	case errors.Is(err, agent.ErrInvalidProfile):
		writeError(c, http.StatusBadRequest, "invalid_agent_profile", "the Agent identity or disclosure policy is invalid")
	case errors.Is(err, impression.ErrNotFound), errors.Is(err, discovery.ErrNotFound):
		writeError(c, http.StatusNotFound, "resource_not_found", "the requested resource was not found")
	case errors.Is(err, impression.ErrConflict), errors.Is(err, discovery.ErrConflict):
		writeError(c, http.StatusConflict, "version_conflict", "the resource changed; reload it before saving again")
	case errors.Is(err, impression.ErrInvalid), errors.Is(err, discovery.ErrInvalid):
		writeError(c, http.StatusBadRequest, "invalid_request", "the requested Impression, Fact, or publication change is invalid")
	case errors.Is(err, discovery.ErrUnauthorized):
		writeError(c, http.StatusUnauthorized, "invalid_agentfacts_token", "the AgentFacts query token is invalid")
	case errors.Is(err, discovery.ErrUnavailable):
		writeError(c, http.StatusConflict, "publication_unavailable", "AgentFacts publication is not configured or cannot be enabled")
	case errors.Is(err, conversation.ErrActiveRun):
		writeError(c, http.StatusConflict, "active_run_exists", "the conversation already has an active run")
	case errors.Is(err, conversation.ErrRunNotActive):
		writeError(c, http.StatusConflict, "run_not_active", "the run is no longer active")
	case errors.Is(err, conversation.ErrInvalidMessage):
		writeError(c, http.StatusBadRequest, "invalid_message", "the message is empty or too long")
	case errors.Is(err, catalog.ErrInvalid):
		writeError(c, http.StatusBadRequest, "invalid_mentions_query", "mention query, kinds, cursor, or limit is invalid")
	case errors.Is(err, memory.ErrInvalid):
		writeError(c, http.StatusBadRequest, "invalid_memory", "memory kind, content, source, or confidence is invalid")
	case errors.Is(err, memory.ErrNotFound):
		writeError(c, http.StatusNotFound, "memory_not_found", "the requested memory was not found")
	case errors.Is(err, skills.ErrInvalid):
		writeError(c, http.StatusBadRequest, "invalid_skill", "SKILL.md frontmatter or content is invalid")
	case errors.Is(err, skills.ErrInvalidBundle):
		writeError(c, http.StatusBadRequest, "invalid_skill_bundle", "use a SKILL.md file or a safe ZIP bundle with SKILL.md at its root")
	case errors.Is(err, skills.ErrTooLarge):
		writeError(c, http.StatusRequestEntityTooLarge, "skill_bundle_too_large", "the Skill bundle exceeds the allowed file or archive size")
	case errors.Is(err, skills.ErrResourceUnreadable):
		writeError(c, http.StatusBadRequest, "skill_resource_unreadable", "the requested Skill resource is not readable UTF-8 text")
	case errors.Is(err, skills.ErrConflict):
		writeError(c, http.StatusConflict, "skill_version_conflict", "this Skill version label already refers to different content")
	case errors.Is(err, skills.ErrNotFound):
		writeError(c, http.StatusNotFound, "skill_not_found", "the requested Skill was not found")
	case errors.Is(err, skills.ErrDisabled):
		writeError(c, http.StatusConflict, "skill_disabled", "this Skill is not enabled for the current Agent")
	case errors.Is(err, mcp.ErrInvalid):
		writeError(c, http.StatusBadRequest, "invalid_mcp_server", "MCP name, endpoint, or tool permission is invalid")
	case errors.Is(err, mcp.ErrConflict):
		writeError(c, http.StatusConflict, "mcp_server_exists", "your plugin library already has an MCP Server with that name")
	case errors.Is(err, mcp.ErrNotFound):
		writeError(c, http.StatusNotFound, "mcp_resource_not_found", "the requested MCP Server or Tool was not found")
	case errors.Is(err, mcp.ErrConnection):
		writeError(c, http.StatusBadGateway, "mcp_connection_failed", "the MCP Server could not be reached or did not return a valid tool catalog")
	case errors.Is(err, mcp.ErrForbidden):
		writeError(c, http.StatusForbidden, "mcp_approval_required", "this MCP Tool is not approved for automatic execution")
	case errors.Is(err, mcp.ErrUnavailable):
		writeError(c, http.StatusConflict, "mcp_tool_unavailable", "this MCP Tool is not connected and enabled for the current Agent")
	default:
		handler.logger.Error("request failed", "requestId", requestIDFrom(c), "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "the server could not complete the request")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{Code: code, Message: message, RequestID: requestIDFrom(c)})
}
