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
	Files            []File    `json:"files"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type File struct {
	Path         string `json:"path"`
	MediaType    string `json:"mediaType"`
	SizeBytes    int64  `json:"sizeBytes"`
	ContentHash  string `json:"contentHash"`
	TextReadable bool   `json:"textReadable"`
	Content      []byte `json:"-"`
}

type ImportRequest struct {
	FileName string
	Data     []byte
	Version  string
}
