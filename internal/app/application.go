package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/agentcontext"
	"github.com/re35t/AegisLink/internal/catalog"
	"github.com/re35t/AegisLink/internal/config"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/httpapi"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/memory"
	"github.com/re35t/AegisLink/internal/postgres"
	agentruntime "github.com/re35t/AegisLink/internal/runtime"
	"github.com/re35t/AegisLink/internal/skills"
)

const authCookieName = "aegislink_session"

type Application struct {
	Server *http.Server

	closeDatabase func() error
	cancel        context.CancelFunc
}

func New(parent context.Context, cfg config.Config, logger *slog.Logger) (*Application, error) {
	root, cancel := context.WithCancel(parent)
	database, err := postgres.Open(root, cfg.Database.URL)
	if err != nil {
		cancel()
		return nil, err
	}
	closeDatabase := database.Close
	closeOnError := func(err error) (*Application, error) {
		_ = closeDatabase()
		cancel()
		return nil, err
	}
	if err := database.Migrate(); err != nil {
		return closeOnError(err)
	}
	accountRepository := postgres.NewAccountRepository(database)
	agentRepository := postgres.NewAgentRepository(database)
	agentProfileRepository := postgres.NewAgentProfileRepository(database)
	conversationRepository := postgres.NewConversationRepository(database)
	memoryRepository := postgres.NewMemoryRepository(database)
	skillRepository := postgres.NewSkillRepository(database)
	mcpRepository := postgres.NewMCPRepository(database)
	if err := conversationRepository.RecoverInterruptedRuns(root); err != nil {
		return closeOnError(err)
	}
	runtime, err := agentruntime.New(root, cfg.Model, cfg.Runtime)
	if err != nil {
		return closeOnError(err)
	}
	accountService, err := account.NewService(accountRepository, cfg.Auth.SessionTTL)
	if err != nil {
		return closeOnError(err)
	}
	agentService := agent.NewService(agentRepository)
	memoryService := memory.NewService(memoryRepository, agentService)
	skillService := skills.NewService(skillRepository, agentService)
	mcpClient := mcp.NewOfficialClient(cfg.MCP.Timeout, cfg.MCP.AllowPrivateNetworks)
	mcpService := mcp.NewService(mcpRepository, agentService, mcpClient)
	profileService := agent.NewProfileService(
		agentProfileRepository,
		agentService,
		skillService,
		mcpService,
		agent.StaticCapabilityProvider{Capabilities: []agent.ProfileCapability{
			{
				ID: "model:" + cfg.Model.ID + ":streaming", Name: "Streaming responses",
				Description: "Streams response text while the Agent Run is active.",
				Kind:        agent.CapabilityModel, Tags: []string{"model", cfg.Model.Driver},
				Source: agent.CapabilitySourceRuntime, Confidence: 1, Callable: true,
			},
			{
				ID: "model:" + cfg.Model.ID + ":tool-calling", Name: "Tool calling",
				Description: "Can request authorized Agent Skills and MCP Tools during a Run.",
				Kind:        agent.CapabilityModel, Tags: []string{"model", cfg.Model.Driver},
				Source: agent.CapabilitySourceRuntime, Confidence: 1, Callable: true,
			},
		}},
	)
	catalogService := catalog.NewService(agentService, mcpService, skillService)
	contextService := agentcontext.NewService(memoryService, skillService, mcpService)
	conversationService := conversation.NewService(
		root,
		conversationRepository,
		agentService,
		runtime,
		contextService,
		logger,
	)
	router := httpapi.NewRouter(httpapi.Dependencies{
		Accounts:      accountService,
		Agents:        agentService,
		Profiles:      profileService,
		Conversations: conversationService,
		Memories:      memoryService,
		Skills:        skillService,
		MCP:           mcpService,
		Catalog:       catalogService,
	}, cfg.Web.Origin, httpapi.AuthConfig{
		CookieName: authCookieName, CookieSecure: cfg.Auth.CookieSecure,
	}, httpapi.ModelInfo{
		ID: cfg.Model.ID, Driver: cfg.Model.Driver, Name: cfg.Model.Name, Capabilities: []string{"streaming", "tool-calling"},
	}, logger)
	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       75 * time.Second,
		WriteTimeout:      0,
	}
	return &Application{Server: server, closeDatabase: closeDatabase, cancel: cancel}, nil
}

func (application *Application) Close() error {
	application.cancel()
	return application.closeDatabase()
}
