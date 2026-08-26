package impression

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type Worker struct {
	repository Repository
	curator    Curator
	logger     *slog.Logger
	lease      time.Duration
}

func NewWorker(repository Repository, curator Curator, logger *slog.Logger) *Worker {
	return &Worker{repository: repository, curator: curator, logger: logger, lease: 5 * time.Minute}
}

func (worker *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := worker.runOne(ctx); err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, context.Canceled) {
			worker.logger.Error("curate Agent run", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (worker *Worker) runOne(ctx context.Context) error {
	now := time.Now().UTC()
	job, err := worker.repository.ClaimJob(ctx, now, worker.lease)
	if err != nil {
		return err
	}
	input, err := worker.repository.LoadCurationInput(ctx, job)
	if err == nil {
		var result Curation
		result, err = worker.curator.Curate(ctx, input)
		if err == nil {
			err = worker.repository.ApplyCuration(ctx, job, result)
		}
	}
	if err == nil {
		return worker.repository.CompleteJob(ctx, job.ID)
	}
	nextAttempt := job.Attempts + 1
	if nextAttempt >= 5 {
		return worker.repository.FailJob(ctx, job.ID, nextAttempt, now, "curation_failed")
	}
	backoff := time.Duration(1<<min(nextAttempt, 6)) * 30 * time.Second
	if failErr := worker.repository.FailJob(ctx, job.ID, nextAttempt, now.Add(backoff), "curation_retry"); failErr != nil {
		return failErr
	}
	return err
}
