import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  Bot,
  ChevronLeft,
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
  type AgentProfile,
  type DisclosurePolicyChange,
} from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { AppShell } from "../../components/layout/AppShell";
import { useInterfacePreferences } from "../settings/preferences";
import { DisclosurePolicyEditor } from "./DisclosurePolicyEditor";

export function AgentProfilePage({ agentId }: { agentId?: string }) {
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
        search: { agentId: selectedAgentId },
        replace: true,
      });
    }
  }, [agentId, navigate, selectedAgentId]);

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
      search: { agentId: nextAgentId },
    });
  };

  const sidebar = (
    <aside className="agent-profile-sidebar">
      <div>
        <span className="eyebrow">Personal Agent</span>
        <h1>{t("Agent Profile", "Agent Profile")}</h1>
        <p>
          {t(
            "Stable identity, effective capabilities, and explicit disclosure boundaries.",
            "稳定身份、有效能力与明确的披露边界。",
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
                "Review what this Agent says about itself and control what may leave its private scope.",
                "审查这个 Agent 如何描述自己，并控制哪些信息可以离开私有范围。",
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
  onReload,
}: {
  agentId: string;
  profile: AgentProfile;
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
      <IdentityEditor
        profile={profile}
        pending={identity.isPending}
        error={profileError(identity.error, t)}
        onSave={(input) => identity.mutate(input)}
      />
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
    </div>
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
