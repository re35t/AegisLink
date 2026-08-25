package catalog

type Group struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type Item struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Category       string `json:"category"`
	Group          Group  `json:"group"`
	Label          string `json:"label"`
	Description    string `json:"description"`
	Action         string `json:"action"`
	Availability   string `json:"availability"`
	DisabledReason string `json:"disabledReason,omitempty"`
	ResourceID     string `json:"resourceId"`
}

type Query struct {
	Query  string
	Kinds  []string
	Cursor string
	Limit  int
}

type Page struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}
