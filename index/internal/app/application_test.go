package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/re35t/AegisLink/index/internal/config"
	"github.com/re35t/AegisLink/index/internal/discovery"
	"github.com/re35t/AegisLink/index/internal/httpapi"
	"github.com/re35t/AegisLink/index/internal/registry"
)

func TestBuildsRunnableIndependentServerAndClosesDependencies(t *testing.T) {
	t.Parallel()
	closed := false
	root, cancel := context.WithCancel(context.Background())
	application := build(config.Config{
		Server: config.Server{Address: "127.0.0.1:0", ShutdownTimeout: 3 * time.Second},
		Security: config.Security{
			RegistrationToken: "0123456789abcdef0123456789abcdef",
			QueryToken:        "abcdef0123456789abcdef0123456789",
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), httpapi.AlwaysReady{}, appRegistry{}, appDiscovery{}, func() error {
		closed = true
		return nil
	}, cancel)
	if application.Server.Addr != "127.0.0.1:0" || application.Server.ReadHeaderTimeout == 0 {
		t.Fatalf("unexpected server: %#v", application.Server)
	}
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	application.Server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ready\"}" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if err := application.Close(); err != nil || !closed {
		t.Fatalf("close error = %v closed=%v", err, closed)
	}
	select {
	case <-root.Done():
	default:
		t.Fatal("application context was not cancelled")
	}
}

type appRegistry struct{}

func (appRegistry) Register(context.Context, []byte) (registry.Registration, bool, error) {
	return registry.Registration{}, false, nil
}

type appDiscovery struct{}

func (appDiscovery) Publish(context.Context, registry.AgentAddr, discovery.Snapshot) error {
	return nil
}
func (appDiscovery) Search(context.Context, discovery.Query) (discovery.Result, error) {
	return discovery.Result{}, nil
}
