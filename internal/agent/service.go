package agent

import "context"

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
