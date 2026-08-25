import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  Bot,
  Check,
  Languages,
  LockKeyhole,
  LogOut,
  Menu,
  Monitor,
  Moon,
  RefreshCw,
  Sun,
  UserRound,
} from "lucide-react";

import {
  APIError,
  api,
  type AccountSettings,
  type AuthResponse,
} from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { AppShell } from "../../components/layout/AppShell";
import { useInterfacePreferences } from "./preferences";

type LanguagePreference = AccountSettings["preferences"]["language"];
type ThemePreference = AccountSettings["preferences"]["theme"];

export function SettingsPage() {
  const navigate = useNavigate();
  const { t } = useInterfacePreferences();
  const [navigationOpen, setNavigationOpen] = useState(false);
  const settings = useQuery({
    queryKey: ["account-settings"],
    queryFn: api.getAccountSettings,
    retry: false,
  });
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: async () => {
      queryClient.clear();
      await navigate({ to: "/" });
    },
  });

  const sidebar = (
    <aside className="settings-sidebar">
      <div className="settings-account-summary">
        <span className="settings-avatar" aria-hidden="true">
          {settings.data?.user.displayName.slice(0, 1).toUpperCase() ?? "A"}
        </span>
        <div>
          <strong>
            {settings.data?.user.displayName ?? t("Your account", "你的账户")}
          </strong>
          <small>
            {settings.data?.account.email ?? t("Loading…", "加载中…")}
          </small>
        </div>
      </div>
      <nav
        className="settings-section-nav"
        aria-label={t("Settings sections", "设置分区")}
      >
        <a href="#general-settings">
          <Monitor size={16} />
          {t("General", "通用")}
        </a>
        <Link to="/settings/agent-profile" search={{ agentId: undefined }}>
          <Bot size={16} />
          {t("Agent Profile", "Agent Profile")}
        </Link>
        <a href="#account-settings">
          <UserRound size={16} />
          {t("Account", "账户")}
        </a>
        <a href="#security-settings">
          <LockKeyhole size={16} />
          {t("Security", "安全")}
        </a>
      </nav>
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
      <section className="settings-page">
        <header className="settings-header">
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
            <h1>{t("Settings", "设置")}</h1>
            <p>
              {t(
                "Manage interface preferences, account identity, and sign-in security.",
                "管理界面偏好、账户身份与登录安全。",
              )}
            </p>
          </div>
        </header>

        <div className="settings-scroll">
          {settings.isPending ? (
            <div className="settings-loading" role="status">
              <span className="settings-loading-line" />
              <span className="settings-loading-line short" />
              <span>{t("Loading account settings…", "正在加载账户设置…")}</span>
            </div>
          ) : settings.isError ? (
            <div className="settings-error" role="alert">
              <strong>
                {t("Settings could not be loaded", "无法加载设置")}
              </strong>
              <span>{localizedSettingsError(settings.error, t)}</span>
              <button type="button" onClick={() => void settings.refetch()}>
                <RefreshCw size={15} />
                {t("Retry", "重试")}
              </button>
            </div>
          ) : (
            <div className="settings-content">
              <GeneralSettings settings={settings.data} />
              <AccountProfileSettings settings={settings.data} />
              <PasswordSettings />
            </div>
          )}
        </div>
      </section>
    </AppShell>
  );
}

