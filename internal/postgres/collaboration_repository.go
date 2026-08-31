package postgres

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/re35t/AegisLink/internal/collaboration"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CollaborationRepository struct{ database *gorm.DB }

func NewCollaborationRepository(database *Database) *CollaborationRepository {
	return &CollaborationRepository{database: database.connection}
}

type collaborationPolicyModel struct {
	AgentID              string `gorm:"primaryKey"`
	OwnerPrincipalID     string
	Enabled              bool
	Revision             int64
	MaxSessionTTLSeconds int64
	MaxRequestsPerHour   int
	MaxActiveSessions    int
	UpdatedAt            time.Time
}

func (collaborationPolicyModel) TableName() string { return "agent_collaboration_policies" }

func defaultCollaborationPolicy(principalID, agentID string) collaboration.Policy {
	return collaboration.Policy{AgentID: agentID, OwnerPrincipalID: principalID, Enabled: false, Revision: 1, MaxSessionTTLSeconds: 3600, MaxRequestsPerHour: 20, MaxActiveSessions: 5, UpdatedAt: time.Now().UTC()}
}

func policyFromModel(row collaborationPolicyModel) collaboration.Policy {
	return collaboration.Policy{AgentID: row.AgentID, OwnerPrincipalID: row.OwnerPrincipalID, Enabled: row.Enabled, Revision: row.Revision, MaxSessionTTLSeconds: row.MaxSessionTTLSeconds, MaxRequestsPerHour: row.MaxRequestsPerHour, MaxActiveSessions: row.MaxActiveSessions, UpdatedAt: row.UpdatedAt}
}

func (repository *CollaborationRepository) ResolveTarget(ctx context.Context, agentAddr string) (collaboration.Target, error) {
	var target collaboration.Target
	result := repository.database.WithContext(ctx).Table("agent_index_states AS idx").
		Select("idx.agent_id, idx.owner_principal_id, idx.agent_addr").
		Joins("JOIN agents ON agents.id = idx.agent_id AND agents.owner_principal_id = idx.owner_principal_id").
		Where("idx.agent_addr = ?", agentAddr).Take(&target)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return collaboration.Target{}, collaboration.ErrNotFound
	}
	if result.Error != nil {
		return collaboration.Target{}, fmt.Errorf("resolve collaboration target: %w", result.Error)
	}
	return target, nil
}

func (repository *CollaborationRepository) GetPolicy(ctx context.Context, principalID, agentID string) (collaboration.Policy, error) {
	var row collaborationPolicyModel
	result := repository.database.WithContext(ctx).Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return defaultCollaborationPolicy(principalID, agentID), nil
	}
	if result.Error != nil {
		return collaboration.Policy{}, fmt.Errorf("get collaboration policy: %w", result.Error)
	}
	return policyFromModel(row), nil
}

func (repository *CollaborationRepository) UpdatePolicy(ctx context.Context, principalID, agentID string, update collaboration.PolicyUpdate) (collaboration.Policy, error) {
	var saved collaborationPolicyModel
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		row := collaborationPolicyModel{AgentID: agentID, OwnerPrincipalID: principalID, Revision: 1, MaxSessionTTLSeconds: 3600, MaxRequestsPerHour: 20, MaxActiveSessions: 5, UpdatedAt: time.Now().UTC()}
		if result := transaction.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "agent_id"}}, DoNothing: true}).Create(&row); result.Error != nil {
			return result.Error
		}
		if result := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Take(&row); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return collaboration.ErrNotFound
			}
			return result.Error
		}
		if row.Revision != update.ExpectedRevision {
			return collaboration.ErrConflict
		}
		now := time.Now().UTC()
		nextRevision := row.Revision + 1
		result := transaction.Model(&row).Updates(map[string]any{
			"enabled": update.Enabled, "revision": nextRevision, "max_session_ttl_seconds": update.MaxSessionTTLSeconds,
			"max_requests_per_hour": update.MaxRequestsPerHour, "max_active_sessions": update.MaxActiveSessions, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		row.Enabled, row.Revision, row.MaxSessionTTLSeconds = update.Enabled, nextRevision, update.MaxSessionTTLSeconds
		row.MaxRequestsPerHour, row.MaxActiveSessions, row.UpdatedAt = update.MaxRequestsPerHour, update.MaxActiveSessions, now
		saved = row
		return nil
	})
	if err != nil {
		return collaboration.Policy{}, fmt.Errorf("update collaboration policy: %w", err)
	}
	return policyFromModel(saved), nil
}

