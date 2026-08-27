package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
	"github.com/re35t/AegisLink/internal/conversation"
	"github.com/re35t/AegisLink/internal/impression"
	"github.com/re35t/AegisLink/internal/mcp"
	"github.com/re35t/AegisLink/internal/memory"
	"github.com/re35t/AegisLink/internal/skills"
	"github.com/re35t/AegisLink/migrations"
)

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	if err := validateDestructiveTestDatabaseURL(databaseURL); err != nil {
		t.Fatal(err)
	}
	return databaseURL
}

func validateDestructiveTestDatabaseURL(databaseURL string) error {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	databaseName, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/"))
	if err != nil {
		return fmt.Errorf("decode TEST_DATABASE_URL database name: %w", err)
	}
	if databaseName == "" || strings.Contains(databaseName, "/") || !strings.HasSuffix(databaseName, "_test") {
		return fmt.Errorf("refusing destructive repository tests against database %q: TEST_DATABASE_URL must name a dedicated *_test database", databaseName)
	}
	return nil
}

func TestRepositoryConversationRunLifecycle(t *testing.T) {
	const ownerID = "01K34A00000000000000000000"
	const agentID = "01K34A00000000000000000001"

	databaseURL := testDatabaseURL(t)
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestDatabase(t, database)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, database, `
		TRUNCATE account_sessions, user_accounts, run_events, runs, messages, conversations, agents, human_principals CASCADE`)
	repository := NewConversationRepository(database)
	mustExec(t, database, `
		INSERT INTO human_principals (id, display_name) VALUES (@p1, 'Test user')`, ownerID)
	mustExec(t, database, `
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		VALUES (@p1, @p2, 'Aegis', '', 'Test prompt')`, agentID, ownerID)
	mustExec(t, database, `INSERT INTO agent_profiles (agent_id, owner_principal_id) VALUES (@p1, @p2)`, agentID, ownerID)
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
	impressionRepository := NewImpressionRepository(database)
	job, err := impressionRepository.ClaimJob(t.Context(), time.Now().UTC(), time.Minute)
	if err != nil || job.SourceRunID != run.ID {
		t.Fatalf("curator job = %#v err=%v", job, err)
	}
	curation := impression.Curation{Generation: impression.GenerationInfo{Model: "test-curator", PromptVersion: "test-v1", GeneratedAt: time.Now().UTC()}, Impressions: []impression.ImpressionDraft{{Action: "create", TargetID: "new-one", Scope: impression.ScopeTask, Kind: impression.KindCurrentTask, Summary: "Testing the Profile pipeline", Details: map[string]any{"language": "Go"}, Tags: []string{"test"}, Confidence: .9, Salience: .8, SourceMessageIDs: []string{"message-1"}}}, Facts: []impression.FactDraft{{Subject: impression.FactProject, Namespace: "project", Key: "language", Value: map[string]any{"value": "Go"}, Rationale: "Repeated project context", Confidence: .8, SourceImpressionIDs: []string{"new-one"}}}}
	if err = impressionRepository.ApplyCuration(t.Context(), job, curation); err != nil {
		t.Fatal(err)
	}
	items, err := impressionRepository.List(t.Context(), ownerID, agentID, "active")
	if err != nil || len(items) != 1 || items[0].Details["language"] != "Go" || len(items[0].Evidence) != 1 {
		t.Fatalf("Impressions=%#v err=%v", items, err)
	}
	update := impression.Curation{
		Generation: impression.GenerationInfo{Model: "test-curator", PromptVersion: "test-v1", GeneratedAt: time.Now().UTC()},
		Impressions: []impression.ImpressionDraft{{
			Action: "update", TargetID: items[0].ID, Scope: impression.ScopeProject, Kind: impression.KindRecentDecision,
			Summary: "Curator JSONB update works", Details: map[string]any{"language": "Go", "updated": true},
			Tags: []string{"curator", "jsonb"}, Confidence: .95, Salience: .85,
		}},
	}
	if err = impressionRepository.ApplyCuration(t.Context(), job, update); err != nil {
		t.Fatal(err)
	}
	items, err = impressionRepository.List(t.Context(), ownerID, agentID, "active")
	if err != nil || len(items) != 1 || items[0].Summary != "Curator JSONB update works" || items[0].Details["updated"] != true || len(items[0].Tags) != 2 || items[0].Tags[1] != "jsonb" {
		t.Fatalf("updated Impressions=%#v err=%v", items, err)
	}
	if err = impressionRepository.CompleteJob(t.Context(), job.ID); err != nil {
		t.Fatal(err)
	}
	candidates, err := impressionRepository.ListCandidates(t.Context(), ownerID, agentID, "pending")
	if err != nil || len(candidates) != 1 || len(candidates[0].SourceImpressionIDs) != 1 {
		t.Fatalf("candidates=%#v err=%v", candidates, err)
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
	databaseURL := testDatabaseURL(t)
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestDatabase(t, database)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, database, `
		TRUNCATE account_sessions, user_accounts, run_events, runs, messages, conversations,
		         mcp_tools, mcp_servers, agent_skills, skill_version_files, skill_versions, skill_packages,
		         memories, agents, human_principals CASCADE`)
	mustExec(t, database, `
		INSERT INTO human_principals (id, display_name) VALUES (@p1, 'One'), (@p2, 'Two')`,
		principalOne, principalTwo)
	mustExec(t, database, `
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		VALUES (@p3, @p1, 'Agent one', '', 'prompt'),
		       (@p4, @p1, 'Agent sibling', '', 'prompt'),
		       (@p5, @p2, 'Agent two', '', 'prompt')`,
		principalOne, principalTwo, agentOne, agentSibling, agentTwo)

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
	var retainedVersionRow struct{ Count int }
	mustScan(t, database, &retainedVersionRow, `
		SELECT count(*) AS count FROM skill_versions WHERE package_id=@p1`, firstSkill.ID)
	retainedVersions := retainedVersionRow.Count
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
	databaseURL := testDatabaseURL(t)
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestDatabase(t, database)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, database, `
		TRUNCATE account_sessions, user_accounts, run_events, runs, messages, conversations, agents, human_principals CASCADE`)
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
	databaseURL := testDatabaseURL(t)
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestDatabase(t, database)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, database, `
		TRUNCATE account_sessions, user_accounts, agents, human_principals CASCADE`)
	mustExec(t, database, `
		INSERT INTO human_principals (id, display_name) VALUES
		  ('profile-owner', 'Profile owner'), ('profile-other', 'Other owner');
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		VALUES ('profile-agent', 'profile-owner', 'Aegis', 'Original', 'secret prompt');
		INSERT INTO agent_profiles (agent_id, owner_principal_id)
		VALUES ('profile-agent', 'profile-owner');
		INSERT INTO memories (id, owner_principal_id, agent_id, kind, content, confidence, source_uri)
		VALUES ('profile-memory', 'profile-owner', 'profile-agent', 'semantic', 'Interested in security', 1, 'manual://test');
		INSERT INTO agent_impressions (
		  id, owner_principal_id, agent_id, scope, impression_kind, summary, details_json, tags,
		  confidence, salience, first_observed_at, last_observed_at, decay_half_life_seconds,
		  status, generator_model, generator_run_id, prompt_version, generated_at
		) VALUES (
		  'profile-impression', 'profile-owner', 'profile-agent', 'user', 'recent-interest',
		  'Interested in security research', '{}', '["security"]', 0.9, 0.8, now(), now(), 2592000,
		  'active', 'test-model', '', 'test-v1', now()
		);
		INSERT INTO agent_impression_evidence (owner_principal_id, agent_id, impression_id, evidence_kind, evidence_id, observed_at)
		VALUES ('profile-owner', 'profile-agent', 'profile-impression', 'memory', 'profile-memory', now());
		INSERT INTO agent_confirmed_facts (
		  id, owner_principal_id, agent_id, subject_kind, namespace, fact_key, value_json,
		  confidence, confirmation_method, confirmed_by, confirmed_at
		) VALUES (
		  'profile-fact', 'profile-owner', 'profile-agent', 'agent', 'interest', 'topic',
		  '{"name":"security"}', 0.9, 'owner-confirmed', 'profile-owner', now()
		)`)

	repository := NewAgentProfileRepository(database)
	facts, err := repository.ListConfirmedFacts(t.Context(), "profile-owner", "profile-agent", false)
	if err != nil || len(facts) != 1 || facts[0].Value["name"] != "security" {
		t.Fatalf("unexpected Profile facts: facts=%#v err=%v", facts, err)
	}
	impressions, err := NewImpressionRepository(database).List(t.Context(), "profile-owner", "profile-agent", "active")
	if err != nil || len(impressions) != 1 || len(impressions[0].Evidence) != 1 || impressions[0].Evidence[0].ID != "profile-memory" {
		t.Fatalf("unexpected Impressions: impressions=%#v err=%v", impressions, err)
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
		SubjectType: agent.SubjectConfirmedFact, SubjectID: "profile-fact",
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
	if err != nil || !policies[agent.PolicyKey{SubjectType: agent.SubjectConfirmedFact, SubjectID: "profile-fact"}].Indexable {
		t.Fatalf("unexpected disclosure policies: policies=%#v err=%v", policies, err)
	}
	if err := repository.UpdateDisclosurePolicies(t.Context(), "profile-owner", "profile-agent", 2, []agent.PolicyChange{{
		SubjectType: agent.SubjectConfirmedFact, SubjectID: "profile-fact", Policy: agent.DefaultDisclosurePolicy(),
	}}); !errors.Is(err, agent.ErrProfileConflict) {
		t.Fatalf("stale Profile update should conflict: %v", err)
	}
	mustExec(t, database, `DELETE FROM agents WHERE id='profile-agent'`)
	var profileCount struct{ Count int }
	mustScan(t, database, &profileCount, `SELECT count(*) AS count FROM agent_profiles WHERE agent_id='profile-agent'`)
	if profileCount.Count != 0 {
		t.Fatalf("Agent Profile did not cascade with Agent: count=%d", profileCount.Count)
	}
}

