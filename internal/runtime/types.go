package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrForcedToolNotCalled = errors.New("forced tool was not called")
	ErrForcedToolMismatch  = errors.New("model called a different tool than the forced tool")
)

type Runtime interface {
	Run(context.Context, Input) <-chan Event
}

type Input struct {
	ExecutionMode ExecutionMode
	Agent         Agent
	Instruction   string
	Messages      []Message
	Tools         []Tool
	ToolChoice    ToolChoice
}

// ExecutionMode controls whether the Runtime runs an Agent loop or one model completion.
// The zero value preserves the normal Agent loop.
type ExecutionMode string

const (
	ExecutionModeAgent      ExecutionMode = "agent"
	ExecutionModeSingleTurn ExecutionMode = "single-turn"
)

type Agent struct {
	Name        string
	Description string
}

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	Invoke      func(context.Context, string) (string, error)
}

type ToolChoiceMode string

const (
	ToolChoiceAuto      ToolChoiceMode = "auto"
	ToolChoiceForceOnce ToolChoiceMode = "force-once"
)

type ToolChoice struct {
	Mode              ToolChoiceMode
	Name              string
	ValidateArguments func(string) error
}

type Event struct {
	Delta string
	Tool  *ToolEvent
	Err   error
}

type ToolEventType string

const (
	ToolStarted   ToolEventType = "started"
	ToolCompleted ToolEventType = "completed"
	ToolFailed    ToolEventType = "failed"
)

type ToolEvent struct {
	Type      ToolEventType
	ID        string
	Name      string
	Arguments string
	Result    string
	Error     string
}

type ModelConfig struct {
	ID           string
	Driver       string
	BaseURL      string
	APIKey       string
	Name         string
	Timeout      time.Duration
	MaxTokens    int
	JSONOutput   bool
	ThinkingMode string
}

type Options struct {
	MaxIterations int
}
