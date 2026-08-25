package catalog

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/skills"
)

const (
	mcpMentionPrefix       = "mcp-tool:"
	skillMentionPrefix     = "skill:"
	DiscoveryMentionID     = "discovery:agent-capabilities"
	DiscoveryResourceID    = "agent-capabilities"
	discoveryCategoryLabel = "AegisLink"
)

var categoryOrder = map[string]int{"mcp": 0, "skills": 1, "discovery": 2}

type AgentReader interface {
	Get(context.Context, string, string) (agent.Agent, error)
}

type MCPReader interface {
	List(context.Context, string, string) ([]mcp.Server, error)
}

type SkillReader interface {
	List(context.Context, string, string) ([]skills.Skill, error)
}

type Service struct {
	agents AgentReader
	mcp    MCPReader
	skills SkillReader
}

func NewService(agents AgentReader, mcpReader MCPReader, skillReader SkillReader) *Service {
	return &Service{agents: agents, mcp: mcpReader, skills: skillReader}
}

func (service *Service) List(ctx context.Context, principalID, agentID string, query Query) (Page, error) {
	if _, err := service.agents.Get(ctx, principalID, agentID); err != nil {
		return Page{}, err
	}
	wanted, err := normalizeKinds(query.Kinds)
	if err != nil || len(query.Query) > 200 {
		return Page{}, ErrInvalid
	}
	limit := query.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return Page{}, ErrInvalid
	}
	offset := 0
	if query.Cursor != "" {
		offset, err = strconv.Atoi(query.Cursor)
		if err != nil || offset < 0 {
			return Page{}, ErrInvalid
		}
	}
	needle := strings.ToLower(strings.TrimSpace(query.Query))
	items := make([]Item, 0)
	if wanted["mcp-tool"] {
		servers, err := service.mcp.List(ctx, principalID, agentID)
		if err != nil {
			return Page{}, err
		}
		for _, server := range servers {
			for _, tool := range server.Tools {
				if !matches(needle, server.Name, tool.Name, tool.Description) {
					continue
				}
				availability, reason := mcp.MentionAvailability(server, tool)
				items = append(items, Item{
					ID: mcpMentionPrefix + tool.ID, Kind: "mcp-tool", Category: "mcp",
					Group: Group{ID: server.ID, Kind: "mcp-plugin", Label: server.Name},
					Label: tool.Name, Description: tool.Description, Action: "force-tool-once",
					Availability: availability, DisabledReason: reason, ResourceID: tool.ID,
				})
			}
		}
	}
	if wanted["skill"] {
		skillItems, err := service.skills.List(ctx, principalID, agentID)
		if err != nil {
			return Page{}, err
		}
		for _, skill := range skillItems {
			if !matches(needle, skill.Name, skill.Description, skill.Version) {
				continue
			}
			availability := "ready"
			reason := ""
			if !skill.Enabled {
				availability = "skill-disabled"
				reason = "Enable this Skill for the current Agent"
			}
			items = append(items, Item{
				ID: skillMentionPrefix + skill.ID, Kind: "skill", Category: "skills",
				Group: Group{ID: agentID, Kind: "agent", Label: "Agent Skills"},
				Label: skill.Name, Description: skill.Description, Action: "use-skill-once",
				Availability: availability, DisabledReason: reason, ResourceID: skill.ID,
			})
		}
	}
	if wanted["discovery"] && matches(needle, "Discovery", "Agent capabilities", "Explore enabled MCP tools and Skills") {
		items = append(items, Item{
			ID: DiscoveryMentionID, Kind: "discovery", Category: "discovery",
			Group: Group{ID: "system", Kind: "system", Label: discoveryCategoryLabel},
			Label: "Agent capabilities", Description: "Discover enabled MCP tools and Skills for this request.",
			Action: "discover-once", Availability: "ready", ResourceID: DiscoveryResourceID,
		})
	}
	sort.SliceStable(items, func(left, right int) bool {
		if categoryOrder[items[left].Category] != categoryOrder[items[right].Category] {
			return categoryOrder[items[left].Category] < categoryOrder[items[right].Category]
		}
		leftKey := strings.ToLower(items[left].Group.Label + "\x00" + items[left].Label)
		rightKey := strings.ToLower(items[right].Group.Label + "\x00" + items[right].Label)
		return leftKey < rightKey
	})
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := Page{Items: items[offset:end]}
	if end < len(items) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

func normalizeKinds(kinds []string) (map[string]bool, error) {
	if len(kinds) == 0 {
		kinds = []string{"mcp-tool", "skill", "discovery"}
	}
	result := make(map[string]bool, len(kinds))
	for _, raw := range kinds {
		kind := strings.TrimSpace(raw)
		if kind != "mcp-tool" && kind != "skill" && kind != "discovery" {
			return nil, ErrInvalid
		}
		result[kind] = true
	}
	return result, nil
}

func matches(needle string, values ...string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.Join(values, " ")), needle)
}
