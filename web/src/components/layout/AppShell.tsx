import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import {
  Brain,
  MessageCircle,
  PlugZap,
  ShieldCheck,
  Sparkles,
  Wrench,
  X,
} from "lucide-react";

interface AppShellProps {
  activeArea: "chat" | "memory" | "skills" | "mcp";
  navigationOpen: boolean;
  navigation: ReactNode;
  children: ReactNode;
  onCloseNavigation(): void;
}

const productDestinations = [
  { id: "chat", label: "Chat", icon: MessageCircle, to: "/" },
  { id: "memory", label: "Memory", icon: Brain, to: "/memory" },
  { id: "skills", label: "Skills", icon: Wrench, to: "/skills" },
  { id: "mcp", label: "MCP", icon: PlugZap, to: "/mcp" },
] as const;

export function AppShell({
  activeArea,
  navigationOpen,
  navigation,
  children,
  onCloseNavigation,
}: AppShellProps) {
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
                key={label}
                to={to}
                className={`rail-destination ${activeArea === id ? "active" : ""}`}
                aria-current={activeArea === id ? "page" : undefined}
                aria-label={label}
                title={label}
              >
                <Icon size={19} />
                <span>{label}</span>
              </Link>
            ))}
          </div>

          <div className="rail-agent" title="Personal Agent">
            <Sparkles size={18} />
            <span>Aegis</span>
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