function GeneralSettings({ settings }: { settings: AccountSettings }) {
  const { t } = useInterfacePreferences();
  const [language, setLanguage] = useState<LanguagePreference>(
    settings.preferences.language,
  );
  const [theme, setTheme] = useState<ThemePreference>(
    settings.preferences.theme,
  );
  const update = useMutation({
    mutationFn: () => api.updateAccountSettings({ language, theme }),
    onSuccess: (next) => {
      queryClient.setQueryData(["account-settings"], next);
    },
  });

  useEffect(() => {
    setLanguage(settings.preferences.language);
    setTheme(settings.preferences.theme);
  }, [settings.preferences.language, settings.preferences.theme]);

  const dirty =
    language !== settings.preferences.language ||
    theme !== settings.preferences.theme;
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    update.mutate();
  };

  return (
    <section
      id="general-settings"
      className="settings-section"
      aria-labelledby="general-title"
    >
      <div className="settings-section-heading">
        <div className="settings-section-icon">
          <Monitor size={18} />
        </div>
        <div>
          <h2 id="general-title">{t("General", "通用")}</h2>
          <p>
            {t(
              "Choose how AegisLink looks and speaks to you.",
              "选择 AegisLink 的显示方式和界面语言。",
            )}
          </p>
        </div>
      </div>

      <form className="settings-form" onSubmit={submit}>
        <label className="settings-field">
          <span>
            <strong>{t("Interface language", "界面语言")}</strong>
            <small>
              {t(
                "Applied to navigation and settings immediately.",
                "立即应用到导航和设置界面。",
              )}
            </small>
          </span>
          <span className="settings-select-wrap">
            <Languages size={16} />
            <select
              value={language}
              onChange={(event) =>
                setLanguage(event.target.value as LanguagePreference)
              }
              disabled={update.isPending}
            >
              <option value="system">{t("Follow system", "跟随系统")}</option>
              <option value="en">English</option>
              <option value="zh-CN">简体中文</option>
            </select>
          </span>
        </label>

        <div
          className="settings-field settings-theme-field"
          role="group"
          aria-labelledby="theme-setting-label"
        >
          <span id="theme-setting-label">
            <strong>{t("Appearance", "外观")}</strong>
            <small>
              {t(
                "Use a light, dark, or system-matched color scheme.",
                "使用浅色、深色或跟随系统的配色。",
              )}
            </small>
          </span>
          <div className="theme-options">
            <ThemeOption
              value="system"
              selected={theme === "system"}
              label={t("System", "系统")}
              icon={<Monitor size={17} />}
              disabled={update.isPending}
              onSelect={setTheme}
            />
            <ThemeOption
              value="light"
              selected={theme === "light"}
              label={t("Light", "浅色")}
              icon={<Sun size={17} />}
              disabled={update.isPending}
              onSelect={setTheme}
            />
            <ThemeOption
              value="dark"
              selected={theme === "dark"}
              label={t("Dark", "深色")}
              icon={<Moon size={17} />}
              disabled={update.isPending}
              onSelect={setTheme}
            />
          </div>
        </div>

        <SettingsFeedback
          pending={update.isPending}
          success={update.isSuccess && !dirty}
          error={localizedSettingsError(update.error, t)}
          pendingText={t("Saving preferences…", "正在保存偏好…")}
          successText={t("Interface preferences saved.", "界面偏好已保存。")}
        />
        <div className="settings-actions">
          <button type="submit" disabled={!dirty || update.isPending}>
            {t("Save preferences", "保存偏好")}
          </button>
        </div>
      </form>
    </section>
  );
}

function ThemeOption({
  value,
  selected,
  label,
  icon,
  disabled,
  onSelect,
}: {
  value: ThemePreference;
  selected: boolean;
  label: string;
  icon: ReactNode;
  disabled: boolean;
  onSelect(value: ThemePreference): void;
}) {
  return (
    <label className={`theme-option ${selected ? "selected" : ""}`}>
      <input
        type="radio"
        name="theme"
        value={value}
        checked={selected}
        disabled={disabled}
        onChange={() => onSelect(value)}
      />
      {icon}
      <span>{label}</span>
      {selected && <Check size={15} aria-hidden="true" />}
    </label>
  );
}

function AccountProfileSettings({ settings }: { settings: AccountSettings }) {
  const { t } = useInterfacePreferences();
  const [displayName, setDisplayName] = useState(settings.user.displayName);
  const update = useMutation({
    mutationFn: () => api.updateAccountSettings({ displayName }),
    onSuccess: (next) => {
      queryClient.setQueryData(["account-settings"], next);
      queryClient.setQueryData<AuthResponse>(["session"], (current) =>
        current ? { ...current, user: next.user } : current,
      );
    },
  });

  useEffect(
    () => setDisplayName(settings.user.displayName),
    [settings.user.displayName],
  );
  const normalized = displayName.trim();
  const dirty = normalized !== settings.user.displayName;

  return (
    <section
      id="account-settings"
      className="settings-section"
      aria-labelledby="account-title"
    >
      <div className="settings-section-heading">
        <div className="settings-section-icon">
          <UserRound size={18} />
        </div>
        <div>
          <h2 id="account-title">{t("Account", "账户")}</h2>
          <p>
            {t(
              "Update the human identity that owns your Personal Agent.",
              "更新拥有个人智能体的用户身份。",
            )}
          </p>
        </div>
      </div>
      <form
        className="settings-form"
        onSubmit={(event) => {
          event.preventDefault();
          update.mutate();
        }}
      >
        <label className="settings-input-field">
          <span>{t("Display name", "显示名称")}</span>
          <input
            type="text"
            autoComplete="name"
            minLength={1}
            maxLength={80}
            required
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            disabled={update.isPending}
          />
          <small>
            {t(
              "Used to identify your Human Principal inside AegisLink.",
              "用于在 AegisLink 中标识你的 Human Principal。",
            )}
          </small>
        </label>
        <label className="settings-input-field">
          <span>{t("Email", "邮箱")}</span>
          <input type="email" value={settings.account.email} readOnly />
          <small>
            {t(
              "Email changes require a future verification flow.",
              "修改邮箱需要后续的验证流程，因此当前为只读。",
            )}
          </small>
        </label>
        <SettingsFeedback
          pending={update.isPending}
          success={update.isSuccess && !dirty}
          error={localizedSettingsError(update.error, t)}
          pendingText={t("Saving profile…", "正在保存资料…")}
          successText={t("Account profile saved.", "账户资料已保存。")}
        />
        <div className="settings-actions">
          <span className="account-status">
            <span />
            {t("Account active", "账户正常")}
          </span>
          <button
            type="submit"
            disabled={!dirty || !normalized || update.isPending}
          >
            {t("Save profile", "保存资料")}
          </button>
        </div>
      </form>
    </section>
  );
}

