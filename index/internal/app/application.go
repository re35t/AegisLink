package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/re35t/AegisLink/index/internal/config"
	"github.com/re35t/AegisLink/index/internal/httpapi"
	"github.com/re35t/AegisLink/index/internal/postgres"
	"github.com/re35t/AegisLink/index/internal/registry"
)

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
	closeOnError := func(err error) (*Application, error) {
		_ = database.Close()
		cancel()
		return nil, err
	}
	if err := database.Migrate(); err != nil {
		return closeOnError(err)
	}
	registryService, err := registry.NewService(postgres.NewRegistryRepository(database))
	if err != nil {
		return closeOnError(err)
	}
	return build(cfg, logger, database, registryService, database.Close, cancel), nil
}

func build(
	cfg config.Config,
	logger *slog.Logger,
	readiness httpapi.Readiness,
	registryService httpapi.Registry,
	closeDatabase func() error,
	cancel context.CancelFunc,
) *Application {
	server := &http.Server{
		Addr: cfg.Server.Address,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Readiness: readiness, Registry: registryService,
			RegistrationToken: cfg.Security.RegistrationToken,
		}, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       75 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return &Application{Server: server, closeDatabase: closeDatabase, cancel: cancel}
}

func (application *Application) Close() error {
	if application.cancel != nil {
		application.cancel()
	}
	if application.closeDatabase != nil {
		return application.closeDatabase()
	}
	return nil
}
