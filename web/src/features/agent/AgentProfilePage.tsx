import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  Bot,
  ChevronLeft,
  FileText,
  Image,
  LogOut,
  Menu,
  RefreshCw,
  Save,
  ShieldCheck,
} from "lucide-react";

import {
  APIError,
  api,
  type AgentInstructions,
  type AgentProfile,
  type DisclosurePolicyChange,
} from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { AppShell } from "../../components/layout/AppShell";
import { useInterfacePreferences } from "../settings/preferences";
import { DisclosurePolicyEditor } from "./DisclosurePolicyEditor";
import {
  FactsSection,
  ImpressionsSection,
  OverviewSection,
  PublicationSection,
} from "./AgentProfileSections";

export type AgentProfileSection =
  | "configuration"
  | "overview"
  | "impressions"
  | "facts"
  | "publication";

export function AgentProfilePage({
  agentId,
  section = "configuration",
}: {
  agentId?: string;
  section?: AgentProfileSection;
}) {
  const navigate = useNavigate();
  const { t } = useInterfacePreferences();
  const [navigationOpen, setNavigationOpen] = useState(false);
  const bootstrap = useQuery({
    queryKey: ["bootstrap"],
    queryFn: api.bootstrap,
  });
  const agents = useQuery({
    queryKey: ["agents"],
    queryFn: api.listAgents,
  });
  const selectedAgentId = agentId ?? bootstrap.data?.agent.id;

  useEffect(() => {
    if (!agentId && selectedAgentId) {
      void navigate({
        to: "/settings/agent-profile",
        search: { agentId: selectedAgentId, section },
        replace: true,
      });
    }
  }, [agentId, navigate, section, selectedAgentId]);

  const profile = useQuery({
    queryKey: ["agent-profile", selectedAgentId],
    queryFn: () => api.getAgentProfile(selectedAgentId!),
    enabled: Boolean(selectedAgentId),
    retry: false,
  });
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: async () => {
      queryClient.clear();
      await navigate({ to: "/" });
    },
  });

  const selectAgent = (nextAgentId: string) => {
    void navigate({
      to: "/settings/agent-profile",
      search: { agentId: nextAgentId, section },
    });
  };

  const sidebar = (
    <aside className="agent-profile-sidebar">
      <div>
        <span className="eyebrow">Personal Agent</span>
        <h1>{t("Agent Profile", "Agent Profile")}</h1>
        <p>
          {t(
            "Configure identity and private instructions, then review the Agent's self model.",
            "配置身份与私有指令，并审查 Agent 的自我模型。",
          )}
        </p>
      </div>
      <label className="agent-profile-selector">
        <span>{t("Agent", "Agent")}</span>
        <select
          value={selectedAgentId ?? ""}
          disabled={agents.isPending || agents.isError}
          onChange={(event) => selectAgent(event.target.value)}
        >
          {!selectedAgentId && (
            <option value="">{t("Loading…", "加载中…")}</option>
          )}
          {agents.data?.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </select>
        {agents.isError && (
          <small role="alert">
            {t("Agent list unavailable", "Agent 列表不可用")}
          </small>
        )}
      </label>
      <nav
        className="agent-profile-sections"
        aria-label={t("Agent Profile sections", "Agent Profile 分区")}
      >
        {(
          [
            "configuration",
            "overview",
            "impressions",
            "facts",
            "publication",
          ] as const
        ).map((item) => (
          <Link
            key={item}
            to="/settings/agent-profile"
            search={{ agentId: selectedAgentId, section: item }}
            className={section === item ? "active" : ""}
          >
            {item === "configuration"
              ? t("Configuration", "配置")
              : item === "overview"
                ? t("Profile", "Profile")
                : item === "impressions"
                  ? "Impressions"
                  : item === "facts"
                    ? t("Facts", "事实")
                    : t("Publication", "发布")}
            {item === "facts" && profile.data?.pendingFactCount ? (
              <span>{profile.data.pendingFactCount}</span>
            ) : null}
          </Link>
        ))}
      </nav>
      <div className="capability-scope">
        <ShieldCheck size={17} />
        <span>
          <strong>{t("Owner-only view", "仅所有者可见")}</strong>
          <small>
            {selectedAgentId ?? t("Resolving Agent…", "正在解析 Agent…")}
          </small>
        </span>
      </div>
      <Link to="/settings" className="agent-profile-back-link">
        <ChevronLeft size={16} />
        {t("Account settings", "账户设置")}
      </Link>
      <button
        type="button"
        className="capability-logout"
        disabled={logout.isPending}
        onClick={() => logout.mutate()}
      >
        <LogOut size={16} />
        {logout.isPending
          ? t("Signing out…", "正在退出…")
          : t("Sign out", "退出登录")}
      </button>
    </aside>
  );

  return (
    <AppShell
      activeArea="settings"
      navigationOpen={navigationOpen}
      onCloseNavigation={() => setNavigationOpen(false)}
      navigation={sidebar}
    >
      <section className="agent-profile-page">
        <header className="agent-profile-header">
          <button
            type="button"
            className="icon-button chat-menu-button"
            aria-label={t("Open navigation", "打开导航")}
            onClick={() => setNavigationOpen(true)}
          >
            <Menu size={18} />
          </button>
          <div>
            <span className="eyebrow">AegisLink</span>
            <h1>{t("Agent Profile", "Agent Profile")}</h1>
            <p>
              {t(
                "Configure this Agent, review its private self model, and control what may leave its scope.",
                "配置这个 Agent、审查其私有自我模型，并控制哪些信息可以离开其范围。",
              )}
            </p>
          </div>
          <span className="agent-profile-agent-badge">
            <Bot size={16} />
            {profile.data?.identity.name ??
              bootstrap.data?.agent.name ??
              t("Loading", "加载中")}
          </span>
        </header>

        <div className="agent-profile-scroll">
          {!selectedAgentId || profile.isPending ? (
            <div className="agent-profile-loading" role="status">
              <span />
              <span />
              {t("Loading Agent Profile…", "正在加载 Agent Profile…")}
            </div>
          ) : profile.isError ? (
            <div className="settings-error" role="alert">
              <strong>
                {t(
                  "Agent Profile could not be loaded",
                  "无法加载 Agent Profile",
                )}
              </strong>
              <span>{profileError(profile.error, t)}</span>
              <button type="button" onClick={() => void profile.refetch()}>
                <RefreshCw size={15} />
                {t("Retry", "重试")}
              </button>
            </div>
          ) : (
            <AgentProfileContent
              agentId={selectedAgentId}
              profile={profile.data}
              section={section}
              onReload={() => void profile.refetch()}
            />
          )}
        </div>
      </section>
    </AppShell>
  );
}

