package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/re35t/AegisLink/internal/conversation"
)

func (repository *ConversationRepository) RecoverInterruptedRuns(ctx context.Context) error {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT runs.id, conversations.owner_principal_id
		FROM runs
		JOIN conversations ON conversations.id=runs.conversation_id
		WHERE runs.status IN ('queued', 'running')
		ORDER BY runs.created_at`)
	if err != nil {
		return fmt.Errorf("list interrupted runs: %w", err)
	}
	defer rows.Close()
	type interruptedRun struct {
		id          string
		principalID string
	}
	var runs []interruptedRun
	for rows.Next() {
		var run interruptedRun
		if err := rows.Scan(&run.id, &run.principalID); err != nil {
			return fmt.Errorf("scan interrupted run: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate interrupted runs: %w", err)
	}
	for _, run := range runs {
		if err := repository.FailRun(ctx, run.principalID, run.id, "server_restarted", false); err != nil {
			return fmt.Errorf("recover run %s: %w", run.id, err)
		}
	}
	return nil
}

func (repository *ConversationRepository) MarkRunRunning(ctx context.Context, principalID, runID string) error {
	result, err := repository.database.ExecContext(ctx, `
		UPDATE runs SET status='running', started_at=now()
		FROM conversations
		WHERE runs.id=$2
		  AND runs.conversation_id=conversations.id
		  AND conversations.owner_principal_id=$1
		  AND runs.status='queued'`, principalID, runID)
	if err != nil {
		return fmt.Errorf("mark run running: %w", err)
	}
	return requireChanged(result)
}

func (repository *ConversationRepository) AppendRunEvent(ctx context.Context, principalID, runID, eventType string, payload any) (conversation.RunEvent, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return conversation.RunEvent{}, fmt.Errorf("begin run event: %w", err)
	}
	defer transaction.Rollback()
	event, err := appendEvent(ctx, transaction, principalID, runID, eventType, payload)
	if err != nil {
		return conversation.RunEvent{}, err
	}
	if err := transaction.Commit(); err != nil {
		return conversation.RunEvent{}, fmt.Errorf("commit run event: %w", err)
	}
	return event, nil
}

func (repository *ConversationRepository) CompleteRun(ctx context.Context, principalID, runID, conversationID, messageID, content string) (conversation.Message, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return conversation.Message{}, fmt.Errorf("begin complete run: %w", err)
	}
	defer transaction.Rollback()
	var status string
	if err := transaction.QueryRowContext(ctx, `
		SELECT runs.status
		FROM runs
		JOIN conversations ON conversations.id=runs.conversation_id
		WHERE runs.id=$2 AND conversations.owner_principal_id=$1
		FOR UPDATE OF runs`, principalID, runID).Scan(&status); err != nil {
		return conversation.Message{}, mapNotFound("lock run", err)
	}
	if status != "running" {
		return conversation.Message{}, conversation.ErrRunNotActive
	}
	var lockedConversationID string
	if err := transaction.QueryRowContext(ctx, `
		SELECT id FROM conversations
		WHERE id=$2 AND owner_principal_id=$1
		FOR UPDATE`, principalID, conversationID).Scan(&lockedConversationID); err != nil {
		return conversation.Message{}, mapNotFound("lock conversation", err)
	}
	message := conversation.Message{ID: messageID, ConversationID: conversationID, RunID: &runID, Role: "assistant", Content: content}
	err = transaction.QueryRowContext(ctx, `
		INSERT INTO messages (id, conversation_id, run_id, role, content, sequence)
		VALUES ($1, $2, $3, 'assistant', $4, COALESCE((SELECT MAX(sequence)+1 FROM messages WHERE conversation_id=$2), 1))
		RETURNING sequence, created_at`, messageID, conversationID, runID, content).Scan(&message.Sequence, &message.CreatedAt)
	if err != nil {
		return conversation.Message{}, fmt.Errorf("insert assistant message: %w", err)
	}
	if _, err := appendEvent(ctx, transaction, principalID, runID, "message.completed", map[string]any{"message": message}); err != nil {
		return conversation.Message{}, err
	}
	if _, err := transaction.ExecContext(ctx, `UPDATE runs SET status='succeeded', finished_at=now() WHERE id=$1`, runID); err != nil {
		return conversation.Message{}, fmt.Errorf("finish run: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, `UPDATE conversations SET updated_at=now() WHERE id=$1`, conversationID); err != nil {
		return conversation.Message{}, fmt.Errorf("touch conversation: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return conversation.Message{}, fmt.Errorf("commit completed run: %w", err)
	}
	return message, nil
}

func (repository *ConversationRepository) FailRun(ctx context.Context, principalID, runID, code string, cancelled bool) error {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin fail run: %w", err)
	}
	defer transaction.Rollback()
	status := "failed"
	eventType := "run.failed"
	if cancelled {
		status = "cancelled"
		eventType = "run.cancelled"
	}
	result, err := transaction.ExecContext(ctx, `
		UPDATE runs SET status=$3, failure_code=$4, finished_at=now()
		FROM conversations
		WHERE runs.id=$2
		  AND runs.conversation_id=conversations.id
		  AND conversations.owner_principal_id=$1
		  AND runs.status IN ('queued', 'running')`, principalID, runID, status, code)
	if err != nil {
		return fmt.Errorf("update failed run: %w", err)
	}
	if err := requireChanged(result); err != nil {
		return err
	}
	if _, err := appendEvent(ctx, transaction, principalID, runID, eventType, map[string]any{"code": code}); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit failed run: %w", err)
	}
	return nil
}

func (repository *ConversationRepository) GetRun(ctx context.Context, principalID, runID string) (conversation.Run, error) {
	var run conversation.Run
	err := repository.database.QueryRowContext(ctx, `
		SELECT runs.id, runs.conversation_id, runs.status, runs.failure_code, runs.created_at, runs.started_at, runs.finished_at
		FROM runs
		JOIN conversations ON conversations.id=runs.conversation_id
		WHERE runs.id=$2 AND conversations.owner_principal_id=$1`, principalID, runID).Scan(
		&run.ID, &run.ConversationID, &run.Status, &run.FailureCode, &run.CreatedAt, &run.StartedAt, &run.FinishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.Run{}, conversation.ErrNotFound
	}
	if err != nil {
		return conversation.Run{}, fmt.Errorf("get run: %w", err)
	}
	return run, nil
}

func (repository *ConversationRepository) ListRunEvents(ctx context.Context, principalID, runID string, after int64) ([]conversation.RunEvent, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT run_events.run_id, run_events.sequence, run_events.event_type, run_events.payload, run_events.occurred_at
		FROM run_events
		JOIN runs ON runs.id=run_events.run_id
		JOIN conversations ON conversations.id=runs.conversation_id
		WHERE run_events.run_id=$2
		  AND conversations.owner_principal_id=$1
		  AND run_events.sequence>$3
		ORDER BY run_events.sequence
		LIMIT 200`, principalID, runID, after)
	if err != nil {
		return nil, fmt.Errorf("list run events: %w", err)
	}
	defer rows.Close()
	events := make([]conversation.RunEvent, 0)
	for rows.Next() {
		var event conversation.RunEvent
		if err := rows.Scan(&event.RunID, &event.Sequence, &event.Type, &event.Payload, &event.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan run event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func appendEvent(ctx context.Context, transaction *sql.Tx, principalID, runID, eventType string, payload any) (conversation.RunEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return conversation.RunEvent{}, fmt.Errorf("encode run event: %w", err)
	}
	var event conversation.RunEvent
	event.RunID = runID
	event.Type = eventType
	event.Payload = encoded
	err = transaction.QueryRowContext(ctx, `
		WITH allocated AS (
			UPDATE runs SET next_event_sequence=runs.next_event_sequence+1
			FROM conversations
			WHERE runs.id=$2
			  AND runs.conversation_id=conversations.id
			  AND conversations.owner_principal_id=$1
			RETURNING runs.next_event_sequence-1 AS sequence
		)
		INSERT INTO run_events (run_id, sequence, event_type, payload)
		SELECT $2, sequence, $3, $4 FROM allocated
		RETURNING sequence, occurred_at`, principalID, runID, eventType, encoded).Scan(&event.Sequence, &event.OccurredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.RunEvent{}, conversation.ErrNotFound
	}
	if err != nil {
		return conversation.RunEvent{}, fmt.Errorf("append run event: %w", err)
	}
	return event, nil
}

func requireChanged(result sql.Result) error {
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}
	if changed == 0 {
		return conversation.ErrRunNotActive
	}
	return nil
}

func mapNotFound(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.ErrNotFound
	}
	return fmt.Errorf("%s: %w", action, err)
}