func TestAgentProfileMigrationBackfillsExistingAgents(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestDatabase(t, database)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := gooseDownTo(database, 7); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := gooseUp(database); err != nil {
			t.Errorf("restore latest schema: %v", err)
		}
	}()
	mustExec(t, database, `
		TRUNCATE account_sessions, user_accounts, agents, human_principals CASCADE;
		INSERT INTO human_principals (id, display_name) VALUES ('backfill-owner', 'Backfill owner');
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt)
		VALUES ('backfill-agent', 'backfill-owner', 'Backfill Agent', '', 'prompt')`)
	if err := gooseUpTo(database, 8); err != nil {
		t.Fatal(err)
	}
	var profileVersion struct{ Version int64 }
	mustScan(t, database, &profileVersion, `
		SELECT version FROM agent_profiles WHERE agent_id='backfill-agent' AND owner_principal_id='backfill-owner'`)
	if profileVersion.Version != 1 {
		t.Fatalf("existing Agent was not backfilled: version=%d", profileVersion.Version)
	}
	mustExec(t, database, `INSERT INTO memories (id, owner_principal_id, agent_id, kind, content, confidence, source_uri) VALUES ('legacy-memory','backfill-owner','backfill-agent','semantic','Legacy evidence',0.8,'manual://test')`)
	mustExec(t, database, `INSERT INTO agent_memory_projections (id,owner_principal_id,agent_id,projection_type,summary,confidence,freshness,generated_at,status) VALUES ('legacy-projection','backfill-owner','backfill-agent','current_task','Legacy current task',0.8,1,now(),'accepted')`)
	mustExec(t, database, `INSERT INTO agent_memory_projection_sources (owner_principal_id,agent_id,projection_id,memory_id) VALUES ('backfill-owner','backfill-agent','legacy-projection','legacy-memory')`)
	mustExec(t, database, `INSERT INTO agent_profile_facts (id,owner_principal_id,agent_id,namespace,fact_key,value_json,source,confidence) VALUES ('legacy-fact','backfill-owner','backfill-agent','legacy','topic','{"value":"security"}','memory_projection',0.9)`)
	mustExec(t, database, `INSERT INTO agent_profile_disclosure_policies (owner_principal_id,agent_id,subject_type,subject_id,visibility,channels,indexable,audiences) VALUES ('backfill-owner','backfill-agent','fact','legacy-fact','public',ARRAY['agent-facts'],true,ARRAY[]::text[])`)
	if err := gooseUpTo(database, 9); err != nil {
		t.Fatal(err)
	}
	var migrated struct {
		CandidateCount  int
		ImpressionCount int
		PolicyCount     int
	}
	mustScan(t, database, &migrated, `SELECT (SELECT count(*) FROM agent_fact_candidates WHERE id='legacy-fact' AND subject_kind='user' AND status='pending') AS candidate_count,(SELECT count(*) FROM agent_impressions WHERE id='legacy-projection' AND status='active') AS impression_count,(SELECT count(*) FROM agent_profile_disclosure_policies WHERE subject_id='legacy-fact') AS policy_count`)
	if migrated.CandidateCount != 1 || migrated.ImpressionCount != 1 || migrated.PolicyCount != 0 {
		t.Fatalf("migration downgrade=%#v", migrated)
	}
}

