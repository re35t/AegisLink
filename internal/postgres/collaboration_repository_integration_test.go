package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
	"github.com/re35t/AegisLink/internal/collaboration"
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