function PasswordSettings() {
  const { t } = useInterfacePreferences();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [validationError, setValidationError] = useState("");
  const change = useMutation({
    mutationFn: () => api.changePassword({ currentPassword, newPassword }),
    onSuccess: () => {
      setCurrentPassword("");
      setNewPassword("");
      setConfirmation("");
    },
  });

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    change.reset();
    if (newPassword !== confirmation) {
      setValidationError(
        t("New passwords do not match.", "两次输入的新密码不一致。"),
      );
      return;
    }
    setValidationError("");
    change.mutate();
  };

  return (
    <section
      id="security-settings"
      className="settings-section"
      aria-labelledby="security-title"
    >
      <div className="settings-section-heading">
        <div className="settings-section-icon">
          <LockKeyhole size={18} />
        </div>
        <div>
          <h2 id="security-title">{t("Security", "安全")}</h2>
          <p>
            {t(
              "Change your password without interrupting this session.",
              "修改密码，同时保持当前会话继续登录。",
            )}
          </p>
        </div>
      </div>
      <form className="settings-form" onSubmit={submit}>
        <div className="password-grid">
          <label className="settings-input-field">
            <span>{t("Current password", "当前密码")}</span>
            <input
              type="password"
              autoComplete="current-password"
              maxLength={128}
              required
              value={currentPassword}
              onChange={(event) => setCurrentPassword(event.target.value)}
              disabled={change.isPending}
            />
          </label>
          <label className="settings-input-field">
            <span>{t("New password", "新密码")}</span>
            <input
              type="password"
              autoComplete="new-password"
              minLength={10}
              maxLength={128}
              required
              value={newPassword}
              onChange={(event) => setNewPassword(event.target.value)}
              disabled={change.isPending}
            />
            <small>
              {t("Use at least 10 characters.", "至少使用 10 个字符。")}
            </small>
          </label>
          <label className="settings-input-field">
            <span>{t("Confirm new password", "确认新密码")}</span>
            <input
              type="password"
              autoComplete="new-password"
              minLength={10}
              maxLength={128}
              required
              value={confirmation}
              onChange={(event) => setConfirmation(event.target.value)}
              disabled={change.isPending}
            />
          </label>
        </div>
        <div className="security-note">
          <LockKeyhole size={15} />
          <span>
            {t(
              "Saving a new password revokes every other active session.",
              "保存新密码后，其他所有活动会话都会被撤销。",
            )}
          </span>
        </div>
        <SettingsFeedback
          pending={change.isPending}
          success={change.isSuccess}
          error={validationError || localizedSettingsError(change.error, t)}
          pendingText={t("Changing password…", "正在修改密码…")}
          successText={t(
            "Password changed. Other sessions were signed out.",
            "密码已修改，其他会话已退出。",
          )}
        />
        <div className="settings-actions">
          <button type="submit" disabled={change.isPending}>
            {t("Change password", "修改密码")}
          </button>
        </div>
      </form>
    </section>
  );
}

function SettingsFeedback({
  pending,
  success,
  error,
  pendingText,
  successText,
}: {
  pending: boolean;
  success: boolean;
  error?: string;
  pendingText: string;
  successText: string;
}) {
  if (error)
    return (
      <div className="settings-feedback error" role="alert">
        {error}
      </div>
    );
  if (pending)
    return (
      <div className="settings-feedback" role="status">
        {pendingText}
      </div>
    );
  if (success)
    return (
      <div className="settings-feedback success" role="status">
        <Check size={15} />
        {successText}
      </div>
    );
  return null;
}

function localizedSettingsError(
  error: Error | null,
  t: (english: string, chinese: string) => string,
) {
  if (!error) return undefined;
  if (error instanceof APIError) {
    switch (error.code) {
      case "current_password_incorrect":
        return t("The current password is incorrect.", "当前密码不正确。");
      case "invalid_new_password":
        return t(
          "Use a different new password of at least 10 characters.",
          "请使用至少 10 个字符且与当前密码不同的新密码。",
        );
      case "invalid_account_settings":
        return t(
          "Display name, language, or theme is invalid.",
          "显示名称、语言或主题设置无效。",
        );
      case "invalid_credentials":
        return t("Your session is no longer valid.", "当前登录会话已失效。");
    }
  }
  return error.message;
}
