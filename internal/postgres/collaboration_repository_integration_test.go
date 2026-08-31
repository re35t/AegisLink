package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/collaboration"
	"github.com/re35t/AegisLink/internal/runtime"
)

func TestCollaborationRepositoryAndA2ATaskStoreLifecycle(t *testing.T) {
	const (
		ownerA    = "01K3COLLAB0000000000000001"
		ownerB    = "01K3COLLAB0000000000000002"
		agentA    = "01K3COLLAB0000000000000011"
		agentB    = "01K3COLLAB0000000000000012"
		agentAddr = "agent_01ARZ3NDEKTSV4RRFFQ69G5FAV"
	)
	database, err := Open(t.Context(), testDatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestDatabase(t, database)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, database, `TRUNCATE account_sessions, user_accounts, agents, human_principals CASCADE`)
	mustExec(t, database, `INSERT INTO human_principals (id, display_name) VALUES (@p1, 'A'), (@p2, 'B')`, ownerA, ownerB)
	mustExec(t, database, `INSERT INTO agents (id, owner_principal_id, name, description, system_prompt) VALUES (@p1, @p2, 'A', '', 'A prompt'), (@p3, @p4, 'B', '', 'B prompt')`, agentA, ownerA, agentB, ownerB)
	mustExec(t, database, `INSERT INTO agent_profiles (agent_id, owner_principal_id) VALUES (@p1, @p2), (@p3, @p4)`, agentA, ownerA, agentB, ownerB)
	mustExec(t, database, `INSERT INTO agent_index_states (agent_id, owner_principal_id, agent_addr) VALUES (@p1, @p2, @p3)`, agentB, ownerB, agentAddr)

	repository := NewCollaborationRepository(database)
	policy, err := repository.UpdatePolicy(t.Context(), ownerB, agentB, collaboration.PolicyUpdate{ExpectedRevision: 1, Enabled: true, MaxSessionTTLSeconds: 1800, MaxRequestsPerHour: 10, MaxActiveSessions: 3})
	if err != nil || !policy.Enabled || policy.Revision != 2 {
		t.Fatalf("policy=%#v err=%v", policy, err)
	}
	target, err := repository.ResolveTarget(t.Context(), agentAddr)
	if err != nil || target.AgentID != agentB || target.OwnerPrincipalID != ownerB {
		t.Fatalf("target=%#v err=%v", target, err)
	}

	now := time.Now().UTC()
	request := collaboration.AssistanceRequest{ID: "request-1", RequesterOwnerPrincipalID: ownerA, RequesterAgentID: agentA, TargetOwnerPrincipalID: ownerB, TargetAgentID: agentB, TargetAgentAddr: agentAddr, Purpose: "review", RequestedScopes: []string{collaboration.ScopeMessageSend}, IdempotencyKeyHash: []byte("idempotency-hash"), RequestDigest: []byte("request-digest"), Status: "evaluating", DecisionCode: "accepted", CreatedAt: now}
	created, replayed, err := repository.CreateRequest(t.Context(), request, policy)
	if err != nil || replayed || created.ID != request.ID {
		t.Fatalf("request=%#v replayed=%v err=%v", created, replayed, err)
	}
	replayedRequest, replayed, err := repository.CreateRequest(t.Context(), request, policy)
	if err != nil || !replayed || replayedRequest.ID != request.ID {
		t.Fatalf("replay=%#v replayed=%v err=%v", replayedRequest, replayed, err)
	}

	session := collaboration.Session{ID: "session-1", AssistanceRequestID: request.ID, RequesterOwnerPrincipalID: ownerA, RequesterAgentID: agentA, TargetOwnerPrincipalID: ownerB, TargetAgentID: agentB, TargetAgentAddr: agentAddr, Status: "active", Scopes: []string{collaboration.ScopeMessageSend, collaboration.ScopeTaskGet}, TokenHash: []byte("12345678901234567890123456789012"), EncryptedToken: []byte("encrypted"), TokenNonce: []byte("nonce"), ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	if err := repository.AcceptRequest(t.Context(), request, session); err != nil {
		t.Fatal(err)
	}
	overview, err := repository.ListOverview(t.Context(), ownerB, agentB)
	if err != nil || len(overview.Requests) != 1 || len(overview.Sessions) != 1 || overview.Requests[0].Status != "accepted" {
		t.Fatalf("overview=%#v err=%v", overview, err)
	}

	store := NewCollaborationTaskStore(database)
	ctx, scope := a2asrv.NewCallContext(t.Context(), a2asrv.NewServiceParams(nil))
	scope.User = a2asrv.NewAuthenticatedUser(session.ID, nil)
	task := &a2a.Task{ID: "task-1", ContextID: session.ID, Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted}}
	version, err := store.Create(ctx, task)
	if err != nil || version != 1 {
		t.Fatalf("create task version=%d err=%v", version, err)
	}
	task.Status.State = a2a.TaskStateCompleted
	version, err = store.Update(ctx, &taskstore.UpdateRequest{Task: task, PrevVersion: version})
	if err != nil || version != 2 {
		t.Fatalf("update task version=%d err=%v", version, err)
	}
	stored, err := store.Get(ctx, task.ID)
	if err != nil || stored.Task.Status.State != a2a.TaskStateCompleted || stored.User != session.ID {
		t.Fatalf("stored=%#v err=%v", stored, err)
	}

	request2 := request
	request2.ID = "request-2"
	request2.IdempotencyKeyHash = []byte("idempotency-hash-2")
	request2.RequestDigest = []byte("request-digest-2")
	request2.Status = "evaluating"
	request2.DecisionCode = "accepted"
	if _, replayed, err := repository.CreateRequest(t.Context(), request2, policy); err != nil || replayed {
		t.Fatalf("second request replayed=%v err=%v", replayed, err)
	}
	if _, err := repository.UpdatePolicy(t.Context(), ownerB, agentB, collaboration.PolicyUpdate{ExpectedRevision: 2, Enabled: false, MaxSessionTTLSeconds: 1800, MaxRequestsPerHour: 10, MaxActiveSessions: 3}); err != nil {
		t.Fatal(err)
	}
	session2 := session
	session2.ID = "session-2"
	session2.AssistanceRequestID = request2.ID
	session2.TokenHash = []byte("abcdefghijklmnopqrstuvwxyz123456")
	if err := repository.AcceptRequest(t.Context(), request2, session2); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("accept after owner opt-out error=%v", err)
	}
}

