package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/agent"
)

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

func (service *Service) List(ctx context.Context, principalID, agentID string) ([]Server, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return nil, err
	}
	return service.repository.List(ctx, principalID, agentID)
}

func (service *Service) Create(ctx context.Context, principalID, agentID, name, endpoint string) (Server, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Server{}, err
	}
	name = strings.TrimSpace(name)
	endpoint = strings.TrimSpace(endpoint)
	if name == "" || utf8.RuneCountInString(name) > 80 || len(endpoint) > 2048 || service.client.ValidateEndpoint(endpoint) != nil {
		return Server{}, ErrInvalid
	}
	return service.repository.Create(ctx, Server{
		ID: ulid.Make().String(), OwnerPrincipalID: principalID, AgentID: agentID,
		Name: name, Endpoint: endpoint, Transport: "streamable-http", Enabled: true, Status: "unchecked", Tools: []Tool{},
	})
}

func (service *Service) SetEnabled(ctx context.Context, principalID, agentID, serverID string, enabled bool) (Server, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Server{}, err
	}
	return service.repository.SetEnabled(ctx, principalID, agentID, serverID, enabled)
}

func (service *Service) Delete(ctx context.Context, principalID, agentID, serverID string) error {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return err
	}
	return service.repository.Delete(ctx, principalID, agentID, serverID)
}

func (service *Service) Refresh(ctx context.Context, principalID, agentID, serverID string) (Server, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Server{}, err
	}
	server, err := service.repository.Get(ctx, principalID, agentID, serverID)
	if err != nil {
		return Server{}, err
	}
	tools, protocol, err := service.client.Discover(ctx, server.Endpoint)
	if err != nil {
		message := err.Error()
		if len(message) > 1000 {
			message = message[:1000]
		}
		_ = service.repository.MarkError(ctx, principalID, agentID, serverID, message)
		return Server{}, fmt.Errorf("%w: %v", ErrConnection, err)
	}
	return service.repository.ReplaceTools(ctx, principalID, agentID, serverID, tools, protocol)
}

func (service *Service) UpdateTool(ctx context.Context, principalID, agentID, serverID, toolName string, enabled bool, risk RiskLevel) (Server, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Server{}, err
	}
	if risk != ReadOnly && risk != ExternalWrite && risk != Destructive {
		return Server{}, ErrInvalid
	}
	return service.repository.UpdateTool(ctx, principalID, agentID, serverID, toolName, enabled, risk)
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
	if !server.Enabled || server.Status != "connected" || !tool.Enabled || tool.RiskLevel != ReadOnly {
		return "", ErrForbidden
	}
	return service.client.Invoke(ctx, server.Endpoint, tool.Name, arguments)
}

func (service *Service) authorize(ctx context.Context, principalID, agentID string) error {
	_, err := service.agents.Get(ctx, principalID, agentID)
	return err
}
