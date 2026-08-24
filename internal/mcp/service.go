package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/agent"
)

const mentionPrefix = "mcp-tool:"

var toolNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

type AgentReader interface {
	Get(context.Context, string, string) (agent.Agent, error)
}

type Service struct {
	repository Repository
	agents     AgentReader
	client     Client
}

func NewService(repository Repository, agents AgentReader, client Client) *Service {
	return &Service{repository: repository, agents: agents, client: client}
}

func (service *Service) ListLibrary(ctx context.Context, principalID string) ([]Server, error) {
	return service.repository.ListLibrary(ctx, principalID)
}

func (service *Service) List(ctx context.Context, principalID, agentID string) ([]Server, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return nil, err
	}
	return service.repository.ListForAgent(ctx, principalID, agentID)
}

func (service *Service) Create(ctx context.Context, principalID, agentID, name, endpoint string) (Server, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Server{}, err
	}
	name = strings.TrimSpace(name)
	endpoint = strings.TrimSpace(endpoint)
	if !validServer(name, endpoint, service.client) {
		return Server{}, ErrInvalid
	}
	return service.repository.Create(ctx, Server{
		ID: ulid.Make().String(), OwnerPrincipalID: principalID, Name: name, Endpoint: endpoint,
		Transport: "streamable-http", Bound: true, Enabled: true, Status: "unchecked", Tools: []Tool{},
	}, agentID)
}

func (service *Service) Update(ctx context.Context, principalID, serverID, name, endpoint string) (Server, error) {
	name = strings.TrimSpace(name)
	endpoint = strings.TrimSpace(endpoint)
	if !validServer(name, endpoint, service.client) {
		return Server{}, ErrInvalid
	}
	return service.repository.Update(ctx, principalID, serverID, name, endpoint)
}

func (service *Service) BindServer(ctx context.Context, principalID, agentID, serverID string) (Server, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Server{}, err
	}
	return service.repository.BindServer(ctx, principalID, agentID, serverID)
}

func (service *Service) UnbindServer(ctx context.Context, principalID, agentID, serverID string) error {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return err
	}
	return service.repository.UnbindServer(ctx, principalID, agentID, serverID)
}

func (service *Service) Delete(ctx context.Context, principalID, serverID string) error {
	return service.repository.Delete(ctx, principalID, serverID)
}

func (service *Service) Refresh(ctx context.Context, principalID, serverID string) (Server, error) {
	server, err := service.repository.Get(ctx, principalID, serverID)
	if err != nil {
		return Server{}, err
	}
	tools, protocol, err := service.client.Discover(ctx, server.Endpoint)
	if err != nil {
		message := err.Error()
		if len(message) > 1000 {
			message = message[:1000]
		}
		_ = service.repository.MarkError(ctx, principalID, serverID, message)
		return Server{}, fmt.Errorf("%w: %v", ErrConnection, err)
	}
	return service.repository.ReplaceTools(ctx, principalID, serverID, tools, protocol)
}

func (service *Service) UpdateToolRisk(ctx context.Context, principalID, serverID, toolID string, risk RiskLevel) (Server, error) {
	if !validRisk(risk) {
		return Server{}, ErrInvalid
	}
	return service.repository.UpdateToolRisk(ctx, principalID, serverID, toolID, risk)
}

func (service *Service) BindTool(ctx context.Context, principalID, agentID, toolID string) (Tool, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Tool{}, err
	}
	server, tool, err := service.repository.GetToolForAgent(ctx, principalID, agentID, toolID)
	if err != nil {
		return Tool{}, err
	}
	if server.Status != "connected" {
		return Tool{}, ErrUnavailable
	}
	if tool.RiskLevel != ReadOnly {
		return Tool{}, ErrForbidden
	}
	return service.repository.BindTool(ctx, principalID, agentID, toolID)
}

func (service *Service) UnbindTool(ctx context.Context, principalID, agentID, toolID string) error {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return err
	}
	return service.repository.UnbindTool(ctx, principalID, agentID, toolID)
}

