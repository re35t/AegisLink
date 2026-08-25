package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/re35t/AegisLink/internal/conversation"
	"gorm.io/gorm"
)

func (repository *ConversationRepository) RecoverInterruptedRuns(ctx context.Context) error {
	type interruptedRun struct {
		ID          string
		PrincipalID string
	}
	runs := make([]interruptedRun, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT runs.id, conversations.owner_principal_id AS principal_id
		FROM runs
		JOIN conversations ON conversations.id=runs.conversation_id
		WHERE runs.status IN ('queued', 'running')
		ORDER BY runs.created_at`).Scan(&runs)
	if result.Error != nil {
		return fmt.Errorf("list interrupted runs: %w", result.Error)
	}
	for _, run := range runs {
		if err := repository.FailRun(ctx, run.PrincipalID, run.ID, "server_restarted", false); err != nil {
			return fmt.Errorf("recover run %s: %w", run.ID, err)
		}
	}
	return nil
}

func (repository *ConversationRepository) MarkRunRunning(ctx context.Context, principalID, runID string) error {
	result := exec(repository.database.WithContext(ctx), `
		UPDATE runs SET status='running', started_at=now()
		FROM conversations
		WHERE runs.id=@p2
		  AND runs.conversation_id=conversations.id
		  AND conversations.owner_principal_id=@p1
		  AND runs.status='queued'`, principalID, runID)
	if result.Error != nil {
		return fmt.Errorf("mark run running: %w", result.Error)
	}
	return requireChanged(result)
}

func (repository *ConversationRepository) AppendRunEvent(ctx context.Context, principalID, runID, eventType string, payload any) (conversation.RunEvent, error) {
	var event conversation.RunEvent
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var err error
		event, err = appendEvent(transaction, principalID, runID, eventType, payload)
		return err
	})
	if err != nil {
		return conversation.RunEvent{}, err
	}
	return event, nil
}

func (repository *ConversationRepository) CompleteRun(ctx context.Context, principalID, runID, conversationID, messageID, content string) (conversation.Message, error) {
	var message conversation.Message
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var lockedRun struct{ Status string }
		if err := scanOne(transaction, &lockedRun, `
			SELECT runs.status
			FROM runs
			JOIN conversations ON conversations.id=runs.conversation_id
			WHERE runs.id=@p2 AND conversations.owner_principal_id=@p1
			FOR UPDATE OF runs`, principalID, runID); err != nil {
			return mapNotFound("lock run", err)
		}
		if lockedRun.Status != "running" {
			return conversation.ErrRunNotActive
		}
		var lockedConversation struct{ ID string }
		if err := scanOne(transaction, &lockedConversation, `
			SELECT id FROM conversations
			WHERE id=@p2 AND owner_principal_id=@p1
			FOR UPDATE`, principalID, conversationID); err != nil {
			return mapNotFound("lock conversation", err)
		}
		message = conversation.Message{ID: messageID, ConversationID: conversationID, RunID: &runID, Role: "assistant", Content: content}
		var inserted struct {
			Sequence  int64
			CreatedAt time.Time
		}
		if err := scanOne(transaction, &inserted, `
			INSERT INTO messages (id, conversation_id, run_id, role, content, sequence)
			VALUES (@p1, @p2, @p3, 'assistant', @p4, COALESCE((SELECT MAX(sequence)+1 FROM messages WHERE conversation_id=@p2), 1))
			RETURNING sequence, created_at`, messageID, conversationID, runID, content); err != nil {
			return fmt.Errorf("insert assistant message: %w", err)
		}
		message.Sequence = inserted.Sequence
		message.CreatedAt = inserted.CreatedAt
		if _, err := appendEvent(transaction, principalID, runID, "message.completed", map[string]any{"message": message}); err != nil {
			return err
		}
		if result := exec(transaction, `UPDATE runs SET status='succeeded', finished_at=now() WHERE id=@p1`, runID); result.Error != nil {
			return fmt.Errorf("finish run: %w", result.Error)
		}
		if result := exec(transaction, `UPDATE conversations SET updated_at=now() WHERE id=@p1`, conversationID); result.Error != nil {
			return fmt.Errorf("touch conversation: %w", result.Error)
		}
		return nil
	})
	if err != nil {
		return conversation.Message{}, err
	}
	return message, nil
}

func (repository *ConversationRepository) FailRun(ctx context.Context, principalID, runID, code string, cancelled bool) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		status := "failed"
		eventType := "run.failed"
		if cancelled {
			status = "cancelled"
			eventType = "run.cancelled"
		}
		result := exec(transaction, `
			UPDATE runs SET status=@p3, failure_code=@p4, finished_at=now()
			FROM conversations
			WHERE runs.id=@p2
			  AND runs.conversation_id=conversations.id
			  AND conversations.owner_principal_id=@p1
			  AND runs.status IN ('queued', 'running')`, principalID, runID, status, code)
		if result.Error != nil {
			return fmt.Errorf("update failed run: %w", result.Error)
		}
		if err := requireChanged(result); err != nil {
			return err
		}
		_, err := appendEvent(transaction, principalID, runID, eventType, map[string]any{"code": code})
		return err
	})
}

func (repository *ConversationRepository) GetRun(ctx context.Context, principalID, runID string) (conversation.Run, error) {
	var row runRow
	err := scanOne(repository.database.WithContext(ctx), &row, `
		SELECT runs.id, runs.conversation_id, runs.status, runs.failure_code, runs.execution_policy,
		       runs.created_at, runs.started_at, runs.finished_at
		FROM runs
		JOIN conversations ON conversations.id=runs.conversation_id
		WHERE runs.id=@p2 AND conversations.owner_principal_id=@p1`, principalID, runID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return conversation.Run{}, conversation.ErrNotFound
	}
	if err != nil {
		return conversation.Run{}, fmt.Errorf("get run: %w", err)
	}
	run, err := row.value()
	if err != nil {
		return conversation.Run{}, fmt.Errorf("get run: %w", err)
	}
	return run, nil
}

func (repository *ConversationRepository) ListRunEvents(ctx context.Context, principalID, runID string, after int64) ([]conversation.RunEvent, error) {
	events := make([]conversation.RunEvent, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT run_events.run_id, run_events.sequence, run_events.event_type AS type,
		       run_events.payload, run_events.occurred_at
		FROM run_events
		JOIN runs ON runs.id=run_events.run_id
		JOIN conversations ON conversations.id=runs.conversation_id
		WHERE run_events.run_id=@p2
		  AND conversations.owner_principal_id=@p1
		  AND run_events.sequence>@p3
		ORDER BY run_events.sequence
		LIMIT 200`, principalID, runID, after).Scan(&events)
	if result.Error != nil {
		return nil, fmt.Errorf("list run events: %w", result.Error)
	}
	return events, nil
}

