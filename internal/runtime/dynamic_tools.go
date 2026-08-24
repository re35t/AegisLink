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
	Name      string            `json:"name"`
	Content   string            `json:"content"`
	Resources []skillFileOutput `json:"resources,omitempty"`
}

type skillFileOutput struct {
	Path         string `json:"path"`
	MediaType    string `json:"mediaType"`
	SizeBytes    int64  `json:"sizeBytes"`
	TextReadable bool   `json:"textReadable"`
}

type skillResourceInput struct {
	Name string `json:"name" jsonschema:"required,description=Exact enabled Skill name"`
	Path string `json:"path" jsonschema:"required,description=Exact resource path returned by load_skill"`
}

type skillResourceOutput struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	Content   string `json:"content"`
}

type dynamicTool struct {
	definition conversation.RuntimeTool
	parameters *jsonschema.Schema
}

func runtimeTools(ctx context.Context, builtIns []tool.BaseTool, agentContext conversation.AgentContext) ([]tool.BaseTool, error) {
	resolved := append([]tool.BaseTool(nil), builtIns...)
	if len(agentContext.Skills) > 0 {
		skillsByName := make(map[string]conversation.RuntimeSkill, len(agentContext.Skills))
		hasReadableResources := false
		for _, item := range agentContext.Skills {
			skillsByName[item.Name] = item
			for _, file := range item.Files {
				if file.Path != "SKILL.md" && file.TextReadable {
					hasReadableResources = true
				}
			}
		}
		loader, err := toolutils.InferTool(
			"load_skill",
			"Load the full SKILL.md instructions for one enabled Agent Skill after matching the user's task to its description.",
			func(_ context.Context, input *skillInput) (*skillOutput, error) {
				skill, exists := skillsByName[input.Name]
				if !exists {
					return nil, fmt.Errorf("skill %q is not enabled for this Agent", input.Name)
				}
				resources := make([]skillFileOutput, 0, len(skill.Files))
				for _, file := range skill.Files {
					if file.Path == "SKILL.md" {
						continue
					}
					resources = append(resources, skillFileOutput{
						Path: file.Path, MediaType: file.MediaType, SizeBytes: file.SizeBytes, TextReadable: file.TextReadable,
					})
				}
				return &skillOutput{Name: input.Name, Content: skill.Content, Resources: resources}, nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("create Skill loader: %w", err)
		}
		resolved = append(resolved, loader)
		if hasReadableResources {
			resourceLoader, err := toolutils.InferTool(
				"read_skill_resource",
				"Read one UTF-8 reference or script from an enabled Skill bundle. This never executes scripts or returns binary assets.",
				func(ctx context.Context, input *skillResourceInput) (*skillResourceOutput, error) {
					skill, exists := skillsByName[input.Name]
					if !exists {
						return nil, fmt.Errorf("skill %q is not enabled for this Agent", input.Name)
					}
					allowed := false
					for _, file := range skill.Files {
						if file.Path == input.Path && file.Path != "SKILL.md" && file.TextReadable {
							allowed = true
							break
						}
					}
					if !allowed || skill.ReadResource == nil {
						return nil, fmt.Errorf("resource %q is not readable for Skill %q", input.Path, input.Name)
					}
					resource, err := skill.ReadResource(ctx, input.Path)
					if err != nil {
						return nil, err
					}
					return &skillResourceOutput{
						Name: input.Name, Path: resource.Path, MediaType: resource.MediaType, Content: resource.Content,
					}, nil
				},
			)
			if err != nil {
				return nil, fmt.Errorf("create Skill resource loader: %w", err)
			}
			resolved = append(resolved, resourceLoader)
		}
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