type assistanceRequestModel struct {
	ID                        string `gorm:"primaryKey"`
	RequesterOwnerPrincipalID string
	RequesterAgentID          string
	TargetOwnerPrincipalID    string
	TargetAgentID             string
	TargetAgentAddr           string
	Purpose                   string
	RequestedScopes           []string `gorm:"type:jsonb;serializer:json"`
	IdempotencyKeyHash        []byte
	RequestDigest             []byte
	Status                    string
	DecisionCode              string
	SessionID                 *string
	CreatedAt                 time.Time
	EvaluatedAt               *time.Time
}

func (assistanceRequestModel) TableName() string { return "assistance_requests" }

func requestFromModel(row assistanceRequestModel) collaboration.AssistanceRequest {
	return collaboration.AssistanceRequest{ID: row.ID, RequesterOwnerPrincipalID: row.RequesterOwnerPrincipalID, RequesterAgentID: row.RequesterAgentID, TargetOwnerPrincipalID: row.TargetOwnerPrincipalID, TargetAgentID: row.TargetAgentID, TargetAgentAddr: row.TargetAgentAddr, Purpose: row.Purpose, RequestedScopes: nonNil(row.RequestedScopes), IdempotencyKeyHash: row.IdempotencyKeyHash, RequestDigest: row.RequestDigest, Status: row.Status, DecisionCode: row.DecisionCode, SessionID: row.SessionID, CreatedAt: row.CreatedAt, EvaluatedAt: row.EvaluatedAt}
}

func requestModel(request collaboration.AssistanceRequest) assistanceRequestModel {
	return assistanceRequestModel{ID: request.ID, RequesterOwnerPrincipalID: request.RequesterOwnerPrincipalID, RequesterAgentID: request.RequesterAgentID, TargetOwnerPrincipalID: request.TargetOwnerPrincipalID, TargetAgentID: request.TargetAgentID, TargetAgentAddr: request.TargetAgentAddr, Purpose: request.Purpose, RequestedScopes: request.RequestedScopes, IdempotencyKeyHash: request.IdempotencyKeyHash, RequestDigest: request.RequestDigest, Status: request.Status, DecisionCode: request.DecisionCode, SessionID: request.SessionID, CreatedAt: request.CreatedAt, EvaluatedAt: request.EvaluatedAt}
}

func (repository *CollaborationRepository) CreateRequest(ctx context.Context, request collaboration.AssistanceRequest, policy collaboration.Policy) (collaboration.AssistanceRequest, bool, error) {
	var resultRequest collaboration.AssistanceRequest
	replayed := false
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var existing assistanceRequestModel
		lookup := transaction.Where("requester_agent_id = ? AND idempotency_key_hash = ?", request.RequesterAgentID, request.IdempotencyKeyHash).Take(&existing)
		if lookup.Error == nil {
			if subtle.ConstantTimeCompare(existing.RequestDigest, request.RequestDigest) != 1 {
				return collaboration.ErrConflict
			}
			resultRequest, replayed = requestFromModel(existing), true
			return nil
		}
		if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		var locked collaborationPolicyModel
		policyResult := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_principal_id = ? AND agent_id = ?", request.TargetOwnerPrincipalID, request.TargetAgentID).Take(&locked)
		if errors.Is(policyResult.Error, gorm.ErrRecordNotFound) || !locked.Enabled {
			return collaboration.ErrForbidden
		}
		if policyResult.Error != nil {
			return policyResult.Error
		}
		var recent int64
		if err := transaction.Model(&assistanceRequestModel{}).Where("target_agent_id = ? AND created_at >= ?", request.TargetAgentID, time.Now().UTC().Add(-time.Hour)).Count(&recent).Error; err != nil {
			return err
		}
		if recent >= int64(locked.MaxRequestsPerHour) {
			return collaboration.ErrRateLimited
		}
		var active int64
		if err := transaction.Table("collaboration_sessions").Where("target_agent_id = ? AND status = 'active' AND expires_at > now()", request.TargetAgentID).Count(&active).Error; err != nil {
			return err
		}
		if active >= int64(locked.MaxActiveSessions) {
			return collaboration.ErrRateLimited
		}
		row := requestModel(request)
		if err := transaction.Create(&row).Error; err != nil {
			return err
		}
		resultRequest = request
		return nil
	})
	if err != nil {
		return collaboration.AssistanceRequest{}, false, fmt.Errorf("create assistance request: %w", err)
	}
	return resultRequest, replayed, nil
}

type collaborationSessionModel struct {
	ID                        string `gorm:"primaryKey"`
	AssistanceRequestID       string
	RequesterOwnerPrincipalID string
	RequesterAgentID          string
	TargetOwnerPrincipalID    string
	TargetAgentID             string
	TargetAgentAddr           string
	Status                    string
	Scopes                    []string `gorm:"type:jsonb;serializer:json"`
	TokenHash                 []byte
	EncryptedToken            []byte
	TokenNonce                []byte
	ExpiresAt                 time.Time
	LastUsedAt                *time.Time
	CreatedAt                 time.Time
	RevokedAt                 *time.Time
}

