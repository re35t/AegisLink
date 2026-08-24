package skills

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
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
	if repository.installed.ContentHash == "" || repository.installed.ID != installed.ID || len(installed.Files) != 1 || installed.Files[0].Path != "SKILL.md" {
		t.Fatalf("repository did not receive immutable version: %#v", repository.installed)
	}
}

func TestImportZipBundleNormalizesRootAndStoresResources(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	files := map[string]string{
		"portable-skill/SKILL.md":                 "---\nname: portable-skill\ndescription: Use portable references.\n---\nRead references/guide.md when needed.",
		"portable-skill/references/guide.md":      "# Guide\nUse the safe path.",
		"portable-skill/scripts/format-output.sh": "#!/bin/sh\necho read-only",
	}
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	repository := &recordingRepository{}
	service := NewService(repository, acceptingAgentReader{})
	installed, err := service.Import(t.Context(), "principal-one", "agent-one", ImportRequest{
		FileName: "portable-skill.zip", Data: archive.Bytes(), Version: "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if installed.Name != "portable-skill" || installed.SourceType != "local" || installed.Version != "1.0.0" || len(installed.Files) != 3 {
		t.Fatalf("unexpected imported Skill: %#v", installed)
	}
	for _, file := range installed.Files {
		if file.Path == "scripts/format-output.sh" && !file.TextReadable {
			t.Fatalf("text script should be readable without becoming executable: %#v", file)
		}
	}
}

func TestImportZipBundleRejectsTraversal(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	manifest, err := writer.Create("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = manifest.Write([]byte("---\nname: unsafe\ndescription: Unsafe bundle.\n---\n"))
	outside, err := writer.Create("../secret.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = outside.Write([]byte("secret"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	service := NewService(&recordingRepository{}, acceptingAgentReader{})
	_, err = service.Import(t.Context(), "principal-one", "agent-one", ImportRequest{
		FileName: "unsafe.zip", Data: archive.Bytes(),
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected traversal rejection, got %v", err)
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

func (repository *recordingRepository) ReadFile(context.Context, string, string, string, string) (File, error) {
	return File{}, ErrNotFound
}
