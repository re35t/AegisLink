package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

type einoTool struct {
	definition Tool
	parameters *jsonschema.Schema
}

func einoTools(definitions []Tool) ([]tool.BaseTool, error) {
	resolved := make([]tool.BaseTool, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Name == "" || definition.Invoke == nil {
			return nil, fmt.Errorf("tool name and invoke function are required")
		}
		var parameters jsonschema.Schema
		if len(definition.InputSchema) == 0 {
			definition.InputSchema = json.RawMessage(`{"type":"object"}`)
		}
		if err := json.Unmarshal(definition.InputSchema, &parameters); err != nil {
			return nil, fmt.Errorf("decode schema for tool %s: %w", definition.Name, err)
		}
		resolved = append(resolved, &einoTool{definition: definition, parameters: &parameters})
	}
	return resolved, nil
}

func (runtimeTool *einoTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        runtimeTool.definition.Name,
		Desc:        runtimeTool.definition.Description,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(runtimeTool.parameters),
	}, nil
}

func (runtimeTool *einoTool) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	return runtimeTool.definition.Invoke(ctx, arguments)
}
