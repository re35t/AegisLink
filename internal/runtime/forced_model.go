package runtime

import (
	"context"
	"sync/atomic"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type forceOnceState struct {
	calls atomic.Uint32
}

// forceOnceModel is created per Run. Its state is shared only by immutable
// WithTools derivatives from that Run, so concurrent Runs cannot affect one
// another or mutate the shared provider model.
type forceOnceModel struct {
	base     model.ToolCallingChatModel
	toolName string
	state    *forceOnceState
}

func newForceOnceModel(base model.ToolCallingChatModel, toolName string) model.ToolCallingChatModel {
	return &forceOnceModel{base: base, toolName: toolName, state: &forceOnceState{}}
}

func (wrapped *forceOnceModel) Generate(ctx context.Context, input []*schema.Message, options ...model.Option) (*schema.Message, error) {
	return wrapped.base.Generate(ctx, input, wrapped.options(options)...)
}

func (wrapped *forceOnceModel) Stream(ctx context.Context, input []*schema.Message, options ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return wrapped.base.Stream(ctx, input, wrapped.options(options)...)
}

func (wrapped *forceOnceModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := wrapped.base.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &forceOnceModel{base: bound, toolName: wrapped.toolName, state: wrapped.state}, nil
}

func (wrapped *forceOnceModel) options(options []model.Option) []model.Option {
	if wrapped.state.calls.Add(1) != 1 {
		return options
	}
	forced := make([]model.Option, 0, len(options)+1)
	forced = append(forced, options...)
	forced = append(forced, model.WithToolChoice(schema.ToolChoiceForced, wrapped.toolName))
	return forced
}
