package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type Eino struct {
	model         model.ToolCallingChatModel
	maxIterations int
}

func New(ctx context.Context, modelConfig ModelConfig, options Options) (*Eino, error) {
	return NewWithRegistry(ctx, modelConfig, options, DefaultModelRegistry())
}

func NewWithRegistry(ctx context.Context, modelConfig ModelConfig, options Options, registry *ModelRegistry) (*Eino, error) {
	if registry == nil {
		return nil, errors.New("model registry is required")
	}
	chatModel, err := registry.NewModel(ctx, modelConfig)
	if err != nil {
		return nil, err
	}
	return NewWithModel(chatModel, options), nil
}

func NewWithModel(chatModel model.ToolCallingChatModel, options ...Options) *Eino {
	resolved := Options{MaxIterations: 8}
	if len(options) > 0 {
		resolved = options[0]
		if resolved.MaxIterations < 1 {
			resolved.MaxIterations = 8
		}
	}
	return &Eino{model: chatModel, maxIterations: resolved.MaxIterations}
}

func (runtime *Eino) Run(ctx context.Context, input Input) <-chan Event {
	output := make(chan Event)
	go func() {
		defer close(output)
		if input.ExecutionMode == ExecutionModeSingleTurn {
			runtime.runSingleTurn(ctx, input, output)
			return
		}
		pendingTools := make(map[string]string)
		tools, err := einoTools(input.Tools)
		if err != nil {
			send(ctx, output, Event{Err: fmt.Errorf("resolve runtime tools: %w", err)})
			return
		}
		runModel := runtime.model
		forcedDecisionPending := input.ToolChoice.Mode == ToolChoiceForceOnce
		if forcedDecisionPending {
			if input.ToolChoice.Name == "" {
				send(ctx, output, Event{Err: ErrForcedToolNotCalled})
				return
			}
			runModel = newForceOnceModel(runtime.model, input.ToolChoice.Name)
		}
		agentRuntime, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
			Name:          input.Agent.Name,
			Description:   input.Agent.Description,
			Instruction:   input.Instruction,
			Model:         runModel,
			ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
			MaxIterations: runtime.maxIterations,
		})
		if err != nil {
			send(ctx, output, Event{Err: fmt.Errorf("create agent: %w", err)})
			return
		}
		runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agentRuntime, EnableStreaming: true})
		iterator := runner.Run(ctx, einoMessages(input.Messages))
		for {
			event, ok := iterator.Next()
			if !ok {
				if forcedDecisionPending {
					send(ctx, output, Event{Err: ErrForcedToolNotCalled})
				}
				return
			}
			if event.Err != nil {
				emitPendingToolFailures(ctx, output, pendingTools, event.Err)
				send(ctx, output, Event{Err: event.Err})
				return
			}
			if event.Output == nil || event.Output.MessageOutput == nil {
				continue
			}
			variant := event.Output.MessageOutput
			var message *schema.Message
			if variant.IsStreaming {
				message, err = streamMessage(ctx, variant.MessageStream, output, variant.Role == schema.Assistant && !forcedDecisionPending)
				if err != nil {
					emitPendingToolFailures(ctx, output, pendingTools, err)
					send(ctx, output, Event{Err: err})
					return
				}
			} else {
				message = variant.Message
				if message != nil && variant.Role == schema.Assistant && !forcedDecisionPending && message.Content != "" && !send(ctx, output, Event{Delta: message.Content}) {
					return
				}
			}
			if message == nil {
				continue
			}
			switch variant.Role {
			case schema.Assistant:
				if forcedDecisionPending {
					forcedDecisionPending = false
					if len(message.ToolCalls) == 0 {
						send(ctx, output, Event{Err: ErrForcedToolNotCalled})
						return
					}
					if len(message.ToolCalls) != 1 || message.ToolCalls[0].Function.Name != input.ToolChoice.Name {
						send(ctx, output, Event{Err: ErrForcedToolMismatch})
						return
					}
					if input.ToolChoice.ValidateArguments != nil {
						if err := input.ToolChoice.ValidateArguments(message.ToolCalls[0].Function.Arguments); err != nil {
							send(ctx, output, Event{Err: err})
							return
						}
					}
				}
				for _, call := range message.ToolCalls {
					if call.ID == "" || call.Function.Name == "" {
						err := errors.New("model returned a tool call without an id or name")
						emitPendingToolFailures(ctx, output, pendingTools, err)
						send(ctx, output, Event{Err: err})
						return
					}
					pendingTools[call.ID] = call.Function.Name
					if !send(ctx, output, Event{Tool: &ToolEvent{Type: ToolStarted, ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments}}) {
						return
					}
				}
			case schema.Tool:
				if message.ToolCallID == "" {
					continue
				}
				if !send(ctx, output, Event{Tool: &ToolEvent{Type: ToolCompleted, ID: message.ToolCallID, Name: pendingTools[message.ToolCallID], Result: message.Content}}) {
					return
				}
				delete(pendingTools, message.ToolCallID)
			}
		}
	}()
	return output
}