function AgentProfileContent({
  agentId,
  profile,
  section,
  onReload,
}: {
  agentId: string;
  profile: AgentProfile;
  section: AgentProfileSection;
  onReload(): void;
}) {
  const { t } = useInterfacePreferences();
  const identity = useMutation({
    mutationFn: (input: {
      name: string;
      description: string;
      avatarUrl: string;
    }) =>
      api.updateAgentProfile(agentId, {
        expectedVersion: profile.version,
        ...input,
      }),
    onSuccess: (next) => updateProfileCaches(agentId, next),
  });
  const instructions = useQuery({
    queryKey: ["agent-instructions", agentId],
    queryFn: () => api.getAgentInstructions(agentId),
    enabled: section === "configuration",
    retry: false,
  });
  const instructionUpdate = useMutation({
    mutationFn: (systemPrompt: string) =>
      api.updateAgentInstructions(agentId, {
        expectedVersion: instructions.data!.version,
        systemPrompt,
      }),
    onSuccess: (next) =>
      queryClient.setQueryData(["agent-instructions", agentId], next),
  });
  const disclosure = useMutation({
    mutationFn: (changes: DisclosurePolicyChange[]) =>
      api.updateAgentProfileDisclosurePolicies(agentId, {
        expectedVersion: profile.version,
        changes,
      }),
    onSuccess: (next) => updateProfileCaches(agentId, next),
  });

  return (
    <div className="agent-profile-content">
      {section === "configuration" && (
        <>
          <IdentityEditor
            profile={profile}
            pending={identity.isPending}
            error={profileError(identity.error, t)}
            onSave={(input) => identity.mutate(input)}
          />
          {instructions.isPending ? (
            <InstructionsLoading />
          ) : instructions.isError ? (
            <InstructionsLoadError
              error={instructions.error}
              onRetry={() => void instructions.refetch()}
            />
          ) : (
            <InstructionsEditor
              instructions={instructions.data}
              pending={instructionUpdate.isPending}
              error={instructionsError(instructionUpdate.error, t)}
              conflict={
                instructionUpdate.error instanceof APIError &&
                instructionUpdate.error.code ===
                  "agent_instructions_version_conflict"
              }
              onReload={() => {
                instructionUpdate.reset();
                void instructions.refetch();
              }}
              onSave={(systemPrompt) => instructionUpdate.mutate(systemPrompt)}
            />
          )}
        </>
      )}
      {section === "overview" && <OverviewSection profile={profile} />}
      {section === "impressions" && (
        <ImpressionsSection agentId={agentId} profile={profile} />
      )}
      {section === "facts" && (
        <>
          <FactsSection agentId={agentId} profile={profile} />
          <DisclosurePolicyEditor
            profile={profile}
            pending={disclosure.isPending}
            error={profileError(disclosure.error, t)}
            conflict={
              disclosure.error instanceof APIError &&
              disclosure.error.code === "profile_version_conflict"
            }
            onReload={onReload}
            onSave={(changes) => disclosure.mutate(changes)}
          />
        </>
      )}
      {section === "publication" && <PublicationSection agentId={agentId} />}
    </div>
  );
}

