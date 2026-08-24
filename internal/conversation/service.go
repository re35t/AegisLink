package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/oklog/ulid/v2"
)

const maxMessageLength = 32 * 1024

type Service struct {
	repository Repository
	agents     AgentReader
	runtime    Runtime
	context    ContextProvider
	logger     *slog.Logger
	root       context.Context

	activeMu sync.Mutex
	active   map[string]context.CancelFunc
}

func NewService(
	root context.Context,
	repository Repository,
	agents AgentReader,
	runtime Runtime,
	contextProvider ContextProvider,
	logger *slog.Logger,
) *Service {
	return &Service{
		repository: repository,
		agents:     agents,
		runtime:    runtime,
		context:    contextProvider,
		logger:     logger,
		root:       root,
		active:     make(map[string]context.CancelFunc),
	}
}

func (service *Service) Ready(ctx context.Context) error {
	return service.repository.Ping(ctx)
}

func (service *Service) ListConversations(ctx context.Context, principalID string) ([]Conversation, error) {
	return service.repository.ListConversations(ctx, principalID)
}

func (service *Service) CreateConversation(ctx context.Context, principalID, title string) (Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New conversation"
	}
	if len(title) > 120 {
		return Conversation{}, fmt.Errorf("title exceeds 120 characters: %w", ErrInvalidMessage)
	}
	agentRecord, err := service.agents.Default(ctx, principalID)
	if err != nil {
		return Conversation{}, err
	}
	return service.repository.CreateConversation(ctx, principalID, agentRecord.ID, title)
}

func (service *Service) GetConversation(ctx context.Context, principalID, id string) (Detail, error) {
	return service.repository.GetConversation(ctx, principalID, id)
}

func (service *Service) SendMessage(ctx context.Context, principalID, conversationID, content string) (Message, Run, error) {
	return service.StartRun(ctx, principalID, RunRequest{
		ConversationID: conversationID,
		MessageID:      ulid.Make().String(),
		RunID:          ulid.Make().String(),
		Content:        content,
	})
}

func (service *Service) StartRun(ctx context.Context, principalID string, request RunRequest) (Message, Run, error) {
	content := strings.TrimSpace(request.Content)
	if content == "" || len(content) > maxMessageLength {
		return Message{}, Run{}, ErrInvalidMessage
	}
	if !validRunIdentifier(request.ConversationID) || !validRunIdentifier(request.MessageID) || !validRunIdentifier(request.RunID) {
		return Message{}, Run{}, ErrInvalidMessage
	}
	policy := ExecutionPolicy{Mode: "auto"}
	if request.Selection != nil {
		if request.Selection.Action != "force-tool-once" || !validRunIdentifier(request.Selection.MentionID) {
			return Message{}, Run{}, ErrInvalidMessage
		}
		detail, err := service.repository.GetConversation(ctx, principalID, request.ConversationID)
		if err != nil {
			return Message{}, Run{}, err
		}
		policy, err = service.context.ResolveToolSelection(ctx, principalID, detail.Conversation.AgentID, request.Selection.MentionID)
		if err != nil {
			return Message{}, Run{}, err
		}
	}

	message, run, err := service.repository.CreateMessageRun(
		ctx,
		principalID,
		request.ConversationID,
		request.MessageID,
		request.RunID,
		content,
		policy,
	)
	if err != nil {
		return Message{}, Run{}, err
	}

	runContext, cancel := context.WithCancel(service.root)
	service.activeMu.Lock()
	service.active[run.ID] = cancel
	service.activeMu.Unlock()

	go service.execute(runContext, principalID, run)
	return message, run, nil
}

func (service *Service) GetRun(ctx context.Context, principalID, runID string) (Run, error) {
	return service.repository.GetRun(ctx, principalID, runID)
}

func (service *Service) ListRunEvents(ctx context.Context, principalID, runID string, after int64) ([]RunEvent, error) {
	return service.repository.ListRunEvents(ctx, principalID, runID, after)
}

func (service *Service) CancelRun(ctx context.Context, principalID, runID string) error {
	run, err := service.repository.GetRun(ctx, principalID, runID)
	if err != nil {
		return err
	}
	service.activeMu.Lock()
	cancel, active := service.active[runID]
	service.activeMu.Unlock()
	if active {
		cancel()
		return nil
	}
	if run.Terminal() {
		return ErrRunNotActive
	}
	return ErrRunNotActive
}

func validRunIdentifier(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value
}
