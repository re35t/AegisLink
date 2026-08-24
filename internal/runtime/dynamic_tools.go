package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/re35t/AegisLink/internal/conversation"
)

type skillInput struct {
	Name string `json:"name" jsonschema:"required,description=Exact enabled Skill name from available_skills"`
}

type skillOutput struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type dynamicTool struct {
	definition conversation.RuntimeTool
	parameters *jsonschema.Schema
}

func runtimeTools(ctx context.Context, builtIns []tool.BaseTool, agentContext conversation.AgentContext) ([]tool.BaseTool, error) {
	resolved := append([]tool.BaseTool(nil), builtIns...)
	if len(agentContext.Skills) > 0 {
		skillsByName := make(map[string]string, len(agentContext.Skills))
		for _, item := range agentContext.Skills {
			skillsByName[item.Name] = item.Content
		}
		loader, err := toolutils.InferTool(
			"load_skill",
			"Load the full SKILL.md instructions for one enabled Agent Skill after matching the user's task to its description.",
			func(_ context.Context, input *skillInput) (*skillOutput, error) {
				content, exists := skillsByName[input.Name]
				if !exists {
					return nil, fmt.Errorf("skill %q is not enabled for this Agent", input.Name)
				}
				return &skillOutput{Name: input.Name, Content: content}, nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("create Skill loader: %w", err)
		}
		resolved = append(resolved, loader)
	}
	for _, definition := range agentContext.Tools {
		if definition.Invoke == nil {
			continue
		}
		var parameters jsonschema.Schema
		if err := json.Unmarshal(definition.InputSchema, &parameters); err != nil {
			return nil, fmt.Errorf("decode schema for tool %s: %w", definition.Name, err)
		}
		resolved = append(resolved, &dynamicTool{definition: definition, parameters: &parameters})
	}
	return resolved, nil
}

func (runtimeTool *dynamicTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        runtimeTool.definition.Name,
		Desc:        runtimeTool.definition.Description,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(runtimeTool.parameters),
	}, nil
}

func (runtimeTool *dynamicTool) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	return runtimeTool.definition.Invoke(ctx, arguments)
}
