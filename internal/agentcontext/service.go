package agentcontext

import (
	"context"

	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/memory"
	"github.com/re35t/AegisLink/internal/skills"
)

type MemoryReader interface {
	Context(context.Context, string, string) ([]memory.Memory, error)
}

type SkillReader interface {
	Enabled(context.Context, string, string) ([]skills.Skill, error)
}

type MCPRuntime interface {
	RuntimeTools(context.Context, string, string) ([]mcp.RuntimeTool, error)
	Invoke(context.Context, string, string, string, string, string) (string, error)
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
		resolved.Skills = append(resolved.Skills, conversation.RuntimeSkill{
			Name: item.Name, Description: item.Description, Content: item.Content,
		})
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
