package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/config"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/httpapi"
	"github.com/re35t/AegisLink/internal/postgres"
	agentruntime "github.com/re35t/AegisLink/internal/runtime"
)

const authCookieName = "aegislink_session"

type Application struct {
	Server *http.Server

	database *sql.DB
	cancel   context.CancelFunc
}

func New(parent context.Context, cfg config.Config, logger *slog.Logger) (*Application, error) {
	root, cancel := context.WithCancel(parent)
	database, err := postgres.Open(root, cfg.Database.URL)
	if err != nil {
		cancel()
		return nil, err
	}
	closeOnError := func(err error) (*Application, error) {
		database.Close()
		cancel()
		return nil, err
	}
	if err := postgres.Migrate(database); err != nil {
		return closeOnError(err)
	}
	accountRepository := postgres.NewAccountRepository(database)
	agentRepository := postgres.NewAgentRepository(database)
	conversationRepository := postgres.NewConversationRepository(database)
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
	conversationService := conversation.NewService(
		root,
		conversationRepository,
		agentService,
		runtime,
		logger,
	)
	router := httpapi.NewRouter(httpapi.Dependencies{
		Accounts:      accountService,
		Agents:        agentService,
		Conversations: conversationService,
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
	return &Application{Server: server, database: database, cancel: cancel}, nil
}

func (application *Application) Close() error {
	application.cancel()
	if err := application.database.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}
