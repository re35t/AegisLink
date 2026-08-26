package impression

import (
	"context"
	"errors"
	"math"
	"time"
)

var (
	ErrNotFound = errors.New("impression resource not found")
	ErrInvalid  = errors.New("invalid impression")
	ErrConflict = errors.New("impression revision conflict")
)

type EvidenceKind string

const (
	EvidenceMessage    EvidenceKind = "message"
	EvidenceRun        EvidenceKind = "run"
	EvidenceToolResult EvidenceKind = "tool-result"
	EvidenceMemory     EvidenceKind = "memory"
	EvidenceImpression EvidenceKind = "impression"
	EvidenceLegacy     EvidenceKind = "legacy"
)

type EvidenceRef struct {
	Kind       EvidenceKind `json:"kind"`
	ID         string       `json:"id"`
	Digest     string       `json:"digest,omitempty"`
	ObservedAt time.Time    `json:"observedAt"`
}

type GenerationInfo struct {
	Model         string    `json:"model"`
	RunID         string    `json:"runId"`
	PromptVersion string    `json:"promptVersion"`
	GeneratedAt   time.Time `json:"generatedAt"`
}

type Scope string

const (
	ScopeUser         Scope = "user"
	ScopeTask         Scope = "task"
	ScopeProject      Scope = "project"
	ScopeEnvironment  Scope = "environment"
	ScopeRelationship Scope = "relationship"
)

type Kind string

const (
	KindCurrentTask       Kind = "current-task"
	KindRecentInterest    Kind = "recent-interest"
	KindKnowledgeExposure Kind = "knowledge-exposure"
	KindAcquiredInfo      Kind = "acquired-information"
	KindOpenLoop          Kind = "open-loop"
	KindTemporaryPref     Kind = "temporary-preference"
	KindWorkingStyle      Kind = "working-style-observation"
	KindRecentDecision    Kind = "recent-decision"
)

type Status string

const (
	StatusActive     Status = "active"
	StatusResolved   Status = "resolved"
	StatusStale      Status = "stale"
	StatusSuperseded Status = "superseded"
	StatusDismissed  Status = "dismissed"
)

type Impression struct {
	ID                   string         `json:"id"`
	Scope                Scope          `json:"scope"`
	Kind                 Kind           `json:"kind"`
	Summary              string         `json:"summary"`
	Details              map[string]any `json:"details"`
	Tags                 []string       `json:"tags"`
	Evidence             []EvidenceRef  `json:"evidence"`
	Confidence           float64        `json:"confidence"`
	Salience             float64        `json:"salience"`
	Freshness            float64        `json:"freshness"`
	FirstObservedAt      time.Time      `json:"firstObservedAt"`
	LastObservedAt       time.Time      `json:"lastObservedAt"`
	ExpiresAt            *time.Time     `json:"expiresAt,omitempty"`
	DecayHalfLifeSeconds int64          `json:"decayHalfLifeSeconds"`
	Status               Status         `json:"status"`
	SupersededByID       *string        `json:"supersededById,omitempty"`
	Generation           GenerationInfo `json:"generation"`
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
}

func (item *Impression) CalculateFreshness(now time.Time) {
	if item.DecayHalfLifeSeconds <= 0 {
		item.DecayHalfLifeSeconds = int64((30 * 24 * time.Hour).Seconds())
	}
	age := now.Sub(item.LastObservedAt)
	if age <= 0 {
		item.Freshness = 1
		return
	}
	item.Freshness = math.Pow(0.5, age.Seconds()/float64(item.DecayHalfLifeSeconds))
	if item.ExpiresAt != nil && !now.Before(*item.ExpiresAt) {
		item.Freshness = 0
	}
}

type FactSubject string

const (
	FactAgent   FactSubject = "agent"
	FactUser    FactSubject = "user"
	FactProject FactSubject = "project"
	FactTask    FactSubject = "task"
)

type FactCandidate struct {
	ID                  string         `json:"id"`
	Subject             FactSubject    `json:"subject"`
	Namespace           string         `json:"namespace"`
	Key                 string         `json:"key"`
	Value               map[string]any `json:"value"`
	SourceImpressionIDs []string       `json:"sourceImpressionIds"`
	Rationale           string         `json:"rationale"`
	Confidence          float64        `json:"confidence"`
	Version             int64          `json:"version"`
	Status              string         `json:"status"`
	ProposedAt          time.Time      `json:"proposedAt"`
	ReviewedAt          *time.Time     `json:"reviewedAt,omitempty"`
}

type Update struct {
	ExpectedContextRevision int64          `json:"expectedContextRevision"`
	Summary                 *string        `json:"summary,omitempty"`
	Details                 map[string]any `json:"details,omitempty"`
	Status                  *Status        `json:"status,omitempty"`
}

type Repository interface {
	List(context.Context, string, string, string) ([]Impression, error)
	ListCandidates(context.Context, string, string, string) ([]FactCandidate, error)
	Update(context.Context, string, string, string, Update) (Impression, error)
	RejectCandidate(context.Context, string, string, string, int64) error
	ApplyCuration(context.Context, Job, Curation) error
	ClaimJob(context.Context, time.Time, time.Duration) (Job, error)
	CompleteJob(context.Context, string) error
	FailJob(context.Context, string, int, time.Time, string) error
	LoadCurationInput(context.Context, Job) (CurationInput, error)
}

type Job struct {
	ID               string
	OwnerPrincipalID string
	AgentID          string
	SourceRunID      string
	Attempts         int
}

type SourceMessage struct {
	ID      string    `json:"id"`
	Role    string    `json:"role"`
	Content string    `json:"content"`
	At      time.Time `json:"at"`
}

type SourceMemory struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
}

type SourceToolResult struct {
	EventID string `json:"eventId"`
	Type    string `json:"type"`
	Summary string `json:"summary"`
}

type CurationInput struct {
	AgentID             string             `json:"agentId"`
	RunID               string             `json:"runId"`
	Messages            []SourceMessage    `json:"messages"`
	Memories            []SourceMemory     `json:"memories"`
	ToolResults         []SourceToolResult `json:"toolResults"`
	ExistingImpressions []Impression       `json:"existingImpressions"`
}

type ImpressionDraft struct {
	Action           string
	TargetID         string
	Scope            Scope
	Kind             Kind
	Summary          string
	Details          map[string]any
	Tags             []string
	Confidence       float64
	Salience         float64
	SourceMessageIDs []string
	SourceMemoryIDs  []string
}

type FactDraft struct {
	Subject             FactSubject
	Namespace           string
	Key                 string
	Value               map[string]any
	Rationale           string
	Confidence          float64
	SourceImpressionIDs []string
}

type Curation struct {
	Generation  GenerationInfo
	Impressions []ImpressionDraft
	Facts       []FactDraft
}

type Curator interface {
	Curate(context.Context, CurationInput) (Curation, error)
}
