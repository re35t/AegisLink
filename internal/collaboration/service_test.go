package collaboration

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/runtime"
)

const testAgentAddr = "agent_01ARZ3NDEKTSV4RRFFQ69G5FAV"

func TestRequestAssistanceCreatesScopedEncryptedSessionAfterEvaluation(t *testing.T) {
	repository := &collaborationRepositoryStub{policy: Policy{AgentID: "agent-b", OwnerPrincipalID: "owner-b", Enabled: true, Revision: 1, MaxSessionTTLSeconds: 1800, MaxRequestsPerHour: 10, MaxActiveSessions: 2}}
	evaluator := &runtimeStub{events: []runtime.Event{{Delta: `{"accepted":true,"decisionCode":"relevant_request","ttlSeconds":7200}`}}}
	service, err := NewService(repository, agentReaderStub{}, profileReaderStub{}, evaluator, "test-encryption-key")
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC) }

	request, err := service.RequestAssistance(t.Context(), "owner-a", "agent-a", testAgentAddr, "Review an API design", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if request.Status != "accepted" || request.SessionID == nil || repository.accepted.ID != *request.SessionID {
		t.Fatalf("request=%#v accepted=%#v", request, repository.accepted)
	}
	if repository.accepted.ExpiresAt.Sub(service.now()) != 30*time.Minute {
		t.Fatalf("TTL was not clamped: %v", repository.accepted.ExpiresAt)
	}
	if len(repository.accepted.TokenHash) != sha256.Size || len(repository.accepted.EncryptedToken) == 0 || len(repository.accepted.TokenNonce) == 0 {
		t.Fatalf("session capability was not protected: %#v", repository.accepted)
	}
	if len(evaluator.inputs) != 1 || evaluator.inputs[0].ExecutionMode != runtime.ExecutionModeSingleTurn || len(evaluator.inputs[0].Tools) != 0 {
		t.Fatalf("evaluation runtime input=%#v", evaluator.inputs)
	}
	if len(repository.invocations) != 1 || repository.invocations[0].Kind != "evaluation" || repository.finishedStatus != "succeeded" {
		t.Fatalf("invocation audit=%#v status=%q", repository.invocations, repository.finishedStatus)
	}
}

func TestRequestAssistanceRequiresOwnerOptInBeforeEvaluation(t *testing.T) {
	repository := &collaborationRepositoryStub{policy: Policy{Enabled: false}}
	evaluator := &runtimeStub{}
	service, err := NewService(repository, agentReaderStub{}, profileReaderStub{}, evaluator, "test-encryption-key")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.RequestAssistance(t.Context(), "owner-a", "agent-a", testAgentAddr, "help", "run-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error=%v", err)
	}
	if len(evaluator.inputs) != 0 {
		t.Fatal("evaluation ran before owner opt-in")
	}
}

func TestRequestAssistanceRechecksOwnerPolicyBeforeCreatingSession(t *testing.T) {
	repository := &collaborationRepositoryStub{
		policy:    Policy{AgentID: "agent-b", OwnerPrincipalID: "owner-b", Enabled: true, Revision: 1, MaxSessionTTLSeconds: 1800, MaxRequestsPerHour: 10, MaxActiveSessions: 2},
		acceptErr: ErrForbidden,
	}
	evaluator := &runtimeStub{events: []runtime.Event{{Delta: `{"accepted":true,"decisionCode":"relevant_request","ttlSeconds":900}`}}}
	service, err := NewService(repository, agentReaderStub{}, profileReaderStub{}, evaluator, "test-encryption-key")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.RequestAssistance(t.Context(), "owner-a", "agent-a", testAgentAddr, "Review an API design", "run-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error=%v", err)
	}
	if repository.finishedStatus != "failed" || repository.rejectedStatus != "failed" || repository.rejectedCode != "policy_changed" {
		t.Fatalf("finished=%q rejected=%q code=%q", repository.finishedStatus, repository.rejectedStatus, repository.rejectedCode)
	}
}

