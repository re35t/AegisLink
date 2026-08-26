package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	internala2a "github.com/re35t/AegisLink/internal/a2a"
	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/catalog"
	"github.com/re35t/AegisLink/internal/config"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/curator"
	"github.com/re35t/AegisLink/internal/discovery"
	"github.com/re35t/AegisLink/internal/harness"
	"github.com/re35t/AegisLink/internal/httpapi"
	"github.com/re35t/AegisLink/internal/impression"
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
	workers       sync.WaitGroup
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
	impressionRepository := postgres.NewImpressionRepository(database)
	discoveryRepository := postgres.NewDiscoveryRepository(database)
	conversationRepository := postgres.NewConversationRepository(database)
	memoryRepository := postgres.NewMemoryRepository(database)
	skillRepository := postgres.NewSkillRepository(database)
	mcpRepository := postgres.NewMCPRepository(database)
	if err := conversationRepository.RecoverInterruptedRuns(root); err != nil {
		return closeOnError(err)
	}
	modelRegistry := agentruntime.DefaultModelRegistry()
	runtime, err := agentruntime.NewWithRegistry(root, runtimeModelConfig(cfg.Model), agentruntime.Options{MaxIterations: cfg.Runtime.MaxIterations}, modelRegistry)
	if err != nil {
		return closeOnError(err)
	}
	curatorConfig := cfg.EffectiveCurator()
	curatorRuntime, err := agentruntime.NewWithRegistry(root, runtimeModelConfig(curatorConfig), agentruntime.Options{MaxIterations: 1}, modelRegistry)
	if err != nil {
		return closeOnError(err)
	}
	curatorService, err := curator.New(curatorRuntime, curatorConfig.Name)
	if err != nil {
		return closeOnError(err)
	}
	accountService, err := account.NewService(accountRepository, cfg.Auth.SessionTTL)
	if err != nil {
		return closeOnError(err)
	}
	agentService := agent.NewService(agentRepository)
	memoryService := memory.NewService(memoryRepository, agentService)
	impressionService := impression.NewService(impressionRepository)
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
	).WithImpressions(impressionService)
	agentCardService := internala2a.NewService(profileService)
	discoveryService, err := discovery.NewService(discoveryRepository, profileService, agentCardService, discovery.NetResolver{}, cfg.Security.AgentKeyEncryptionKey)
	if err != nil {
		return closeOnError(err)
	}
	catalogService := catalog.NewService(agentService, mcpService, skillService)
	agentHarness := harness.New(runtime, memoryService, skillService, mcpService, profileService, impressionService)
	conversationService := conversation.NewService(
		root,
		conversationRepository,
		agentService,
		agentHarness,
		logger,
	)
	router := httpapi.NewRouter(httpapi.Dependencies{
		Accounts:      accountService,
		Agents:        agentService,
		Profiles:      profileService,
		Impressions:   impressionService,
		Discovery:     discoveryService,
		AgentCards:    agentCardService,
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
	application := &Application{Server: server, closeDatabase: closeDatabase, cancel: cancel}
	worker := impression.NewWorker(impressionRepository, curatorService, logger)
	application.workers.Add(1)
	go func() {
		defer application.workers.Done()
		worker.Run(root)
	}()
	return application, nil
}

func runtimeModelConfig(cfg config.Model) agentruntime.ModelConfig {
	return agentruntime.ModelConfig{
		ID: cfg.ID, Driver: cfg.Driver, BaseURL: cfg.BaseURL, APIKey: cfg.APIKey,
		Name: cfg.Name, Timeout: cfg.Timeout, MaxTokens: cfg.MaxTokens,
	}
}

func (application *Application) Close() error {
	application.cancel()
	application.workers.Wait()
	return application.closeDatabase()
}
