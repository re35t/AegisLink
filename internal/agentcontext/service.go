package agentcontext

import (
	"context"
	"strings"

	"github.com/re35t/AegisLink/internal/catalog"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/memory"
	"github.com/re35t/AegisLink/internal/skills"
)

type MemoryReader interface {
	Context(context.Context, string, string) ([]memory.Memory, error)
}

type SkillReader interface {
	List(context.Context, string, string) ([]skills.Skill, error)
	Enabled(context.Context, string, string) ([]skills.Skill, error)
	ReadFile(context.Context, string, string, string, string) (skills.File, error)
}

type MCPRuntime interface {
	RuntimeTools(context.Context, string, string) ([]mcp.RuntimeTool, error)
	ResolveMention(context.Context, string, string, string) (mcp.RuntimeTool, error)
	Invoke(context.Context, string, string, string, string, string) (string, error)
}

func (service *Service) ResolveSelection(ctx context.Context, principalID, agentID string, selection conversation.RunSelection) (conversation.ExecutionPolicy, error) {
	switch selection.Action {
	case "force-tool-once":
		tool, err := service.mcp.ResolveMention(ctx, principalID, agentID, selection.MentionID)
		if err != nil {
			return conversation.ExecutionPolicy{}, err
		}
		return conversation.ExecutionPolicy{
			Mode: "force-tool-once", Kind: "mcp-tool", Action: selection.Action,
			MentionID: selection.MentionID, ResourceID: tool.ToolID, Label: tool.Name,
			ToolID: tool.ToolID, ToolName: tool.Name, QualifiedToolName: tool.QualifiedName,
		}, nil
	case "use-skill-once":
		skillID, ok := strings.CutPrefix(selection.MentionID, "skill:")
		if !ok || skillID == "" {
			return conversation.ExecutionPolicy{}, skills.ErrNotFound
		}
		items, err := service.skills.List(ctx, principalID, agentID)
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
			return conversation.ExecutionPolicy{
				Mode: "use-skill-once", Kind: "skill", Action: selection.Action,
				MentionID: selection.MentionID, ResourceID: item.ID, Label: item.Name,
				SkillID: item.ID, SkillName: item.Name, QualifiedToolName: "load_skill",
			}, nil
		}
		return conversation.ExecutionPolicy{}, skills.ErrNotFound
	case "discover-once":
		if selection.MentionID != catalog.DiscoveryMentionID {
			return conversation.ExecutionPolicy{}, catalog.ErrInvalid
		}
		return conversation.ExecutionPolicy{
			Mode: "discover-once", Kind: "discovery", Action: selection.Action,
			MentionID: selection.MentionID, ResourceID: catalog.DiscoveryResourceID,
			Label: "Agent capabilities", QualifiedToolName: "discover_capabilities",
		}, nil
	default:
		return conversation.ExecutionPolicy{}, catalog.ErrInvalid
	}
}

type Service struct {
	memories MemoryReader
	skills   SkillReader
	mcp      MCPRuntime
}

func NewService(memories MemoryReader, skillReader SkillReader, mcpRuntime MCPRuntime) *Service {
	return &Service{memories: memories, skills: skillReader, mcp: mcpRuntime}
}

func (service *Service) Resolve(ctx context.Context, principalID, agentID string) (conversation.AgentContext, error) {
	memories, err := service.memories.Context(ctx, principalID, agentID)
	if err != nil {
		return conversation.AgentContext{}, err
	}
	enabledSkills, err := service.skills.Enabled(ctx, principalID, agentID)
	if err != nil {
		return conversation.AgentContext{}, err
	}
	mcpTools, err := service.mcp.RuntimeTools(ctx, principalID, agentID)
	if err != nil {
		return conversation.AgentContext{}, err
	}
	resolved := conversation.AgentContext{
		Memories: make([]conversation.RuntimeMemory, 0, len(memories)),
		Skills:   make([]conversation.RuntimeSkill, 0, len(enabledSkills)),
		Tools:    make([]conversation.RuntimeTool, 0, len(mcpTools)),
	}
	for _, item := range memories {
		resolved.Memories = append(resolved.Memories, conversation.RuntimeMemory{
			ID: item.ID, Kind: string(item.Kind), Content: item.Content, Source: item.SourceURI,
		})
	}
	for _, item := range enabledSkills {
		skill := item
		resolvedSkill := conversation.RuntimeSkill{
			Name: skill.Name, Description: skill.Description, Content: skill.Content,
			Files: make([]conversation.RuntimeSkillFile, 0, len(skill.Files)),
			ReadResource: func(readContext context.Context, filePath string) (conversation.RuntimeSkillResource, error) {
				file, err := service.skills.ReadFile(readContext, principalID, agentID, skill.ID, filePath)
				if err != nil {
					return conversation.RuntimeSkillResource{}, err
				}
				return conversation.RuntimeSkillResource{
					Path: file.Path, MediaType: file.MediaType, Content: string(file.Content),
				}, nil
			},
		}
		for _, file := range skill.Files {
			resolvedSkill.Files = append(resolvedSkill.Files, conversation.RuntimeSkillFile{
				Path: file.Path, MediaType: file.MediaType, SizeBytes: file.SizeBytes, TextReadable: file.TextReadable,
			})
		}
		resolved.Skills = append(resolved.Skills, resolvedSkill)
	}
	for _, item := range mcpTools {
		tool := item
		resolved.Tools = append(resolved.Tools, conversation.RuntimeTool{
			Name: tool.QualifiedName, Description: tool.Description, InputSchema: tool.InputSchema,
			Invoke: func(callContext context.Context, arguments string) (string, error) {
				return service.mcp.Invoke(callContext, principalID, agentID, tool.ServerID, tool.Name, arguments)
			},
		})
	}
	return resolved, nil
}
