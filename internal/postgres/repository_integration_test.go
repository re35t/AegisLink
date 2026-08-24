package postgres

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/memory"
	"github.com/re35t/AegisLink/internal/skills"
	"github.com/re35t/AegisLink/migrations"
)

func TestRepositoryConversationRunLifecycle(t *testing.T) {
	const ownerID = "01K34A00000000000000000000"
	const agentID = "01K34A00000000000000000001"

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		TRUNCATE account_sessions, user_accounts, run_events, runs, messages, conversations, agents, human_principals CASCADE`); err != nil {
		t.Fatal(err)
	}
	repository := NewConversationRepository(database)
	if _, err := database.ExecContext(t.Context(), `
		INSERT INTO human_principals (id, display_name) VALUES ($1, 'Test user')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		VALUES ($1, $2, 'Aegis', '', 'Test prompt')`, agentID, ownerID); err != nil {
		t.Fatal(err)
	}
	created, err := repository.CreateConversation(t.Context(), ownerID, agentID, "New conversation")
	if err != nil {
		t.Fatal(err)
	}
	message, run, err := repository.CreateMessageRun(t.Context(), ownerID, created.ID, "message-1", "run-1", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if message.Sequence != 1 || run.Status != "queued" {
		t.Fatalf("unexpected initial state: message=%#v run=%#v", message, run)
	}
	_, _, err = repository.CreateMessageRun(t.Context(), ownerID, created.ID, "message-2", "run-2", "duplicate")
	if !errors.Is(err, conversation.ErrActiveRun) {
		t.Fatalf("expected active run conflict, got %v", err)
	}
	if err := repository.MarkRunRunning(t.Context(), ownerID, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AppendRunEvent(t.Context(), ownerID, run.ID, "run.started", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	assistant, err := repository.CompleteRun(t.Context(), ownerID, run.ID, created.ID, "message-3", "hello back")
	if err != nil {
		t.Fatal(err)
	}
	if assistant.Sequence != 2 {
		t.Fatalf("assistant sequence = %d", assistant.Sequence)
	}
	events, err := repository.ListRunEvents(t.Context(), ownerID, run.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Sequence != 1 || events[1].Type != "message.completed" {
		t.Fatalf("unexpected events: %#v", events)
	}
	if _, err := repository.GetRun(t.Context(), "another-principal", run.ID); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatalf("run should not be visible across principals, got %v", err)
	}
	detail, err := repository.GetConversation(t.Context(), ownerID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Messages) != 2 || detail.ActiveRun != nil {
		t.Fatalf("unexpected detail: %#v", detail)
	}
}

func TestAgentCapabilitiesRemainIsolated(t *testing.T) {
	const (
		principalOne = "principal-capability-one"
		principalTwo = "principal-capability-two"
		agentOne     = "agent-capability-one"
		agentSibling = "agent-capability-sibling"
		agentTwo     = "agent-capability-two"
	)
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		TRUNCATE account_sessions, user_accounts, run_events, runs, messages, conversations,
		         mcp_tools, mcp_servers, agent_skills, skill_version_files, skill_versions, skill_packages,
		         memories, agents, human_principals CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		INSERT INTO human_principals (id, display_name) VALUES ($1, 'One'), ($2, 'Two')`,
		principalOne, principalTwo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		VALUES ($3, $1, 'Agent one', '', 'prompt'),
		       ($4, $1, 'Agent sibling', '', 'prompt'),
		       ($5, $2, 'Agent two', '', 'prompt')`,
		principalOne, principalTwo, agentOne, agentSibling, agentTwo); err != nil {
		t.Fatal(err)
	}

	memoryRepository := NewMemoryRepository(database)
	createdMemory, err := memoryRepository.Create(t.Context(), memory.Memory{
		ID: "memory-one", OwnerPrincipalID: principalOne, AgentID: agentOne,
		Kind: memory.Semantic, Content: "private preference", Confidence: 1, SourceURI: "manual://test", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if createdMemory.AgentID != agentOne {
		t.Fatalf("memory agent = %q", createdMemory.AgentID)
	}
	otherMemories, err := memoryRepository.List(t.Context(), principalTwo, agentTwo)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherMemories) != 0 {
		t.Fatalf("agent two can see agent one's memory: %#v", otherMemories)
	}

	skillRepository := NewSkillRepository(database)
	firstContent := "---\nname: private-skill\ndescription: private v1\n---\n"
	firstSkill, err := skillRepository.Install(t.Context(), skills.Skill{
		ID: "skill-package-one", VersionID: "skill-version-one", OwnerPrincipalID: principalOne, AgentID: agentOne,
		Name: "private-skill", Description: "private v1", Version: "1.0.0", SourceType: "inline",
		Content: firstContent, ContentHash: "sha256:one", Enabled: true,
		Files: []skills.File{{
			Path: "SKILL.md", MediaType: "text/markdown", SizeBytes: int64(len(firstContent)),
			ContentHash: "sha256:manifest-one", TextReadable: true, Content: []byte(firstContent),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	secondContent := "---\nname: private-skill\ndescription: private v2\n---\n"
	resourceContent := "private reference"
	secondSkill, err := skillRepository.Install(t.Context(), skills.Skill{
		ID: "ignored-package-id", VersionID: "skill-version-two", OwnerPrincipalID: principalOne, AgentID: agentSibling,
		Name: "private-skill", Description: "private v2", Version: "2.0.0", SourceType: "inline",
		Content: secondContent, ContentHash: "sha256:two", Enabled: true,
		Files: []skills.File{
			{Path: "SKILL.md", MediaType: "text/markdown", SizeBytes: int64(len(secondContent)), ContentHash: "sha256:manifest-two", TextReadable: true, Content: []byte(secondContent)},
			{Path: "references/private.md", MediaType: "text/markdown", SizeBytes: int64(len(resourceContent)), ContentHash: "sha256:reference", TextReadable: true, Content: []byte(resourceContent)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstSkill.ID != secondSkill.ID || firstSkill.VersionID == secondSkill.VersionID {
		t.Fatalf("expected shared package with distinct versions: first=%#v second=%#v", firstSkill, secondSkill)
	}
	firstAgentSkills, err := skillRepository.List(t.Context(), principalOne, agentOne)
	if err != nil {
		t.Fatal(err)
	}
	siblingSkills, err := skillRepository.List(t.Context(), principalOne, agentSibling)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstAgentSkills) != 1 || firstAgentSkills[0].Version != "1.0.0" || len(firstAgentSkills[0].Files) != 1 ||
		len(siblingSkills) != 1 || siblingSkills[0].Version != "2.0.0" || len(siblingSkills[0].Files) != 2 {
		t.Fatalf("Agent version bindings leaked: first=%#v sibling=%#v", firstAgentSkills, siblingSkills)
	}
	resource, err := skillRepository.ReadFile(t.Context(), principalOne, agentSibling, secondSkill.ID, "references/private.md")
	if err != nil || string(resource.Content) != "private reference" {
		t.Fatalf("Skill resource was not readable by selected Agent: file=%#v err=%v", resource, err)
	}
	if _, err := skillRepository.ReadFile(t.Context(), principalOne, agentOne, firstSkill.ID, "references/private.md"); !errors.Is(err, skills.ErrNotFound) {
		t.Fatalf("Agent should not read a resource from another selected version, got %v", err)
	}
	_, err = skillRepository.Install(t.Context(), skills.Skill{
		ID: "another-package-id", VersionID: "conflicting-version", OwnerPrincipalID: principalOne, AgentID: agentSibling,
		Name: "private-skill", Description: "conflict", Version: "2.0.0", SourceType: "inline",
		Content: "different", ContentHash: "sha256:different", Enabled: true,
	})
	if !errors.Is(err, skills.ErrConflict) {
		t.Fatalf("expected immutable version conflict, got %v", err)
	}
	otherSkills, err := skillRepository.List(t.Context(), principalTwo, agentTwo)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherSkills) != 0 {
		t.Fatalf("agent two can see agent one's skill: %#v", otherSkills)
	}
	if err := skillRepository.Uninstall(t.Context(), principalOne, agentOne, firstSkill.ID); err != nil {
		t.Fatal(err)
	}
	firstAgentSkills, err = skillRepository.List(t.Context(), principalOne, agentOne)
	if err != nil {
		t.Fatal(err)
	}
	siblingSkills, err = skillRepository.List(t.Context(), principalOne, agentSibling)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstAgentSkills) != 0 || len(siblingSkills) != 1 || siblingSkills[0].Version != "2.0.0" {
		t.Fatalf("uninstall should remove only one Agent binding: first=%#v sibling=%#v", firstAgentSkills, siblingSkills)
	}
	var retainedVersions int
	if err := database.QueryRowContext(t.Context(), `
		SELECT count(*) FROM skill_versions WHERE package_id=$1`, firstSkill.ID,
	).Scan(&retainedVersions); err != nil {
		t.Fatal(err)
	}
	if retainedVersions != 2 {
		t.Fatalf("uninstall removed immutable Skill history: versions=%d", retainedVersions)
	}

	mcpRepository := NewMCPRepository(database)
	server, err := mcpRepository.Create(t.Context(), mcp.Server{
		ID: "server-one", OwnerPrincipalID: principalOne, AgentID: agentOne,
		Name: "private-mcp", Endpoint: "https://example.com/mcp",
	})
	if err != nil {
		t.Fatal(err)
	}
	server, err = mcpRepository.ReplaceTools(t.Context(), principalOne, agentOne, server.ID, []mcp.Tool{{
		Name: "read", Description: "read data", InputSchema: json.RawMessage(`{"type":"object"}`),
		Enabled: true, RiskLevel: mcp.ReadOnly,
	}}, "2025-11-25")
	if err != nil {
		t.Fatal(err)
	}
	if len(server.Tools) != 1 {
		t.Fatalf("server tools = %#v", server.Tools)
	}
	otherTools, err := mcpRepository.RuntimeTools(t.Context(), principalTwo, agentTwo)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherTools) != 0 {
		t.Fatalf("agent two can execute agent one's MCP tool: %#v", otherTools)
	}
	if _, err := mcpRepository.Get(t.Context(), principalTwo, agentTwo, server.ID); !errors.Is(err, mcp.ErrNotFound) {
		t.Fatalf("cross-agent MCP lookup should be hidden, got %v", err)
	}
}

func TestAccountRegistrationBindsPrincipalAgentAndSession(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		TRUNCATE account_sessions, user_accounts, run_events, runs, messages, conversations, agents, human_principals CASCADE`); err != nil {
		t.Fatal(err)
	}
	repository := NewAccountRepository(database)
	registration := account.Registration{
		User: account.User{ID: "principal-1", DisplayName: "Test user"},
		Account: account.Account{
			ID: "account-1", PrincipalID: "principal-1", Email: "test@example.com", PasswordHash: "encoded", Status: "active",
		},
		Agent: agent.Agent{
			ID: "agent-1", OwnerPrincipalID: "principal-1", Name: "Aegis", SystemPrompt: "Test prompt",
		},
	}
	identity, err := repository.CreateAccountWithAgent(t.Context(), registration)
	if err != nil {
		t.Fatal(err)
	}
	if identity.User.ID != registration.User.ID || identity.Agent.OwnerPrincipalID != registration.User.ID {
		t.Fatalf("unexpected identity: %#v", identity)
	}
	tokenHash := []byte("01234567890123456789012345678901")
	expiresAt := time.Now().Add(time.Hour)
	if err := repository.CreateSession(t.Context(), account.Session{
		ID: "session-1", AccountID: registration.Account.ID, TokenHash: tokenHash, ExpiresAt: expiresAt,
	}); err != nil {
		t.Fatal(err)
	}
	actor, err := repository.AuthenticateSession(t.Context(), tokenHash, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if actor.User.ID != registration.User.ID || actor.AccountID != registration.Account.ID {
		t.Fatalf("unexpected actor: %#v", actor)
	}
}

