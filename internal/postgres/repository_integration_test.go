package postgres

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/conversation"
)

func TestRepositoryConversationRunLifecycle(t *testing.T) {
	const ownerID = "01K34A00000000000000000000"
	const agentID = "01K34A00000000000000000001"

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		TRUNCATE account_sessions, user_accounts, run_events, runs, messages, conversations, agents, human_principals CASCADE`); err != nil {
		t.Fatal(err)
	}
	repository := NewConversationRepository(database)
	if _, err := database.ExecContext(t.Context(), `
		INSERT INTO human_principals (id, display_name) VALUES ($1, 'Test user')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		VALUES ($1, $2, 'Aegis', '', 'Test prompt')`, agentID, ownerID); err != nil {
		t.Fatal(err)
	}
	created, err := repository.CreateConversation(t.Context(), ownerID, agentID, "New conversation")
	if err != nil {
		t.Fatal(err)
	}
	message, run, err := repository.CreateMessageRun(t.Context(), ownerID, created.ID, "message-1", "run-1", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if message.Sequence != 1 || run.Status != "queued" {
		t.Fatalf("unexpected initial state: message=%#v run=%#v", message, run)
	}
	_, _, err = repository.CreateMessageRun(t.Context(), ownerID, created.ID, "message-2", "run-2", "duplicate")
	if !errors.Is(err, conversation.ErrActiveRun) {
		t.Fatalf("expected active run conflict, got %v", err)
	}
	if err := repository.MarkRunRunning(t.Context(), ownerID, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AppendRunEvent(t.Context(), ownerID, run.ID, "run.started", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	assistant, err := repository.CompleteRun(t.Context(), ownerID, run.ID, created.ID, "message-3", "hello back")
	if err != nil {
		t.Fatal(err)
	}
	if assistant.Sequence != 2 {
		t.Fatalf("assistant sequence = %d", assistant.Sequence)
	}
	events, err := repository.ListRunEvents(t.Context(), ownerID, run.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Sequence != 1 || events[1].Type != "message.completed" {
		t.Fatalf("unexpected events: %#v", events)
	}
	if _, err := repository.GetRun(t.Context(), "another-principal", run.ID); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatalf("run should not be visible across principals, got %v", err)
	}
	detail, err := repository.GetConversation(t.Context(), ownerID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Messages) != 2 || detail.ActiveRun != nil {
		t.Fatalf("unexpected detail: %#v", detail)
	}
}

func TestAccountRegistrationBindsPrincipalAgentAndSession(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		TRUNCATE account_sessions, user_accounts, run_events, runs, messages, conversations, agents, human_principals CASCADE`); err != nil {
		t.Fatal(err)
	}
	repository := NewAccountRepository(database)
	registration := account.Registration{
		User: account.User{ID: "principal-1", DisplayName: "Test user"},
		Account: account.Account{
			ID: "account-1", PrincipalID: "principal-1", Email: "test@example.com", PasswordHash: "encoded", Status: "active",
		},
		Agent: agent.Agent{
			ID: "agent-1", OwnerPrincipalID: "principal-1", Name: "Aegis", SystemPrompt: "Test prompt",
		},
	}
	identity, err := repository.CreateAccountWithAgent(t.Context(), registration)
	if err != nil {
		t.Fatal(err)
	}
	if identity.User.ID != registration.User.ID || identity.Agent.OwnerPrincipalID != registration.User.ID {
		t.Fatalf("unexpected identity: %#v", identity)
	}
	tokenHash := []byte("01234567890123456789012345678901")
	expiresAt := time.Now().Add(time.Hour)
	if err := repository.CreateSession(t.Context(), account.Session{
		ID: "session-1", AccountID: registration.Account.ID, TokenHash: tokenHash, ExpiresAt: expiresAt,
	}); err != nil {
		t.Fatal(err)
	}
	actor, err := repository.AuthenticateSession(t.Context(), tokenHash, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if actor.User.ID != registration.User.ID || actor.AccountID != registration.Account.ID {
		t.Fatalf("unexpected actor: %#v", actor)
	}
}
