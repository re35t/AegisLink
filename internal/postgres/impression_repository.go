package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/impression"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ImpressionRepository struct {
	database *gorm.DB
}

var _ impression.Repository = (*ImpressionRepository)(nil)

func NewImpressionRepository(database *Database) *ImpressionRepository {
	return &ImpressionRepository{database: database.connection}
}

type impressionModel struct {
	ID                   string
	OwnerPrincipalID     string
	AgentID              string
	Scope                impression.Scope
	ImpressionKind       impression.Kind
	Summary              string
	Details              map[string]any `gorm:"column:details_json;serializer:json;type:jsonb"`
	Tags                 []string       `gorm:"serializer:json;type:jsonb"`
	Confidence           float64
	Salience             float64
	FirstObservedAt      time.Time
	LastObservedAt       time.Time
	ExpiresAt            *time.Time
	DecayHalfLifeSeconds int64
	Status               impression.Status
	SupersededByID       *string
	GeneratorModel       string
	GeneratorRunID       string
	PromptVersion        string
	GeneratedAt          time.Time
	OwnerEdited          bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (impressionModel) TableName() string { return "agent_impressions" }

type impressionEvidenceModel struct {
	OwnerPrincipalID string
	AgentID          string
	ImpressionID     string
	EvidenceKind     impression.EvidenceKind
	EvidenceID       string
	Digest           string
	ObservedAt       time.Time
}

func (impressionEvidenceModel) TableName() string { return "agent_impression_evidence" }

type factCandidateModel struct {
	ID               string
	OwnerPrincipalID string
	AgentID          string
	SubjectKind      impression.FactSubject
	Namespace        string
	FactKey          string
	Value            map[string]any `gorm:"column:value_json;serializer:json;type:jsonb"`
	ValueHash        string
	Rationale        string
	Confidence       float64
	CandidateVersion int64
	Status           string
	GeneratorModel   string
	GeneratorRunID   string
	PromptVersion    string
	ProposedAt       time.Time
	ReviewedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (factCandidateModel) TableName() string { return "agent_fact_candidates" }

type factCandidateSourceModel struct {
	OwnerPrincipalID string
	AgentID          string
	CandidateID      string
	ImpressionID     string
}

func (factCandidateSourceModel) TableName() string { return "agent_fact_candidate_sources" }

type profileJobModel struct {
	ID               string
	OwnerPrincipalID string
	AgentID          string
	SourceRunID      string
	JobKind          string
	Status           string
	Attempts         int
	AvailableAt      time.Time
	LeaseExpiresAt   *time.Time
	LastErrorCode    string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (profileJobModel) TableName() string { return "agent_profile_jobs" }

func (repository *ImpressionRepository) List(ctx context.Context, principalID, agentID, status string) ([]impression.Impression, error) {
	models := make([]impressionModel, 0)
	query := repository.database.WithContext(ctx).
		Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID)
	if status == string(impression.StatusActive) || status == string(impression.StatusStale) {
		query = query.Where("status IN ?", []impression.Status{impression.StatusActive, impression.StatusStale})
	} else if status != "" {
		query = query.Where("status = ?", status)
	}
	if result := query.Order("last_observed_at DESC, id").Find(&models); result.Error != nil {
		return nil, fmt.Errorf("list Agent impressions: %w", result.Error)
	}
	items := make([]impression.Impression, 0, len(models))
	for _, model := range models {
		item, err := repository.impressionValue(ctx, model)
		if err != nil {
			return nil, err
		}
		if status != "" && string(item.Status) != status {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *ImpressionRepository) impressionValue(ctx context.Context, model impressionModel) (impression.Impression, error) {
	evidenceRows := make([]impressionEvidenceModel, 0)
	if result := repository.database.WithContext(ctx).Where("impression_id = ?", model.ID).
		Order("observed_at, evidence_kind, evidence_id").Find(&evidenceRows); result.Error != nil {
		return impression.Impression{}, fmt.Errorf("list Impression evidence: %w", result.Error)
	}
	item := impression.Impression{
		ID: model.ID, Scope: model.Scope, Kind: model.ImpressionKind, Summary: model.Summary,
		Details: nonNilMap(model.Details), Tags: nonNilStringSlice(model.Tags), Confidence: model.Confidence,
		Salience: model.Salience, FirstObservedAt: model.FirstObservedAt, LastObservedAt: model.LastObservedAt,
		ExpiresAt: model.ExpiresAt, DecayHalfLifeSeconds: model.DecayHalfLifeSeconds, Status: model.Status,
		SupersededByID: model.SupersededByID, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
		Generation: impression.GenerationInfo{Model: model.GeneratorModel, RunID: model.GeneratorRunID, PromptVersion: model.PromptVersion, GeneratedAt: model.GeneratedAt},
		Evidence:   make([]impression.EvidenceRef, 0, len(evidenceRows)),
	}
	for _, row := range evidenceRows {
		item.Evidence = append(item.Evidence, impression.EvidenceRef{Kind: row.EvidenceKind, ID: row.EvidenceID, Digest: row.Digest, ObservedAt: row.ObservedAt})
	}
	item.CalculateFreshness(time.Now().UTC())
	if item.Status == impression.StatusActive && item.ExpiresAt != nil && !time.Now().UTC().Before(*item.ExpiresAt) {
		item.Status = impression.StatusStale
	}
	return item, nil
}

func (repository *ImpressionRepository) ListCandidates(ctx context.Context, principalID, agentID, status string) ([]impression.FactCandidate, error) {
	models := make([]factCandidateModel, 0)
	query := repository.database.WithContext(ctx).Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if result := query.Order("proposed_at DESC, id").Find(&models); result.Error != nil {
		return nil, fmt.Errorf("list Fact candidates: %w", result.Error)
	}
	items := make([]impression.FactCandidate, 0, len(models))
	for _, model := range models {
		sources := make([]factCandidateSourceModel, 0)
		if result := repository.database.WithContext(ctx).Where("candidate_id = ?", model.ID).Order("impression_id").Find(&sources); result.Error != nil {
			return nil, fmt.Errorf("list Fact candidate sources: %w", result.Error)
		}
		item := impression.FactCandidate{
			ID: model.ID, Subject: model.SubjectKind, Namespace: model.Namespace, Key: model.FactKey,
			Value: nonNilMap(model.Value), Rationale: model.Rationale, Confidence: model.Confidence,
			Version: model.CandidateVersion, Status: model.Status, ProposedAt: model.ProposedAt,
			ReviewedAt: model.ReviewedAt, SourceImpressionIDs: make([]string, 0, len(sources)),
		}
		for _, source := range sources {
			item.SourceImpressionIDs = append(item.SourceImpressionIDs, source.ImpressionID)
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *ImpressionRepository) Update(ctx context.Context, principalID, agentID, impressionID string, update impression.Update) (impression.Impression, error) {
	var updated impressionModel
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var profile struct {
			AgentID         string
			ContextRevision int64
		}
		if result := transaction.Table("agent_profiles").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).Take(&profile); result.Error != nil {
			return mapImpressionNotFound(result.Error)
		}
		if profile.ContextRevision != update.ExpectedContextRevision {
			return impression.ErrConflict
		}
		if result := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_principal_id = ? AND agent_id = ? AND id = ?", principalID, agentID, impressionID).Take(&updated); result.Error != nil {
			return mapImpressionNotFound(result.Error)
		}
		values := map[string]any{"owner_edited": true, "updated_at": time.Now().UTC()}
		if update.Summary != nil {
			values["summary"] = *update.Summary
		}
		if update.Details != nil {
			values["details_json"] = update.Details
		}
		if update.Status != nil {
			values["status"] = *update.Status
		}
		if result := transaction.Model(&updated).Updates(values); result.Error != nil {
			return fmt.Errorf("update Impression: %w", result.Error)
		}
		if result := transaction.Table("agent_profiles").Where("agent_id = ?", agentID).
			Updates(map[string]any{"context_revision": gorm.Expr("context_revision + 1"), "updated_at": time.Now().UTC()}); result.Error != nil {
			return fmt.Errorf("advance Profile context revision: %w", result.Error)
		}
		return transaction.Where("id = ?", impressionID).Take(&updated).Error
	})
	if err != nil {
		return impression.Impression{}, err
	}
	return repository.impressionValue(ctx, updated)
}

func (repository *ImpressionRepository) RejectCandidate(ctx context.Context, principalID, agentID, candidateID string, expectedVersion int64) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var candidate factCandidateModel
		result := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_principal_id = ? AND agent_id = ?", candidateID, principalID, agentID).Take(&candidate)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return impression.ErrNotFound
		}
		if result.Error != nil {
			return fmt.Errorf("lock Fact candidate: %w", result.Error)
		}
		if candidate.Status != "pending" || candidate.CandidateVersion != expectedVersion {
			return impression.ErrConflict
		}
		now := time.Now().UTC()
		if result := transaction.Model(&candidate).Updates(map[string]any{
			"status": "rejected", "candidate_version": gorm.Expr("candidate_version + 1"),
			"reviewed_at": now, "updated_at": now,
		}); result.Error != nil {
			return fmt.Errorf("reject Fact candidate: %w", result.Error)
		}
		if result := transaction.Table("agent_profiles").Where("owner_principal_id = ? AND agent_id = ?", principalID, agentID).
			Updates(map[string]any{"context_revision": gorm.Expr("context_revision + 1"), "updated_at": now}); result.Error != nil {
			return fmt.Errorf("advance Profile context revision: %w", result.Error)
		}
		return nil
	})
}

