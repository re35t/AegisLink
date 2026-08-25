package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/re35t/AegisLink/internal/skills"
	"gorm.io/gorm"
)

type SkillRepository struct {
	database *gorm.DB
}

type skillRow struct {
	ID               string
	VersionID        string
	OwnerPrincipalID string
	AgentID          string
	Name             string
	Description      string
	Version          string
	SourceType       string
	Content          string
	ContentHash      string
	Enabled          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (row skillRow) skill() skills.Skill {
	return skills.Skill{
		ID: row.ID, VersionID: row.VersionID, OwnerPrincipalID: row.OwnerPrincipalID, AgentID: row.AgentID,
		Name: row.Name, Description: row.Description, Version: row.Version, SourceType: row.SourceType,
		Content: row.Content, ContentHash: row.ContentHash, Enabled: row.Enabled,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

var _ skills.Repository = (*SkillRepository)(nil)

func NewSkillRepository(database *gorm.DB) *SkillRepository {
	return &SkillRepository{database: database}
}

func (repository *SkillRepository) List(ctx context.Context, principalID, agentID string) ([]skills.Skill, error) {
	return repository.list(ctx, principalID, agentID, false)
}

func (repository *SkillRepository) Enabled(ctx context.Context, principalID, agentID string) ([]skills.Skill, error) {
	return repository.list(ctx, principalID, agentID, true)
}

func (repository *SkillRepository) list(ctx context.Context, principalID, agentID string, enabledOnly bool) ([]skills.Skill, error) {
	rows := make([]skillRow, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT packages.id, versions.id AS version_id, packages.owner_principal_id, bindings.agent_id,
		       packages.name, versions.description, versions.version, versions.source_type,
		       versions.manifest_content AS content, versions.content_hash, bindings.enabled,
		       packages.created_at, GREATEST(packages.updated_at, bindings.updated_at, versions.created_at) AS updated_at
		FROM agent_skills bindings
		JOIN skill_packages packages
		  ON packages.id=bindings.package_id AND packages.owner_principal_id=bindings.owner_principal_id
		JOIN skill_versions versions
		  ON versions.id=bindings.version_id AND versions.package_id=bindings.package_id
		 AND versions.owner_principal_id=bindings.owner_principal_id
		WHERE bindings.owner_principal_id=$1 AND bindings.agent_id=$2
		  AND (NOT $3 OR bindings.enabled)
		ORDER BY packages.name`, principalID, agentID, enabledOnly).Scan(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("list skills: %w", result.Error)
	}
	items := make([]skills.Skill, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.skill())
	}
	for index := range items {
		items[index].Files, result.Error = listSkillFiles(repository.database.WithContext(ctx), items[index].VersionID)
		if result.Error != nil {
			return nil, result.Error
		}
	}
	return items, nil
}

func (repository *SkillRepository) Install(ctx context.Context, item skills.Skill) (skills.Skill, error) {
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var packageResult struct {
			ID        string
			CreatedAt time.Time
		}
		if err := scanOne(transaction, &packageResult, `
			INSERT INTO skill_packages (id, owner_principal_id, name)
			VALUES ($1, $2, $3)
			ON CONFLICT (owner_principal_id, name) DO UPDATE SET name=EXCLUDED.name
			RETURNING id, created_at`, item.ID, item.OwnerPrincipalID, item.Name); err != nil {
			return fmt.Errorf("create or resolve skill package: %w", err)
		}
		item.ID = packageResult.ID
		item.CreatedAt = packageResult.CreatedAt

		requestedVersionID := item.VersionID
		var versionResult struct{ ID string }
		err := scanOne(transaction, &versionResult, `
			INSERT INTO skill_versions (
				id, package_id, owner_principal_id, version, description,
				manifest_content, content_hash, source_type
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
			RETURNING id`,
			requestedVersionID, item.ID, item.OwnerPrincipalID, item.Version, item.Description,
			item.Content, item.ContentHash, item.SourceType)
		createdVersion := err == nil
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := resolveExistingSkillVersion(transaction, &item); err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("create skill version: %w", err)
		} else {
			item.VersionID = versionResult.ID
		}

		if createdVersion {
			for _, file := range item.Files {
				result := exec(transaction, `
					INSERT INTO skill_version_files (
						version_id, path, media_type, size_bytes, content_hash, text_readable, content
					) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
					item.VersionID, file.Path, file.MediaType, file.SizeBytes,
					file.ContentHash, file.TextReadable, file.Content)
				if result.Error != nil {
					return fmt.Errorf("store Skill bundle file %q: %w", file.Path, result.Error)
				}
			}
		}

		var bindingResult struct {
			Enabled   bool
			UpdatedAt time.Time
		}
		if err := scanOne(transaction, &bindingResult, `
			INSERT INTO agent_skills (owner_principal_id, agent_id, package_id, version_id, enabled)
			VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (agent_id, package_id) DO UPDATE
			SET version_id=EXCLUDED.version_id, enabled=true, updated_at=now()
			RETURNING enabled, updated_at`,
			item.OwnerPrincipalID, item.AgentID, item.ID, item.VersionID); err != nil {
			return fmt.Errorf("bind skill version to agent: %w", err)
		}
		item.Enabled = bindingResult.Enabled
		item.UpdatedAt = bindingResult.UpdatedAt
		item.Files, err = listSkillFiles(transaction, item.VersionID)
		return err
	})
	if err != nil {
		return skills.Skill{}, err
	}
	return item, nil
}

