package conversation

import (
	"encoding/json"
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
	ID             string     `json:"id"`
	ConversationID string     `json:"conversationId"`
	Status         string     `json:"status"`
	FailureCode    *string    `json:"failureCode,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
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