function InstructionsLoading() {
  const { t } = useInterfacePreferences();
  return (
    <section className="agent-profile-section" aria-live="polite">
      <div className="agent-profile-loading" role="status">
        <span />
        <span />
        {t("Loading private instructions…", "正在加载私有指令…")}
      </div>
    </section>
  );
}

function InstructionsLoadError({
  error,
  onRetry,
}: {
  error: unknown;
  onRetry(): void;
}) {
  const { t } = useInterfacePreferences();
  return (
    <section className="agent-profile-section">
      <div className="settings-error" role="alert">
        <strong>{t("Instructions could not be loaded", "无法加载指令")}</strong>
        <span>{instructionsError(error, t)}</span>
        <button type="button" onClick={onRetry}>
          <RefreshCw size={15} />
          {t("Retry", "重试")}
        </button>
      </div>
    </section>
  );
}

export function InstructionsEditor({
  instructions,
  pending,
  error,
  conflict,
  onReload,
  onSave,
}: {
  instructions: AgentInstructions;
  pending: boolean;
  error: string;
  conflict: boolean;
  onReload(): void;
  onSave(systemPrompt: string): void;
}) {
  const { t } = useInterfacePreferences();
  const [systemPrompt, setSystemPrompt] = useState(instructions.systemPrompt);

  useEffect(() => {
    setSystemPrompt(instructions.systemPrompt);
  }, [instructions.agentId, instructions.systemPrompt, instructions.version]);

  const normalized = systemPrompt.trim();
  const dirty = normalized !== instructions.systemPrompt;
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (dirty) onSave(normalized);
  };

  return (
    <section
      className="agent-profile-section"
      aria-labelledby="agent-instructions-title"
    >
      <div className="agent-profile-section-heading">
        <div>
          <span className="eyebrow">Behavior</span>
          <h2 id="agent-instructions-title">
            {t("Custom instructions", "自定义指令")}
          </h2>
          <p>
            {t(
              "Private owner instructions applied to every Run. Identity is composed automatically from the Agent name and description.",
              "应用于每次 Run 的所有者私有指令；Identity 会根据 Agent 名称与描述自动组合。",
            )}
          </p>
        </div>
        <span className="profile-version">v{instructions.version}</span>
      </div>
      <form className="agent-profile-instructions-form" onSubmit={submit}>
        <label htmlFor="agent-system-prompt">
          <span>{t("System prompt", "System Prompt")}</span>
          <span className="profile-prompt-input">
            <FileText size={16} aria-hidden="true" />
            <textarea
              id="agent-system-prompt"
              rows={10}
              maxLength={32768}
              value={systemPrompt}
              disabled={pending}
              aria-describedby="agent-instructions-help"
              onChange={(event) => setSystemPrompt(event.target.value)}
            />
          </span>
        </label>
        <small id="agent-instructions-help">
          {t(
            "Never included in Agent Profile, AgentFacts, Run events, or logs.",
            "不会进入 Agent Profile、AgentFacts、Run Event 或日志。",
          )}{" "}
          {systemPrompt.length.toLocaleString()}/32,768
        </small>
        <div className="profile-form-actions">
          <span className={error ? "inline-error" : ""} aria-live="polite">
            {pending
              ? t("Saving instructions…", "正在保存指令…")
              : error ||
                (!dirty &&
                  t("Instructions are up to date.", "指令已是最新状态。"))}
          </span>
          {conflict && (
            <button
              type="button"
              className="secondary-button"
              onClick={onReload}
            >
              <RefreshCw size={15} />
              {t("Reload", "重新加载")}
            </button>
          )}
          <button type="submit" disabled={pending || !dirty}>
            <Save size={16} />
            {t("Save instructions", "保存指令")}
          </button>
        </div>
      </form>
    </section>
  );
}

