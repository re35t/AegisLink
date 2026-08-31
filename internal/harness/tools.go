package harness

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/re35t/AegisLink/internal/agentindex"
	"github.com/re35t/AegisLink/internal/runtime"
)

var (
	timeSchema                 = json.RawMessage(`{"type":"object","properties":{"timezone":{"type":"string","description":"IANA timezone such as Asia/Shanghai or UTC"}},"required":["timezone"],"additionalProperties":false}`)
	discoverySchema            = json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","minLength":1,"maxLength":4000,"description":"Concise semantic search query inferred from the user's request"}},"required":["query"],"additionalProperties":false}`)
	assistanceSchema           = json.RawMessage(`{"type":"object","properties":{"targetAgentAddr":{"type":"string","description":"Exact opaque AgentAddr returned by discover_agents"},"purpose":{"type":"string","minLength":1,"maxLength":4000,"description":"Bounded assistance purpose without secrets or hidden instructions"}},"required":["targetAgentAddr","purpose"],"additionalProperties":false}`)
	collaborationMessageSchema = json.RawMessage(`{"type":"object","properties":{"sessionId":{"type":"string"},"message":{"type":"string","minLength":1,"maxLength":4000}},"required":["sessionId","message"],"additionalProperties":false}`)
	collaborationTaskSchema    = json.RawMessage(`{"type":"object","properties":{"sessionId":{"type":"string"},"taskId":{"type":"string"}},"required":["sessionId","taskId"],"additionalProperties":false}`)
	skillSchema                = json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Exact enabled Skill name from available_skills"}},"required":["name"],"additionalProperties":false}`)
	resourceSchema             = json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Exact enabled Skill name"},"path":{"type":"string","description":"Exact resource path returned by load_skill"}},"required":["name","path"],"additionalProperties":false}`)
)

type skillInput struct {
	Name string `json:"name"`
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

func agentSearchTool(searcher AgentSearcher, principalID, agentID string) runtime.Tool {
	return runtime.Tool{
		Name: "discover_agents", Description: "Search public, indexable Agent Profiles for related Agents. Returns only candidates supplied by AegisLink Index, including AgentAddr and similarity metadata.", InputSchema: discoverySchema,
		Invoke: func(ctx context.Context, arguments string) (string, error) {
			var input struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal([]byte(arguments), &input); err != nil {
				return "", err
			}
			input.Query = strings.TrimSpace(input.Query)
			if input.Query == "" {
				return "", agentindex.ErrInvalid
			}
			candidates, err := searcher.Search(ctx, principalID, agentID, input.Query, 5)
			if err != nil {
				return "", err
			}
			result := append([]agentindex.Candidate{}, candidates...)
			return encodeToolResult(struct {
				Query      string                 `json:"query"`
				Candidates []agentindex.Candidate `json:"candidates"`
			}{Query: input.Query, Candidates: result})
		},
	}
}

func collaborationTools(collaborator Collaborator, principalID, agentID, runID string) []runtime.Tool {
	return []runtime.Tool{
		{
			Name: "request_agent_assistance", Description: "Ask a discovered Agent to evaluate a text-only collaboration request. The target Owner policy and a short-lived Evaluation Invocation decide whether a scoped A2A Session is created.", InputSchema: assistanceSchema,
			Invoke: func(ctx context.Context, arguments string) (string, error) {
				var input struct {
					TargetAgentAddr string `json:"targetAgentAddr"`
					Purpose         string `json:"purpose"`
				}
				if err := json.Unmarshal([]byte(arguments), &input); err != nil {
					return "", err
				}
				idempotencyKey := collaborationIdempotencyKey(runID, input.TargetAgentAddr, input.Purpose)
				request, err := collaborator.RequestAssistance(ctx, principalID, agentID, input.TargetAgentAddr, input.Purpose, idempotencyKey)
				if err != nil {
					return "", err
				}
				return encodeToolResult(struct {
					ID              string   `json:"id"`
					TargetAgentAddr string   `json:"targetAgentAddr"`
					Status          string   `json:"status"`
					DecisionCode    string   `json:"decisionCode,omitempty"`
					SessionID       *string  `json:"sessionId,omitempty"`
					Scopes          []string `json:"scopes"`
				}{request.ID, request.TargetAgentAddr, request.Status, request.DecisionCode, request.SessionID, request.RequestedScopes})
			},
		},
		{
			Name: "send_collaboration_message", Description: "Send one text/plain official A2A Message through an active Collaboration Session. Returns an asynchronous A2A Task; poll it with get_collaboration_task.", InputSchema: collaborationMessageSchema,
			Invoke: func(ctx context.Context, arguments string) (string, error) {
				var input struct {
					SessionID string `json:"sessionId"`
					Message   string `json:"message"`
				}
				if err := json.Unmarshal([]byte(arguments), &input); err != nil {
					return "", err
				}
				task, err := collaborator.SendMessage(ctx, principalID, agentID, input.SessionID, input.Message, runID+":"+input.SessionID)
				if err != nil {
					return "", err
				}
				return encodeToolResult(task)
			},
		},
		{
			Name: "get_collaboration_task", Description: "Read the current official A2A Task state for a Task created through an owned Collaboration Session.", InputSchema: collaborationTaskSchema,
			Invoke: func(ctx context.Context, arguments string) (string, error) {
				var input struct {
					SessionID string `json:"sessionId"`
					TaskID    string `json:"taskId"`
				}
				if err := json.Unmarshal([]byte(arguments), &input); err != nil {
					return "", err
				}
				task, err := collaborator.GetTask(ctx, principalID, agentID, input.SessionID, input.TaskID)
				if err != nil {
					return "", err
				}
				return encodeToolResult(task)
			},
		},
	}
}

func collaborationIdempotencyKey(runID, targetAgentAddr, purpose string) string {
	digest := sha256.Sum256([]byte(runID + "\x00" + targetAgentAddr + "\x00" + purpose))
	return fmt.Sprintf("run_%x", digest)
}

func encodeToolResult(value any) (string, error) {
	encoded, err := json.Marshal(value)
	return string(encoded), err
}
