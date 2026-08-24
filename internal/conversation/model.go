package conversation

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type Conversation struct {
	ID               string    `json:"id"`
	OwnerPrincipalID string    `json:"-"`
	AgentID          string    `json:"agentId"`
	Title            string    `json:"title"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	RunID          *string   `json:"runId,omitempty"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Sequence       int64     `json:"sequence"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Run struct {
	ID              string          `json:"id"`
	ConversationID  string          `json:"conversationId"`
	Status          string          `json:"status"`
	FailureCode     *string         `json:"failureCode,omitempty"`
	ExecutionPolicy ExecutionPolicy `json:"executionPolicy"`
	CreatedAt       time.Time       `json:"createdAt"`
	StartedAt       *time.Time      `json:"startedAt,omitempty"`
	FinishedAt      *time.Time      `json:"finishedAt,omitempty"`
}

type ExecutionPolicy struct {
	Mode              string `json:"mode"`
	MentionID         string `json:"mentionId,omitempty"`
	ToolID            string `json:"toolId,omitempty"`
	ToolName          string `json:"toolName,omitempty"`
	QualifiedToolName string `json:"-"`
}

func (policy *ExecutionPolicy) Scan(value any) error {
	encoded, ok := value.([]byte)
	if !ok {
		if text, textOK := value.(string); textOK {
			encoded = []byte(text)
		} else {
			return fmt.Errorf("scan execution policy from %T", value)
		}
	}
	return json.Unmarshal(encoded, policy)
}

func (policy ExecutionPolicy) Value() (driver.Value, error) {
	return json.Marshal(policy)
}

func (run Run) Terminal() bool {
	return run.Status == "succeeded" || run.Status == "failed" || run.Status == "cancelled"
}

type RunEvent struct {
	RunID      string          `json:"runId"`
	Sequence   int64           `json:"sequence"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurredAt"`
}

type Detail struct {
	Conversation Conversation `json:"conversation"`
	Messages     []Message    `json:"messages"`
	ActiveRun    *Run         `json:"activeRun,omitempty"`
}
