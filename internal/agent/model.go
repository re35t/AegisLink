package agent

import "time"

type Agent struct {
	ID               string    `json:"id"`
	OwnerPrincipalID string    `json:"-"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	SystemPrompt     string    `json:"-"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type Instructions struct {
	AgentID      string    `json:"agentId"`
	SystemPrompt string    `json:"systemPrompt"`
	Version      int64     `json:"version"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type InstructionsUpdate struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	SystemPrompt    string `json:"systemPrompt"`
}
