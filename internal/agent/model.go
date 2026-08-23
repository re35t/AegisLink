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