func resolveExistingSkillVersion(transaction *gorm.DB, item *skills.Skill) error {
	var row skillRow
	err := scanOne(transaction, &row, `
		SELECT id AS version_id, version, description, source_type,
		       manifest_content AS content, content_hash
		FROM skill_versions
		WHERE package_id=$1 AND owner_principal_id=$2
		  AND (version=$3 OR content_hash=$4)
		ORDER BY CASE WHEN content_hash=$4 THEN 0 ELSE 1 END, created_at, id
		LIMIT 1`, item.ID, item.OwnerPrincipalID, item.Version, item.ContentHash)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return skills.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("resolve skill version: %w", err)
	}
	existing := row.skill()
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
	var row skillRow
	err := scanOne(repository.database.WithContext(ctx), &row, `
		UPDATE agent_skills bindings
		SET enabled=$4, updated_at=now()
		FROM skill_packages packages, skill_versions versions
		WHERE bindings.owner_principal_id=$1 AND bindings.agent_id=$2 AND bindings.package_id=$3
		  AND packages.id=bindings.package_id AND packages.owner_principal_id=bindings.owner_principal_id
		  AND versions.id=bindings.version_id AND versions.package_id=bindings.package_id
		  AND versions.owner_principal_id=bindings.owner_principal_id
		RETURNING packages.id, versions.id AS version_id, packages.owner_principal_id, bindings.agent_id,
		          packages.name, versions.description, versions.version, versions.source_type,
		          versions.manifest_content AS content, versions.content_hash, bindings.enabled,
		          packages.created_at, GREATEST(packages.updated_at, bindings.updated_at, versions.created_at) AS updated_at`,
		principalID, agentID, skillID, enabled)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return skills.Skill{}, skills.ErrNotFound
	}
	if err != nil {
		return skills.Skill{}, fmt.Errorf("set skill enabled: %w", err)
	}
	item := row.skill()
	item.Files, err = listSkillFiles(repository.database.WithContext(ctx), item.VersionID)
	if err != nil {
		return skills.Skill{}, err
	}
	return item, nil
}

func (repository *SkillRepository) Uninstall(ctx context.Context, principalID, agentID, skillID string) error {
	result := exec(repository.database.WithContext(ctx), `
		DELETE FROM agent_skills
		WHERE owner_principal_id=$1 AND agent_id=$2 AND package_id=$3`, principalID, agentID, skillID)
	if result.Error != nil {
		return fmt.Errorf("unbind skill package: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return skills.ErrNotFound
	}
	return nil
}

func (repository *SkillRepository) ReadFile(ctx context.Context, principalID, agentID, skillID, filePath string) (skills.File, error) {
	var file skills.File
	err := scanOne(repository.database.WithContext(ctx), &file, `
		SELECT files.path, files.media_type, files.size_bytes, files.content_hash,
		       files.text_readable, files.content
		FROM agent_skills bindings
		JOIN skill_version_files files ON files.version_id=bindings.version_id
		WHERE bindings.owner_principal_id=$1 AND bindings.agent_id=$2
		  AND bindings.package_id=$3 AND bindings.enabled AND files.path=$4`,
		principalID, agentID, skillID, filePath)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return skills.File{}, skills.ErrNotFound
	}
	if err != nil {
		return skills.File{}, fmt.Errorf("read Skill bundle file: %w", err)
	}
	return file, nil
}

func listSkillFiles(database *gorm.DB, versionID string) ([]skills.File, error) {
	files := make([]skills.File, 0)
	result := raw(database, `
		SELECT path, media_type, size_bytes, content_hash, text_readable
		FROM skill_version_files
		WHERE version_id=$1
		ORDER BY path`, versionID).Scan(&files)
	if result.Error != nil {
		return nil, fmt.Errorf("list Skill bundle files: %w", result.Error)
	}
	return files, nil
}
