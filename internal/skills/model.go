package skills

import "time"

type Skill struct {
	ID               string    `json:"id"`
	VersionID        string    `json:"versionId"`
	OwnerPrincipalID string    `json:"-"`
	AgentID          string    `json:"agentId"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Version          string    `json:"version"`
	SourceType       string    `json:"sourceType"`
	Content          string    `json:"content"`
	ContentHash      string    `json:"contentHash"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}
