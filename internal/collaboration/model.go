package collaboration

import (
	"context"
	"errors"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/re35t/AegisLink/internal/agent"
)

const (
	ScopeMessageSend = "a2a:message:send"
	ScopeTaskGet     = "a2a:task:get"
	ScopeTaskCancel  = "a2a:task:cancel"
)

var (
	ErrNotFound       = errors.New("collaboration resource not found")
	ErrForbidden      = errors.New("collaboration is not authorized")
	ErrInvalid        = errors.New("invalid collaboration request")
	ErrConflict       = errors.New("collaboration state conflict")
	ErrRateLimited    = errors.New("collaboration request rate limited")
	ErrUnavailable    = errors.New("collaboration is unavailable")
	ErrEvaluationDeny = errors.New("target Agent rejected assistance")
)

type Policy struct {
	AgentID              string    `json:"agentId"`
	Enabled              bool      `json:"enabled"`
	Revision             int64     `json:"revision"`
	MaxSessionTTLSeconds int64     `json:"maxSessionTtlSeconds"`
	MaxRequestsPerHour   int       `json:"maxRequestsPerHour"`
	MaxActiveSessions    int       `json:"maxActiveSessions"`
	EncryptionReady      bool      `json:"encryptionReady" gorm:"-"`
	UpdatedAt            time.Time `json:"updatedAt"`
	OwnerPrincipalID     string    `json:"-"`
}

type PolicyUpdate struct {
	ExpectedRevision     int64
	Enabled              bool
	MaxSessionTTLSeconds int64
	MaxRequestsPerHour   int
	MaxActiveSessions    int
}

type Target struct {
	AgentID          string
	OwnerPrincipalID string
	AgentAddr        string
}

type AssistanceRequest struct {
	ID                        string     `json:"id"`
	RequesterOwnerPrincipalID string     `json:"-"`
	RequesterAgentID          string     `json:"requesterAgentId"`
	TargetOwnerPrincipalID    string     `json:"-"`
	TargetAgentID             string     `json:"targetAgentId"`
	TargetAgentAddr           string     `json:"targetAgentAddr"`
	Purpose                   string     `json:"purpose"`
	RequestedScopes           []string   `json:"requestedScopes"`
	IdempotencyKeyHash        []byte     `json:"-"`
	RequestDigest             []byte     `json:"-"`
	Status                    string     `json:"status"`
	DecisionCode              string     `json:"decisionCode,omitempty"`
	SessionID                 *string    `json:"sessionId,omitempty"`
	CreatedAt                 time.Time  `json:"createdAt"`
	EvaluatedAt               *time.Time `json:"evaluatedAt,omitempty"`
}

type Session struct {
	ID                        string     `json:"id"`
	AssistanceRequestID       string     `json:"assistanceRequestId"`
	RequesterOwnerPrincipalID string     `json:"-"`
	RequesterAgentID          string     `json:"requesterAgentId"`
	TargetOwnerPrincipalID    string     `json:"-"`
	TargetAgentID             string     `json:"targetAgentId"`
	TargetAgentAddr           string     `json:"targetAgentAddr"`
	Status                    string     `json:"status"`
	Scopes                    []string   `json:"scopes"`
	TokenHash                 []byte     `json:"-"`
	EncryptedToken            []byte     `json:"-"`
	TokenNonce                []byte     `json:"-"`
	ExpiresAt                 time.Time  `json:"expiresAt"`
	LastUsedAt                *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt                 time.Time  `json:"createdAt"`
	RevokedAt                 *time.Time `json:"revokedAt,omitempty"`
}

type Invocation struct {
	ID                  string
	Kind                string
	AssistanceRequestID *string
	SessionID           *string
	TaskID              *string
	TargetAgentID       string
	Status              string
	FailureCode         string
	StartedAt           time.Time
	FinishedAt          *time.Time
}

type Overview struct {
	Policy   Policy              `json:"policy"`
	Requests []AssistanceRequest `json:"requests"`
	Sessions []Session           `json:"sessions"`
}

type Repository interface {
	ResolveTarget(context.Context, string) (Target, error)
	GetPolicy(context.Context, string, string) (Policy, error)
	UpdatePolicy(context.Context, string, string, PolicyUpdate) (Policy, error)
	CreateRequest(context.Context, AssistanceRequest, Policy) (AssistanceRequest, bool, error)
	AcceptRequest(context.Context, AssistanceRequest, Session) error
	RejectRequest(context.Context, string, string, string) error
	GetOwnedSession(context.Context, string, string, string) (Session, error)
	AuthenticateSession(context.Context, []byte, string, string) (Session, error)
	ListOverview(context.Context, string, string) (Overview, error)
	RevokeSession(context.Context, string, string, string) error
	StartInvocation(context.Context, Invocation) error
	FinishInvocation(context.Context, string, string, string) error
	LoadContextTasks(context.Context, string, int) ([]*a2a.Task, error)
}

type AgentReader interface {
	Get(context.Context, string, string) (agent.Agent, error)
}

type ProfileReader interface {
	Get(context.Context, string, string) (agent.Profile, error)
}

type Runtime interface {
	Run(context.Context, RuntimeInput) <-chan RuntimeEvent
}

type RuntimeInput struct {
	AgentName        string
	AgentDescription string
	Instruction      string
	Messages         []RuntimeMessage
}

type RuntimeMessage struct {
	Role    string
	Content string
}

type RuntimeEvent struct {
	Delta string
	Err   error
}

type A2AHandler interface {
	SendMessage(context.Context, *a2a.SendMessageRequest) (a2a.SendMessageResult, error)
	GetTask(context.Context, *a2a.GetTaskRequest) (*a2a.Task, error)
}