func TestSkillPackageAndBundleMigrationsPreserveInstalledSkill(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(database, ".", 0); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := goose.Up(database, "."); err != nil {
			t.Errorf("restore latest schema: %v", err)
		}
	}()
	if err := goose.UpTo(database, ".", 3); err != nil {
		t.Fatal(err)
	}
	legacyStatements := []string{
		`INSERT INTO human_principals (id, display_name) VALUES ('migration-principal', 'Migration user')`,
		`INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		 VALUES ('migration-agent', 'migration-principal', 'Migration agent', '', 'prompt')`,
		`INSERT INTO skills (
			id, owner_principal_id, name, description, version,
			manifest_content, content_hash, source_type
		 ) VALUES (
			'migration-skill', 'migration-principal', 'migrated-skill', 'Migrated skill', '1.0.0',
			'---\nname: migrated-skill\ndescription: Migrated skill\n---\n', 'sha256:migrated', 'inline'
		 )`,
		`INSERT INTO agent_skills (owner_principal_id, agent_id, skill_id, enabled)
		 VALUES ('migration-principal', 'migration-agent', 'migration-skill', true)`,
	}
	for _, statement := range legacyStatements {
		if _, err := database.ExecContext(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := goose.UpTo(database, ".", 4); err != nil {
		t.Fatal(err)
	}
	var packageID, versionID, version, contentHash string
	var enabled bool
	err = database.QueryRowContext(t.Context(), `
		SELECT packages.id, versions.id, versions.version, versions.content_hash, bindings.enabled
		FROM agent_skills bindings
		JOIN skill_packages packages ON packages.id=bindings.package_id
		JOIN skill_versions versions ON versions.id=bindings.version_id
		WHERE bindings.agent_id='migration-agent'`,
	).Scan(&packageID, &versionID, &version, &contentHash, &enabled)
	if err != nil {
		t.Fatal(err)
	}
	if packageID != "migration-skill" || versionID != "migration-skill" || version != "1.0.0" || contentHash != "sha256:migrated" || !enabled {
		t.Fatalf("legacy Skill was not preserved: package=%q versionID=%q version=%q hash=%q enabled=%v", packageID, versionID, version, contentHash, enabled)
	}
	if err := goose.UpTo(database, ".", 5); err != nil {
		t.Fatal(err)
	}
	var filePath, fileHash string
	var fileContent []byte
	err = database.QueryRowContext(t.Context(), `
		SELECT path, content_hash, content
		FROM skill_version_files
		WHERE version_id='migration-skill'`,
	).Scan(&filePath, &fileHash, &fileContent)
	if err != nil {
		t.Fatal(err)
	}
	if filePath != "SKILL.md" || fileHash != "sha256:migrated" || !strings.Contains(string(fileContent), "migrated-skill") {
		t.Fatalf("legacy Skill manifest was not migrated into bundle files: path=%q hash=%q content=%q", filePath, fileHash, fileContent)
	}
}