func appendEvent(transaction *gorm.DB, principalID, runID, eventType string, payload any) (conversation.RunEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return conversation.RunEvent{}, fmt.Errorf("encode run event: %w", err)
	}
	event := conversation.RunEvent{RunID: runID, Type: eventType, Payload: encoded}
	var inserted struct {
		Sequence   int64
		OccurredAt time.Time
	}
	err = scanOne(transaction, &inserted, `
		WITH allocated AS (
			UPDATE runs SET next_event_sequence=runs.next_event_sequence+1
			FROM conversations
			WHERE runs.id=@p2
			  AND runs.conversation_id=conversations.id
			  AND conversations.owner_principal_id=@p1
			RETURNING runs.next_event_sequence-1 AS sequence
		)
		INSERT INTO run_events (run_id, sequence, event_type, payload)
		SELECT @p2, sequence, @p3, @p4 FROM allocated
		RETURNING sequence, occurred_at`, principalID, runID, eventType, encoded)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return conversation.RunEvent{}, conversation.ErrNotFound
	}
	if err != nil {
		return conversation.RunEvent{}, fmt.Errorf("append run event: %w", err)
	}
	event.Sequence = inserted.Sequence
	event.OccurredAt = inserted.OccurredAt
	return event, nil
}

func requireChanged(result *gorm.DB) error {
	if result.RowsAffected == 0 {
		return conversation.ErrRunNotActive
	}
	return nil
}

func mapNotFound(action string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return conversation.ErrNotFound
	}
	return fmt.Errorf("%s: %w", action, err)
}
