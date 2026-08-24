package mcp

import (
	"encoding/json"
	"time"
)

type RiskLevel string

const (
	ReadOnly      RiskLevel = "read-only"
	ExternalWrite RiskLevel = "external-write"
	Destructive   RiskLevel = "destructive"
)

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Enabled     bool            `json:"enabled"`
	RiskLevel   RiskLevel       `json:"riskLevel"`
}

type Server struct {
	ID               string     `json:"id"`
	OwnerPrincipalID string     `json:"-"`
	AgentID          string     `json:"agentId"`
	Name             string     `json:"name"`
	Endpoint         string     `json:"endpoint"`
	Transport        string     `json:"transport"`
	Enabled          bool       `json:"enabled"`
	Status           string     `json:"status"`
	ProtocolVersion  string     `json:"protocolVersion,omitempty"`
	LastError        string     `json:"lastError,omitempty"`
	LastCheckedAt    *time.Time `json:"lastCheckedAt,omitempty"`
	Tools            []Tool     `json:"tools"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type RuntimeTool struct {
	ServerID      string
	ServerName    string
	Name          string
	QualifiedName string
	Description   string
	InputSchema   json.RawMessage
}
