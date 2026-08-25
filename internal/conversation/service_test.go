package conversation

import (
	"context"
	"errors"
	"testing"

	"github.com/re35t/AegisLink/internal/mcp"
)

func TestStartRunRejectsUnresolvedSelectionBeforeWritingMessageOrRun(t *testing.T) {
	t.Parallel()
	repository := &selectionRepository{}
	service := NewService(
		context.Background(), repository, nil, nil,
		selectionContext{err: mcp.ErrUnavailable}, nil,
	)
	_, _, err := service.StartRun(t.Context(), "principal-one", RunRequest{
		ConversationID: "conversation-one", MessageID: "message-one", RunID: "run-one", Content: "use it",
		Selection: &RunSelection{MentionID: "mcp-tool:opaque", Action: "force-tool-once"},
	})
	if !errors.Is(err, mcp.ErrUnavailable) {
		t.Fatalf("error = %v", err)
	}
	if repository.createCalled {
		t.Fatal("message/run was persisted before selection authorization completed")
	}
}

type selectionRepository struct {
	Repository
	createCalled bool
}

func (*selectionRepository) GetConversation(context.Context, string, string) (Detail, error) {
	return Detail{Conversation: Conversation{ID: "conversation-one", AgentID: "agent-one"}}, nil
}

func (repository *selectionRepository) CreateMessageRun(
	context.Context, string, string, string, string, string, ExecutionPolicy,
) (Message, Run, error) {
	repository.createCalled = true
	return Message{}, Run{}, nil
}

type selectionContext struct {
	ContextProvider
	err error
}

func (contextProvider selectionContext) ResolveSelection(context.Context, string, string, RunSelection) (ExecutionPolicy, error) {
	return ExecutionPolicy{}, contextProvider.err
}