func TestAgentInstructionsRepositoryAndMigration(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestDatabase(t, database)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := gooseDownTo(database, 10); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := gooseUp(database); err != nil {
			t.Errorf("restore latest schema: %v", err)
		}
	}()
	mustExec(t, database, `
		TRUNCATE account_sessions, user_accounts, agents, human_principals CASCADE;
		INSERT INTO human_principals (id, display_name) VALUES ('instructions-owner', 'Instructions owner'), ('instructions-other', 'Other owner');
		INSERT INTO agents (id, owner_principal_id, name, description, system_prompt) VALUES
		  ('instructions-agent', 'instructions-owner', 'Chet', 'Personal Agent', 'You are Aegis, a concise and reliable personal assistant. Answer in the language used by the user.'),
		  ('custom-instructions-agent', 'instructions-owner', 'Custom', '', 'Keep this custom prompt.')`)
	if err := gooseUpTo(database, 11); err != nil {
		t.Fatal(err)
	}

	repository := NewAgentRepository(database)
	instructions, err := repository.GetInstructions(t.Context(), "instructions-owner", "instructions-agent")
	if err != nil || instructions.SystemPrompt != "Be concise and reliable. Answer in the language used by the user." || instructions.Version != 1 {
		t.Fatalf("unexpected migrated instructions: instructions=%#v err=%v", instructions, err)
	}
	custom, err := repository.GetInstructions(t.Context(), "instructions-owner", "custom-instructions-agent")
	if err != nil || custom.SystemPrompt != "Keep this custom prompt." {
		t.Fatalf("custom prompt changed during migration: instructions=%#v err=%v", custom, err)
	}
	if _, err := repository.GetInstructions(t.Context(), "instructions-other", "instructions-agent"); !errors.Is(err, agent.ErrNotFound) {
		t.Fatalf("instructions crossed Principal scope: %v", err)
	}

	updated, err := repository.UpdateInstructions(t.Context(), "instructions-owner", "instructions-agent", agent.InstructionsUpdate{
		ExpectedVersion: 1,
		SystemPrompt:    "Answer directly.",
	})
	if err != nil || updated.SystemPrompt != "Answer directly." || updated.Version != 2 {
		t.Fatalf("unexpected updated instructions: instructions=%#v err=%v", updated, err)
	}
	_, err = repository.UpdateInstructions(t.Context(), "instructions-owner", "instructions-agent", agent.InstructionsUpdate{
		ExpectedVersion: 1,
		SystemPrompt:    "Stale update",
	})
	if !errors.Is(err, agent.ErrInstructionsConflict) {
		t.Fatalf("stale instructions update error = %v", err)
	}
}