func TestCollaborationEndToEndWithOfficialA2ATask(t *testing.T) {
	const (
		ownerA    = "01K3COLLABE2E000000000001"
		ownerB    = "01K3COLLABE2E000000000002"
		agentA    = "01K3COLLABE2E000000000011"
		agentB    = "01K3COLLABE2E000000000012"
		agentAddr = "agent_01ARZ3NDEKTSV4RRFFQ69G5FB0"
	)
	database, err := Open(t.Context(), testDatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestDatabase(t, database)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, database, `TRUNCATE account_sessions, user_accounts, agents, human_principals CASCADE`)
	mustExec(t, database, `INSERT INTO human_principals (id, display_name) VALUES (@p1, 'Requester Owner'), (@p2, 'Target Owner')`, ownerA, ownerB)
	mustExec(t, database, `INSERT INTO agents (id, owner_principal_id, name, description, system_prompt) VALUES (@p1, @p2, 'Requester', '', 'Coordinate safely.'), (@p3, @p4, 'Piano Mentor', 'Piano specialist', 'Give concise piano advice.')`, agentA, ownerA, agentB, ownerB)
	mustExec(t, database, `INSERT INTO agent_profiles (agent_id, owner_principal_id) VALUES (@p1, @p2), (@p3, @p4)`, agentA, ownerA, agentB, ownerB)
	mustExec(t, database, `INSERT INTO agent_index_states (agent_id, owner_principal_id, agent_addr) VALUES (@p1, @p2, @p3)`, agentB, ownerB, agentAddr)

	repository := NewCollaborationRepository(database)
	if _, err := repository.UpdatePolicy(t.Context(), ownerB, agentB, collaboration.PolicyUpdate{
		ExpectedRevision: 1, Enabled: true, MaxSessionTTLSeconds: 1800, MaxRequestsPerHour: 10, MaxActiveSessions: 3,
	}); err != nil {
		t.Fatal(err)
	}
	evaluator := &collaborationRuntimeStub{events: []runtime.Event{{Delta: `{"accepted":true,"decisionCode":"piano_request_accepted","ttlSeconds":900}`}}}
	responder := &collaborationRuntimeStub{events: []runtime.Event{{Delta: "Practice scales slowly, use a metronome, and review one short piece daily."}}}
	agentReader := collaborationAgentReaderStub{}
	profileReader := collaborationProfileReaderStub{}
	service, err := collaboration.NewService(repository, agentReader, profileReader, evaluator, "integration-test-collaboration-key")
	if err != nil {
		t.Fatal(err)
	}
	executor := collaboration.NewA2AExecutor(repository, agentReader, profileReader, responder)
	handler := a2asrv.NewHandler(
		executor,
		a2asrv.WithTaskStore(NewCollaborationTaskStore(database)),
		a2asrv.WithCallInterceptors(collaboration.NewA2AAuthenticator(service)),
	)
	service.WithA2AHandler(handler)

	request, err := service.RequestAssistance(t.Context(), ownerA, agentA, agentAddr, "Provide safe text-only piano practice advice", "e2e-request-1")
	if err != nil {
		t.Fatal(err)
	}
	if request.Status != "accepted" || request.SessionID == nil {
		t.Fatalf("assistance request=%#v", request)
	}
	task, err := service.SendMessage(t.Context(), ownerA, agentA, *request.SessionID, "Recommend a beginner piano practice plan", "e2e-message-1")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for task.Status.State != a2a.TaskStateCompleted && task.Status.State != a2a.TaskStateFailed && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		task, err = service.GetTask(t.Context(), ownerA, agentA, *request.SessionID, string(task.ID))
		if err != nil {
			t.Fatal(err)
		}
	}
	if task.Status.State != a2a.TaskStateCompleted || len(task.Artifacts) != 1 || task.Artifacts[0].Parts[0].Text() == "" {
		t.Fatalf("A2A task=%#v", task)
	}
	var audits []struct {
		Kind   string
		Status string
	}
	if result := database.connection.Table("collaboration_invocations").Select("kind, status").Order("started_at").Scan(&audits); result.Error != nil {
		t.Fatal(result.Error)
	}
	if len(audits) != 2 || audits[0].Kind != "evaluation" || audits[0].Status != "succeeded" || audits[1].Kind != "collaboration" || audits[1].Status != "succeeded" {
		t.Fatalf("invocation audits=%#v", audits)
	}
}

type collaborationRuntimeStub struct {
	events []runtime.Event
}

func (stub *collaborationRuntimeStub) Run(_ context.Context, _ runtime.Input) <-chan runtime.Event {
	output := make(chan runtime.Event, len(stub.events))
	for _, event := range stub.events {
		output <- event
	}
	close(output)
	return output
}

type collaborationAgentReaderStub struct{}

func (collaborationAgentReaderStub) Get(_ context.Context, principalID, agentID string) (agent.Agent, error) {
	return agent.Agent{ID: agentID, OwnerPrincipalID: principalID, Name: "Piano Mentor", Description: "Piano specialist", SystemPrompt: "Give concise piano advice."}, nil
}

type collaborationProfileReaderStub struct{}

func (collaborationProfileReaderStub) Get(_ context.Context, _, agentID string) (agent.Profile, error) {
	return agent.Profile{AgentID: agentID, Version: 2, Identity: agent.ProfileIdentity{Name: "Piano Mentor", Description: "Piano specialist"}}, nil
}