func (collaborationSessionModel) TableName() string { return "collaboration_sessions" }
func sessionFromModel(row collaborationSessionModel) collaboration.Session {
	return collaboration.Session{ID: row.ID, AssistanceRequestID: row.AssistanceRequestID, RequesterOwnerPrincipalID: row.RequesterOwnerPrincipalID, RequesterAgentID: row.RequesterAgentID, TargetOwnerPrincipalID: row.TargetOwnerPrincipalID, TargetAgentID: row.TargetAgentID, TargetAgentAddr: row.TargetAgentAddr, Status: row.Status, Scopes: nonNil(row.Scopes), TokenHash: row.TokenHash, EncryptedToken: row.EncryptedToken, TokenNonce: row.TokenNonce, ExpiresAt: row.ExpiresAt, LastUsedAt: row.LastUsedAt, CreatedAt: row.CreatedAt, RevokedAt: row.RevokedAt}
}
func sessionModel(row collaboration.Session) collaborationSessionModel {
	return collaborationSessionModel{ID: row.ID, AssistanceRequestID: row.AssistanceRequestID, RequesterOwnerPrincipalID: row.RequesterOwnerPrincipalID, RequesterAgentID: row.RequesterAgentID, TargetOwnerPrincipalID: row.TargetOwnerPrincipalID, TargetAgentID: row.TargetAgentID, TargetAgentAddr: row.TargetAgentAddr, Status: row.Status, Scopes: row.Scopes, TokenHash: row.TokenHash, EncryptedToken: row.EncryptedToken, TokenNonce: row.TokenNonce, ExpiresAt: row.ExpiresAt, LastUsedAt: row.LastUsedAt, CreatedAt: row.CreatedAt, RevokedAt: row.RevokedAt}
}

func (repository *CollaborationRepository) AcceptRequest(ctx context.Context, request collaboration.AssistanceRequest, session collaboration.Session) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var policy collaborationPolicyModel
		policyResult := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_principal_id = ? AND agent_id = ?", request.TargetOwnerPrincipalID, request.TargetAgentID).Take(&policy)
		if errors.Is(policyResult.Error, gorm.ErrRecordNotFound) || !policy.Enabled {
			return collaboration.ErrForbidden
		}
		if policyResult.Error != nil {
			return policyResult.Error
		}
		var active int64
		if err := transaction.Table("collaboration_sessions").Where("target_agent_id = ? AND status = 'active' AND expires_at > now()", request.TargetAgentID).Count(&active).Error; err != nil {
			return err
		}
		if active >= int64(policy.MaxActiveSessions) {
			return collaboration.ErrRateLimited
		}
		now := time.Now().UTC()
		maximumExpiry := now.Add(time.Duration(policy.MaxSessionTTLSeconds) * time.Second)
		if session.ExpiresAt.After(maximumExpiry) {
			session.ExpiresAt = maximumExpiry
		}
		if !session.ExpiresAt.After(now) {
			return collaboration.ErrInvalid
		}
		row := sessionModel(session)
		if err := transaction.Create(&row).Error; err != nil {
			return err
		}
		result := transaction.Model(&assistanceRequestModel{}).Where("id = ? AND status = 'evaluating'", request.ID).
			Updates(map[string]any{"status": "accepted", "decision_code": request.DecisionCode, "session_id": session.ID, "evaluated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return collaboration.ErrConflict
		}
		return nil
	})
}

func (repository *CollaborationRepository) RejectRequest(ctx context.Context, requestID, status, code string) error {
	result := repository.database.WithContext(ctx).Model(&assistanceRequestModel{}).Where("id = ? AND status = 'evaluating'", requestID).
		Updates(map[string]any{"status": status, "decision_code": code, "evaluated_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("finish assistance request: %w", result.Error)
	}
	return nil
}

func (repository *CollaborationRepository) GetOwnedSession(ctx context.Context, principalID, agentID, sessionID string) (collaboration.Session, error) {
	var row collaborationSessionModel
	result := repository.database.WithContext(ctx).Where("id = ? AND ((requester_owner_principal_id = ? AND requester_agent_id = ?) OR (target_owner_principal_id = ? AND target_agent_id = ?))", sessionID, principalID, agentID, principalID, agentID).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return collaboration.Session{}, collaboration.ErrNotFound
	}
	if result.Error != nil {
		return collaboration.Session{}, result.Error
	}
	return sessionFromModel(row), nil
}

