package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/re35t/AegisLink/internal/skills"
)

type SkillRepository struct {
	database *sql.DB
}

var _ skills.Repository = (*SkillRepository)(nil)

func NewSkillRepository(database *sql.DB) *SkillRepository {
	return &SkillRepository{database: database}
}

func (repository *SkillRepository) List(ctx context.Context, principalID, agentID string) ([]skills.Skill, error) {
	return repository.list(ctx, principalID, agentID, false)
}

func (repository *SkillRepository) Enabled(ctx context.Context, principalID, agentID string) ([]skills.Skill, error) {
	return repository.list(ctx, principalID, agentID, true)
}

func (repository *SkillRepository) list(ctx context.Context, principalID, agentID string, enabledOnly bool) ([]skills.Skill, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT packages.id, versions.id, packages.owner_principal_id, bindings.agent_id,
		       packages.name, versions.description, versions.version, versions.source_type,
		       versions.manifest_content, versions.content_hash, bindings.enabled,
		       packages.created_at, GREATEST(packages.updated_at, bindings.updated_at, versions.created_at)
		FROM agent_skills bindings
		JOIN skill_packages packages
		  ON packages.id=bindings.package_id AND packages.owner_principal_id=bindings.owner_principal_id
		JOIN skill_versions versions
		  ON versions.id=bindings.version_id AND versions.package_id=bindings.package_id
		 AND versions.owner_principal_id=bindings.owner_principal_id
		WHERE bindings.owner_principal_id=$1 AND bindings.agent_id=$2
		  AND (NOT $3 OR bindings.enabled)
		ORDER BY packages.name`, principalID, agentID, enabledOnly)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()
	items := make([]skills.Skill, 0)
	for rows.Next() {
		var item skills.Skill
		if err := rows.Scan(skillScanTargets(&item)...); err != nil {
			return nil, fmt.Errorf("scan skill: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *SkillRepository) Install(ctx context.Context, item skills.Skill) (skills.Skill, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return skills.Skill{}, fmt.Errorf("begin skill install: %w", err)
	}
	defer transaction.Rollback()

	err = transaction.QueryRowContext(ctx, `
		INSERT INTO skill_packages (id, owner_principal_id, name)
		VALUES ($1, $2, $3)
		ON CONFLICT (owner_principal_id, name) DO UPDATE SET name=EXCLUDED.name
		RETURNING id, created_at`, item.ID, item.OwnerPrincipalID, item.Name,
	).Scan(&item.ID, &item.CreatedAt)
	if err != nil {
		return skills.Skill{}, fmt.Errorf("create or resolve skill package: %w", err)
	}

	requestedVersionID := item.VersionID
	err = transaction.QueryRowContext(ctx, `
		INSERT INTO skill_versions (
			id, package_id, owner_principal_id, version, description,
			manifest_content, content_hash, source_type
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT DO NOTHING
		RETURNING id`,
		requestedVersionID, item.ID, item.OwnerPrincipalID, item.Version, item.Description,
		item.Content, item.ContentHash, item.SourceType,
	).Scan(&item.VersionID)
	if errors.Is(err, sql.ErrNoRows) {
		if err := resolveExistingSkillVersion(ctx, transaction, &item); err != nil {
			return skills.Skill{}, err
		}
	} else if err != nil {
		return skills.Skill{}, fmt.Errorf("create skill version: %w", err)
	}

	err = transaction.QueryRowContext(ctx, `
		INSERT INTO agent_skills (owner_principal_id, agent_id, package_id, version_id, enabled)
		VALUES ($1, $2, $3, $4, true)
		ON CONFLICT (agent_id, package_id) DO UPDATE
		SET version_id=EXCLUDED.version_id, enabled=true, updated_at=now()
		RETURNING enabled, updated_at`,
		item.OwnerPrincipalID, item.AgentID, item.ID, item.VersionID,
	).Scan(&item.Enabled, &item.UpdatedAt)
	if err != nil {
		return skills.Skill{}, fmt.Errorf("bind skill version to agent: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return skills.Skill{}, fmt.Errorf("commit skill install: %w", err)
	}
	return item, nil
}

func resolveExistingSkillVersion(ctx context.Context, transaction *sql.Tx, item *skills.Skill) error {
	var existing skills.Skill
	err := transaction.QueryRowContext(ctx, `
		SELECT id, version, description, source_type, manifest_content, content_hash
		FROM skill_versions
		WHERE package_id=$1 AND owner_principal_id=$2
		  AND (version=$3 OR content_hash=$4)
		ORDER BY CASE WHEN content_hash=$4 THEN 0 ELSE 1 END, created_at, id
		LIMIT 1`, item.ID, item.OwnerPrincipalID, item.Version, item.ContentHash,
	).Scan(
		&existing.VersionID, &existing.Version, &existing.Description, &existing.SourceType,
		&existing.Content, &existing.ContentHash,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return skills.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("resolve skill version: %w", err)
	}
	if existing.ContentHash != item.ContentHash {
		return skills.ErrConflict
	}
	item.VersionID = existing.VersionID
	item.Version = existing.Version
	item.Description = existing.Description
	item.SourceType = existing.SourceType
	item.Content = existing.Content
	item.ContentHash = existing.ContentHash
	return nil
}

func (repository *SkillRepository) SetEnabled(ctx context.Context, principalID, agentID, skillID string, enabled bool) (skills.Skill, error) {
	var item skills.Skill
	err := repository.database.QueryRowContext(ctx, `
		UPDATE agent_skills bindings
		SET enabled=$4, updated_at=now()
		FROM skill_packages packages, skill_versions versions
		WHERE bindings.owner_principal_id=$1 AND bindings.agent_id=$2 AND bindings.package_id=$3
		  AND packages.id=bindings.package_id AND packages.owner_principal_id=bindings.owner_principal_id
		  AND versions.id=bindings.version_id AND versions.package_id=bindings.package_id
		  AND versions.owner_principal_id=bindings.owner_principal_id
		RETURNING packages.id, versions.id, packages.owner_principal_id, bindings.agent_id,
		          packages.name, versions.description, versions.version, versions.source_type,
		          versions.manifest_content, versions.content_hash, bindings.enabled,
		          packages.created_at, GREATEST(packages.updated_at, bindings.updated_at, versions.created_at)`,
		principalID, agentID, skillID, enabled,
	).Scan(skillScanTargets(&item)...)
	if errors.Is(err, sql.ErrNoRows) {
		return skills.Skill{}, skills.ErrNotFound
	}
	if err != nil {
		return skills.Skill{}, fmt.Errorf("set skill enabled: %w", err)
	}
	return item, nil
}

func (repository *SkillRepository) Uninstall(ctx context.Context, principalID, agentID, skillID string) error {
	result, err := repository.database.ExecContext(ctx, `
		DELETE FROM agent_skills
		WHERE owner_principal_id=$1 AND agent_id=$2 AND package_id=$3`, principalID, agentID, skillID)
	if err != nil {
		return fmt.Errorf("unbind skill package: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read unbound skill rows: %w", err)
	}
	if changed == 0 {
		return skills.ErrNotFound
	}
	return nil
}

func skillScanTargets(item *skills.Skill) []any {
	return []any{
		&item.ID, &item.VersionID, &item.OwnerPrincipalID, &item.AgentID, &item.Name,
		&item.Description, &item.Version, &item.SourceType, &item.Content, &item.ContentHash,
		&item.Enabled, &item.CreatedAt, &item.UpdatedAt,
	}
}