function IdentityEditor({
  profile,
  pending,
  error,
  onSave,
}: {
  profile: AgentProfile;
  pending: boolean;
  error: string;
  onSave(input: { name: string; description: string; avatarUrl: string }): void;
}) {
  const { t } = useInterfacePreferences();
  const [name, setName] = useState(profile.identity.name);
  const [description, setDescription] = useState(profile.identity.description);
  const [avatarUrl, setAvatarUrl] = useState(profile.identity.avatarUrl);

  useEffect(() => {
    setName(profile.identity.name);
    setDescription(profile.identity.description);
    setAvatarUrl(profile.identity.avatarUrl);
  }, [profile.identity, profile.version]);

  const normalized = {
    name: name.trim(),
    description: description.trim(),
    avatarUrl: avatarUrl.trim(),
  };
  const dirty =
    normalized.name !== profile.identity.name ||
    normalized.description !== profile.identity.description ||
    normalized.avatarUrl !== profile.identity.avatarUrl;

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (dirty && normalized.name) onSave(normalized);
  };

  return (
    <section
      className="agent-profile-section"
      aria-labelledby="profile-identity-title"
    >
      <div className="agent-profile-section-heading identity-heading">
        <AgentAvatar
          name={profile.identity.name}
          avatarUrl={profile.identity.avatarUrl}
        />
        <div>
          <span className="eyebrow">Identity</span>
          <h2 id="profile-identity-title">
            {t("Stable identity", "稳定身份")}
          </h2>
          <p>
            {t(
              "This identity persists across conversations and Runtime restarts.",
              "该身份会跨 Conversation 与 Runtime 重启持续存在。",
            )}
          </p>
        </div>
      </div>
      <form className="agent-profile-identity-form" onSubmit={submit}>
        <label>
          <span>{t("Agent name", "Agent 名称")}</span>
          <input
            type="text"
            required
            minLength={1}
            maxLength={80}
            value={name}
            disabled={pending}
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        <label>
          <span>{t("Description", "描述")}</span>
          <textarea
            rows={3}
            maxLength={1000}
            value={description}
            disabled={pending}
            onChange={(event) => setDescription(event.target.value)}
          />
        </label>
        <label>
          <span>{t("Avatar URL", "头像 URL")}</span>
          <span className="profile-avatar-input">
            <Image size={16} />
            <input
              type="url"
              inputMode="url"
              maxLength={2048}
              placeholder="https://…"
              value={avatarUrl}
              disabled={pending}
              onChange={(event) => setAvatarUrl(event.target.value)}
            />
          </span>
          <small>
            {t(
              "HTTPS only. Empty uses the Agent initials.",
              "仅支持 HTTPS；留空时显示 Agent 首字母。",
            )}
          </small>
        </label>
        <div className="profile-form-actions">
          <span className={error ? "inline-error" : ""} aria-live="polite">
            {pending
              ? t("Saving identity…", "正在保存身份…")
              : error ||
                (!dirty && t("Identity is up to date.", "身份信息已是最新。"))}
          </span>
          <button
            type="submit"
            disabled={pending || !dirty || !normalized.name}
          >
            <Save size={16} />
            {t("Save identity", "保存身份")}
          </button>
        </div>
      </form>
    </section>
  );
}

function AgentAvatar({ name, avatarUrl }: { name: string; avatarUrl: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [avatarUrl]);
  if (avatarUrl && !failed) {
    return (
      <img
        className="agent-profile-avatar"
        src={avatarUrl}
        alt=""
        referrerPolicy="no-referrer"
        onError={() => setFailed(true)}
      />
    );
  }
  return (
    <span className="agent-profile-avatar fallback" aria-hidden="true">
      {name.trim().slice(0, 1).toUpperCase() || "A"}
    </span>
  );
}

function updateProfileCaches(agentId: string, profile: AgentProfile) {
  queryClient.setQueryData(["agent-profile", agentId], profile);
  void queryClient.invalidateQueries({ queryKey: ["agents"] });
  void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
}

function profileError(
  error: unknown,
  t: (english: string, chinese: string) => string,
) {
  if (!error) return "";
  if (error instanceof APIError) {
    switch (error.code) {
      case "profile_version_conflict":
        return t(
          "This Profile changed elsewhere. Reload it before saving again.",
          "该 Profile 已在其他位置发生变化，请重新加载后再保存。",
        );
      case "invalid_agent_profile":
        return t(
          "Check the identity fields and disclosure combinations.",
          "请检查身份字段与披露策略组合。",
        );
      case "resource_not_found":
        return t(
          "This Agent is unavailable or is not owned by your account.",
          "该 Agent 不存在，或不属于当前账户。",
        );
    }
  }
  return error instanceof Error
    ? error.message
    : t("The request could not be completed.", "无法完成请求。");
}

function instructionsError(
  error: unknown,
  t: (english: string, chinese: string) => string,
) {
  if (!error) return "";
  if (error instanceof APIError) {
    switch (error.code) {
      case "agent_instructions_version_conflict":
        return t(
          "These instructions changed elsewhere. Reload them before saving again.",
          "这些指令已在其他位置发生变化，请重新加载后再保存。",
        );
      case "invalid_agent_instructions":
        return t(
          "Keep the instructions within 32,768 characters.",
          "请将指令控制在 32,768 个字符以内。",
        );
      case "resource_not_found":
        return t(
          "This Agent is unavailable or is not owned by your account.",
          "该 Agent 不存在，或不属于当前账户。",
        );
    }
  }
  return error instanceof Error
    ? error.message
    : t("The request could not be completed.", "无法完成请求。");
}
