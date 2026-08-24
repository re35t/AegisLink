package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/conversation"
)

type ConversationRepository struct {
	database *sql.DB
}

var _ conversation.Repository = (*ConversationRepository)(nil)

func NewConversationRepository(database *sql.DB) *ConversationRepository {
	return &ConversationRepository{database: database}
}

func (repository *ConversationRepository) Ping(ctx context.Context) error {
	return repository.database.PingContext(ctx)
}

func (repository *ConversationRepository) ListConversations(ctx context.Context, ownerID string) ([]conversation.Conversation, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, owner_principal_id, agent_id, title, created_at, updated_at
		FROM conversations WHERE owner_principal_id=$1 ORDER BY updated_at DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()
	items := make([]conversation.Conversation, 0)
	for rows.Next() {
		var item conversation.Conversation
		if err := scanConversation(rows, &item); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *ConversationRepository) CreateConversation(ctx context.Context, ownerID, agentID, title string) (conversation.Conversation, error) {
	item := conversation.Conversation{ID: ulid.Make().String(), OwnerPrincipalID: ownerID, AgentID: agentID, Title: title}
	err := repository.database.QueryRowContext(ctx, `
		INSERT INTO conversations (id, owner_principal_id, agent_id, title) VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at`, item.ID, item.OwnerPrincipalID, item.AgentID, item.Title).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return conversation.Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return item, nil
}

func (repository *ConversationRepository) GetConversation(ctx context.Context, ownerID, id string) (conversation.Detail, error) {
	var detail conversation.Detail
	err := repository.database.QueryRowContext(ctx, `
		SELECT id, owner_principal_id, agent_id, title, created_at, updated_at
		FROM conversations WHERE id=$1 AND owner_principal_id=$2`, id, ownerID).Scan(
		&detail.Conversation.ID, &detail.Conversation.OwnerPrincipalID, &detail.Conversation.AgentID, &detail.Conversation.Title,
		&detail.Conversation.CreatedAt, &detail.Conversation.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.Detail{}, conversation.ErrNotFound
	}
	if err != nil {
		return conversation.Detail{}, fmt.Errorf("get conversation: %w", err)
	}
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, conversation_id, run_id, role, content, sequence, created_at
		FROM messages WHERE conversation_id=$1 ORDER BY sequence`, id)
	if err != nil {
		return conversation.Detail{}, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	detail.Messages = make([]conversation.Message, 0)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return conversation.Detail{}, fmt.Errorf("scan message: %w", err)
		}
		detail.Messages = append(detail.Messages, message)
	}
	if err := rows.Err(); err != nil {
		return conversation.Detail{}, err
	}
	var active conversation.Run
	err = repository.database.QueryRowContext(ctx, `
		SELECT id, conversation_id, status, failure_code, execution_policy, created_at, started_at, finished_at
		FROM runs WHERE conversation_id=$1 AND status IN ('queued', 'running')`, id).Scan(
		&active.ID, &active.ConversationID, &active.Status, &active.FailureCode, &active.ExecutionPolicy,
		&active.CreatedAt, &active.StartedAt, &active.FinishedAt)
	if err == nil {
		detail.ActiveRun = &active
	} else if !errors.Is(err, sql.ErrNoRows) {
		return conversation.Detail{}, fmt.Errorf("get active run: %w", err)
	}
	return detail, nil
}

func (repository *ConversationRepository) CreateMessageRun(ctx context.Context, ownerID, conversationID, messageID, runID, content string, policy conversation.ExecutionPolicy) (conversation.Message, conversation.Run, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return conversation.Message{}, conversation.Run{}, fmt.Errorf("begin message run: %w", err)
	}
	defer transaction.Rollback()
	var title string
	err = transaction.QueryRowContext(ctx, `SELECT title FROM conversations WHERE id=$1 AND owner_principal_id=$2 FOR UPDATE`, conversationID, ownerID).Scan(&title)
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.Message{}, conversation.Run{}, conversation.ErrNotFound
	}
	if err != nil {
		return conversation.Message{}, conversation.Run{}, fmt.Errorf("lock conversation: %w", err)
	}

	message := conversation.Message{ID: messageID, ConversationID: conversationID, Role: "user", Content: content}
	err = transaction.QueryRowContext(ctx, `
		INSERT INTO messages (id, conversation_id, role, content, sequence)
		VALUES ($1, $2, 'user', $3, COALESCE((SELECT MAX(sequence)+1 FROM messages WHERE conversation_id=$2), 1))
		RETURNING sequence, created_at`, messageID, conversationID, content).Scan(&message.Sequence, &message.CreatedAt)
	if err != nil {
		return conversation.Message{}, conversation.Run{}, fmt.Errorf("insert user message: %w", err)
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return conversation.Message{}, conversation.Run{}, fmt.Errorf("encode run execution policy: %w", err)
	}
	run := conversation.Run{ID: runID, ConversationID: conversationID, Status: "queued", ExecutionPolicy: policy}
	err = transaction.QueryRowContext(ctx, `
		INSERT INTO runs (id, conversation_id, input_message_id, status, execution_policy) VALUES ($1, $2, $3, 'queued', $4)
		RETURNING created_at`, runID, conversationID, messageID, policyJSON).Scan(&run.CreatedAt)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.ConstraintName == "runs_one_active_per_conversation" {
			return conversation.Message{}, conversation.Run{}, conversation.ErrActiveRun
		}
		return conversation.Message{}, conversation.Run{}, fmt.Errorf("insert run: %w", err)
	}
	if title == "New conversation" {
		if _, err := transaction.ExecContext(ctx, `UPDATE conversations SET title=$2, updated_at=now() WHERE id=$1`, conversationID, messageTitle(content)); err != nil {
			return conversation.Message{}, conversation.Run{}, fmt.Errorf("set conversation title: %w", err)
		}
	} else if _, err := transaction.ExecContext(ctx, `UPDATE conversations SET updated_at=now() WHERE id=$1`, conversationID); err != nil {
		return conversation.Message{}, conversation.Run{}, fmt.Errorf("touch conversation: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return conversation.Message{}, conversation.Run{}, fmt.Errorf("commit message run: %w", err)
	}
	return message, run, nil
}

type scanner interface {
	Scan(...any) error
}

func scanConversation(row scanner, item *conversation.Conversation) error {
	return row.Scan(&item.ID, &item.OwnerPrincipalID, &item.AgentID, &item.Title, &item.CreatedAt, &item.UpdatedAt)
}

func scanMessage(row scanner) (conversation.Message, error) {
	var item conversation.Message
	var runID sql.NullString
	if err := row.Scan(&item.ID, &item.ConversationID, &runID, &item.Role, &item.Content, &item.Sequence, &item.CreatedAt); err != nil {
		return conversation.Message{}, err
	}
	if runID.Valid {
		item.RunID = &runID.String
	}
	return item, nil
}

func messageTitle(content string) string {
	compact := strings.Join(strings.Fields(content), " ")
	runes := []rune(compact)
	if len(runes) > 48 {
		return string(runes[:48]) + "..."
	}
	return compact
}
