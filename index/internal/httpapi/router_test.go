package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/re35t/AegisLink/index/internal/registry"
)

const testToken = "0123456789abcdef0123456789abcdef"

func TestHealthAndReadiness(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, test := range []struct {
		name       string
		readiness  Readiness
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "health", readiness: failingReadiness{}, path: "/healthz", wantStatus: http.StatusOK, wantBody: `"status":"ok"`},
		{name: "ready", readiness: AlwaysReady{}, path: "/readyz", wantStatus: http.StatusOK, wantBody: `"status":"ready"`},
		{name: "not ready", readiness: failingReadiness{}, path: "/readyz", wantStatus: http.StatusServiceUnavailable, wantBody: `"code":"not_ready"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			NewRouter(Dependencies{Readiness: test.readiness}, logger).ServeHTTP(response, request)
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestRegisterAgentAddrAndReplay(t *testing.T) {
	t.Parallel()
	service := &fakeRegistry{}
	router := testRouter(service)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, registrationRequest(t, `{}`, "registration-one"))
	if response.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") != "" {
		t.Fatalf("unexpected Location header = %q", response.Header().Get("Location"))
	}
	var created registry.Registration
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.SchemaVersion != registry.DraftSchemaVersion || created.AgentAddr != "agent_01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("created registration = %#v", created)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 || fields["schemaVersion"] == nil || fields["agentAddr"] == nil || fields["createdAt"] == nil {
		t.Fatalf("unexpected response fields = %v", fields)
	}

	replay := httptest.NewRecorder()
	router.ServeHTTP(replay, registrationRequest(t, `{}`, "registration-one"))
	if replay.Code != http.StatusOK || replay.Header().Get("Idempotency-Replayed") != "true" || replay.Body.String() != response.Body.String() {
		t.Fatalf("replay = %d headers=%v body=%s", replay.Code, replay.Header(), replay.Body.String())
	}
}

func TestRegistrationFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		authorize  bool
		key        string
		body       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{name: "missing bearer", key: "key", body: `{}`, wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "missing idempotency key", authorize: true, body: `{}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "unknown field", authorize: true, key: "key", body: `{"agentName":"A"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "null body", authorize: true, key: "key", body: `null`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "array body", authorize: true, key: "key", body: `[]`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "missing body", authorize: true, key: "key", body: ``, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "trailing value", authorize: true, key: "key", body: `{} {}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "body too large", authorize: true, key: "key", body: `{"` + strings.Repeat("x", maxRegistrationBodyBytes) + `":1}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "invalid domain request", authorize: true, key: "key", body: `{}`, serviceErr: registry.ErrInvalid, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "idempotency conflict", authorize: true, key: "key", body: `{}`, serviceErr: registry.ErrIdempotencyConflict, wantStatus: http.StatusConflict, wantCode: "idempotency_conflict"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeRegistry{registerError: test.serviceErr}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/registry/agents", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			if test.authorize {
				request.Header.Set("Authorization", "Bearer "+testToken)
			}
			if test.key != "" {
				request.Header.Set("Idempotency-Key", test.key)
			}
			response := httptest.NewRecorder()
			testRouter(service).ServeHTTP(response, request)
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestResolveAndDiscoveryRoutesRemainAbsent(t *testing.T) {
	t.Parallel()
	router := testRouter(&fakeRegistry{})
	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/registry/agents/agent_01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{method: http.MethodPost, path: "/api/v1/discovery/search"},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d", test.method, test.path, response.Code)
		}
	}
}

func registrationRequest(t *testing.T, body, key string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/registry/agents", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+testToken)
	request.Header.Set("Idempotency-Key", key)
	return request
}

func testRouter(service Registry) http.Handler {
	return NewRouter(Dependencies{
		Readiness: AlwaysReady{}, Registry: service, RegistrationToken: testToken,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

type fakeRegistry struct {
	registration  registry.Registration
	keyHash       []byte
	registerError error
}

func (service *fakeRegistry) Register(_ context.Context, keyHash []byte) (registry.Registration, bool, error) {
	if service.registerError != nil {
		return registry.Registration{}, false, service.registerError
	}
	if service.registration.AgentAddr != "" {
		if bytes.Equal(service.keyHash, keyHash) {
			return service.registration, true, nil
		}
		return registry.Registration{}, false, registry.ErrIdempotencyConflict
	}
	service.keyHash = append([]byte(nil), keyHash...)
	service.registration = registry.Registration{
		SchemaVersion: registry.DraftSchemaVersion,
		AgentAddr:     "agent_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		CreatedAt:     time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC),
	}
	return service.registration, false, nil
}

type failingReadiness struct{}

func (failingReadiness) Ready(context.Context) error { return errors.New("not ready") }
