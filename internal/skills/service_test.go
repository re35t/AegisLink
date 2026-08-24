package skills

import (
	"context"
	"strings"
	"testing"

	"github.com/re35t/AegisLink/internal/agent"
)

func TestParseSkill(t *testing.T) {
	metadata, err := parseSkill("---\nname: go-review\ndescription: Review Go changes when code quality matters.\n---\n\n# Instructions\n")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Name != "go-review" || metadata.Description == "" {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
}

func TestParseSkillRejectsInvalidName(t *testing.T) {
	_, err := parseSkill("---\nname: Go Review\ndescription: invalid\n---\nbody")
	if err != ErrInvalid {
		t.Fatalf("error = %v", err)
	}
}

func TestInstallCreatesContentAddressedLocalVersion(t *testing.T) {
	repository := &recordingRepository{}
	service := NewService(repository, acceptingAgentReader{})
	installed, err := service.Install(
		t.Context(), "principal-one", "agent-one",
		"---\nname: go-review\ndescription: Review Go changes.\n---\nbody", "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if installed.ID == "" || installed.VersionID == "" || installed.ID == installed.VersionID {
		t.Fatalf("package/version identifiers were not separated: %#v", installed)
	}
	if !strings.HasPrefix(installed.Version, "local-") || installed.SourceType != "inline" {
		t.Fatalf("unexpected local version metadata: %#v", installed)
	}
	if repository.installed.ContentHash == "" || repository.installed != installed {
		t.Fatalf("repository did not receive immutable version: %#v", repository.installed)
	}
}

type acceptingAgentReader struct{}

func (acceptingAgentReader) Get(context.Context, string, string) (agent.Agent, error) {
	return agent.Agent{}, nil
}

type recordingRepository struct {
	installed Skill
}

func (repository *recordingRepository) List(context.Context, string, string) ([]Skill, error) {
	return nil, nil
}

func (repository *recordingRepository) Install(_ context.Context, skill Skill) (Skill, error) {
	repository.installed = skill
	return skill, nil
}

func (repository *recordingRepository) SetEnabled(context.Context, string, string, string, bool) (Skill, error) {
	return Skill{}, nil
}

func (repository *recordingRepository) Uninstall(context.Context, string, string, string) error {
	return nil
}

func (repository *recordingRepository) Enabled(context.Context, string, string) ([]Skill, error) {
	return nil, nil
}
