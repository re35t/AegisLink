package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/memory"
	"github.com/re35t/AegisLink/internal/skills"
)

const requestIDKey = "requestId"

type AccountService interface {
	Register(context.Context, string, string, string) (account.AuthResult, error)
	Login(context.Context, string, string) (account.AuthResult, error)
	Authenticate(context.Context, string) (account.Actor, error)
	Logout(context.Context, string) error
	GetSettings(context.Context, account.Actor) (account.Settings, error)
	UpdateSettings(context.Context, account.Actor, account.SettingsUpdate) (account.Settings, error)
	ChangePassword(context.Context, account.Actor, string, string) error
}

type AgentService interface {
	Bootstrap(context.Context, string) (agent.Agent, error)
	List(context.Context, string) ([]agent.Agent, error)
}

type ConversationService interface {
	Ready(context.Context) error
	ListConversations(context.Context, string) ([]conversation.Conversation, error)
	CreateConversation(context.Context, string, string) (conversation.Conversation, error)
	GetConversation(context.Context, string, string) (conversation.Detail, error)
	SendMessage(context.Context, string, string, string) (conversation.Message, conversation.Run, error)
	StartRun(context.Context, string, conversation.RunRequest) (conversation.Message, conversation.Run, error)
	GetRun(context.Context, string, string) (conversation.Run, error)
	ListRunEvents(context.Context, string, string, int64) ([]conversation.RunEvent, error)
	CancelRun(context.Context, string, string) error
}

type MemoryService interface {
	List(context.Context, string, string) ([]memory.Memory, error)
	Create(context.Context, string, string, memory.Kind, string, string, float64) (memory.Memory, error)
	Update(context.Context, string, string, string, memory.Update) (memory.Memory, error)
	Forget(context.Context, string, string, string) error
}

type SkillService interface {
	List(context.Context, string, string) ([]skills.Skill, error)
	Install(context.Context, string, string, string, string) (skills.Skill, error)
	Import(context.Context, string, string, skills.ImportRequest) (skills.Skill, error)
	SetEnabled(context.Context, string, string, string, bool) (skills.Skill, error)
	Uninstall(context.Context, string, string, string) error
}

type MCPService interface {
	ListLibrary(context.Context, string) ([]mcp.Server, error)
	List(context.Context, string, string) ([]mcp.Server, error)
	Create(context.Context, string, string, string, string) (mcp.Server, error)
	Update(context.Context, string, string, string, string) (mcp.Server, error)
	BindServer(context.Context, string, string, string) (mcp.Server, error)
	UnbindServer(context.Context, string, string, string) error
	Delete(context.Context, string, string) error
	Refresh(context.Context, string, string) (mcp.Server, error)
	UpdateToolRisk(context.Context, string, string, string, mcp.RiskLevel) (mcp.Server, error)
	BindTool(context.Context, string, string, string) (mcp.Tool, error)
	UnbindTool(context.Context, string, string, string) error
	Mentions(context.Context, string, string, mcp.MentionQuery) (mcp.MentionPage, error)
}

type Dependencies struct {
	Accounts      AccountService
	Agents        AgentService
	Conversations ConversationService
	Memories      MemoryService
	Skills        SkillService
	MCP           MCPService
}

type AuthConfig struct {
	CookieName   string
	CookieSecure bool
}

