package harness

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/re35t/AegisLink/internal/agentindex"
	"github.com/re35t/AegisLink/internal/catalog"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/runtime"
	"github.com/re35t/AegisLink/internal/skills"
)

type Harness struct {
	runtime      runtime.Runtime
	memories     MemoryReader
	skills       SkillReader
	mcp          MCPRuntime
	facts        FactReader
	impressions  ImpressionReader
	agentSearch  AgentSearcher
	collaborator Collaborator
}

func New(agentRuntime runtime.Runtime, memories MemoryReader, skillReader SkillReader, mcpRuntime MCPRuntime, facts FactReader, impressions ImpressionReader, agentSearch AgentSearcher, collaborators ...Collaborator) *Harness {
	var collaborator Collaborator
	if len(collaborators) > 0 {
		collaborator = collaborators[0]
	}
	return &Harness{runtime: agentRuntime, memories: memories, skills: skillReader, mcp: mcpRuntime, facts: facts, impressions: impressions, agentSearch: agentSearch, collaborator: collaborator}
}

func (harness *Harness) ResolveSelection(ctx context.Context, principalID, agentID string, selection conversation.RunSelection) (conversation.ExecutionPolicy, error) {
	switch selection.Action {
	case "force-tool-once":
		tool, err := harness.mcp.ResolveMention(ctx, principalID, agentID, selection.MentionID)
		if err != nil {
			return conversation.ExecutionPolicy{}, err
		}
		return conversation.ExecutionPolicy{Mode: "force-tool-once", Kind: "mcp-tool", Action: selection.Action, MentionID: selection.MentionID, ResourceID: tool.ToolID, Label: tool.Name, ToolID: tool.ToolID, ToolName: tool.Name, QualifiedToolName: tool.QualifiedName}, nil
	case "use-skill-once":
		skillID, ok := strings.CutPrefix(selection.MentionID, "skill:")
		if !ok || skillID == "" {
			return conversation.ExecutionPolicy{}, skills.ErrNotFound
		}
		items, err := harness.skills.List(ctx, principalID, agentID)
		if err != nil {
			return conversation.ExecutionPolicy{}, err
		}
		for _, item := range items {
			if item.ID != skillID {
				continue
			}
			if !item.Enabled {
				return conversation.ExecutionPolicy{}, skills.ErrDisabled
			}
			return conversation.ExecutionPolicy{Mode: "use-skill-once", Kind: "skill", Action: selection.Action, MentionID: selection.MentionID, ResourceID: item.ID, Label: item.Name, SkillID: item.ID, SkillName: item.Name, QualifiedToolName: "load_skill"}, nil
		}
		return conversation.ExecutionPolicy{}, skills.ErrNotFound
	case "discover-once":
		if selection.MentionID != catalog.DiscoveryMentionID {
			return conversation.ExecutionPolicy{}, catalog.ErrInvalid
		}
		if harness.agentSearch == nil {
			return conversation.ExecutionPolicy{}, agentindex.ErrUnavailable
		}
		return conversation.ExecutionPolicy{Mode: "discover-once", Kind: "discovery", Action: selection.Action, MentionID: selection.MentionID, ResourceID: catalog.DiscoveryResourceID, Label: "Find related Agents", QualifiedToolName: "discover_agents"}, nil
	default:
		return conversation.ExecutionPolicy{}, catalog.ErrInvalid
	}
}

func (harness *Harness) Run(ctx context.Context, input conversation.HarnessInput) (<-chan conversation.HarnessOutput, error) {
	currentMessage := ""
	if len(input.Messages) > 0 {
		currentMessage = input.Messages[len(input.Messages)-1].Content
	}
	agentContext, err := harness.resolveContext(ctx, input.PrincipalID, input.Agent.ID, currentMessage)
	if err != nil {
		return nil, err
	}
	messages := make([]runtime.Message, 0, len(input.Messages))
	for _, message := range input.Messages {
		messages = append(messages, runtime.Message{Role: runtime.Role(message.Role), Content: message.Content})
	}
	choice := runtime.ToolChoice{Mode: runtime.ToolChoiceAuto}
	if input.Policy.Mode == "force-tool-once" || input.Policy.Mode == "use-skill-once" || input.Policy.Mode == "discover-once" {
		choice = runtime.ToolChoice{Mode: runtime.ToolChoiceForceOnce, Name: input.Policy.QualifiedToolName}
		if input.Policy.Mode == "use-skill-once" {
			expectedName := input.Policy.SkillName
			choice.ValidateArguments = func(arguments string) error {
				var selected skillInput
				if expectedName == "" || json.Unmarshal([]byte(arguments), &selected) != nil || selected.Name != expectedName {
					return conversation.ErrSelectedSkillMismatch
				}
				return nil
			}
		}
	}
	tools := harnessTools(agentContext)
	if harness.agentSearch != nil {
		tools = append(tools, agentSearchTool(harness.agentSearch, input.PrincipalID, input.Agent.ID))
	}
	if harness.collaborator != nil {
		tools = append(tools, collaborationTools(harness.collaborator, input.PrincipalID, input.Agent.ID, input.RunID)...)
	}
	events := harness.runtime.Run(ctx, runtime.Input{
		Agent:       runtime.Agent{Name: input.Agent.Name, Description: input.Agent.Description},
		Instruction: agentInstruction(input.Agent, agentContext, input.Policy), Messages: messages,
		Tools: tools, ToolChoice: choice,
	})
	output := make(chan conversation.HarnessOutput)
	go func() {
		defer close(output)
		for event := range events {
			value := conversation.HarnessOutput{Delta: event.Delta, Err: mapRuntimeError(event.Err)}
			if event.Tool != nil {
				value.Tool = &conversation.HarnessToolEvent{Type: conversation.HarnessToolEventType(event.Tool.Type), ID: event.Tool.ID, Name: event.Tool.Name, Arguments: event.Tool.Arguments, Result: event.Tool.Result, Error: event.Tool.Error}
			}
			select {
			case output <- value:
			case <-ctx.Done():
				return
			}
		}
	}()
	return output, nil
}

func mapRuntimeError(err error) error {
	switch {
	case errors.Is(err, runtime.ErrForcedToolNotCalled):
		return conversation.ErrForcedToolNotCalled
	case errors.Is(err, runtime.ErrForcedToolMismatch):
		return conversation.ErrForcedToolMismatch
	default:
		return err
	}
}
