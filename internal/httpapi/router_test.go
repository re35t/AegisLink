package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	internala2a "github.com/re35t/AegisLink/internal/a2a"
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

func TestImpressionRouteUsesOwnerSession(t *testing.T) {
	router := newTestRouter(&fakeService{}, ModelInfo{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-1/impressions?status=active", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-1/impressions?status=active", nil)
	authorize(request)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"impressions"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func TestPublicAgentFactsUsesRealHost(t *testing.T) {
	router := newTestRouter(&fakeService{}, ModelInfo{})
	request := httptest.NewRequest(http.MethodGet, "http://agent.example.test/.well-known/agentfacts.json", nil)
	request.Host = "agent.example.test"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("ETag") == "" {
		t.Fatalf("response = %d headers=%v", response.Code, response.Header())
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

func TestAccountSettingsAndPasswordEndpoints(t *testing.T) {
	t.Parallel()
	service := &fakeService{}
	router := newTestRouter(service, ModelInfo{})

	settingsRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/account/settings", strings.NewReader(`{
		"displayName":"Updated user","language":"zh-CN","theme":"dark"
	}`))
	settingsRequest.Header.Set("Content-Type", "application/json")
	authorize(settingsRequest)
	settingsResponse := httptest.NewRecorder()
	router.ServeHTTP(settingsResponse, settingsRequest)
	if settingsResponse.Code != http.StatusOK || service.settingsUpdate == nil || service.settingsUpdate.Language == nil || *service.settingsUpdate.Language != account.LanguageChinese {
		t.Fatalf("unexpected settings response: status=%d body=%q update=%#v", settingsResponse.Code, settingsResponse.Body.String(), service.settingsUpdate)
	}

	passwordRequest := httptest.NewRequest(http.MethodPost, "/api/v1/account/password", strings.NewReader(`{
		"currentPassword":"current-password","newPassword":"different-password"
	}`))
	passwordRequest.Header.Set("Content-Type", "application/json")
	authorize(passwordRequest)
	passwordResponse := httptest.NewRecorder()
	router.ServeHTTP(passwordResponse, passwordRequest)
	if passwordResponse.Code != http.StatusNoContent || service.currentPassword != "current-password" || service.newPassword != "different-password" {
		t.Fatalf("unexpected password response: status=%d body=%q", passwordResponse.Code, passwordResponse.Body.String())
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

func TestAgentProfileEndpointsReturnOwnerViewAndAcceptUpdates(t *testing.T) {
	t.Parallel()
	service := &fakeService{}
	router := newTestRouter(service, ModelInfo{})

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-1/profile", nil)
	authorize(getRequest)
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), `"agentId":"agent-1"`) || strings.Contains(getResponse.Body.String(), "systemPrompt") {
		t.Fatalf("unexpected profile response: status=%d body=%q", getResponse.Code, getResponse.Body.String())
	}

	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/agents/agent-1/profile", strings.NewReader(`{
		"expectedVersion":1,"name":"Research Agent","avatarUrl":"https://example.test/avatar.png"
	}`))
	patchRequest.Header.Set("Content-Type", "application/json")
	authorize(patchRequest)
	patchResponse := httptest.NewRecorder()
	router.ServeHTTP(patchResponse, patchRequest)
	if patchResponse.Code != http.StatusOK || service.profileUpdate == nil || service.profileUpdate.Name == nil || *service.profileUpdate.Name != "Research Agent" {
		t.Fatalf("unexpected profile update: status=%d body=%q update=%#v", patchResponse.Code, patchResponse.Body.String(), service.profileUpdate)
	}

	policyRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/agents/agent-1/profile/disclosure-policies", strings.NewReader(`{
		"expectedVersion":2,"changes":[{"subjectType":"identity","subjectId":"agent-1","policy":{"visibility":"public","channels":["agent-card"],"indexable":false,"audiences":[]}}]
	}`))
	policyRequest.Header.Set("Content-Type", "application/json")
	authorize(policyRequest)
	policyResponse := httptest.NewRecorder()
	router.ServeHTTP(policyResponse, policyRequest)
	if policyResponse.Code != http.StatusOK || len(service.policyChanges) != 1 || service.policyChanges[0].Policy.Visibility != agent.VisibilityPublic {
		t.Fatalf("unexpected policy update: status=%d body=%q changes=%#v", policyResponse.Code, policyResponse.Body.String(), service.policyChanges)
	}
}

func TestAgentProfileVersionConflictUsesStableError(t *testing.T) {
	t.Parallel()
	router := newTestRouter(&fakeService{profileError: agent.ErrProfileConflict}, ModelInfo{})
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/agents/agent-1/profile", strings.NewReader(`{"expectedVersion":1,"name":"Aegis"}`))
	request.Header.Set("Content-Type", "application/json")
	authorize(request)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "profile_version_conflict") {
		t.Fatalf("unexpected profile conflict: status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestImportSkillAcceptsMultipartBundle(t *testing.T) {
	t.Parallel()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("bundle", "custom.md")
	if err != nil {
		t.Fatal(err)
	}
	content := "---\nname: custom\ndescription: Custom imported Skill.\n---\n"
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("version", "1.2.0"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	skillService := &fakeSkillService{}
	router := newTestRouterWithSkills(&fakeService{}, ModelInfo{}, skillService)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agents/agent-1/skills/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	authorize(request)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if skillService.imported == nil || skillService.imported.FileName != "custom.md" || skillService.imported.Version != "1.2.0" || string(skillService.imported.Data) != content {
		t.Fatalf("unexpected import request: %#v", skillService.imported)
	}
}

func TestMentionCatalogReturnsDistinctCapabilityKinds(t *testing.T) {
	t.Parallel()
	router := newTestRouter(&fakeService{}, ModelInfo{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-1/mentions?kinds=mcp-tool,skill,discovery", nil)
	authorize(request)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	var page catalog.Page
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 3 || page.Items[0].Kind != "mcp-tool" || page.Items[1].Kind != "skill" || page.Items[2].Kind != "discovery" {
		t.Fatalf("items = %#v", page.Items)
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
		"forwardedProps":{"aegislink":{"selection":{"mentionId":"opaque-mention","action":"force-tool-once"}}}
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	authorize(request)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("unexpected AG-UI response: status=%d content-type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	if service.startRequest == nil || service.startRequest.MessageID != "message-client" || service.startRequest.Content != "hi" ||
		service.startRequest.Selection == nil || service.startRequest.Selection.MentionID != "opaque-mention" ||
		service.startRequest.Selection.Action != "force-tool-once" {
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
	profileError      error
	run               conversation.Run
	events            []conversation.RunEvent
	startRequest      *conversation.RunRequest
	settingsUpdate    *account.SettingsUpdate
	currentPassword   string
	newPassword       string
	profileUpdate     *agent.ProfileUpdate
	policyChanges     []agent.PolicyChange
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
	return account.Actor{AccountID: "account-1", SessionID: "session-1", User: account.User{ID: "user-1", DisplayName: "Test user"}}, nil
}
func (fake *fakeService) Logout(context.Context, string) error { return nil }
func (fake *fakeService) GetSettings(context.Context, account.Actor) (account.Settings, error) {
	return account.Settings{
		Account:     account.AccountSummary{Email: "test@example.com", Status: "active"},
		User:        account.User{ID: "user-1", DisplayName: "Test user"},
		Preferences: account.Preferences{Language: account.LanguageSystem, Theme: account.ThemeSystem},
	}, nil
}
func (fake *fakeService) UpdateSettings(_ context.Context, _ account.Actor, update account.SettingsUpdate) (account.Settings, error) {
	fake.settingsUpdate = &update
	settings, _ := fake.GetSettings(context.Background(), account.Actor{})
	if update.DisplayName != nil {
		settings.User.DisplayName = *update.DisplayName
	}
	if update.Language != nil {
		settings.Preferences.Language = *update.Language
	}
	if update.Theme != nil {
		settings.Preferences.Theme = *update.Theme
	}
	return settings, nil
}
func (fake *fakeService) ChangePassword(_ context.Context, _ account.Actor, currentPassword, newPassword string) error {
	fake.currentPassword = currentPassword
	fake.newPassword = newPassword
	return nil
}
func (fake *fakeService) Ready(context.Context) error { return nil }
func (fake *fakeService) Bootstrap(context.Context, string) (agent.Agent, error) {
	return agent.Agent{}, nil
}
func (fake *fakeService) List(context.Context, string) ([]agent.Agent, error) { return nil, nil }
func (fake *fakeService) Get(context.Context, string, string) (agent.Profile, error) {
	if fake.profileError != nil {
		return agent.Profile{}, fake.profileError
	}
	return testAgentProfile(), nil
}
func (fake *fakeService) Update(_ context.Context, _, _ string, update agent.ProfileUpdate) (agent.Profile, error) {
	if fake.profileError != nil {
		return agent.Profile{}, fake.profileError
	}
	fake.profileUpdate = &update
	profile := testAgentProfile()
	profile.Version++
	return profile, nil
}
func (fake *fakeService) UpdatePolicies(_ context.Context, _, _ string, _ int64, changes []agent.PolicyChange) (agent.Profile, error) {
	if fake.profileError != nil {
		return agent.Profile{}, fake.profileError
	}
	fake.policyChanges = changes
	profile := testAgentProfile()
	profile.Version++
	return profile, nil
}
func (fake *fakeService) ConfirmCandidate(context.Context, string, string, string, int64, int64, agent.ConfirmFactUpdate) (agent.Profile, error) {
	profile := testAgentProfile()
	profile.Version++
	return profile, fake.profileError
}
func (fake *fakeService) RevokeFact(context.Context, string, string, string, int64) (agent.Profile, error) {
	profile := testAgentProfile()
	profile.Version++
	return profile, fake.profileError
}
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

type fakeMemoryService struct{}

func (*fakeMemoryService) List(context.Context, string, string) ([]memory.Memory, error) {
	return nil, nil
}
func (*fakeMemoryService) Create(context.Context, string, string, memory.Kind, string, string, float64) (memory.Memory, error) {
	return memory.Memory{}, nil
}
func (*fakeMemoryService) Update(context.Context, string, string, string, memory.Update) (memory.Memory, error) {
	return memory.Memory{}, nil
}
func (*fakeMemoryService) Forget(context.Context, string, string, string) error { return nil }

type fakeSkillService struct {
	imported *skills.ImportRequest
}

func (*fakeSkillService) List(context.Context, string, string) ([]skills.Skill, error) {
	return nil, nil
}
func (*fakeSkillService) Install(context.Context, string, string, string, string) (skills.Skill, error) {
	return skills.Skill{}, nil
}
func (fake *fakeSkillService) Import(_ context.Context, _, _ string, request skills.ImportRequest) (skills.Skill, error) {
	fake.imported = &request
	return skills.Skill{Name: "custom", Version: request.Version, Files: []skills.File{}}, nil
}
func (*fakeSkillService) SetEnabled(context.Context, string, string, string, bool) (skills.Skill, error) {
	return skills.Skill{}, nil
}
func (*fakeSkillService) Uninstall(context.Context, string, string, string) error { return nil }

type fakeMCPService struct{}

func (*fakeMCPService) ListLibrary(context.Context, string) ([]mcp.Server, error) {
	return nil, nil
}
func (*fakeMCPService) List(context.Context, string, string) ([]mcp.Server, error) {
	return nil, nil
}
func (*fakeMCPService) Create(context.Context, string, string, string, string) (mcp.Server, error) {
	return mcp.Server{}, nil
}
func (*fakeMCPService) Update(context.Context, string, string, string, string) (mcp.Server, error) {
	return mcp.Server{}, nil
}
func (*fakeMCPService) BindServer(context.Context, string, string, string) (mcp.Server, error) {
	return mcp.Server{}, nil
}
func (*fakeMCPService) UnbindServer(context.Context, string, string, string) error { return nil }
func (*fakeMCPService) Delete(context.Context, string, string) error               { return nil }
func (*fakeMCPService) Refresh(context.Context, string, string) (mcp.Server, error) {
	return mcp.Server{}, nil
}
func (*fakeMCPService) UpdateToolRisk(context.Context, string, string, string, mcp.RiskLevel) (mcp.Server, error) {
	return mcp.Server{}, nil
}
func (*fakeMCPService) BindTool(context.Context, string, string, string) (mcp.Tool, error) {
	return mcp.Tool{}, nil
}
func (*fakeMCPService) UnbindTool(context.Context, string, string, string) error { return nil }
func (*fakeMCPService) Mentions(context.Context, string, string, mcp.MentionQuery) (mcp.MentionPage, error) {
	return mcp.MentionPage{}, nil
}

type fakeCatalogService struct{}

func (*fakeCatalogService) List(context.Context, string, string, catalog.Query) (catalog.Page, error) {
	return catalog.Page{Items: []catalog.Item{
		{ID: "mcp-tool:one", Kind: "mcp-tool", Category: "mcp", Action: "force-tool-once", Availability: "ready", ResourceID: "one"},
		{ID: "skill:one", Kind: "skill", Category: "skills", Action: "use-skill-once", Availability: "ready", ResourceID: "one"},
		{ID: catalog.DiscoveryMentionID, Kind: "discovery", Category: "discovery", Action: "discover-once", Availability: "ready", ResourceID: catalog.DiscoveryResourceID},
	}}, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestRouter(service *fakeService, model ModelInfo) http.Handler {
	return newTestRouterWithSkills(service, model, &fakeSkillService{})
}

func newTestRouterWithSkills(service *fakeService, model ModelInfo, skillService SkillService) http.Handler {
	return NewRouter(
		Dependencies{
			Accounts: service, Agents: service, Profiles: service, Conversations: service,
			Impressions: &fakeImpressionService{}, Discovery: &fakeDiscoveryService{}, AgentCards: &fakeAgentCardService{},
			Memories: &fakeMemoryService{}, Skills: skillService, MCP: &fakeMCPService{}, Catalog: &fakeCatalogService{},
		},
		"http://127.0.0.1:5173",
		AuthConfig{CookieName: "aegislink_session"},
		model,
		discardLogger(),
	)
}

type fakeImpressionService struct{}

func (*fakeImpressionService) List(context.Context, string, string, string) ([]impression.Impression, error) {
	return []impression.Impression{}, nil
}
func (*fakeImpressionService) ListCandidates(context.Context, string, string, string) ([]impression.FactCandidate, error) {
	return []impression.FactCandidate{}, nil
}
func (*fakeImpressionService) Update(context.Context, string, string, string, impression.Update) (impression.Impression, error) {
	return impression.Impression{}, nil
}
func (*fakeImpressionService) RejectCandidate(context.Context, string, string, string, int64) error {
	return nil
}

type fakeDiscoveryService struct{}

func (*fakeDiscoveryService) Get(context.Context, string, string) (discovery.Publication, error) {
	return discovery.Publication{Tokens: []discovery.AccessToken{}}, nil
}
func (*fakeDiscoveryService) Update(context.Context, string, string, discovery.SettingsUpdate) (discovery.Publication, error) {
	return discovery.Publication{}, nil
}
func (*fakeDiscoveryService) VerifyHostname(context.Context, string, string) (discovery.Publication, error) {
	return discovery.Publication{}, nil
}
func (*fakeDiscoveryService) RotateKey(context.Context, string, string) (discovery.Publication, error) {
	return discovery.Publication{}, nil
}
func (*fakeDiscoveryService) CreateToken(context.Context, string, string, discovery.TokenRequest) (discovery.CreatedAccessToken, error) {
	return discovery.CreatedAccessToken{}, nil
}
func (*fakeDiscoveryService) RevokeToken(context.Context, string, string, string) error { return nil }
func (*fakeDiscoveryService) PublicDocument(context.Context, string) (discovery.Document, string, int64, error) {
	return discovery.Document{SchemaVersion: "aegislink.agent-facts/1.0-draft", Services: []map[string]any{}, Claims: []discovery.Claim{}}, "sha256:test", 3600, nil
}
func (*fakeDiscoveryService) Query(context.Context, string, string, discovery.QueryRequest) (discovery.Document, error) {
	return discovery.Document{}, nil
}
func (*fakeDiscoveryService) JWKS(context.Context, string) (map[string]any, error) {
	return map[string]any{"keys": []any{}}, nil
}
func (*fakeDiscoveryService) Revocations(context.Context, string) (map[string]any, error) {
	return map[string]any{"publicationIds": []string{}, "keyIds": []string{}}, nil
}

type fakeAgentCardService struct{}

func (*fakeAgentCardService) Preview(context.Context, string, string) (internala2a.Preview, error) {
	return internala2a.Preview{}, nil
}

func testAgentProfile() agent.Profile {
	now := time.Now()
	return agent.Profile{
		AgentID: "agent-1", Version: 1, CreatedAt: now, UpdatedAt: now,
		Identity: agent.ProfileIdentity{
			ID: "agent-1", Name: "Aegis", Description: "Personal Agent", HumanLinked: true,
			Disclosure: agent.DefaultDisclosurePolicy(),
		},
		Capabilities: []agent.ProfileCapability{}, ConfirmedFacts: []agent.ConfirmedFact{}, Impressions: []impression.Impression{},
	}
}

func authorize(request *http.Request) {
	request.AddCookie(&http.Cookie{Name: "aegislink_session", Value: "session-token"})
}