type ModelInfo struct {
	ID           string   `json:"id"`
	Driver       string   `json:"driver"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
}

func NewRouter(dependencies Dependencies, webOrigin string, auth AuthConfig, model ModelInfo, logger *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(requestID(), requestLogger(logger), cors(webOrigin), originGuard(webOrigin), gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.Error("panic recovered", "requestId", requestIDFrom(c), "panic", recovered)
		writeError(c, http.StatusInternalServerError, "internal_error", "the server could not complete the request")
	}))
	handler := &handler{
		accounts:      dependencies.Accounts,
		agents:        dependencies.Agents,
		conversations: dependencies.Conversations,
		memories:      dependencies.Memories,
		skills:        dependencies.Skills,
		mcp:           dependencies.MCP,
		auth:          auth,
		model:         model,
		logger:        logger,
	}

	router.GET("/healthz", handler.health)
	router.GET("/readyz", handler.ready)
	api := router.Group("/api/v1")
	api.POST("/auth/register", handler.register)
	api.POST("/auth/login", handler.login)
	api.POST("/auth/logout", handler.logout)

	protected := api.Group("")
	protected.Use(authenticate(dependencies.Accounts, auth.CookieName, logger))
	protected.GET("/auth/session", handler.session)
	protected.GET("/account/settings", handler.getAccountSettings)
	protected.PATCH("/account/settings", handler.updateAccountSettings)
	protected.POST("/account/password", handler.changeAccountPassword)
	protected.GET("/bootstrap", handler.bootstrap)
	protected.POST("/ag-ui", handler.runAgent)
	protected.GET("/agents", handler.listAgents)
	protected.GET("/agents/:agentId/memories", handler.listMemories)
	protected.POST("/agents/:agentId/memories", handler.createMemory)
	protected.PATCH("/agents/:agentId/memories/:memoryId", handler.updateMemory)
	protected.DELETE("/agents/:agentId/memories/:memoryId", handler.forgetMemory)
	protected.GET("/agents/:agentId/skills", handler.listSkills)
	protected.POST("/agents/:agentId/skills", handler.installSkill)
	protected.POST("/agents/:agentId/skills/import", handler.importSkill)
	protected.PATCH("/agents/:agentId/skills/:skillId", handler.updateSkill)
	protected.DELETE("/agents/:agentId/skills/:skillId", handler.uninstallSkill)
	protected.GET("/agents/:agentId/mcp-servers", handler.listMCPServers)
	protected.GET("/agents/:agentId/mentions", handler.listMentions)
	protected.PUT("/agents/:agentId/mcp-servers/:serverId", handler.bindMCPServer)
	protected.DELETE("/agents/:agentId/mcp-servers/:serverId", handler.unbindMCPServer)
	protected.PUT("/agents/:agentId/mcp-tools/:toolId", handler.bindMCPTool)
	protected.DELETE("/agents/:agentId/mcp-tools/:toolId", handler.unbindMCPTool)
	protected.GET("/mcp-servers", handler.listMCPLibrary)
	protected.POST("/mcp-servers", handler.createMCPServer)
	protected.PATCH("/mcp-servers/:serverId", handler.updateMCPServer)
	protected.DELETE("/mcp-servers/:serverId", handler.deleteMCPServer)
	protected.POST("/mcp-servers/:serverId/refresh", handler.refreshMCPServer)
	protected.PATCH("/mcp-servers/:serverId/tools/:toolId", handler.updateMCPToolRisk)
	protected.GET("/conversations", handler.listConversations)
	protected.POST("/conversations", handler.createConversation)
	protected.GET("/conversations/:conversationId", handler.getConversation)
	protected.POST("/conversations/:conversationId/messages", handler.sendMessage)
	protected.GET("/runs/:runId/events", handler.runEvents)
	protected.POST("/runs/:runId/cancel", handler.cancelRun)
	return router
}

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" || len(id) > 128 {
			id = ulid.Make().String()
		}
		c.Set(requestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.Info("http request",
			"requestId", requestIDFrom(c),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"durationMs", time.Since(started).Milliseconds(),
		)
	}
}

func cors(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestOrigin := c.GetHeader("Origin")
		if requestOrigin == origin {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Last-Event-ID, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			if requestOrigin != origin {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func originGuard(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		requestOrigin := c.GetHeader("Origin")
		if requestOrigin != "" && requestOrigin != origin {
			writeError(c, http.StatusForbidden, "origin_not_allowed", "request origin is not allowed")
			return
		}
		c.Next()
	}
}

func requestIDFrom(c *gin.Context) string {
	value, _ := c.Get(requestIDKey)
	id, _ := value.(string)
	return id
}