func TestSkillPackageAndBundleMigrationsPreserveInstalledSkill(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestDatabase(t, database)
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := gooseDownTo(database, 0); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := gooseUp(database); err != nil {
			t.Errorf("restore latest schema: %v", err)
		}
	}()
	if err := gooseUpTo(database, 3); err != nil {
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
		mustExec(t, database, statement)
	}
	if err := gooseUpTo(database, 4); err != nil {
		t.Fatal(err)
	}
	var packageID, versionID, version, contentHash string
	var enabled bool
	var migratedSkill struct {
		PackageID   string `gorm:"column:package_id"`
		VersionID   string `gorm:"column:version_id"`
		Version     string
		ContentHash string
		Enabled     bool
	}
	err = scanOne(database.connection.WithContext(t.Context()), &migratedSkill, `
		SELECT packages.id AS package_id, versions.id AS version_id, versions.version, versions.content_hash, bindings.enabled
		FROM agent_skills bindings
		JOIN skill_packages packages ON packages.id=bindings.package_id
		JOIN skill_versions versions ON versions.id=bindings.version_id
		WHERE bindings.agent_id='migration-agent'`)
	if err != nil {
		t.Fatal(err)
	}
	packageID, versionID, version, contentHash, enabled = migratedSkill.PackageID, migratedSkill.VersionID, migratedSkill.Version, migratedSkill.ContentHash, migratedSkill.Enabled
	if packageID != "migration-skill" || versionID != "migration-skill" || version != "1.0.0" || contentHash != "sha256:migrated" || !enabled {
		t.Fatalf("legacy Skill was not preserved: package=%q versionID=%q version=%q hash=%q enabled=%v", packageID, versionID, version, contentHash, enabled)
	}
	if err := gooseUpTo(database, 5); err != nil {
		t.Fatal(err)
	}
	var filePath, fileHash string
	var fileContent []byte
	var fileRow struct {
		Path        string
		ContentHash string
		Content     []byte
	}
	err = scanOne(database.connection.WithContext(t.Context()), &fileRow, `
		SELECT path, content_hash, content
		FROM skill_version_files
		WHERE version_id='migration-skill'`)
	if err != nil {
		t.Fatal(err)
	}
	filePath, fileHash, fileContent = fileRow.Path, fileRow.ContentHash, fileRow.Content
	if filePath != "SKILL.md" || fileHash != "sha256:migrated" || !strings.Contains(string(fileContent), "migrated-skill") {
		t.Fatalf("legacy Skill manifest was not migrated into bundle files: path=%q hash=%q content=%q", filePath, fileHash, fileContent)
	}
}

