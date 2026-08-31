package curator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/re35t/AegisLink/internal/impression"
	"github.com/re35t/AegisLink/internal/runtime"
)

func TestCurateUsesNoToolRuntimeAndMapsResponse(t *testing.T) {
	stub := &runtimeStub{events: []runtime.Event{{Delta: `{"impressions":[{"action":"create","ref":"new-one","targetId":"","scope":"task","kind":"current-task","summary":"Testing Harness split","details":{},"tags":["go"],"confidence":0.9,"salience":0.8,"sourceMessageIds":["message-one"],"sourceMemoryIds":[]}],"facts":[{"subject":"project","namespace":"architecture","key":"runtime","value":{"value":"minimal"},"rationale":"explicit refactor","confidence":0.8,"sourceImpressionIds":["new-one"]}]}`}}}
	service, err := New(stub, "test-curator")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Curate(t.Context(), impression.CurationInput{AgentID: "agent-one", RunID: "run-one"})
	if err != nil {
		t.Fatal(err)
	}
	if stub.input.ExecutionMode != runtime.ExecutionModeSingleTurn || len(stub.input.Tools) != 0 || stub.input.ToolChoice.Mode != runtime.ToolChoiceAuto {
		t.Fatalf("curator runtime input = %#v", stub.input)
	}
	if len(stub.input.Messages) != 1 || !strings.Contains(stub.input.Messages[0].Content, "<curation_evidence>") {
		t.Fatalf("curator evidence message = %#v", stub.input.Messages)
	}
	for _, expected := range []string{"One explicit first-person statement can be enough", "propose the candidate so the owner can decide", "Do not propose secrets"} {
		if !strings.Contains(stub.input.Instruction, expected) {
			t.Fatalf("curator instruction missing %q: %s", expected, stub.input.Instruction)
		}
	}
	if len(result.Impressions) != 1 || result.Impressions[0].TargetID != "new-one" || len(result.Facts) != 1 {
		t.Fatalf("curation = %#v", result)
	}
	if result.Generation.Model != "test-curator" || result.Generation.RunID != "run-one" || result.Generation.PromptVersion != promptVersion || result.Generation.GeneratedAt.IsZero() {
		t.Fatalf("generation = %#v", result.Generation)
	}
}

func TestCurateRejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, content := range []string{
		`{"impressions":[],"facts":[],"unknown":true}`,
		`{"impressions":[],"facts":[]} {}`,
	} {
		service, err := New(&runtimeStub{events: []runtime.Event{{Delta: content}}}, "test-curator")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Curate(t.Context(), impression.CurationInput{}); err == nil {
			t.Fatalf("expected strict decode error for %q", content)
		}
	}
}

func TestCurateRetriesOneIncompleteResponse(t *testing.T) {
	t.Parallel()
	stub := &runtimeSequenceStub{runs: [][]runtime.Event{
		{{Delta: `{"impressions":[`}},
		{{Delta: `{"impressions":[],"facts":[]}`}},
	}}
	service, err := New(stub, "test-curator")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Curate(t.Context(), impression.CurationInput{RunID: "run-one"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stub.inputs) != 2 || !strings.Contains(stub.inputs[1].Instruction, "prior response was truncated") {
		t.Fatalf("retry inputs = %#v", stub.inputs)
	}
	if len(result.Impressions) != 0 || len(result.Facts) != 0 {
		t.Fatalf("unexpected curation = %#v", result)
	}
}

func TestDecodeResponseMarksTruncatedJSONAsIncomplete(t *testing.T) {
	t.Parallel()
	_, err := decodeResponse(`{"impressions":[],"facts":[`)
	if !errors.Is(err, errIncompleteResponse) {
		t.Fatalf("decode error = %v", err)
	}
}

func TestCuratePropagatesRuntimeFailureAndRejectsToolEvent(t *testing.T) {
	for _, events := range [][]runtime.Event{
		{{Err: errors.New("model failed")}},
		{{Tool: &runtime.ToolEvent{Type: runtime.ToolStarted, ID: "unexpected", Name: "tool"}}},
		{},
	} {
		service, err := New(&runtimeStub{events: events}, "test-curator")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Curate(t.Context(), impression.CurationInput{}); err == nil {
			t.Fatalf("expected curator failure for events %#v", events)
		}
	}
}

func TestCurateUsesRuntimeSingleTurnCompletion(t *testing.T) {
	t.Parallel()
	model := &singleTurnModel{}
	agentRuntime := runtime.NewWithModel(model, runtime.Options{MaxIterations: 1})
	service, err := New(agentRuntime, "test-curator")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Curate(t.Context(), impression.CurationInput{AgentID: "agent-one", RunID: "run-one"})
	if err != nil {
		t.Fatal(err)
	}
	if model.streamCalled || len(model.input) != 2 {
		t.Fatalf("curator must use one direct completion: input=%#v streamCalled=%v", model.input, model.streamCalled)
	}
	if len(result.Impressions) != 0 || len(result.Facts) != 0 {
		t.Fatalf("unexpected curation = %#v", result)
	}
}

type runtimeStub struct {
	input  runtime.Input
	events []runtime.Event
}

type runtimeSequenceStub struct {
	inputs []runtime.Input
	runs   [][]runtime.Event
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
	return nil, errors.New("curator must not stream")
}

func (model *singleTurnModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return model, nil
}

func (stub *runtimeStub) Run(_ context.Context, input runtime.Input) <-chan runtime.Event {
	stub.input = input
	output := make(chan runtime.Event, len(stub.events))
	for _, event := range stub.events {
		output <- event
	}
	close(output)
	return output
}

func (stub *runtimeSequenceStub) Run(_ context.Context, input runtime.Input) <-chan runtime.Event {
	index := len(stub.inputs)
	stub.inputs = append(stub.inputs, input)
	var events []runtime.Event
	if index < len(stub.runs) {
		events = stub.runs[index]
	}
	output := make(chan runtime.Event, len(events))
	for _, event := range events {
		output <- event
	}
	close(output)
	return output
}
