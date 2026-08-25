package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/conversation"
	"gorm.io/gorm"
)

type ConversationRepository struct {
	database *gorm.DB
}

var _ conversation.Repository = (*ConversationRepository)(nil)

func NewConversationRepository(database *gorm.DB) *ConversationRepository {
	return &ConversationRepository{database: database}
}

func (repository *ConversationRepository) Ping(ctx context.Context) error {
	sqlDatabase, err := repository.database.DB()
	if err != nil {
		return fmt.Errorf("access postgres connection pool: %w", err)
	}
	return sqlDatabase.PingContext(ctx)
}

func (repository *ConversationRepository) ListConversations(ctx context.Context, ownerID string) ([]conversation.Conversation, error) {
	items := make([]conversation.Conversation, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT id, owner_principal_id, agent_id, title, created_at, updated_at
		FROM conversations WHERE owner_principal_id=$1 ORDER BY updated_at DESC`, ownerID).Scan(&items)
	if result.Error != nil {
		return nil, fmt.Errorf("list conversations: %w", result.Error)
	}
	return items, nil
}

func (repository *ConversationRepository) CreateConversation(ctx context.Context, ownerID, agentID, title string) (conversation.Conversation, error) {
	item := conversation.Conversation{ID: ulid.Make().String(), OwnerPrincipalID: ownerID, AgentID: agentID, Title: title}
	var timestamps struct {
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	err := scanOne(repository.database.WithContext(ctx), &timestamps, `
		INSERT INTO conversations (id, owner_principal_id, agent_id, title) VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at`, item.ID, item.OwnerPrincipalID, item.AgentID, item.Title)
	if err != nil {
		return conversation.Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	item.CreatedAt = timestamps.CreatedAt
	item.UpdatedAt = timestamps.UpdatedAt
	return item, nil
}

func (repository *ConversationRepository) GetConversation(ctx context.Context, ownerID, id string) (conversation.Detail, error) {
	var detail conversation.Detail
	err := scanOne(repository.database.WithContext(ctx), &detail.Conversation, `
		SELECT id, owner_principal_id, agent_id, title, created_at, updated_at
		FROM conversations WHERE id=$1 AND owner_principal_id=$2`, id, ownerID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return conversation.Detail{}, conversation.ErrNotFound
	}
	if err != nil {
		return conversation.Detail{}, fmt.Errorf("get conversation: %w", err)
	}
	detail.Messages = make([]conversation.Message, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT id, conversation_id, run_id, role, content, sequence, created_at
		FROM messages WHERE conversation_id=$1 ORDER BY sequence`, id).Scan(&detail.Messages)
	if result.Error != nil {
		return conversation.Detail{}, fmt.Errorf("list messages: %w", result.Error)
	}
	var active conversation.Run
	err = scanOne(repository.database.WithContext(ctx), &active, `
		SELECT id, conversation_id, status, failure_code, execution_policy, created_at, started_at, finished_at
		FROM runs WHERE conversation_id=$1 AND status IN ('queued', 'running')`, id)
	if err == nil {
		detail.ActiveRun = &active
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return conversation.Detail{}, fmt.Errorf("get active run: %w", err)
	}
	return detail, nil
}

func (repository *ConversationRepository) CreateMessageRun(ctx context.Context, ownerID, conversationID, messageID, runID, content string, policy conversation.ExecutionPolicy) (conversation.Message, conversation.Run, error) {
	var message conversation.Message
	var run conversation.Run
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var locked struct{ Title string }
		err := scanOne(transaction, &locked, `SELECT title FROM conversations WHERE id=$1 AND owner_principal_id=$2 FOR UPDATE`, conversationID, ownerID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return conversation.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock conversation: %w", err)
		}

		message = conversation.Message{ID: messageID, ConversationID: conversationID, Role: "user", Content: content}
		var messageResult struct {
			Sequence  int64
			CreatedAt time.Time
		}
		if err := scanOne(transaction, &messageResult, `
			INSERT INTO messages (id, conversation_id, role, content, sequence)
			VALUES ($1, $2, 'user', $3, COALESCE((SELECT MAX(sequence)+1 FROM messages WHERE conversation_id=$2), 1))
			RETURNING sequence, created_at`, messageID, conversationID, content); err != nil {
			return fmt.Errorf("insert user message: %w", err)
		}
		message.Sequence = messageResult.Sequence
		message.CreatedAt = messageResult.CreatedAt

		policyJSON, err := json.Marshal(policy)
		if err != nil {
			return fmt.Errorf("encode run execution policy: %w", err)
		}
		run = conversation.Run{ID: runID, ConversationID: conversationID, Status: "queued", ExecutionPolicy: policy}
		var runResult struct{ CreatedAt time.Time }
		if err := scanOne(transaction, &runResult, `
			INSERT INTO runs (id, conversation_id, input_message_id, status, execution_policy) VALUES ($1, $2, $3, 'queued', $4)
			RETURNING created_at`, runID, conversationID, messageID, policyJSON); err != nil {
			if isUniqueViolation(err) {
				return conversation.ErrActiveRun
			}
			return fmt.Errorf("insert run: %w", err)
		}
		run.CreatedAt = runResult.CreatedAt
		var result *gorm.DB
		if locked.Title == "New conversation" {
			result = exec(transaction, `UPDATE conversations SET title=$2, updated_at=now() WHERE id=$1`, conversationID, messageTitle(content))
		} else {
			result = exec(transaction, `UPDATE conversations SET updated_at=now() WHERE id=$1`, conversationID)
		}
		if result.Error != nil {
			return fmt.Errorf("touch conversation: %w", result.Error)
		}
		return nil
	})
	if err != nil {
		return conversation.Message{}, conversation.Run{}, err
	}
	return message, run, nil
}

func messageTitle(content string) string {
	compact := strings.Join(strings.Fields(content), " ")
	runes := []rune(compact)
	if len(runes) > 48 {
		return string(runes[:48]) + "..."
	}
	return compact
}
