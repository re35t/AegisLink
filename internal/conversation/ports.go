package conversation

import (
	"context"

	"github.com/re35t/AegisLink/internal/agent"
)

// RunRequest contains transport-neutral identifiers and content for starting a
// run. HTTP adapters may supply protocol identifiers, while other callers can
// continue to let the service generate them.
type RunRequest struct {
	ConversationID string
	MessageID      string
	RunID          string
	Content        string
	Selection      *RunSelection
}

type RunSelection struct {
	MentionID string
	Action    string
}

type HarnessInput struct {
	RunID       string
	PrincipalID string
	Agent       agent.Agent
	Messages    []Message
	Policy      ExecutionPolicy
}

type HarnessOutput struct {
	Delta string
	Tool  *HarnessToolEvent
	Err   error
}

type HarnessToolEventType string

const (
	HarnessToolStarted   HarnessToolEventType = "started"
	HarnessToolCompleted HarnessToolEventType = "completed"
	HarnessToolFailed    HarnessToolEventType = "failed"
)

// HarnessToolEvent is the transport-neutral lifecycle emitted by the Agent
// Harness. Model SDK and AG-UI types must not cross this boundary.
type HarnessToolEvent struct {
	Type      HarnessToolEventType
	ID        string
	Name      string
	Arguments string
	Result    string
	Error     string
}

type Harness interface {
	Run(context.Context, HarnessInput) (<-chan HarnessOutput, error)
	ResolveSelection(context.Context, string, string, RunSelection) (ExecutionPolicy, error)
}

type AgentReader interface {
	Default(context.Context, string) (agent.Agent, error)
	Get(context.Context, string, string) (agent.Agent, error)
}

type Repository interface {
	Ping(context.Context) error
	RecoverInterruptedRuns(context.Context) error
	ListConversations(context.Context, string) ([]Conversation, error)
	CreateConversation(context.Context, string, string, string) (Conversation, error)
	GetConversation(context.Context, string, string) (Detail, error)
	CreateMessageRun(context.Context, string, string, string, string, string, ExecutionPolicy) (Message, Run, error)
	MarkRunRunning(context.Context, string, string) error
	AppendRunEvent(context.Context, string, string, string, any) (RunEvent, error)
	CompleteRun(context.Context, string, string, string, string, string) (Message, error)
	FailRun(context.Context, string, string, string, bool) error
	GetRun(context.Context, string, string) (Run, error)
	ListRunEvents(context.Context, string, string, int64) ([]RunEvent, error)
}
