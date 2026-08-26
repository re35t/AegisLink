package impression

import (
	"context"
	"strings"
	"unicode/utf8"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) List(ctx context.Context, principalID, agentID, status string) ([]Impression, error) {
	return service.repository.List(ctx, principalID, agentID, status)
}

func (service *Service) ListCandidates(ctx context.Context, principalID, agentID, status string) ([]FactCandidate, error) {
	return service.repository.ListCandidates(ctx, principalID, agentID, status)
}

func (service *Service) Update(ctx context.Context, principalID, agentID, impressionID string, update Update) (Impression, error) {
	if update.ExpectedContextRevision < 1 || (update.Summary == nil && update.Details == nil && update.Status == nil) {
		return Impression{}, ErrInvalid
	}
	if update.Summary != nil {
		value := strings.TrimSpace(*update.Summary)
		if value == "" || utf8.RuneCountInString(value) > 4000 {
			return Impression{}, ErrInvalid
		}
		update.Summary = &value
	}
	if update.Status != nil && *update.Status != StatusActive && *update.Status != StatusDismissed {
		return Impression{}, ErrInvalid
	}
	return service.repository.Update(ctx, principalID, agentID, impressionID, update)
}

func (service *Service) RejectCandidate(ctx context.Context, principalID, agentID, candidateID string, expectedVersion int64) error {
	if expectedVersion < 1 {
		return ErrInvalid
	}
	return service.repository.RejectCandidate(ctx, principalID, agentID, candidateID, expectedVersion)
}
