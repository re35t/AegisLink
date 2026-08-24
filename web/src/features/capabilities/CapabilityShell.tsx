import { useState, type ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Bot, LogOut, Menu, ShieldCheck } from "lucide-react";

import { api } from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { AppShell } from "../../components/layout/AppShell";

export type CapabilityArea = "memory" | "skills" | "mcp";

interface CapabilityShellProps {
  area: CapabilityArea;
  eyebrow: string;
  title: string;
  description: string;
  children(agentId: string): ReactNode;
}

export function CapabilityShell({
  area,
  eyebrow,
  title,
  description,
  children,
}: CapabilityShellProps) {
  const navigate = useNavigate();
  const [navigationOpen, setNavigationOpen] = useState(false);
  const bootstrap = useQuery({
    queryKey: ["bootstrap"],
    queryFn: api.bootstrap,
  });
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: async () => {
      queryClient.clear();
      await navigate({ to: "/" });
    },
  });

  const agent = bootstrap.data?.agent;
  const sidebar = (
    <div className="capability-sidebar">
      <div>
        <span className="eyebrow">Personal agent</span>
        <h1>{agent?.name ?? "Aegis"}</h1>
        <p>
          These resources belong to this agent. A second agent gets separate
          memory, skill bindings, and MCP connections.
        </p>
      </div>
      <div className="capability-scope">
        <ShieldCheck size={17} />
        <span>
          <strong>Isolated scope</strong>
          <small>{agent?.id ?? "Loading agent…"}</small>
        </span>
      </div>
      <button
        type="button"
        className="capability-logout"
        disabled={logout.isPending}
        onClick={() => logout.mutate()}
      >
        <LogOut size={16} />
        Sign out
      </button>
    </div>
  );

  return (
    <AppShell
      activeArea={area}
      navigationOpen={navigationOpen}
      onCloseNavigation={() => setNavigationOpen(false)}
      navigation={sidebar}
    >
      <section className="capability-page">
        <header className="capability-header">
          <button
            type="button"
            className="icon-button chat-menu-button"
            aria-label="Open navigation"
            onClick={() => setNavigationOpen(true)}
          >
            <Menu size={18} />
          </button>
          <div>
            <span className="eyebrow">{eyebrow}</span>
            <h1>{title}</h1>
            <p>{description}</p>
          </div>
          <div className="capability-agent-badge">
            <Bot size={16} />
            {agent?.name ?? "Loading"}
          </div>
        </header>

        <div className="capability-scroll">
          {bootstrap.isError ? (
            <CapabilityError
              message={bootstrap.error.message}
              onRetry={() => void bootstrap.refetch()}
            />
          ) : agent ? (
            children(agent.id)
          ) : (
            <div className="capability-empty">Loading agent scope…</div>
          )}
        </div>
      </section>
    </AppShell>
  );
}

export function CapabilityError({
  message,
  onRetry,
}: {
  message: string;
  onRetry(): void;
}) {
  return (
    <div className="capability-empty capability-error" role="alert">
      <strong>Could not load this resource</strong>
      <span>{message}</span>
      <button type="button" onClick={onRetry}>
        Retry
      </button>
    </div>
  );
}
