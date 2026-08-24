package conversation

import (
	"context"
	"encoding/json"

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
}

type RuntimeInput struct {
	Agent    agent.Agent
	Messages []Message
	Context  AgentContext
}

type AgentContext struct {
	Memories []RuntimeMemory
	Skills   []RuntimeSkill
	Tools    []RuntimeTool
}

type RuntimeMemory struct {
	ID      string
	Kind    string
	Content string
	Source  string
}

type RuntimeSkill struct {
	Name        string
	Description string
	Content     string
}

type RuntimeTool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	Invoke      func(context.Context, string) (string, error)
}

type RuntimeOutput struct {
	Delta string
	Tool  *RuntimeToolEvent
	Err   error
}

type RuntimeToolEventType string

const (
	RuntimeToolStarted   RuntimeToolEventType = "started"
	RuntimeToolCompleted RuntimeToolEventType = "completed"
	RuntimeToolFailed    RuntimeToolEventType = "failed"
)

// RuntimeToolEvent is the transport-neutral lifecycle emitted by a Runtime.
// Model SDK and AG-UI types must not cross this boundary.
type RuntimeToolEvent struct {
	Type      RuntimeToolEventType
	ID        string
	Name      string
	Arguments string
	Result    string
	Error     string
}

type Runtime interface {
	Stream(context.Context, RuntimeInput) <-chan RuntimeOutput
}

type ContextProvider interface {
	Resolve(context.Context, string, string) (AgentContext, error)
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
	CreateMessageRun(context.Context, string, string, string, string, string) (Message, Run, error)
	MarkRunRunning(context.Context, string, string) error
	AppendRunEvent(context.Context, string, string, string, any) (RunEvent, error)
	CompleteRun(context.Context, string, string, string, string, string) (Message, error)
	FailRun(context.Context, string, string, string, bool) error
	GetRun(context.Context, string, string) (Run, error)
	ListRunEvents(context.Context, string, string, int64) ([]RunEvent, error)
}
