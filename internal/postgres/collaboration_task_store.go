package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CollaborationTaskStore struct{ database *gorm.DB }

func NewCollaborationTaskStore(database *Database) *CollaborationTaskStore {
	return &CollaborationTaskStore{database: database.connection}
}

type collaborationTaskModel struct {
	ID        string `gorm:"primaryKey"`
	SessionID string
	ContextID string
	State     string
	TaskJSON  a2a.Task `gorm:"column:task_json;serializer:json;type:jsonb"`
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (collaborationTaskModel) TableName() string { return "collaboration_a2a_tasks" }

func taskStoreUser(ctx context.Context) (string, error) {
	name, err := a2asrv.NewTaskStoreAuthenticator()(ctx)
	if err != nil || name == "" {
		return "", a2a.ErrUnauthenticated
	}
	return name, nil
}

func (store *CollaborationTaskStore) Create(ctx context.Context, task *a2a.Task) (taskstore.TaskVersion, error) {
	user, err := taskStoreUser(ctx)
	if err != nil {
		return taskstore.TaskVersionMissing, err
	}
	if task == nil || task.ContextID != user {
		return taskstore.TaskVersionMissing, a2a.ErrUnauthorized
	}
	now := time.Now().UTC()
	copyTask, err := copyA2ATask(task)
	if err != nil {
		return taskstore.TaskVersionMissing, err
	}
	row := collaborationTaskModel{ID: string(task.ID), SessionID: user, ContextID: task.ContextID, State: string(task.Status.State), TaskJSON: *copyTask, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := store.database.WithContext(ctx).Create(&row).Error; err != nil {
		if isUniqueViolation(err) {
			return taskstore.TaskVersionMissing, taskstore.ErrTaskAlreadyExists
		}
		return taskstore.TaskVersionMissing, err
	}
	return 1, nil
}

func (store *CollaborationTaskStore) Update(ctx context.Context, update *taskstore.UpdateRequest) (taskstore.TaskVersion, error) {
	user, err := taskStoreUser(ctx)
	if err != nil {
		return taskstore.TaskVersionMissing, err
	}
	if update == nil || update.Task == nil || update.Task.ContextID != user {
		return taskstore.TaskVersionMissing, a2a.ErrUnauthorized
	}
	var next int64
	err = store.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var row collaborationTaskModel
		result := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND session_id = ?", update.Task.ID, user).Take(&row)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return a2a.ErrTaskNotFound
		}
		if result.Error != nil {
			return result.Error
		}
		if update.PrevVersion != taskstore.TaskVersionMissing && row.Version != int64(update.PrevVersion) {
			return taskstore.ErrConcurrentModification
		}
		copyTask, err := copyA2ATask(update.Task)
		if err != nil {
			return err
		}
		next = row.Version + 1
		return transaction.Model(&row).Updates(map[string]any{"task_json": copyTask, "state": string(update.Task.Status.State), "version": next, "updated_at": time.Now().UTC()}).Error
	})
	return taskstore.TaskVersion(next), err
}

func (store *CollaborationTaskStore) Get(ctx context.Context, taskID a2a.TaskID) (*taskstore.StoredTask, error) {
	user, err := taskStoreUser(ctx)
	if err != nil {
		return nil, err
	}
	var row collaborationTaskModel
	result := store.database.WithContext(ctx).Where("id = ? AND session_id = ?", taskID, user).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, a2a.ErrTaskNotFound
	}
	if result.Error != nil {
		return nil, result.Error
	}
	copyTask, err := copyA2ATask(&row.TaskJSON)
	if err != nil {
		return nil, err
	}
	return &taskstore.StoredTask{Task: copyTask, Version: taskstore.TaskVersion(row.Version), User: user}, nil
}

func (store *CollaborationTaskStore) List(ctx context.Context, request *a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	user, err := taskStoreUser(ctx)
	if err != nil {
		return nil, err
	}
	if request == nil {
		request = &a2a.ListTasksRequest{}
	}
	pageSize := request.PageSize
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize < 1 || pageSize > 100 {
		return nil, a2a.ErrInvalidParams
	}
	offset, err := decodeTaskPageToken(request.PageToken)
	if err != nil {
		return nil, a2a.ErrInvalidParams
	}
	query := store.database.WithContext(ctx).Model(&collaborationTaskModel{}).Where("session_id = ?", user)
	if request.ContextID != "" {
		query = query.Where("context_id = ?", request.ContextID)
	}
	if request.Status != "" {
		query = query.Where("state = ?", string(request.Status))
	}
	if request.StatusTimestampAfter != nil {
		query = query.Where("updated_at > ?", request.StatusTimestampAfter.UTC())
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []collaborationTaskModel
	if err := query.Order("updated_at DESC, id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	tasks := make([]*a2a.Task, 0, len(rows))
	for _, row := range rows {
		task, err := copyA2ATask(&row.TaskJSON)
		if err != nil {
			return nil, err
		}
		if request.HistoryLength != nil && *request.HistoryLength >= 0 && len(task.History) > *request.HistoryLength {
			task.History = task.History[len(task.History)-*request.HistoryLength:]
		}
		if !request.IncludeArtifacts {
			task.Artifacts = nil
		}
		tasks = append(tasks, task)
	}
	next := ""
	if offset+len(rows) < int(total) {
		next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset + len(rows))))
	}
	return &a2a.ListTasksResponse{Tasks: tasks, TotalSize: int(total), PageSize: pageSize, NextPageToken: next}, nil
}

func copyA2ATask(task *a2a.Task) (*a2a.Task, error) {
	encoded, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("encode A2A task: %w", err)
	}
	var copied a2a.Task
	if err := json.Unmarshal(encoded, &copied); err != nil {
		return nil, fmt.Errorf("decode A2A task: %w", err)
	}
	return &copied, nil
}

func decodeTaskPageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil || offset < 0 {
		return 0, errors.New("invalid page token")
	}
	return offset, nil
}

var _ taskstore.Store = (*CollaborationTaskStore)(nil)
