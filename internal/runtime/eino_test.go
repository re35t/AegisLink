package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var errSelectedSkillMismatch = errors.New("selected skill mismatch")

func TestEinoStreamsAssistantDeltasWithConversationHistory(t *testing.T) {
	t.Parallel()
	fake := &fakeModel{}
	runtime := NewWithModel(fake)
	outputs := runtime.Run(t.Context(), Input{
		Agent: Agent{Name: "Aegis"}, Instruction: "Be concise.",
		Messages: []Message{
			{Role: RoleUser, Content: "hello"},
			{Role: RoleAssistant, Content: "hi"},
			{Role: RoleUser, Content: "continue"},
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
	if fake.input[0].Content != "Be concise." {
		t.Fatalf("system instruction = %q", fake.input[0].Content)
	}
}

func TestEinoStreamsSingleShotResponseWithOneIteration(t *testing.T) {
	t.Parallel()
	fake := &fakeModel{}
	agentRuntime := NewWithModel(fake, Options{MaxIterations: 1})

	var answer strings.Builder
	for output := range agentRuntime.Run(t.Context(), Input{
		Agent: Agent{Name: "curator"}, Instruction: "Return JSON only.",
		Messages: []Message{{Role: RoleUser, Content: "{}"}},
	}) {
		if output.Err != nil {
			t.Fatalf("unexpected runtime error: %v", output.Err)
		}
		answer.WriteString(output.Delta)
	}
	if answer.String() != "hello from eino" {
		t.Fatalf("unexpected answer %q", answer.String())
	}
}

func TestEinoRunsSingleTurnWithoutReActOrTools(t *testing.T) {
	t.Parallel()
	model := &singleTurnModel{}
	agentRuntime := NewWithModel(model, Options{MaxIterations: 1})

	var answer strings.Builder
	for output := range agentRuntime.Run(t.Context(), Input{
		ExecutionMode: ExecutionModeSingleTurn,
		Instruction:   "Return JSON only.",
		Messages:      []Message{{Role: RoleUser, Content: "{}"}},
		ToolChoice:    ToolChoice{Mode: ToolChoiceAuto},
	}) {
		if output.Err != nil {
			t.Fatalf("unexpected runtime error: %v", output.Err)
		}
		answer.WriteString(output.Delta)
	}
	if answer.String() != `{"impressions":[],"facts":[]}` {
		t.Fatalf("unexpected answer %q", answer.String())
	}
	if model.streamCalled || len(model.input) != 2 || model.input[0].Role != schema.System {
		t.Fatalf("single-turn model input = %#v, streamCalled=%v", model.input, model.streamCalled)
	}
}

func TestEinoRunsReActToolLoop(t *testing.T) {
	t.Parallel()
	echoTool := Tool{Name: "echo", Description: "Echo text for a deterministic ReAct test.", InputSchema: []byte(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`), Invoke: func(_ context.Context, arguments string) (string, error) {
		return arguments, nil
	}}
	fake := &reactModel{}
	runtime := NewWithModel(fake, Options{MaxIterations: 4})

	var answer strings.Builder
	var toolEvents []*ToolEvent
	for output := range runtime.Run(t.Context(), Input{
		Agent: Agent{Name: "Aegis"}, Instruction: "Use tools when requested.",
		Messages: []Message{{Role: RoleUser, Content: "Echo hello from tool."}}, Tools: []Tool{echoTool},
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
	if !fake.sawToolResult || !hasToolNamed(fake.tools, "echo") {
		t.Fatalf("ReAct loop did not bind and observe the tool result: %#v", fake)
	}
	if len(toolEvents) != 2 {
		t.Fatalf("tool lifecycle events = %#v", toolEvents)
	}
	if toolEvents[0].Type != ToolStarted ||
		toolEvents[0].ID != "call-echo" ||
		toolEvents[0].Name != "echo" ||
		toolEvents[0].Arguments != `{"text":"hello from tool"}` {
		t.Fatalf("unexpected tool start event: %#v", toolEvents[0])
	}
	if toolEvents[1].Type != ToolCompleted ||
		toolEvents[1].ID != "call-echo" ||
		!strings.Contains(toolEvents[1].Result, "hello from tool") {
		t.Fatalf("unexpected tool completion event: %#v", toolEvents[1])
	}
}

func TestEinoForcesOnlyTheFirstModelDecision(t *testing.T) {
	t.Parallel()
	echoTool := Tool{Name: "echo", Description: "Echo text", InputSchema: []byte(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`), Invoke: func(_ context.Context, arguments string) (string, error) { return arguments, nil }}
	fake := &reactModel{}
	runtime := NewWithModel(fake, Options{MaxIterations: 4})
	for output := range runtime.Run(t.Context(), Input{
		Agent: Agent{Name: "Aegis"}, Messages: []Message{{Role: RoleUser, Content: "echo"}}, Tools: []Tool{echoTool},
		ToolChoice: ToolChoice{Mode: ToolChoiceForceOnce, Name: "echo"},
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
		{name: "ignored", model: &fakeModel{}, expected: ErrForcedToolNotCalled},
		{name: "mismatch", model: &wrongToolModel{}, expected: ErrForcedToolMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime := NewWithModel(test.model)
			var runtimeErr error
			for output := range runtime.Run(t.Context(), Input{
				Agent: Agent{Name: "Aegis"}, Messages: []Message{{Role: RoleUser, Content: "use it"}},
				ToolChoice: ToolChoice{Mode: ToolChoiceForceOnce, Name: "expected"},
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

func TestEinoForcesSelectedSkillAndValidatesItsName(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		modelName string
		expected  error
	}{
		{name: "selected", modelName: "go-review"},
		{name: "different skill", modelName: "other-skill", expected: errSelectedSkillMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := &selectedSkillModel{skillName: test.modelName}
			runtime := NewWithModel(fake, Options{MaxIterations: 4})
			var runtimeErr error
			for output := range runtime.Run(t.Context(), Input{
				Agent: Agent{Name: "Aegis"}, Messages: []Message{{Role: RoleUser, Content: "review this"}},
				Tools: []Tool{{Name: "load_skill", InputSchema: []byte(`{"type":"object"}`), Invoke: func(context.Context, string) (string, error) { return `{}`, nil }}},
				ToolChoice: ToolChoice{Mode: ToolChoiceForceOnce, Name: "load_skill", ValidateArguments: func(arguments string) error {
					if strings.Contains(arguments, `"go-review"`) {
						return nil
					}
					return errSelectedSkillMismatch
				}},
			}) {
				if output.Err != nil {
					runtimeErr = output.Err
				}
			}
			if !errors.Is(runtimeErr, test.expected) {
				t.Fatalf("error = %v, expected %v", runtimeErr, test.expected)
			}
			if fake.options[0].ToolChoice == nil || *fake.options[0].ToolChoice != schema.ToolChoiceForced || fake.options[0].AllowedToolNames[0] != "load_skill" {
				t.Fatalf("selected Skill was not forced: %#v", fake.options[0])
			}
		})
	}
}

type fakeModel struct {
	input []*schema.Message
	tools []*schema.ToolInfo
}

type singleTurnModel struct {
	input        []*schema.Message
	streamCalled bool
}

func (model *singleTurnModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	model.input = input
	return schema.AssistantMessage(`{"impressions":[],"facts":[]}`, nil), nil
}

func (model *singleTurnModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	model.streamCalled = true
	return nil, errors.New("single-turn runtime must not stream")
}

func (model *singleTurnModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return model, nil
}

func hasToolNamed(tools []*schema.ToolInfo, name string) bool {
	for _, item := range tools {
		if item.Name == name {
			return true
		}
	}
	return false
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

type reactModel struct {
	calls         int
	tools         []*schema.ToolInfo
	options       []*model.Options
	sawToolResult bool
}

type selectedSkillModel struct {
	calls     int
	skillName string
	options   []*model.Options
}

func (*selectedSkillModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("unexpected Generate")
}

func (fake *selectedSkillModel) Stream(_ context.Context, _ []*schema.Message, options ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	fake.calls++
	fake.options = append(fake.options, model.GetCommonOptions(&model.Options{}, options...))
	if fake.calls == 1 {
		index := 0
		return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("", []schema.ToolCall{{
			Index: &index, ID: "call-skill", Type: "function",
			Function: schema.FunctionCall{Name: "load_skill", Arguments: `{"name":"` + fake.skillName + `"}`},
		}})}), nil
	}
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("skill applied", nil)}), nil
}

func (fake *selectedSkillModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return fake, nil
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