func (repository *ImpressionRepository) ClaimJob(ctx context.Context, now time.Time, lease time.Duration) (impression.Job, error) {
	var claimed profileJobModel
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		result := transaction.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("(status = ? OR (status = ? AND lease_expires_at < ?)) AND available_at <= ?", "pending", "running", now, now).
			Order("created_at, id").Take(&claimed)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return impression.ErrNotFound
		}
		if result.Error != nil {
			return fmt.Errorf("claim curator job: %w", result.Error)
		}
		leaseExpiresAt := now.Add(lease)
		if result := transaction.Model(&claimed).Updates(map[string]any{
			"status": "running", "lease_expires_at": leaseExpiresAt, "updated_at": now,
		}); result.Error != nil {
			return fmt.Errorf("lease curator job: %w", result.Error)
		}
		claimed.LeaseExpiresAt = &leaseExpiresAt
		return nil
	})
	if err != nil {
		return impression.Job{}, err
	}
	return impression.Job{ID: claimed.ID, OwnerPrincipalID: claimed.OwnerPrincipalID, AgentID: claimed.AgentID, SourceRunID: claimed.SourceRunID, Attempts: claimed.Attempts}, nil
}

func (repository *ImpressionRepository) CompleteJob(ctx context.Context, id string) error {
	result := repository.database.WithContext(ctx).Model(&profileJobModel{}).Where("id = ? AND status = ?", id, "running").Updates(map[string]any{
		"status": "succeeded", "lease_expires_at": nil, "last_error_code": "", "updated_at": time.Now().UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("complete curator job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return impression.ErrNotFound
	}
	return nil
}

func (repository *ImpressionRepository) FailJob(ctx context.Context, id string, attempts int, availableAt time.Time, code string) error {
	status := "pending"
	if attempts >= 5 {
		status = "failed"
	}
	result := repository.database.WithContext(ctx).Model(&profileJobModel{}).Where("id = ?", id).Updates(map[string]any{
		"status": status, "attempts": attempts, "available_at": availableAt,
		"lease_expires_at": nil, "last_error_code": code, "updated_at": time.Now().UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("fail curator job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return impression.ErrNotFound
	}
	return nil
}

func (repository *ImpressionRepository) LoadCurationInput(ctx context.Context, job impression.Job) (impression.CurationInput, error) {
	type messageRow struct {
		ID        string
		Role      string
		Content   string
		CreatedAt time.Time
	}
	messages := make([]messageRow, 0)
	result := repository.database.WithContext(ctx).Table("messages").
		Select("messages.id, messages.role, messages.content, messages.created_at").
		Joins("JOIN runs ON runs.conversation_id = messages.conversation_id").
		Joins("JOIN conversations ON conversations.id = runs.conversation_id").
		Where("runs.id = ? AND conversations.owner_principal_id = ?", job.SourceRunID, job.OwnerPrincipalID).
		Order("messages.sequence DESC").Limit(12).Find(&messages)
	if result.Error != nil {
		return impression.CurationInput{}, fmt.Errorf("load curator messages: %w", result.Error)
	}
	input := impression.CurationInput{AgentID: job.AgentID, RunID: job.SourceRunID, Messages: make([]impression.SourceMessage, 0, len(messages))}
	for index := len(messages) - 1; index >= 0; index-- {
		row := messages[index]
		input.Messages = append(input.Messages, impression.SourceMessage{ID: row.ID, Role: row.Role, Content: truncateText(row.Content, 8000), At: row.CreatedAt})
	}
	type memoryRow struct {
		ID         string
		Kind       string
		Content    string
		Confidence float64
	}
	memories := make([]memoryRow, 0)
	if result := repository.database.WithContext(ctx).Table("memories").Select("id, kind, content, confidence").
		Where("owner_principal_id = ? AND agent_id = ? AND status = ?", job.OwnerPrincipalID, job.AgentID, "active").
		Order("updated_at DESC").Limit(50).Find(&memories); result.Error != nil {
		return impression.CurationInput{}, fmt.Errorf("load curator memories: %w", result.Error)
	}
	input.Memories = make([]impression.SourceMemory, 0, len(memories))
	for _, row := range memories {
		input.Memories = append(input.Memories, impression.SourceMemory{ID: row.ID, Kind: row.Kind, Content: truncateText(row.Content, 4000), Confidence: row.Confidence})
	}
	type toolResultRow struct {
		Sequence  int64
		EventType string
		Payload   json.RawMessage
	}
	toolResults := make([]toolResultRow, 0)
	if result := repository.database.WithContext(ctx).Table("run_events").Select("sequence, event_type, payload").
		Where("run_id = ? AND event_type IN ?", job.SourceRunID, []string{"tool.completed", "tool.failed"}).Order("sequence DESC").Limit(20).Find(&toolResults); result.Error != nil {
		return impression.CurationInput{}, fmt.Errorf("load curator Tool results: %w", result.Error)
	}
	input.ToolResults = make([]impression.SourceToolResult, 0, len(toolResults))
	for index := len(toolResults) - 1; index >= 0; index-- {
		row := toolResults[index]
		input.ToolResults = append(input.ToolResults, impression.SourceToolResult{EventID: fmt.Sprintf("%s:%d", job.SourceRunID, row.Sequence), Type: row.EventType, Summary: truncateText(string(row.Payload), 4000)})
	}
	existing, err := repository.List(ctx, job.OwnerPrincipalID, job.AgentID, "")
	if err != nil {
		return impression.CurationInput{}, err
	}
	for _, item := range existing {
		if item.Status == impression.StatusActive || item.Status == impression.StatusDismissed {
			input.ExistingImpressions = append(input.ExistingImpressions, item)
		}
		if len(input.ExistingImpressions) >= 40 {
			break
		}
	}
	return input, nil
}

func (repository *ImpressionRepository) ApplyCuration(ctx context.Context, job impression.Job, curation impression.Curation) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		changed := false
		localIDs := make(map[string]string)
		messageTimes := make(map[string]time.Time)
		memoryIDs := make(map[string]struct{})
		input, err := repository.LoadCurationInput(ctx, job)
		if err != nil {
			return err
		}
		for _, message := range input.Messages {
			messageTimes[message.ID] = message.At
		}
		for _, memory := range input.Memories {
			memoryIDs[memory.ID] = struct{}{}
		}
		now := time.Now().UTC()
		for _, draft := range curation.Impressions {
			if err := validateImpressionDraft(draft); err != nil {
				return err
			}
			switch draft.Action {
			case "create":
				id := ulid.Make().String()
				if draft.TargetID != "" {
					localIDs[draft.TargetID] = id
				}
				expiresAt := now.Add(30 * 24 * time.Hour)
				model := impressionModel{
					ID: id, OwnerPrincipalID: job.OwnerPrincipalID, AgentID: job.AgentID,
					Scope: draft.Scope, ImpressionKind: draft.Kind, Summary: draft.Summary,
					Details: nonNilMap(draft.Details), Tags: nonNilStringSlice(draft.Tags), Confidence: draft.Confidence,
					Salience: draft.Salience, FirstObservedAt: now, LastObservedAt: now, ExpiresAt: &expiresAt,
					DecayHalfLifeSeconds: int64((30 * 24 * time.Hour).Seconds()), Status: impression.StatusActive,
					GeneratorModel: curation.Generation.Model, GeneratorRunID: job.SourceRunID,
					PromptVersion: curation.Generation.PromptVersion, GeneratedAt: now,
				}
				if result := transaction.Create(&model); result.Error != nil {
					return fmt.Errorf("create Impression: %w", result.Error)
				}
				if err := createEvidence(transaction, job, id, draft, messageTimes, memoryIDs, now); err != nil {
					return err
				}
				changed = true
			case "update":
				targetID := translateLocalID(localIDs, draft.TargetID)
				result := transaction.Model(&impressionModel{}).
					Where("id = ? AND owner_principal_id = ? AND agent_id = ? AND status <> ?", targetID, job.OwnerPrincipalID, job.AgentID, impression.StatusDismissed).
					Updates(map[string]any{"summary": draft.Summary, "details_json": draft.Details, "tags": draft.Tags,
						"confidence": draft.Confidence, "salience": draft.Salience, "last_observed_at": now,
						"expires_at": now.Add(30 * 24 * time.Hour), "updated_at": now})
				if result.Error != nil {
					return fmt.Errorf("update generated Impression: %w", result.Error)
				}
				changed = changed || result.RowsAffected > 0
			case "resolve", "supersede":
				targetID := translateLocalID(localIDs, draft.TargetID)
				status := impression.StatusResolved
				if draft.Action == "supersede" {
					status = impression.StatusSuperseded
				}
				result := transaction.Model(&impressionModel{}).
					Where("id = ? AND owner_principal_id = ? AND agent_id = ? AND status = ?", targetID, job.OwnerPrincipalID, job.AgentID, impression.StatusActive).
					Updates(map[string]any{"status": status, "updated_at": now})
				if result.Error != nil {
					return fmt.Errorf("close generated Impression: %w", result.Error)
				}
				changed = changed || result.RowsAffected > 0
			}
		}
		for _, draft := range curation.Facts {
			if err := validateFactDraft(draft); err != nil {
				return err
			}
			sources := make([]string, 0, len(draft.SourceImpressionIDs))
			for _, source := range draft.SourceImpressionIDs {
				sources = append(sources, translateLocalID(localIDs, source))
			}
			encoded, _ := json.Marshal(draft.Value)
			digest := sha256.Sum256(encoded)
			candidate := factCandidateModel{
				ID: ulid.Make().String(), OwnerPrincipalID: job.OwnerPrincipalID, AgentID: job.AgentID,
				SubjectKind: draft.Subject, Namespace: draft.Namespace, FactKey: draft.Key,
				Value: draft.Value, ValueHash: hex.EncodeToString(digest[:]), Rationale: draft.Rationale,
				Confidence: draft.Confidence, CandidateVersion: 1, Status: "pending",
				GeneratorModel: curation.Generation.Model, GeneratorRunID: job.SourceRunID,
				PromptVersion: curation.Generation.PromptVersion, ProposedAt: now,
			}
			result := transaction.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
			if result.Error != nil {
				return fmt.Errorf("create Fact candidate: %w", result.Error)
			}
			if result.RowsAffected == 0 {
				continue
			}
			for _, source := range sources {
				link := factCandidateSourceModel{OwnerPrincipalID: job.OwnerPrincipalID, AgentID: job.AgentID, CandidateID: candidate.ID, ImpressionID: source}
				if result := transaction.Create(&link); result.Error != nil {
					return fmt.Errorf("link Fact candidate source: %w", result.Error)
				}
			}
			changed = true
		}
		if changed {
			if result := transaction.Table("agent_profiles").Where("owner_principal_id = ? AND agent_id = ?", job.OwnerPrincipalID, job.AgentID).
				Updates(map[string]any{"context_revision": gorm.Expr("context_revision + 1"), "updated_at": now}); result.Error != nil {
				return fmt.Errorf("advance Profile context revision: %w", result.Error)
			}
		}
		return nil
	})
}

func createEvidence(transaction *gorm.DB, job impression.Job, impressionID string, draft impression.ImpressionDraft, messageTimes map[string]time.Time, memoryIDs map[string]struct{}, now time.Time) error {
	for _, id := range draft.SourceMessageIDs {
		observedAt, ok := messageTimes[id]
		if !ok {
			continue
		}
		row := impressionEvidenceModel{OwnerPrincipalID: job.OwnerPrincipalID, AgentID: job.AgentID, ImpressionID: impressionID, EvidenceKind: impression.EvidenceMessage, EvidenceID: id, ObservedAt: observedAt}
		if result := transaction.Create(&row); result.Error != nil {
			return fmt.Errorf("create message evidence: %w", result.Error)
		}
	}
	for _, id := range draft.SourceMemoryIDs {
		if _, ok := memoryIDs[id]; !ok {
			continue
		}
		row := impressionEvidenceModel{OwnerPrincipalID: job.OwnerPrincipalID, AgentID: job.AgentID, ImpressionID: impressionID, EvidenceKind: impression.EvidenceMemory, EvidenceID: id, ObservedAt: now}
		if result := transaction.Create(&row); result.Error != nil {
			return fmt.Errorf("create Memory evidence: %w", result.Error)
		}
	}
	return nil
}

func validateImpressionDraft(draft impression.ImpressionDraft) error {
	if draft.Action != "create" && draft.Action != "update" && draft.Action != "resolve" && draft.Action != "supersede" {
		return impression.ErrInvalid
	}
	if draft.Action != "create" && strings.TrimSpace(draft.TargetID) == "" {
		return impression.ErrInvalid
	}
	if draft.Action == "create" || draft.Action == "update" {
		if !validScope(draft.Scope) || !validKind(draft.Kind) || strings.TrimSpace(draft.Summary) == "" || len(draft.Summary) > 4000 || draft.Confidence < 0 || draft.Confidence > 1 || draft.Salience < 0 || draft.Salience > 1 {
			return impression.ErrInvalid
		}
	}
	return nil
}

func validateFactDraft(draft impression.FactDraft) error {
	if !validFactSubject(draft.Subject) || strings.TrimSpace(draft.Namespace) == "" || len(draft.Namespace) > 80 || strings.TrimSpace(draft.Key) == "" || len(draft.Key) > 120 || draft.Value == nil || draft.Confidence < 0 || draft.Confidence > 1 || len(draft.SourceImpressionIDs) == 0 {
		return impression.ErrInvalid
	}
	return nil
}

func validScope(value impression.Scope) bool {
	return value == impression.ScopeUser || value == impression.ScopeTask || value == impression.ScopeProject || value == impression.ScopeEnvironment || value == impression.ScopeRelationship
}

func validKind(value impression.Kind) bool {
	switch value {
	case impression.KindCurrentTask, impression.KindRecentInterest, impression.KindKnowledgeExposure, impression.KindAcquiredInfo, impression.KindOpenLoop, impression.KindTemporaryPref, impression.KindWorkingStyle, impression.KindRecentDecision:
		return true
	default:
		return false
	}
}

func validFactSubject(value impression.FactSubject) bool {
	return value == impression.FactAgent || value == impression.FactUser || value == impression.FactProject || value == impression.FactTask
}

func translateLocalID(ids map[string]string, value string) string {
	if translated, ok := ids[value]; ok {
		return translated
	}
	return value
}

func mapImpressionNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return impression.ErrNotFound
	}
	return err
}

func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func nonNilStringSlice(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}

func truncateText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}
