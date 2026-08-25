package skills

import (
	"context"
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
	return service.installFiles(ctx, principalID, agentID, []File{newFile("SKILL.md", []byte(content))}, version, "inline")
}

func (service *Service) Import(ctx context.Context, principalID, agentID string, request ImportRequest) (Skill, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return Skill{}, err
	}
	files, err := parseImportedBundle(request.FileName, request.Data)
	if err != nil {
		return Skill{}, err
	}
	return service.installFiles(ctx, principalID, agentID, files, request.Version, "local")

}

func (service *Service) installFiles(ctx context.Context, principalID, agentID string, files []File, version, sourceType string) (Skill, error) {
	manifest, ok := fileByPath(files, "SKILL.md")
	if !ok {
		return Skill{}, ErrInvalidBundle
	}
	metadata, err := parseSkill(string(manifest.Content))
	if err != nil {
		return Skill{}, err
	}
	hashValue := bundleContentHash(files)
	version, err = normalizedVersion(version, hashValue)
	if err != nil {
		return Skill{}, err
	}
	return service.repository.Install(ctx, Skill{
		ID: ulid.Make().String(), VersionID: ulid.Make().String(), OwnerPrincipalID: principalID, AgentID: agentID,
		Name: metadata.Name, Description: metadata.Description, Version: version,
		SourceType: sourceType, Content: string(manifest.Content), ContentHash: hashValue, Files: files, Enabled: true,
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

func (service *Service) ProfileCapabilities(ctx context.Context, principalID, agentID string) ([]agent.ProfileCapability, error) {
	items, err := service.List(ctx, principalID, agentID)
	if err != nil {
		return nil, err
	}
	capabilities := make([]agent.ProfileCapability, 0, len(items))
	for _, item := range items {
		capabilities = append(capabilities, agent.ProfileCapability{
			ID:          "skill:" + item.ID,
			Name:        item.Name,
			Description: item.Description,
			Kind:        agent.CapabilitySkill,
			Tags:        []string{"skill", item.SourceType},
			Source:      agent.CapabilitySourceRuntime,
			Confidence:  1,
			Callable:    item.Enabled,
		})
	}
	return capabilities, nil
}

func (service *Service) ReadFile(ctx context.Context, principalID, agentID, skillID, filePath string) (File, error) {
	if err := service.authorize(ctx, principalID, agentID); err != nil {
		return File{}, err
	}
	file, err := service.repository.ReadFile(ctx, principalID, agentID, skillID, filePath)
	if err != nil {
		return File{}, err
	}
	if !file.TextReadable || !utf8.Valid(file.Content) {
		return File{}, ErrResourceUnreadable
	}
	return file, nil
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
