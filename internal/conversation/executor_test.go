package conversation

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/re35t/AegisLink/internal/agent"
)

func TestExecuteMapsHarnessPreparationFailure(t *testing.T) {
	repository := &executorRepository{}
	service := NewService(
		context.Background(), repository, executorAgentReader{},
		executorHarness{err: errors.New("context unavailable")},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	service.active["run-one"] = func() {}
	service.execute(t.Context(), "principal-one", Run{ID: "run-one", ConversationID: "conversation-one"})
	if repository.failureCode != "agent_context_load_failed" || repository.cancelled {
		t.Fatalf("failure code=%q cancelled=%v", repository.failureCode, repository.cancelled)
	}
}

type executorRepository struct {
	Repository
	failureCode string
	cancelled   bool
}

func (*executorRepository) MarkRunRunning(context.Context, string, string) error { return nil }
func (*executorRepository) AppendRunEvent(context.Context, string, string, string, any) (RunEvent, error) {
	return RunEvent{}, nil
}
func (*executorRepository) GetConversation(context.Context, string, string) (Detail, error) {
	return Detail{Conversation: Conversation{ID: "conversation-one", AgentID: "agent-one"}}, nil
}
func (repository *executorRepository) FailRun(_ context.Context, _, _, code string, cancelled bool) error {
	repository.failureCode = code
	repository.cancelled = cancelled
	return nil
}

type executorAgentReader struct{ AgentReader }

func (executorAgentReader) Get(context.Context, string, string) (agent.Agent, error) {
	return agent.Agent{ID: "agent-one", Name: "Aegis"}, nil
}

type executorHarness struct {
	err error
}

func (executorHarness) ResolveSelection(context.Context, string, string, RunSelection) (ExecutionPolicy, error) {
	return ExecutionPolicy{}, nil
}
func (harness executorHarness) Run(context.Context, HarnessInput) (<-chan HarnessOutput, error) {
	return nil, harness.err
}
