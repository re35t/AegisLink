package memory

import "time"

type Kind string

const (
	Semantic Kind = "semantic"
	Episodic Kind = "episodic"
)

type Memory struct {
	ID               string     `json:"id"`
	OwnerPrincipalID string     `json:"-"`
	AgentID          string     `json:"agentId"`
	Kind             Kind       `json:"kind"`
	Content          string     `json:"content"`
	Confidence       float64    `json:"confidence"`
	SourceURI        string     `json:"sourceUri"`
	Status           string     `json:"status"`
	LastConfirmedAt  *time.Time `json:"lastConfirmedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type Update struct {
	Kind       *Kind
	Content    *string
	Confidence *float64
	Confirmed  bool
}