func TestMCPLibraryMigrationPreservesBindingsAndRefreshIdentity(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	database, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestDatabase(t, database)
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := gooseDownTo(database, 0); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := gooseUp(database); err != nil {
			t.Errorf("restore latest schema: %v", err)
		}
	}()
	if err := gooseUpTo(database, 6); err != nil {
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
		mustExec(t, database, statement)
	}
	if err := gooseUpTo(database, 7); err != nil {
		t.Fatal(err)
	}

	var toolID string
	var serverEnabled, toolEnabled bool
	var migratedTool struct {
		ToolID        string `gorm:"column:tool_id"`
		ServerEnabled bool
		ToolEnabled   bool
	}
	err = scanOne(database.connection.WithContext(t.Context()), &migratedTool, `
		SELECT tools.id AS tool_id, server_bindings.enabled AS server_enabled, tool_bindings.enabled AS tool_enabled
		FROM mcp_tools tools
		JOIN agent_mcp_servers server_bindings
		  ON server_bindings.server_id=tools.server_id AND server_bindings.agent_id='mcp-migration-agent'
		JOIN agent_mcp_tools tool_bindings
		  ON tool_bindings.tool_id=tools.id AND tool_bindings.agent_id='mcp-migration-agent'
		WHERE tools.server_id='mcp-migration-server' AND tools.name='read'`)
	toolID, serverEnabled, toolEnabled = migratedTool.ToolID, migratedTool.ServerEnabled, migratedTool.ToolEnabled
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

func mustExec(t *testing.T, database *Database, query string, args ...any) {
	t.Helper()
	result := exec(database.connection.WithContext(t.Context()), query, args...)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
}

func mustScan(t *testing.T, database *Database, destination any, query string, args ...any) {
	t.Helper()
	if err := scanOne(database.connection.WithContext(t.Context()), destination, query, args...); err != nil {
		t.Fatal(err)
	}
}

func closeTestDatabase(t *testing.T, database *Database) {
	t.Helper()
	if err := database.Close(); err != nil {
		t.Errorf("close database pool: %v", err)
	}
}

func gooseDownTo(database *Database, version int64) error {
	sqlDatabase, err := database.connection.DB()
	if err != nil {
		return err
	}
	return goose.DownTo(sqlDatabase, ".", version)
}

func gooseUpTo(database *Database, version int64) error {
	sqlDatabase, err := database.connection.DB()
	if err != nil {
		return err
	}
	return goose.UpTo(sqlDatabase, ".", version)
}

func gooseUp(database *Database) error {
	sqlDatabase, err := database.connection.DB()
	if err != nil {
		return err
	}
	return goose.Up(sqlDatabase, ".")
}