func (repository *CollaborationRepository) AuthenticateSession(ctx context.Context, tokenHash []byte, agentAddr, method string) (collaboration.Session, error) {
	var row collaborationSessionModel
	result := repository.database.WithContext(ctx).Where("token_hash = ? AND target_agent_addr = ? AND status = 'active' AND expires_at > now()", tokenHash, agentAddr).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return collaboration.Session{}, collaboration.ErrForbidden
	}
	if result.Error != nil {
		return collaboration.Session{}, result.Error
	}
	if stringsContainsSend(method) {
		now := time.Now().UTC()
		_ = repository.database.WithContext(ctx).Model(&row).Update("last_used_at", now).Error
		row.LastUsedAt = &now
	}
	return sessionFromModel(row), nil
}

func (repository *CollaborationRepository) ListOverview(ctx context.Context, principalID, agentID string) (collaboration.Overview, error) {
	policy, err := repository.GetPolicy(ctx, principalID, agentID)
	if err != nil {
		return collaboration.Overview{}, err
	}
	var requestRows []assistanceRequestModel
	if err := repository.database.WithContext(ctx).Where("(requester_owner_principal_id = ? AND requester_agent_id = ?) OR (target_owner_principal_id = ? AND target_agent_id = ?)", principalID, agentID, principalID, agentID).Order("created_at DESC").Limit(100).Find(&requestRows).Error; err != nil {
		return collaboration.Overview{}, err
	}
	var sessionRows []collaborationSessionModel
	if err := repository.database.WithContext(ctx).Where("(requester_owner_principal_id = ? AND requester_agent_id = ?) OR (target_owner_principal_id = ? AND target_agent_id = ?)", principalID, agentID, principalID, agentID).Order("created_at DESC").Limit(100).Find(&sessionRows).Error; err != nil {
		return collaboration.Overview{}, err
	}
	result := collaboration.Overview{Policy: policy, Requests: make([]collaboration.AssistanceRequest, 0, len(requestRows)), Sessions: make([]collaboration.Session, 0, len(sessionRows))}
	for _, row := range requestRows {
		result.Requests = append(result.Requests, requestFromModel(row))
	}
	for _, row := range sessionRows {
		result.Sessions = append(result.Sessions, sessionFromModel(row))
	}
	return result, nil
}

func (repository *CollaborationRepository) RevokeSession(ctx context.Context, principalID, agentID, sessionID string) error {
	now := time.Now().UTC()
	result := repository.database.WithContext(ctx).Model(&collaborationSessionModel{}).
		Where("id = ? AND status = 'active' AND ((requester_owner_principal_id = ? AND requester_agent_id = ?) OR (target_owner_principal_id = ? AND target_agent_id = ?))", sessionID, principalID, agentID, principalID, agentID).
		Updates(map[string]any{"status": "revoked", "revoked_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return collaboration.ErrNotFound
	}
	return nil
}

type collaborationInvocationModel struct {
	ID                  string `gorm:"primaryKey"`
	Kind                string
	AssistanceRequestID *string
	SessionID           *string
	TaskID              *string
	TargetAgentID       string
	Status              string
	FailureCode         string
	StartedAt           time.Time
	FinishedAt          *time.Time
}

func (collaborationInvocationModel) TableName() string { return "collaboration_invocations" }

func (repository *CollaborationRepository) StartInvocation(ctx context.Context, invocation collaboration.Invocation) error {
	row := collaborationInvocationModel{ID: invocation.ID, Kind: invocation.Kind, AssistanceRequestID: invocation.AssistanceRequestID, SessionID: invocation.SessionID, TaskID: invocation.TaskID, TargetAgentID: invocation.TargetAgentID, Status: invocation.Status, FailureCode: invocation.FailureCode, StartedAt: invocation.StartedAt}
	return repository.database.WithContext(ctx).Create(&row).Error
}

func (repository *CollaborationRepository) FinishInvocation(ctx context.Context, invocationID, status, failureCode string) error {
	result := repository.database.WithContext(ctx).Model(&collaborationInvocationModel{}).Where("id = ? AND status = 'running'", invocationID).Updates(map[string]any{"status": status, "failure_code": failureCode, "finished_at": time.Now().UTC()})
	return result.Error
}

func (repository *CollaborationRepository) LoadContextTasks(ctx context.Context, sessionID string, limit int) ([]*a2a.Task, error) {
	if limit < 1 || limit > 100 {
		return nil, collaboration.ErrInvalid
	}
	var rows []collaborationTaskModel
	if err := repository.database.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load collaboration context: %w", err)
	}
	result := make([]*a2a.Task, 0, len(rows))
	for index := len(rows) - 1; index >= 0; index-- {
		copyTask, err := copyA2ATask(&rows[index].TaskJSON)
		if err != nil {
			return nil, err
		}
		result = append(result, copyTask)
	}
	return result, nil
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
func stringsContainsSend(value string) bool {
	return value == "SendMessage" || value == "message/send" || value == "send_message"
}

var _ collaboration.Repository = (*CollaborationRepository)(nil)
