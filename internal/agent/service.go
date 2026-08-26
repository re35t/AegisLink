package agent

import (
	"context"
	"strings"
	"unicode/utf8"
)

const maximumSystemPromptLength = 32 * 1024

// Service owns Personal Agent queries and keeps owner scoping out of HTTP
// handlers and conversation orchestration.
type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) Bootstrap(ctx context.Context, principalID string) (Agent, error) {
	return service.Default(ctx, principalID)
}

func (service *Service) Default(ctx context.Context, principalID string) (Agent, error) {
	return service.repository.Default(ctx, principalID)
}

func (service *Service) List(ctx context.Context, principalID string) ([]Agent, error) {
	return service.repository.List(ctx, principalID)
}

func (service *Service) Get(ctx context.Context, principalID, id string) (Agent, error) {
	return service.repository.Get(ctx, principalID, id)
}

func (service *Service) GetInstructions(ctx context.Context, principalID, id string) (Instructions, error) {
	return service.repository.GetInstructions(ctx, principalID, id)
}

func (service *Service) UpdateInstructions(ctx context.Context, principalID, id string, update InstructionsUpdate) (Instructions, error) {
	if update.ExpectedVersion < 1 {
		return Instructions{}, ErrInvalidInstructions
	}
	update.SystemPrompt = strings.TrimSpace(update.SystemPrompt)
	if utf8.RuneCountInString(update.SystemPrompt) > maximumSystemPromptLength {
		return Instructions{}, ErrInvalidInstructions
	}
	return service.repository.UpdateInstructions(ctx, principalID, id, update)
}
