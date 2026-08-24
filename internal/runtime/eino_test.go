package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/conversation"
)

func TestEinoStreamsAssistantDeltasWithConversationHistory(t *testing.T) {
	t.Parallel()
	fake := &fakeModel{}
	runtime := NewWithModel(fake)
	outputs := runtime.Stream(t.Context(), conversation.RuntimeInput{
		Agent: agent.Agent{Name: "Aegis", SystemPrompt: "Be concise."},
		Messages: []conversation.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
			{Role: "user", Content: "continue"},
		},
	})
	var answer strings.Builder
	for output := range outputs {
		if output.Err != nil {
			t.Fatalf("unexpected runtime error: %v", output.Err)
		}
		answer.WriteString(output.Delta)
	}
	if answer.String() != "hello from eino" {
		t.Fatalf("unexpected answer %q", answer.String())
	}
	if len(fake.input) != 4 || fake.input[0].Role != schema.System || fake.input[3].Content != "continue" {
		t.Fatalf("unexpected model input: %#v", fake.input)
	}
}

func TestEinoRunsReActToolLoop(t *testing.T) {
	t.Parallel()
	echoTool, err := toolutils.InferTool(
		"echo",
		"Echo text for a deterministic ReAct test.",
		func(_ context.Context, input *echoInput) (*echoOutput, error) {
			return &echoOutput{Text: input.Text}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fake := &reactModel{}
	runtime := NewWithModel(fake, Options{Tools: []tool.BaseTool{echoTool}, MaxIterations: 4})

	var answer strings.Builder
	var toolEvents []*conversation.RuntimeToolEvent
	for output := range runtime.Stream(t.Context(), conversation.RuntimeInput{
		Agent:    agent.Agent{Name: "Aegis", SystemPrompt: "Use tools when requested."},
		Messages: []conversation.Message{{Role: "user", Content: "Echo hello from tool."}},
	}) {
		if output.Err != nil {
			t.Fatalf("unexpected runtime error: %v", output.Err)
		}
		if output.Tool != nil {
			toolEvents = append(toolEvents, output.Tool)
		}
		answer.WriteString(output.Delta)
	}

	if answer.String() != "tool result observed" {
		t.Fatalf("unexpected answer %q", answer.String())
	}
	if !fake.sawToolResult || len(fake.tools) != 1 || fake.tools[0].Name != "echo" {
		t.Fatalf("ReAct loop did not bind and observe the tool result: %#v", fake)
	}
	if len(toolEvents) != 2 {
		t.Fatalf("tool lifecycle events = %#v", toolEvents)
	}
	if toolEvents[0].Type != conversation.RuntimeToolStarted ||
		toolEvents[0].ID != "call-echo" ||
		toolEvents[0].Name != "echo" ||
		toolEvents[0].Arguments != `{"text":"hello from tool"}` {
		t.Fatalf("unexpected tool start event: %#v", toolEvents[0])
	}
	if toolEvents[1].Type != conversation.RuntimeToolCompleted ||
		toolEvents[1].ID != "call-echo" ||
		!strings.Contains(toolEvents[1].Result, "hello from tool") {
		t.Fatalf("unexpected tool completion event: %#v", toolEvents[1])
	}
}

func TestEinoForcesOnlyTheFirstModelDecision(t *testing.T) {
	t.Parallel()
	echoTool, err := toolutils.InferTool(
		"echo", "Echo text", func(_ context.Context, input *echoInput) (*echoOutput, error) {
			return &echoOutput{Text: input.Text}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fake := &reactModel{}
	runtime := NewWithModel(fake, Options{Tools: []tool.BaseTool{echoTool}, MaxIterations: 4})
	for output := range runtime.Stream(t.Context(), conversation.RuntimeInput{
		Agent: agent.Agent{Name: "Aegis"}, Messages: []conversation.Message{{Role: "user", Content: "echo"}},
		Policy: conversation.ExecutionPolicy{Mode: "force-tool-once", QualifiedToolName: "echo"},
	}) {
		if output.Err != nil {
			t.Fatalf("unexpected runtime error: %v", output.Err)
		}
	}
	if len(fake.options) != 2 {
		t.Fatalf("model calls = %d", len(fake.options))
	}
	if fake.options[0].ToolChoice == nil || *fake.options[0].ToolChoice != schema.ToolChoiceForced ||
		len(fake.options[0].AllowedToolNames) != 1 || fake.options[0].AllowedToolNames[0] != "echo" {
		t.Fatalf("first call was not precisely forced: %#v", fake.options[0])
	}
	if fake.options[1].ToolChoice != nil || len(fake.options[1].AllowedToolNames) != 0 {
		t.Fatalf("second call did not return to auto mode: %#v", fake.options[1])
	}
}

func TestEinoRejectsIgnoredAndMismatchedForcedTool(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		model    model.ToolCallingChatModel
		expected error
	}{
		{name: "ignored", model: &fakeModel{}, expected: conversation.ErrForcedToolNotCalled},
		{name: "mismatch", model: &wrongToolModel{}, expected: conversation.ErrForcedToolMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime := NewWithModel(test.model)
			var runtimeErr error
			for output := range runtime.Stream(t.Context(), conversation.RuntimeInput{
				Agent: agent.Agent{Name: "Aegis"}, Messages: []conversation.Message{{Role: "user", Content: "use it"}},
				Policy: conversation.ExecutionPolicy{Mode: "force-tool-once", QualifiedToolName: "expected"},
			}) {
				if output.Err != nil {
					runtimeErr = output.Err
				}
			}
			if !errors.Is(runtimeErr, test.expected) {
				t.Fatalf("error = %v, expected %v", runtimeErr, test.expected)
			}
		})
	}
}

type fakeModel struct {
	input []*schema.Message
	tools []*schema.ToolInfo
}

func (fake *fakeModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("hello from eino", nil), nil
}

func (fake *fakeModel) Stream(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	fake.input = input
	return schema.StreamReaderFromArray([]*schema.Message{
		schema.AssistantMessage("hello ", nil),
		schema.AssistantMessage("from eino", nil),
	}), nil
}

func (fake *fakeModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	fake.tools = tools
	return fake, nil
}

type echoInput struct {
	Text string `json:"text" jsonschema:"required,description=text to echo"`
}

type echoOutput struct {
	Text string `json:"text"`
}

type reactModel struct {
	calls         int
	tools         []*schema.ToolInfo
	options       []*model.Options
	sawToolResult bool
}

func (fake *reactModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("unexpected non-streaming model call")
}

func (fake *reactModel) Stream(_ context.Context, input []*schema.Message, options ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	fake.calls++
	resolved := model.GetCommonOptions(&model.Options{}, options...)
	fake.options = append(fake.options, resolved)
	fake.tools = resolved.Tools
	if fake.calls == 1 {
		index := 0
		return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("", []schema.ToolCall{{
			Index: &index,
			ID:    "call-echo",
			Type:  "function",
			Function: schema.FunctionCall{
				Name:      "echo",
				Arguments: `{"text":"hello from tool"}`,
			},
		}})}), nil
	}
	for _, message := range input {
		if message.Role == schema.Tool && message.ToolCallID == "call-echo" && strings.Contains(message.Content, "hello from tool") {
			fake.sawToolResult = true
		}
	}
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("tool result observed", nil)}), nil
}

type wrongToolModel struct{}

func (*wrongToolModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("unexpected Generate")
}

func (*wrongToolModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	index := 0
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("", []schema.ToolCall{{
		Index: &index, ID: "wrong-call", Type: "function",
		Function: schema.FunctionCall{Name: "wrong", Arguments: `{}`},
	}})}), nil
}

func (fake *wrongToolModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return fake, nil
}

func (fake *reactModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	fake.tools = tools
	return fake, nil
}
