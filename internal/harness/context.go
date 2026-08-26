package harness

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/impression"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/memory"
	"github.com/re35t/AegisLink/internal/runtime"
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

type FactReader interface {
	RuntimeFacts(context.Context, string, string) ([]agent.ConfirmedFact, error)
}

type ImpressionReader interface {
	List(context.Context, string, string, string) ([]impression.Impression, error)
}

type agentContext struct {
	memories    []runtimeMemory
	facts       []runtimeFact
	impressions []runtimeImpression
	skills      []runtimeSkill
	tools       []runtime.Tool
}

type runtimeMemory struct {
	id      string
	kind    string
	content string
	source  string
}

type runtimeFact struct {
	id        string
	subject   string
	namespace string
	key       string
	value     string
}

type runtimeImpression struct {
	id         string
	kind       string
	summary    string
	confidence float64
}

type runtimeSkill struct {
	id           string
	name         string
	description  string
	content      string
	files        []runtimeSkillFile
	readResource func(context.Context, string) (runtimeSkillResource, error)
}

type runtimeSkillFile struct {
	path         string
	mediaType    string
	sizeBytes    int64
	textReadable bool
}

type runtimeSkillResource struct {
	path      string
	mediaType string
	content   string
}

func (harness *Harness) resolveContext(ctx context.Context, principalID, agentID, currentMessage string) (agentContext, error) {
	memories, err := harness.memories.Context(ctx, principalID, agentID)
	if err != nil {
		return agentContext{}, err
	}
	enabledSkills, err := harness.skills.Enabled(ctx, principalID, agentID)
	if err != nil {
		return agentContext{}, err
	}
	mcpTools, err := harness.mcp.RuntimeTools(ctx, principalID, agentID)
	if err != nil {
		return agentContext{}, err
	}
	resolved := agentContext{
		memories: make([]runtimeMemory, 0, len(memories)), facts: []runtimeFact{}, impressions: []runtimeImpression{},
		skills: make([]runtimeSkill, 0, len(enabledSkills)), tools: make([]runtime.Tool, 0, len(mcpTools)),
	}
	facts, err := harness.facts.RuntimeFacts(ctx, principalID, agentID)
	if err != nil {
		return agentContext{}, err
	}
	for _, fact := range facts {
		encoded, _ := json.Marshal(fact.Value)
		resolved.facts = append(resolved.facts, runtimeFact{id: fact.ID, subject: string(fact.Subject), namespace: fact.Namespace, key: fact.Key, value: string(encoded)})
	}
	items, err := harness.impressions.List(ctx, principalID, agentID, string(impression.StatusActive))
	if err != nil {
		return agentContext{}, err
	}
	query := strings.ToLower(currentMessage)
	sort.SliceStable(items, func(i, j int) bool { return impressionScore(items[i], query) > impressionScore(items[j], query) })
	for _, item := range items {
		if item.Freshness <= 0 || len(resolved.impressions) >= 12 {
			continue
		}
		resolved.impressions = append(resolved.impressions, runtimeImpression{id: item.ID, kind: string(item.Kind), summary: item.Summary, confidence: item.Confidence})
	}
	for _, item := range memories {
		resolved.memories = append(resolved.memories, runtimeMemory{id: item.ID, kind: string(item.Kind), content: item.Content, source: item.SourceURI})
	}
	for _, item := range enabledSkills {
		skill := item
		resolvedSkill := runtimeSkill{
			id: skill.ID, name: skill.Name, description: skill.Description, content: skill.Content,
			files: make([]runtimeSkillFile, 0, len(skill.Files)),
			readResource: func(readContext context.Context, filePath string) (runtimeSkillResource, error) {
				file, err := harness.skills.ReadFile(readContext, principalID, agentID, skill.ID, filePath)
				if err != nil {
					return runtimeSkillResource{}, err
				}
				return runtimeSkillResource{path: file.Path, mediaType: file.MediaType, content: string(file.Content)}, nil
			},
		}
		for _, file := range skill.Files {
			resolvedSkill.files = append(resolvedSkill.files, runtimeSkillFile{path: file.Path, mediaType: file.MediaType, sizeBytes: file.SizeBytes, textReadable: file.TextReadable})
		}
		resolved.skills = append(resolved.skills, resolvedSkill)
	}
	for _, item := range mcpTools {
		tool := item
		resolved.tools = append(resolved.tools, runtime.Tool{
			Name: tool.QualifiedName, Description: tool.Description, InputSchema: tool.InputSchema,
			Invoke: func(callContext context.Context, arguments string) (string, error) {
				return harness.mcp.Invoke(callContext, principalID, agentID, tool.ServerID, tool.Name, arguments)
			},
		})
	}
	return resolved, nil
}

func impressionScore(item impression.Impression, query string) float64 {
	score := item.Confidence * item.Salience * item.Freshness
	if query != "" {
		for _, token := range strings.Fields(strings.ToLower(item.Summary)) {
			if len(token) >= 3 && strings.Contains(query, token) {
				return score + 1
			}
		}
	}
	return score
}
