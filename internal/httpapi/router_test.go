package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/conversation"
)

func TestErrorResponseIncludesStableCodeAndRequestID(t *testing.T) {
	t.Parallel()
	router := newTestRouter(&fakeService{conversationError: conversation.ErrNotFound}, ModelInfo{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/missing", nil)
	authorize(request)
	request.Header.Set("X-Request-ID", "request-123")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "resource_not_found" || body.RequestID != "request-123" {
		t.Fatalf("unexpected error response: %#v", body)
	}
}

func TestProtectedRouteRequiresSessionCookie(t *testing.T) {
	t.Parallel()
	router := newTestRouter(&fakeService{}, ModelInfo{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/bootstrap", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "authentication_required") {
		t.Fatalf("unexpected unauthenticated response: status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestRegisterSetsProtectedSessionCookie(t *testing.T) {
	t.Parallel()
	router := newTestRouter(&fakeService{}, ModelInfo{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{
		"displayName":"Test user","email":"test@example.com","password":"long-enough-password"
	}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "aegislink_session" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected session cookie: %#v", cookies)
	}
}

func TestRunEventsReplaysPersistedEvents(t *testing.T) {
	t.Parallel()
	payload := json.RawMessage(`{"delta":"hello"}`)
	service := &fakeService{
		run:    conversation.Run{ID: "run-1", Status: "succeeded"},
		events: []conversation.RunEvent{{RunID: "run-1", Sequence: 2, Type: "message.delta", Payload: payload, OccurredAt: time.Now()}},
	}
	router := newTestRouter(service, ModelInfo{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/runs/run-1/events", nil)
	authorize(request)
	request.Header.Set("Last-Event-ID", "1")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "id: 2\nevent: message.delta") {
		t.Fatalf("unexpected SSE response: status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestBootstrapExposesStableModelMetadata(t *testing.T) {
	t.Parallel()
	model := ModelInfo{
		ID: "deepseek-primary", Driver: "deepseek", Name: "deepseek-v4-flash", Capabilities: []string{"streaming", "tool-calling"},
	}
	router := newTestRouter(&fakeService{}, model)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/bootstrap", nil)
	authorize(request)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Model ModelInfo `json:"model"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Model.ID != model.ID || body.Model.Name != model.Name || len(body.Model.Capabilities) != 2 {
		t.Fatalf("unexpected model metadata: %#v", body.Model)
	}
}

func TestAGUIRunStreamsProtocolLifecycleAndTextEvents(t *testing.T) {
	t.Parallel()
	service := &fakeService{
		run: conversation.Run{ID: "run-client", ConversationID: "conversation-1", Status: "succeeded"},
		events: []conversation.RunEvent{
			{RunID: "run-client", Sequence: 1, Type: "run.started", Payload: json.RawMessage(`{}`)},
			{RunID: "run-client", Sequence: 2, Type: "tool.started", Payload: json.RawMessage(`{"toolCallId":"call-time","toolName":"get_current_time","arguments":"{\"timezone\":\"Asia/Shanghai\"}"}`)},
			{RunID: "run-client", Sequence: 3, Type: "tool.completed", Payload: json.RawMessage(`{"toolCallId":"call-time","result":"2026-08-23T10:00:00+08:00"}`)},
			{RunID: "run-client", Sequence: 4, Type: "message.delta", Payload: json.RawMessage(`{"delta":"hello"}`)},
			{RunID: "run-client", Sequence: 5, Type: "message.completed", Payload: json.RawMessage(`{}`)},
		},
	}
	router := newTestRouter(service, ModelInfo{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ag-ui", strings.NewReader(`{
		"threadId":"conversation-1",
		"runId":"run-client",
		"state":{},
		"messages":[{"id":"message-client","role":"user","content":"hi"}],
		"tools":[],
		"context":[],
		"forwardedProps":{}
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	authorize(request)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("unexpected AG-UI response: status=%d content-type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	if service.startRequest == nil || service.startRequest.MessageID != "message-client" || service.startRequest.Content != "hi" {
		t.Fatalf("unexpected start request: %#v", service.startRequest)
	}
	body := response.Body.String()
	wantInOrder := []string{
		"RUN_STARTED",
		"TOOL_CALL_START",
		"TOOL_CALL_ARGS",
		"TOOL_CALL_END",
		"TOOL_CALL_RESULT",
		"TEXT_MESSAGE_START",
		"TEXT_MESSAGE_CONTENT",
		"TEXT_MESSAGE_END",
		"RUN_FINISHED",
	}
	position := -1
	for _, value := range wantInOrder {
		next := strings.Index(body[position+1:], value)
		if next < 0 {
			t.Fatalf("AG-UI event %q missing from %q", value, body)
		}
		position += next + 1
	}
	if !strings.Contains(body, `"toolCallId":"call-time"`) || !strings.Contains(body, `"toolCallName":"get_current_time"`) {
		t.Fatalf("structured tool fields missing from %q", body)
	}
}

type fakeService struct {
	conversationError error
	run               conversation.Run
	events            []conversation.RunEvent
	startRequest      *conversation.RunRequest
}

func (fake *fakeService) Register(context.Context, string, string, string) (account.AuthResult, error) {
	return account.AuthResult{
		User: account.User{ID: "user-1", DisplayName: "Test user"}, Agent: agent.Agent{ID: "agent-1"}, SessionToken: "session-token",
	}, nil
}
func (fake *fakeService) Login(context.Context, string, string) (account.AuthResult, error) {
	return fake.Register(context.Background(), "", "", "")
}
func (fake *fakeService) Authenticate(_ context.Context, token string) (account.Actor, error) {
	if token != "session-token" {
		return account.Actor{}, account.ErrUnauthenticated
	}
	return account.Actor{AccountID: "account-1", User: account.User{ID: "user-1", DisplayName: "Test user"}}, nil
}
func (fake *fakeService) Logout(context.Context, string) error { return nil }
func (fake *fakeService) Ready(context.Context) error          { return nil }
func (fake *fakeService) Bootstrap(context.Context, string) (agent.Agent, error) {
	return agent.Agent{}, nil
}
func (fake *fakeService) List(context.Context, string) ([]agent.Agent, error) { return nil, nil }
func (fake *fakeService) ListConversations(context.Context, string) ([]conversation.Conversation, error) {
	return nil, nil
}
func (fake *fakeService) CreateConversation(context.Context, string, string) (conversation.Conversation, error) {
	return conversation.Conversation{}, nil
}
func (fake *fakeService) GetConversation(context.Context, string, string) (conversation.Detail, error) {
	return conversation.Detail{}, fake.conversationError
}
func (fake *fakeService) SendMessage(context.Context, string, string, string) (conversation.Message, conversation.Run, error) {
	return conversation.Message{}, conversation.Run{}, nil
}
func (fake *fakeService) StartRun(_ context.Context, _ string, request conversation.RunRequest) (conversation.Message, conversation.Run, error) {
	fake.startRequest = &request
	return conversation.Message{}, fake.run, nil
}
func (fake *fakeService) GetRun(context.Context, string, string) (conversation.Run, error) {
	if fake.run.ID == "" {
		return conversation.Run{}, conversation.ErrNotFound
	}
	return fake.run, nil
}
func (fake *fakeService) ListRunEvents(context.Context, string, string, int64) ([]conversation.RunEvent, error) {
	return fake.events, nil
}
func (fake *fakeService) CancelRun(context.Context, string, string) error { return nil }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestRouter(service *fakeService, model ModelInfo) http.Handler {
	return NewRouter(
		Dependencies{Accounts: service, Agents: service, Conversations: service},
		"http://127.0.0.1:5173",
		AuthConfig{CookieName: "aegislink_session"},
		model,
		discardLogger(),
	)
}

func authorize(request *http.Request) {
	request.AddCookie(&http.Cookie{Name: "aegislink_session", Value: "session-token"})
}
