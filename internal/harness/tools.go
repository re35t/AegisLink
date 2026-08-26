package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/re35t/AegisLink/internal/runtime"
)

var (
	timeSchema      = json.RawMessage(`{"type":"object","properties":{"timezone":{"type":"string","description":"IANA timezone such as Asia/Shanghai or UTC"}},"required":["timezone"],"additionalProperties":false}`)
	discoverySchema = json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Optional capability need inferred from the user request"}},"additionalProperties":false}`)
	skillSchema     = json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Exact enabled Skill name from available_skills"}},"required":["name"],"additionalProperties":false}`)
	resourceSchema  = json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Exact enabled Skill name"},"path":{"type":"string","description":"Exact resource path returned by load_skill"}},"required":["name","path"],"additionalProperties":false}`)
)

type skillInput struct {
	Name string `json:"name"`
}
type capabilityOutput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
type skillResourceOutput struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	Content   string `json:"content"`
}
type skillResourceInput struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func harnessTools(agentContext agentContext) []runtime.Tool {
	resolved := []runtime.Tool{{
		Name: "get_current_time", Description: "Get the current time in an IANA timezone. Use this instead of guessing the current date or time.", InputSchema: timeSchema,
		Invoke: func(_ context.Context, arguments string) (string, error) {
			var input struct {
				Timezone string `json:"timezone"`
			}
			if err := json.Unmarshal([]byte(arguments), &input); err != nil {
				return "", err
			}
			location, err := time.LoadLocation(input.Timezone)
			if err != nil {
				return "", fmt.Errorf("invalid timezone %q", input.Timezone)
			}
			return encodeToolResult(map[string]string{"timezone": input.Timezone, "time": time.Now().In(location).Format(time.RFC3339)})
		},
	}, {
		Name: "discover_capabilities", Description: "Inspect the current Agent's enabled Skills and MCP tools so the Agent can explain which capabilities fit the request.", InputSchema: discoverySchema,
		Invoke: func(_ context.Context, arguments string) (string, error) {
			var input struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal([]byte(arguments), &input); err != nil {
				return "", err
			}
			skills := make([]capabilityOutput, 0, len(agentContext.skills))
			for _, item := range agentContext.skills {
				skills = append(skills, capabilityOutput{Name: item.name, Description: item.description})
			}
			mcpTools := make([]capabilityOutput, 0, len(agentContext.tools))
			for _, item := range agentContext.tools {
				mcpTools = append(mcpTools, capabilityOutput{Name: item.Name, Description: item.Description})
			}
			return encodeToolResult(struct {
				Query       string             `json:"query,omitempty"`
				MemoryCount int                `json:"memoryCount"`
				Skills      []capabilityOutput `json:"skills"`
				MCPTools    []capabilityOutput `json:"mcpTools"`
			}{input.Query, len(agentContext.memories), skills, mcpTools})
		},
	}}
	if len(agentContext.skills) > 0 {
		skillsByName := make(map[string]runtimeSkill, len(agentContext.skills))
		hasReadableResources := false
		for _, item := range agentContext.skills {
			skillsByName[item.name] = item
			for _, file := range item.files {
				if file.path != "SKILL.md" && file.textReadable {
					hasReadableResources = true
				}
			}
		}
		resolved = append(resolved, runtime.Tool{
			Name: "load_skill", Description: "Load the full SKILL.md instructions for one enabled Agent Skill after matching the user's task to its description.", InputSchema: skillSchema,
			Invoke: func(_ context.Context, arguments string) (string, error) {
				var input skillInput
				if err := json.Unmarshal([]byte(arguments), &input); err != nil {
					return "", err
				}
				skill, exists := skillsByName[input.Name]
				if !exists {
					return "", fmt.Errorf("skill %q is not enabled for this Agent", input.Name)
				}
				type fileOutput struct {
					Path         string `json:"path"`
					MediaType    string `json:"mediaType"`
					SizeBytes    int64  `json:"sizeBytes"`
					TextReadable bool   `json:"textReadable"`
				}
				resources := make([]fileOutput, 0, len(skill.files))
				for _, file := range skill.files {
					if file.path != "SKILL.md" {
						resources = append(resources, fileOutput{file.path, file.mediaType, file.sizeBytes, file.textReadable})
					}
				}
				return encodeToolResult(struct {
					Name      string       `json:"name"`
					Content   string       `json:"content"`
					Resources []fileOutput `json:"resources,omitempty"`
				}{input.Name, skill.content, resources})
			},
		})
		if hasReadableResources {
			resolved = append(resolved, runtime.Tool{
				Name: "read_skill_resource", Description: "Read one UTF-8 reference or script from an enabled Skill bundle. This never executes scripts or returns binary assets.", InputSchema: resourceSchema,
				Invoke: func(ctx context.Context, arguments string) (string, error) {
					var input skillResourceInput
					if err := json.Unmarshal([]byte(arguments), &input); err != nil {
						return "", err
					}
					skill, exists := skillsByName[input.Name]
					if !exists {
						return "", fmt.Errorf("skill %q is not enabled for this Agent", input.Name)
					}
					allowed := false
					for _, file := range skill.files {
						if file.path == input.Path && file.path != "SKILL.md" && file.textReadable {
							allowed = true
							break
						}
					}
					if !allowed || skill.readResource == nil {
						return "", fmt.Errorf("resource %q is not readable for Skill %q", input.Path, input.Name)
					}
					resource, err := skill.readResource(ctx, input.Path)
					if err != nil {
						return "", err
					}
					return encodeToolResult(skillResourceOutput{Name: input.Name, Path: resource.path, MediaType: resource.mediaType, Content: resource.content})
				},
			})
		}
	}
	return append(resolved, agentContext.tools...)
}

func encodeToolResult(value any) (string, error) {
	encoded, err := json.Marshal(value)
	return string(encoded), err
}
