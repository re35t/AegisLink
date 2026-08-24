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
	ID          string          `json:"id"`
	ServerID    string          `json:"serverId"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Enabled     bool            `json:"enabled"`
	RiskLevel   RiskLevel       `json:"riskLevel"`
}

// Server is a user-owned MCP connection. Bound and Enabled are populated only
// when the record is projected for one Agent.
type Server struct {
	ID               string     `json:"id"`
	OwnerPrincipalID string     `json:"-"`
	Name             string     `json:"name"`
	Endpoint         string     `json:"endpoint"`
	Transport        string     `json:"transport"`
	Bound            bool       `json:"bound"`
	Enabled          bool       `json:"enabled"`
	Status           string     `json:"status"`
	ProtocolVersion  string     `json:"protocolVersion,omitempty"`
	LastError        string     `json:"lastError,omitempty"`
	LastCheckedAt    *time.Time `json:"lastCheckedAt,omitempty"`
	Tools            []Tool     `json:"tools"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type MentionGroup struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type Mention struct {
	ID             string       `json:"id"`
	Kind           string       `json:"kind"`
	Category       string       `json:"category"`
	Group          MentionGroup `json:"group"`
	Label          string       `json:"label"`
	Description    string       `json:"description"`
	Action         string       `json:"action"`
	Availability   string       `json:"availability"`
	DisabledReason string       `json:"disabledReason,omitempty"`
	ToolID         string       `json:"toolId"`
}

type MentionQuery struct {
	Query  string
	Kinds  []string
	Cursor string
	Limit  int
}

type MentionPage struct {
	Items      []Mention `json:"items"`
	NextCursor string    `json:"nextCursor,omitempty"`
}

type RuntimeTool struct {
	ToolID        string
	ServerID      string
	ServerName    string
	Name          string
	QualifiedName string
	Description   string
	InputSchema   json.RawMessage
}