// runSingleTurn performs exactly one model completion without exposing tools or
// constructing a ReAct loop. It is appropriate for trusted internal callers
// that need a deterministic structured response, such as background curation.
func (runtime *Eino) runSingleTurn(ctx context.Context, input Input, output chan<- Event) {
	if len(input.Tools) != 0 {
		send(ctx, output, Event{Err: errors.New("single-turn runtime does not support tools")})
		return
	}
	if input.ToolChoice.Mode != "" && input.ToolChoice.Mode != ToolChoiceAuto {
		send(ctx, output, Event{Err: errors.New("single-turn runtime does not support forced tools")})
		return
	}
	messages := make([]*schema.Message, 0, len(input.Messages)+1)
	if instruction := strings.TrimSpace(input.Instruction); instruction != "" {
		messages = append(messages, schema.SystemMessage(instruction))
	}
	messages = append(messages, einoMessages(input.Messages)...)
	message, err := runtime.model.Generate(ctx, messages)
	if err != nil {
		send(ctx, output, Event{Err: err})
		return
	}
	if message == nil {
		send(ctx, output, Event{Err: errors.New("model returned a nil single-turn response")})
		return
	}
	if len(message.ToolCalls) != 0 {
		send(ctx, output, Event{Err: errors.New("single-turn model returned an unexpected tool call")})
		return
	}
	if strings.TrimSpace(message.Content) == "" {
		send(ctx, output, Event{Err: errors.New("model returned an empty single-turn response")})
		return
	}
	send(ctx, output, Event{Delta: message.Content})
}

func einoMessages(messages []Message) []*schema.Message {
	result := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case RoleUser:
			result = append(result, schema.UserMessage(message.Content))
		case RoleAssistant:
			result = append(result, schema.AssistantMessage(message.Content, nil))
		}
	}
	return result
}

func streamMessage(ctx context.Context, reader *schema.StreamReader[*schema.Message], output chan<- Event, emitText bool) (*schema.Message, error) {
	defer reader.Close()
	chunks := make([]*schema.Message, 0, 8)
	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			if len(chunks) == 0 {
				return nil, nil
			}
			message, err := schema.ConcatMessages(chunks)
			if err != nil {
				return nil, fmt.Errorf("concatenate model stream: %w", err)
			}
			return message, nil
		}
		if err != nil {
			return nil, fmt.Errorf("receive model stream: %w", err)
		}
		if chunk == nil {
			continue
		}
		chunks = append(chunks, chunk)
		if emitText && chunk.Content != "" && !send(ctx, output, Event{Delta: chunk.Content}) {
			return nil, ctx.Err()
		}
	}
}

func emitPendingToolFailures(ctx context.Context, output chan<- Event, pending map[string]string, cause error) {
	for id, name := range pending {
		if !send(ctx, output, Event{Tool: &ToolEvent{Type: ToolFailed, ID: id, Name: name, Error: cause.Error()}}) {
			return
		}
		delete(pending, id)
	}
}

func send(ctx context.Context, output chan<- Event, value Event) bool {
	select {
	case output <- value:
		return true
	case <-ctx.Done():
		return false
	}
}
