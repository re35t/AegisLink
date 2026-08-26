package impression

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceValidatesOwnerCorrections(t *testing.T) {
	repository := &repositoryStub{updated: Impression{ID: "impression-one"}}
	service := NewService(repository)
	summary := "  Corrected observation  "
	status := StatusDismissed
	_, err := service.Update(t.Context(), "owner", "agent", "impression-one", Update{ExpectedContextRevision: 2, Summary: &summary, Status: &status})
	if err != nil {
		t.Fatal(err)
	}
	if repository.update.Summary == nil || *repository.update.Summary != "Corrected observation" {
		t.Fatalf("update = %#v", repository.update)
	}
	invalid := StatusResolved
	if _, err = service.Update(t.Context(), "owner", "agent", "impression-one", Update{ExpectedContextRevision: 2, Status: &invalid}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
}

func TestFreshnessUsesDynamicHalfLifeAndExpiry(t *testing.T) {
	now := time.Now().UTC()
	observed := now.Add(-24 * time.Hour)
	expiry := now.Add(-time.Minute)
	item := Impression{LastObservedAt: observed, DecayHalfLifeSeconds: int64((24 * time.Hour).Seconds())}
	item.CalculateFreshness(now)
	if item.Freshness < .49 || item.Freshness > .51 {
		t.Fatalf("freshness = %f", item.Freshness)
	}
	item.ExpiresAt = &expiry
	item.CalculateFreshness(now)
	if item.Freshness != 0 {
		t.Fatalf("expired freshness = %f", item.Freshness)
	}
}

type repositoryStub struct {
	update  Update
	updated Impression
}

func (stub *repositoryStub) List(context.Context, string, string, string) ([]Impression, error) {
	return nil, nil
}
func (stub *repositoryStub) ListCandidates(context.Context, string, string, string) ([]FactCandidate, error) {
	return nil, nil
}
func (stub *repositoryStub) Update(_ context.Context, _, _, _ string, update Update) (Impression, error) {
	stub.update = update
	return stub.updated, nil
}
func (stub *repositoryStub) RejectCandidate(context.Context, string, string, string, int64) error {
	return nil
}
func (stub *repositoryStub) ApplyCuration(context.Context, Job, Curation) error { return nil }
func (stub *repositoryStub) ClaimJob(context.Context, time.Time, time.Duration) (Job, error) {
	return Job{}, ErrNotFound
}
func (stub *repositoryStub) CompleteJob(context.Context, string) error { return nil }
func (stub *repositoryStub) FailJob(context.Context, string, int, time.Time, string) error {
	return nil
}
func (stub *repositoryStub) LoadCurationInput(context.Context, Job) (CurationInput, error) {
	return CurationInput{}, nil
}