func (service *Service) Mentions(ctx context.Context, principalID, agentID string, query MentionQuery) (MentionPage, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return MentionPage{}, err
	}
	for _, kind := range query.Kinds {
		if strings.TrimSpace(kind) != "mcp-tool" {
			return MentionPage{}, ErrInvalid
		}
	}
	if len(query.Query) > 200 {
		return MentionPage{}, ErrInvalid
	}
	limit := query.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return MentionPage{}, ErrInvalid
	}
	offset := 0
	if query.Cursor != "" {
		parsed, err := strconv.Atoi(query.Cursor)
		if err != nil || parsed < 0 {
			return MentionPage{}, ErrInvalid
		}
		offset = parsed
	}
	servers, err := service.repository.ListForAgent(ctx, principalID, agentID)
	if err != nil {
		return MentionPage{}, err
	}
	needle := strings.ToLower(strings.TrimSpace(query.Query))
	items := make([]Mention, 0)
	for _, server := range servers {
		for _, tool := range server.Tools {
			if needle != "" && !strings.Contains(strings.ToLower(server.Name+" "+tool.Name+" "+tool.Description), needle) {
				continue
			}
			availability, reason := mentionAvailability(server, tool)
			items = append(items, Mention{
				ID: mentionPrefix + tool.ID, Kind: "mcp-tool", Category: "tools",
				Group: MentionGroup{ID: server.ID, Kind: "mcp-plugin", Label: server.Name},
				Label: tool.Name, Description: tool.Description, Action: "force-tool-once",
				Availability: availability, DisabledReason: reason, ToolID: tool.ID,
			})
		}
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := MentionPage{Items: items[offset:end]}
	if end < len(items) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

func (service *Service) ResolveMention(ctx context.Context, principalID, agentID, mentionID string) (RuntimeTool, error) {
	toolID, ok := strings.CutPrefix(mentionID, mentionPrefix)
	if !ok || toolID == "" {
		return RuntimeTool{}, ErrInvalid
	}
	server, tool, err := service.repository.GetToolForAgent(ctx, principalID, agentID, toolID)
	if err != nil {
		return RuntimeTool{}, err
	}
	if !server.Bound || !server.Enabled || server.Status != "connected" || !tool.Enabled {
		return RuntimeTool{}, ErrUnavailable
	}
	if tool.RiskLevel != ReadOnly {
		return RuntimeTool{}, ErrForbidden
	}
	resolved := RuntimeTool{
		ToolID: tool.ID, ServerID: server.ID, ServerName: server.Name, Name: tool.Name,
		Description: tool.Description, InputSchema: tool.InputSchema,
	}
	resolved.QualifiedName = qualifiedToolName(resolved)
	return resolved, nil
}

func (service *Service) RuntimeTools(ctx context.Context, principalID, agentID string) ([]RuntimeTool, error) {
	tools, err := service.repository.RuntimeTools(ctx, principalID, agentID)
	if err != nil {
		return nil, err
	}
	for index := range tools {
		tools[index].QualifiedName = qualifiedToolName(tools[index])
	}
	return tools, nil
}

func qualifiedToolName(runtimeTool RuntimeTool) string {
	serverPart := strings.Trim(toolNameSanitizer.ReplaceAllString(runtimeTool.ServerName, "_"), "_")
	toolPart := strings.Trim(toolNameSanitizer.ReplaceAllString(runtimeTool.Name, "_"), "_")
	if serverPart == "" {
		serverPart = "server"
	}
	if toolPart == "" {
		toolPart = "tool"
	}
	digest := sha256.Sum256([]byte(runtimeTool.ServerID + "\x00" + runtimeTool.Name))
	suffix := hex.EncodeToString(digest[:4])
	base := "mcp__" + serverPart + "__" + toolPart
	const maximumBaseLength = 64 - 2 - 8
	if len(base) > maximumBaseLength {
		base = base[:maximumBaseLength]
	}
	return base + "__" + suffix
}

func (service *Service) Invoke(ctx context.Context, principalID, agentID, serverID, toolName, arguments string) (string, error) {
	server, tool, err := service.repository.RuntimeTarget(ctx, principalID, agentID, serverID, toolName)
	if err != nil {
		return "", err
	}
	if !server.Bound || !server.Enabled || server.Status != "connected" || !tool.Enabled || tool.RiskLevel != ReadOnly {
		return "", ErrForbidden
	}
	return service.client.Invoke(ctx, server.Endpoint, tool.Name, arguments)
}

func (service *Service) authorize(ctx context.Context, principalID, agentID string) error {
	_, err := service.agents.Get(ctx, principalID, agentID)
	return err
}

func validServer(name, endpoint string, client Client) bool {
	return name != "" && utf8.RuneCountInString(name) <= 80 && len(endpoint) <= 2048 && client.ValidateEndpoint(endpoint) == nil
}

func validRisk(risk RiskLevel) bool {
	return risk == ReadOnly || risk == ExternalWrite || risk == Destructive
}

func mentionAvailability(server Server, tool Tool) (string, string) {
	if server.Status != "connected" {
		return "server-offline", "Connect and discover this MCP Server first"
	}
	if tool.RiskLevel != ReadOnly {
		return "approval-required", "Write and destructive tools require a future approval flow"
	}
	if !server.Bound || !server.Enabled {
		return "needs-agent-enable", "Enable this plugin for the current Agent"
	}
	if !tool.Enabled {
		return "tool-disabled", "Enable this tool for the current Agent"
	}
	return "ready", ""
}
