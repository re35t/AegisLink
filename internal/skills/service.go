package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/agent"
	"gopkg.in/yaml.v3"
)

const maximumSkillBytes = 128 * 1024

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type AgentReader interface {
	Get(context.Context, string, string) (agent.Agent, error)
}

type Service struct {
	repository Repository
	agents     AgentReader
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func NewService(repository Repository, agents AgentReader) *Service {
	return &Service{repository: repository, agents: agents}
}

func (service *Service) List(ctx context.Context, principalID, agentID string) ([]Skill, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return nil, err
	}
	return service.repository.List(ctx, principalID, agentID)
}

func (service *Service) Install(ctx context.Context, principalID, agentID, content, version string) (Skill, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Skill{}, err
	}
	metadata, err := parseSkill(content)
	if err != nil {
		return Skill{}, err
	}
	hash := sha256.Sum256([]byte(content))
	hashValue := "sha256:" + hex.EncodeToString(hash[:])
	version = strings.TrimSpace(version)
	if version == "" || version == "local" {
		version = "local-" + hex.EncodeToString(hash[:6])
	}
	if utf8.RuneCountInString(version) > 80 {
		return Skill{}, ErrInvalid
	}
	return service.repository.Install(ctx, Skill{
		ID: ulid.Make().String(), VersionID: ulid.Make().String(), OwnerPrincipalID: principalID, AgentID: agentID,
		Name: metadata.Name, Description: metadata.Description, Version: version,
		SourceType: "inline", Content: content, ContentHash: hashValue, Enabled: true,
	})
}

func (service *Service) SetEnabled(ctx context.Context, principalID, agentID, skillID string, enabled bool) (Skill, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Skill{}, err
	}
	return service.repository.SetEnabled(ctx, principalID, agentID, skillID, enabled)
}

func (service *Service) Uninstall(ctx context.Context, principalID, agentID, skillID string) error {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return err
	}
	return service.repository.Uninstall(ctx, principalID, agentID, skillID)
}

func (service *Service) Enabled(ctx context.Context, principalID, agentID string) ([]Skill, error) {
	return service.repository.Enabled(ctx, principalID, agentID)
}

func (service *Service) authorize(ctx context.Context, principalID, agentID string) error {
	_, err := service.agents.Get(ctx, principalID, agentID)
	return err
}

func parseSkill(content string) (frontmatter, error) {
	if len(content) == 0 || len(content) > maximumSkillBytes || !utf8.ValidString(content) {
		return frontmatter{}, ErrInvalid
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return frontmatter{}, ErrInvalid
	}
	closing := strings.Index(normalized[4:], "\n---\n")
	if closing < 0 {
		return frontmatter{}, ErrInvalid
	}
	closing += 4
	var metadata frontmatter
	if err := yaml.Unmarshal([]byte(normalized[4:closing]), &metadata); err != nil {
		return frontmatter{}, ErrInvalid
	}
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.Description = strings.TrimSpace(metadata.Description)
	if len(metadata.Name) > 64 || !skillNamePattern.MatchString(metadata.Name) || metadata.Description == "" || utf8.RuneCountInString(metadata.Description) > 1024 {
		return frontmatter{}, ErrInvalid
	}
	return metadata, nil
}