func TestOfficialA2AHandlerUsesSessionAuthAndPersistsTaskSemantics(t *testing.T) {
	token := "cls_test-capability"
	hash := sha256.Sum256([]byte(token))
	repository := &collaborationRepositoryStub{authenticated: Session{
		ID: "session-1", RequesterAgentID: "agent-a", TargetAgentID: "agent-b", TargetOwnerPrincipalID: "owner-b",
		TargetAgentAddr: testAgentAddr, Status: "active", Scopes: defaultScopes(), TokenHash: hash[:], ExpiresAt: time.Now().Add(time.Hour),
	}}
	agentRuntime := &runtimeStub{events: []runtime.Event{{Delta: "bounded answer"}}}
	service, err := NewService(repository, agentReaderStub{}, profileReaderStub{}, &runtimeStub{}, "test-encryption-key")
	if err != nil {
		t.Fatal(err)
	}
	executor := NewA2AExecutor(repository, agentReaderStub{}, profileReaderStub{}, agentRuntime)
	handler := a2asrv.NewHandler(executor, a2asrv.WithCallInterceptors(NewA2AAuthenticator(service)))
	ctx, _ := a2asrv.NewCallContext(t.Context(), a2asrv.NewServiceParams(map[string][]string{"Authorization": {"Bearer " + token}}))
	message := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("Need a concise answer"))
	message.ID = "message-1"
	message.ContextID = "session-1"
	result, err := handler.SendMessage(ctx, &a2a.SendMessageRequest{Tenant: testAgentAddr, Message: message})
	if err != nil {
		t.Fatal(err)
	}
	task, ok := result.(*a2a.Task)
	if !ok || task.Status.State != a2a.TaskStateCompleted || len(task.Artifacts) != 1 || task.Artifacts[0].Parts[0].Text() != "bounded answer" {
		t.Fatalf("A2A task=%#v", result)
	}
	if len(agentRuntime.inputs) != 1 || len(agentRuntime.inputs[0].Tools) != 0 {
		t.Fatalf("collaboration invocation exposed tools: %#v", agentRuntime.inputs)
	}
	if len(repository.invocations) != 1 || repository.invocations[0].Kind != "collaboration" || repository.finishedStatus != "succeeded" {
		t.Fatalf("invocation audit=%#v", repository.invocations)
	}
}

