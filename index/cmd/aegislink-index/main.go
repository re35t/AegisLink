package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/re35t/AegisLink/index/internal/app"
	"github.com/re35t/AegisLink/index/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid Index configuration", "error", err)
		os.Exit(1)
	}

	root, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(root, cfg, logger)
	if err != nil {
		logger.Error("initialize AegisLink Index", "error", err)
		os.Exit(1)
	}
	serverError := make(chan error, 1)
	go func() {
		logger.Info("AegisLink Index listening", "address", cfg.Server.Address)
		serverError <- application.Server.ListenAndServe()
	}()

	select {
	case <-root.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()
		if err := application.Server.Shutdown(shutdownContext); err != nil {
			logger.Error("shut down Index", "error", err)
		}
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Index stopped", "error", err)
			_ = application.Close()
			os.Exit(1)
		}
	}
	if err := application.Close(); err != nil {
		logger.Error("close Index dependencies", "error", err)
	}
}
