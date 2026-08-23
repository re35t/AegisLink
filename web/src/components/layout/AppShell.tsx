import type { ReactNode } from "react";
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
  navigationOpen: boolean;
  navigation: ReactNode;
  children: ReactNode;
  onCloseNavigation(): void;
}

const productDestinations = [
  { label: "Chat", icon: MessageCircle, current: true },
  { label: "Memory", icon: Brain, current: false },
  { label: "Skills", icon: Wrench, current: false },
  { label: "MCP", icon: PlugZap, current: false },
] as const;

export function AppShell({
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
            {productDestinations.map(({ label, icon: Icon, current }) => (
              <button
                key={label}
                type="button"
                className={`rail-destination ${current ? "active" : ""}`}
                aria-current={current ? "page" : undefined}
                aria-label={current ? label : `${label} — coming soon`}
                title={current ? label : `${label} — coming soon`}
                disabled={!current}
              >
                <Icon size={19} />
                <span>{label}</span>
              </button>
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