func TestOfficialA2AJSONRPCClientUsesPublishedCardAndSessionCapability(t *testing.T) {
	token := "cls_http-test-capability"
	hash := sha256.Sum256([]byte(token))
	repository := &collaborationRepositoryStub{
		policy: Policy{AgentID: "agent-b", OwnerPrincipalID: "owner-b", Enabled: true, Revision: 1, MaxSessionTTLSeconds: 1800, MaxRequestsPerHour: 10, MaxActiveSessions: 2},
		authenticated: Session{
			ID: "session-http", RequesterAgentID: "agent-a", TargetAgentID: "agent-b", TargetOwnerPrincipalID: "owner-b",
			TargetAgentAddr: testAgentAddr, Status: "active", Scopes: defaultScopes(), TokenHash: hash[:], ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	agentRuntime := &runtimeStub{events: []runtime.Event{{Delta: "official JSON-RPC answer"}}}
	service, err := NewService(repository, agentReaderStub{}, profileReaderStub{}, &runtimeStub{}, "test-encryption-key")
	if err != nil {
		t.Fatal(err)
	}
	executor := NewA2AExecutor(repository, agentReaderStub{}, profileReaderStub{}, agentRuntime)
	handler := a2asrv.NewHandler(executor, a2asrv.WithCallInterceptors(NewA2AAuthenticator(service)))
	server := httptest.NewServer(a2asrv.NewJSONRPCHandler(handler))
	defer server.Close()

	card, err := service.AgentCard(t.Context(), testAgentAddr, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(card)
	if err != nil || len(encoded) == 0 {
		t.Fatalf("marshal AgentCard: bytes=%q err=%v", encoded, err)
	}
	credentials := a2aclient.NewInMemoryCredentialsStore()
	sessionID := a2aclient.SessionID("client-session")
	credentials.Set(sessionID, a2a.SecuritySchemeName("collaborationSession"), a2aclient.AuthCredential(token))
	client, err := a2aclient.NewFromCard(t.Context(), &card, a2aclient.WithCallInterceptors(&a2aclient.AuthInterceptor{Service: credentials}))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Destroy()

	message := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("Use the official client"))
	message.ID = "message-http"
	message.ContextID = repository.authenticated.ID
	ctx := a2aclient.AttachSessionID(t.Context(), sessionID)
	result, err := client.SendMessage(ctx, &a2a.SendMessageRequest{Message: message})
	if err != nil {
		t.Fatal(err)
	}
	task, ok := result.(*a2a.Task)
	if !ok || task.Status.State != a2a.TaskStateCompleted || len(task.Artifacts) != 1 || task.Artifacts[0].Parts[0].Text() != "official JSON-RPC answer" {
		t.Fatalf("official HTTP client task=%#v", result)
	}
}

type runtimeStub struct {
	events []runtime.Event
	inputs []runtime.Input
}

func (stub *runtimeStub) Run(_ context.Context, input runtime.Input) <-chan runtime.Event {
	stub.inputs = append(stub.inputs, input)
	result := make(chan runtime.Event, len(stub.events))
	for _, event := range stub.events {
		result <- event
	}
	close(result)
	return result
}

type agentReaderStub struct{}

func (agentReaderStub) Get(_ context.Context, principalID, agentID string) (agent.Agent, error) {
	return agent.Agent{ID: agentID, OwnerPrincipalID: principalID, Name: "Agent " + agentID, Description: "API review", SystemPrompt: "Be careful."}, nil
}

type profileReaderStub struct{}

func (profileReaderStub) Get(_ context.Context, _, agentID string) (agent.Profile, error) {
	return agent.Profile{AgentID: agentID, Version: 2, Identity: agent.ProfileIdentity{Name: "Reviewer", Description: "Reviews APIs"}, Capabilities: []agent.ProfileCapability{}}, nil
}

type collaborationRepositoryStub struct {
	policy         Policy
	authenticated  Session
	accepted       Session
	invocations    []Invocation
	finishedStatus string
	request        *AssistanceRequest
	acceptErr      error
	rejectedStatus string
	rejectedCode   string
}

func (stub *collaborationRepositoryStub) ResolveTarget(context.Context, string) (Target, error) {
	return Target{AgentID: "agent-b", OwnerPrincipalID: "owner-b", AgentAddr: testAgentAddr}, nil
}
func (stub *collaborationRepositoryStub) GetPolicy(context.Context, string, string) (Policy, error) {
	return stub.policy, nil
}
func (stub *collaborationRepositoryStub) UpdatePolicy(context.Context, string, string, PolicyUpdate) (Policy, error) {
	return stub.policy, nil
}
func (stub *collaborationRepositoryStub) CreateRequest(_ context.Context, request AssistanceRequest, _ Policy) (AssistanceRequest, bool, error) {
	stub.request = &request
	return request, false, nil
}
func (stub *collaborationRepositoryStub) AcceptRequest(_ context.Context, _ AssistanceRequest, session Session) error {
	if stub.acceptErr != nil {
		return stub.acceptErr
	}
	stub.accepted = session
	return nil
}
func (stub *collaborationRepositoryStub) RejectRequest(_ context.Context, _ string, status, code string) error {
	stub.rejectedStatus, stub.rejectedCode = status, code
	return nil
}
func (stub *collaborationRepositoryStub) GetOwnedSession(context.Context, string, string, string) (Session, error) {
	return stub.authenticated, nil
}
func (stub *collaborationRepositoryStub) AuthenticateSession(_ context.Context, tokenHash []byte, addr, _ string) (Session, error) {
	if addr != stub.authenticated.TargetAgentAddr || string(tokenHash) != string(stub.authenticated.TokenHash) {
		return Session{}, ErrForbidden
	}
	return stub.authenticated, nil
}
func (stub *collaborationRepositoryStub) ListOverview(context.Context, string, string) (Overview, error) {
	return Overview{Policy: stub.policy, Requests: []AssistanceRequest{}, Sessions: []Session{}}, nil
}
func (stub *collaborationRepositoryStub) RevokeSession(context.Context, string, string, string) error {
	return nil
}
func (stub *collaborationRepositoryStub) StartInvocation(_ context.Context, invocation Invocation) error {
	stub.invocations = append(stub.invocations, invocation)
	return nil
}
func (stub *collaborationRepositoryStub) FinishInvocation(_ context.Context, _ string, status, _ string) error {
	stub.finishedStatus = status
	return nil
}
func (stub *collaborationRepositoryStub) LoadContextTasks(context.Context, string, int) ([]*a2a.Task, error) {
	return []*a2a.Task{}, nil
}
