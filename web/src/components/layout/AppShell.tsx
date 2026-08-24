import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import {
  Brain,
  MessageCircle,
  PlugZap,
  ShieldCheck,
  Sparkles,
  Settings as SettingsIcon,
  Wrench,
  X,
} from "lucide-react";

import { useInterfacePreferences } from "../../features/settings/preferences";

interface AppShellProps {
  activeArea: "chat" | "memory" | "skills" | "mcp" | "settings";
  navigationOpen: boolean;
  navigation: ReactNode;
  children: ReactNode;
  onCloseNavigation(): void;
}

const productDestinations = [
  { id: "chat", label: ["Chat", "对话"], icon: MessageCircle, to: "/" },
  { id: "memory", label: ["Memory", "记忆"], icon: Brain, to: "/memory" },
  { id: "skills", label: ["Skills", "技能"], icon: Wrench, to: "/skills" },
  { id: "mcp", label: ["MCP", "MCP"], icon: PlugZap, to: "/mcp" },
] as const;

export function AppShell({
  activeArea,
  navigationOpen,
  navigation,
  children,
  onCloseNavigation,
}: AppShellProps) {
  const { t } = useInterfacePreferences();
  return (
    <main className="app-shell">
      <aside
        className={`navigation-shell ${navigationOpen ? "navigation-shell-open" : ""}`}
        aria-label="AegisLink navigation"
      >
        <nav className="product-rail" aria-label="Product areas">
          <div className="rail-brand" aria-label="AegisLink">
            <ShieldCheck size={20} />
          </div>

          <div className="rail-destinations">
            {productDestinations.map(({ id, label, icon: Icon, to }) => (
              <Link
                key={id}
                to={to}
                className={`rail-destination ${activeArea === id ? "active" : ""}`}
                aria-current={activeArea === id ? "page" : undefined}
                aria-label={t(label[0], label[1])}
                title={t(label[0], label[1])}
              >
                <Icon size={19} />
                <span>{t(label[0], label[1])}</span>
              </Link>
            ))}
          </div>

          <div className="rail-footer">
            <Link
              to="/settings"
              className={`rail-destination ${activeArea === "settings" ? "active" : ""}`}
              aria-current={activeArea === "settings" ? "page" : undefined}
              aria-label={t("Settings", "设置")}
              title={t("Settings", "设置")}
            >
              <SettingsIcon size={19} />
              <span>{t("Settings", "设置")}</span>
            </Link>
            <div
              className="rail-agent"
              title={t("Personal Agent", "个人智能体")}
            >
              <Sparkles size={18} />
              <span>Aegis</span>
            </div>
          </div>
        </nav>

        <section className="navigation-panel">
          <button
            type="button"
            className="icon-button navigation-close"
            onClick={onCloseNavigation}
            aria-label="Close navigation"
            title="Close navigation"
          >
            <X size={18} />
          </button>
          {navigation}
        </section>
      </aside>

      {navigationOpen && (
        <button
          type="button"
          className="navigation-scrim"
          onClick={onCloseNavigation}
          aria-label="Close navigation"
        />
      )}

      <section className="workspace-main">{children}</section>
    </main>
  );
}
