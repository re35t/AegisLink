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
	policy := conversation.ExecutionPolicy{
		Mode: "force-tool-once", MentionID: "opaque-mention", ToolID: "tool-one",
		ToolName: "read", QualifiedToolName: "mcp__plugin__read__deadbeef",
	}
	message, run, err := repository.CreateMessageRun(t.Context(), ownerID, created.ID, "message-1", "run-1", "hello", policy)
	if err != nil {
		t.Fatal(err)
	}
	if message.Sequence != 1 || run.Status != "queued" || run.ExecutionPolicy.MentionID != "opaque-mention" {
		t.Fatalf("unexpected initial state: message=%#v run=%#v", message, run)
	}
	_, _, err = repository.CreateMessageRun(t.Context(), ownerID, created.ID, "message-2", "run-2", "duplicate", conversation.ExecutionPolicy{Mode: "auto"})
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
	storedRun, err := repository.GetRun(t.Context(), ownerID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedRun.ExecutionPolicy.Mode != "force-tool-once" || storedRun.ExecutionPolicy.ToolID != "tool-one" ||
		storedRun.ExecutionPolicy.ToolName != "read" || storedRun.ExecutionPolicy.QualifiedToolName != "" {
		t.Fatalf("execution policy snapshot was not restored safely: %#v", storedRun.ExecutionPolicy)
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
		ID: "server-one", OwnerPrincipalID: principalOne,
		Name: "private-mcp", Endpoint: "https://example.com/mcp",
	}, agentOne)
	if err != nil {
		t.Fatal(err)
	}
	server, err = mcpRepository.ReplaceTools(t.Context(), principalOne, server.ID, []mcp.Tool{{
		Name: "read", Description: "read data", InputSchema: json.RawMessage(`{"type":"object"}`),
		RiskLevel: mcp.ReadOnly,
	}}, "2025-11-25")
	if err != nil {
		t.Fatal(err)
	}
	if len(server.Tools) != 1 {
		t.Fatalf("server tools = %#v", server.Tools)
	}
	if _, err := mcpRepository.BindTool(t.Context(), principalOne, agentOne, server.Tools[0].ID); err != nil {
		t.Fatal(err)
	}
	otherTools, err := mcpRepository.RuntimeTools(t.Context(), principalTwo, agentTwo)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherTools) != 0 {
		t.Fatalf("agent two can execute agent one's MCP tool: %#v", otherTools)
	}
	if _, err := mcpRepository.Get(t.Context(), principalTwo, server.ID); !errors.Is(err, mcp.ErrNotFound) {
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
	profileRecord, err := NewAgentProfileRepository(database).GetProfile(t.Context(), registration.User.ID, registration.Agent.ID)
	if err != nil || profileRecord.Version != 1 {
		t.Fatalf("default Agent Profile was not created: profile=%#v err=%v", profileRecord, err)
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
	settings, err := repository.GetSettings(t.Context(), registration.Account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Preferences.Language != account.LanguageSystem || settings.Preferences.Theme != account.ThemeSystem {
		t.Fatalf("unexpected default preferences: %#v", settings.Preferences)
	}
	displayName := "Updated user"
	language := account.LanguageChinese
	theme := account.ThemeDark
	settings, err = repository.UpdateSettings(t.Context(), registration.Account.ID, registration.User.ID, account.SettingsUpdate{
		DisplayName: &displayName, Language: &language, Theme: &theme,
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.User.DisplayName != displayName || settings.Preferences.Language != language || settings.Preferences.Theme != theme {
		t.Fatalf("settings update was not persisted: %#v", settings)
	}
	otherTokenHash := []byte("another-session-token-hash-00001")
	if err := repository.CreateSession(t.Context(), account.Session{
		ID: "session-2", AccountID: registration.Account.ID, TokenHash: otherTokenHash, ExpiresAt: expiresAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.ChangePassword(t.Context(), registration.Account.ID, "session-1", "new-encoded-password"); err != nil {
		t.Fatal(err)
	}
	passwordHash, err := repository.PasswordHash(t.Context(), registration.Account.ID)
	if err != nil || passwordHash != "new-encoded-password" {
		t.Fatalf("password was not updated: hash=%q err=%v", passwordHash, err)
	}
	if _, err := repository.AuthenticateSession(t.Context(), otherTokenHash, time.Now()); !errors.Is(err, account.ErrUnauthenticated) {
		t.Fatalf("other session should be revoked, got %v", err)
	}
	if _, err := repository.AuthenticateSession(t.Context(), tokenHash, time.Now()); err != nil {
		t.Fatalf("current session should remain active: %v", err)
	}
}

func TestAgentProfileRepositoryLifecycleAndIsolation(t *testing.T) {
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
		TRUNCATE account_sessions, user_accounts, agents, human_principals CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		INSERT INTO human_principals (id, display_name) VALUES
		  ('profile-owner', 'Profile owner'), ('profile-other', 'Other owner');
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		VALUES ('profile-agent', 'profile-owner', 'Aegis', 'Original', 'secret prompt');
		INSERT INTO agent_profiles (agent_id, owner_principal_id)
		VALUES ('profile-agent', 'profile-owner');
		INSERT INTO memories (id, owner_principal_id, agent_id, kind, content, confidence, source_uri)
		VALUES ('profile-memory', 'profile-owner', 'profile-agent', 'semantic', 'Interested in security', 1, 'manual://test');
		INSERT INTO agent_profile_facts (
		  id, owner_principal_id, agent_id, namespace, fact_key, value_json, source, confidence
		) VALUES (
		  'profile-fact', 'profile-owner', 'profile-agent', 'interest', 'topic', '{"name":"security"}', 'memory_projection', 0.9
		);
		INSERT INTO agent_memory_projections (
		  id, owner_principal_id, agent_id, projection_type, summary, confidence, freshness, generated_at, status
		) VALUES (
		  'profile-projection', 'profile-owner', 'profile-agent', 'research_interest', 'Interested in security research', 0.9, 1, now(), 'accepted'
		);
		INSERT INTO agent_memory_projection_sources (owner_principal_id, agent_id, projection_id, memory_id)
		VALUES ('profile-owner', 'profile-agent', 'profile-projection', 'profile-memory')`); err != nil {
		t.Fatal(err)
	}

	repository := NewAgentProfileRepository(database)
	facts, err := repository.ListProfileFacts(t.Context(), "profile-owner", "profile-agent")
	if err != nil || len(facts) != 1 || facts[0].Value["name"] != "security" {
		t.Fatalf("unexpected Profile facts: facts=%#v err=%v", facts, err)
	}
	projections, err := repository.ListMemoryProjections(t.Context(), "profile-owner", "profile-agent")
	if err != nil || len(projections) != 1 || len(projections[0].SourceMemoryIDs) != 1 || projections[0].SourceMemoryIDs[0] != "profile-memory" {
		t.Fatalf("unexpected Memory projections: projections=%#v err=%v", projections, err)
	}
	if _, err := repository.GetProfile(t.Context(), "profile-other", "profile-agent"); !errors.Is(err, agent.ErrNotFound) {
		t.Fatalf("Profile must not cross Principal scope: %v", err)
	}
	name := "Updated Aegis"
	avatar := "https://example.test/aegis.png"
	if err := repository.UpdateProfileIdentity(t.Context(), "profile-owner", "profile-agent", agent.ProfileUpdate{
		ExpectedVersion: 1, Name: &name, AvatarURL: &avatar,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateDisclosurePolicies(t.Context(), "profile-owner", "profile-agent", 2, []agent.PolicyChange{{
		SubjectType: agent.SubjectFact, SubjectID: "profile-fact",
		Policy: agent.DisclosurePolicy{
			Visibility: agent.VisibilityPublic, Channels: []agent.DisclosureChannel{agent.ChannelAgentFacts}, Indexable: true, Audiences: []string{},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	record, err := repository.GetProfile(t.Context(), "profile-owner", "profile-agent")
	if err != nil || record.Version != 3 || record.AvatarURL != avatar {
		t.Fatalf("unexpected updated Profile record: record=%#v err=%v", record, err)
	}
	policies, err := repository.ListDisclosurePolicies(t.Context(), "profile-owner", "profile-agent")
	if err != nil || !policies[agent.PolicyKey{SubjectType: agent.SubjectFact, SubjectID: "profile-fact"}].Indexable {
		t.Fatalf("unexpected disclosure policies: policies=%#v err=%v", policies, err)
	}
	if err := repository.UpdateDisclosurePolicies(t.Context(), "profile-owner", "profile-agent", 2, []agent.PolicyChange{{
		SubjectType: agent.SubjectFact, SubjectID: "profile-fact", Policy: agent.DefaultDisclosurePolicy(),
	}}); !errors.Is(err, agent.ErrProfileConflict) {
		t.Fatalf("stale Profile update should conflict: %v", err)
	}
	if _, err := database.ExecContext(t.Context(), `DELETE FROM agents WHERE id='profile-agent'`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.QueryRowContext(t.Context(), `SELECT count(*) FROM agent_profiles WHERE agent_id='profile-agent'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("Agent Profile did not cascade with Agent: count=%d err=%v", count, err)
	}
}

func TestAgentProfileMigrationBackfillsExistingAgents(t *testing.T) {
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
	if err := goose.DownTo(database, ".", 7); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := goose.Up(database, "."); err != nil {
			t.Errorf("restore latest schema: %v", err)
		}
	}()
	if _, err := database.ExecContext(t.Context(), `
		TRUNCATE account_sessions, user_accounts, agents, human_principals CASCADE;
		INSERT INTO human_principals (id, display_name) VALUES ('backfill-owner', 'Backfill owner');
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		VALUES ('backfill-agent', 'backfill-owner', 'Backfill Agent', '', 'prompt')`); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(database, ".", 8); err != nil {
		t.Fatal(err)
	}
	var version int64
	if err := database.QueryRowContext(t.Context(), `
		SELECT version FROM agent_profiles WHERE agent_id='backfill-agent' AND owner_principal_id='backfill-owner'`).Scan(&version); err != nil || version != 1 {
		t.Fatalf("existing Agent was not backfilled: version=%d err=%v", version, err)
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

func TestMCPLibraryMigrationPreservesBindingsAndRefreshIdentity(t *testing.T) {
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
	if err := goose.UpTo(database, ".", 6); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO human_principals (id, display_name) VALUES ('mcp-migration-principal', 'MCP Migration')`,
		`INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		 VALUES ('mcp-migration-agent', 'mcp-migration-principal', 'Agent', '', 'prompt'),
		        ('mcp-migration-sibling', 'mcp-migration-principal', 'Sibling', '', 'prompt')`,
		`INSERT INTO mcp_servers (
			id, owner_principal_id, agent_id, name, endpoint, enabled, status, protocol_version
		 ) VALUES (
			'mcp-migration-server', 'mcp-migration-principal', 'mcp-migration-agent',
			'legacy-plugin', 'https://example.com/mcp', true, 'connected', '2025-11-25'
		 )`,
		`INSERT INTO mcp_tools (server_id, name, description, input_schema, enabled, risk_level)
		 VALUES ('mcp-migration-server', 'read', 'legacy read', '{"type":"object"}', true, 'read-only')`,
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := goose.UpTo(database, ".", 7); err != nil {
		t.Fatal(err)
	}

	var toolID string
	var serverEnabled, toolEnabled bool
	err = database.QueryRowContext(t.Context(), `
		SELECT tools.id, server_bindings.enabled, tool_bindings.enabled
		FROM mcp_tools tools
		JOIN agent_mcp_servers server_bindings
		  ON server_bindings.server_id=tools.server_id AND server_bindings.agent_id='mcp-migration-agent'
		JOIN agent_mcp_tools tool_bindings
		  ON tool_bindings.tool_id=tools.id AND tool_bindings.agent_id='mcp-migration-agent'
		WHERE tools.server_id='mcp-migration-server' AND tools.name='read'`,
	).Scan(&toolID, &serverEnabled, &toolEnabled)
	if err != nil || toolID == "" || !serverEnabled || !toolEnabled {
		t.Fatalf("legacy MCP binding was not preserved: tool=%q server=%v toolEnabled=%v err=%v", toolID, serverEnabled, toolEnabled, err)
	}

	repository := NewMCPRepository(database)
	server, err := repository.ReplaceTools(t.Context(), "mcp-migration-principal", "mcp-migration-server", []mcp.Tool{
		{Name: "read", Description: "refreshed", InputSchema: json.RawMessage(`{"type":"object"}`), RiskLevel: mcp.ReadOnly},
		{Name: "new-tool", Description: "new", InputSchema: json.RawMessage(`{"type":"object"}`), RiskLevel: mcp.ReadOnly},
	}, "2025-11-25")
	if err != nil {
		t.Fatal(err)
	}
	if len(server.Tools) != 2 {
		t.Fatalf("refreshed tools = %#v", server.Tools)
	}
	projected, err := repository.GetForAgent(t.Context(), "mcp-migration-principal", "mcp-migration-agent", server.ID)
	if err != nil {
		t.Fatal(err)
	}
	var refreshed, discovered mcp.Tool
	for _, tool := range projected.Tools {
		switch tool.Name {
		case "read":
			refreshed = tool
		case "new-tool":
			discovered = tool
		}
	}
	if refreshed.ID != toolID || !refreshed.Enabled || discovered.ID == "" || discovered.Enabled {
		t.Fatalf("refresh identity/default enablement is wrong: old=%#v new=%#v", refreshed, discovered)
	}
	if _, err := repository.BindTool(t.Context(), "mcp-migration-principal", "mcp-migration-sibling", toolID); err != nil {
		t.Fatal(err)
	}
	siblingTools, err := repository.RuntimeTools(t.Context(), "mcp-migration-principal", "mcp-migration-sibling")
	if err != nil || len(siblingTools) != 1 {
		t.Fatalf("sibling binding was not independently enabled: tools=%#v err=%v", siblingTools, err)
	}
	if _, err := repository.UpdateToolRisk(t.Context(), "mcp-migration-principal", server.ID, toolID, mcp.ExternalWrite); err != nil {
		t.Fatal(err)
	}
	for _, agentID := range []string{"mcp-migration-agent", "mcp-migration-sibling"} {
		tools, err := repository.RuntimeTools(t.Context(), "mcp-migration-principal", agentID)
		if err != nil || len(tools) != 0 {
			t.Fatalf("risk change did not revoke %s: tools=%#v err=%v", agentID, tools, err)
		}
	}
}
