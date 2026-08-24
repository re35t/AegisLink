package memory

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/agent"
)

const (
	maximumContentLength = 4000
	maximumSourceLength  = 1000
	contextLimit         = 20
)

type AgentReader interface {
	Get(context.Context, string, string) (agent.Agent, error)
}

type Service struct {
	repository Repository
	agents     AgentReader
}

func NewService(repository Repository, agents AgentReader) *Service {
	return &Service{repository: repository, agents: agents}
}

func (service *Service) List(ctx context.Context, principalID, agentID string) ([]Memory, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return nil, err
	}
	return service.repository.List(ctx, principalID, agentID)
}

func (service *Service) Create(ctx context.Context, principalID, agentID string, kind Kind, content, sourceURI string, confidence float64) (Memory, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Memory{}, err
	}
	content = strings.TrimSpace(content)
	sourceURI = strings.TrimSpace(sourceURI)
	if !validKind(kind) || content == "" || utf8.RuneCountInString(content) > maximumContentLength || utf8.RuneCountInString(sourceURI) > maximumSourceLength || confidence < 0 || confidence > 1 {
		return Memory{}, ErrInvalid
	}
	return service.repository.Create(ctx, Memory{
		ID: ulid.Make().String(), OwnerPrincipalID: principalID, AgentID: agentID,
		Kind: kind, Content: content, SourceURI: sourceURI, Confidence: confidence, Status: "active",
	})
}

func (service *Service) Update(ctx context.Context, principalID, agentID, memoryID string, update Update) (Memory, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Memory{}, err
	}
	if update.Kind != nil && !validKind(*update.Kind) {
		return Memory{}, ErrInvalid
	}
	if update.Content != nil {
		trimmed := strings.TrimSpace(*update.Content)
		if trimmed == "" || utf8.RuneCountInString(trimmed) > maximumContentLength {
			return Memory{}, ErrInvalid
		}
		update.Content = &trimmed
	}
	if update.Confidence != nil && (*update.Confidence < 0 || *update.Confidence > 1) {
		return Memory{}, ErrInvalid
	}
	if update.Kind == nil && update.Content == nil && update.Confidence == nil && !update.Confirmed {
		return Memory{}, ErrInvalid
	}
	return service.repository.Update(ctx, principalID, agentID, memoryID, update)
}

func (service *Service) Forget(ctx context.Context, principalID, agentID, memoryID string) error {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return err
	}
	return service.repository.Forget(ctx, principalID, agentID, memoryID)
}

func (service *Service) Context(ctx context.Context, principalID, agentID string) ([]Memory, error) {
	return service.repository.Context(ctx, principalID, agentID, contextLimit)
}

func (service *Service) authorize(ctx context.Context, principalID, agentID string) error {
	_, err := service.agents.Get(ctx, principalID, agentID)
	return err
}

func validKind(kind Kind) bool {
	return kind == Semantic || kind == Episodic
}
