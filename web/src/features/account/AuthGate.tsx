import type { ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Bot, RefreshCw } from "lucide-react";

import { APIError, api, type AuthResponse } from "../../api/client";
import { LoginPage } from "./LoginPage";
import { AgentSetupPage } from "../agent/AgentSetupPage";
import { InterfacePreferencesProvider } from "../settings/preferences";

interface AuthGateProps {
  children: ReactNode;
}

export function AuthGate({ children }: AuthGateProps) {
  const queryClient = useQueryClient();
  const session = useQuery({
    queryKey: ["session"],
    queryFn: api.session,
    retry: false,
  });
  const bootstrap = useQuery({
    queryKey: ["bootstrap"],
    queryFn: api.bootstrap,
    enabled: session.isSuccess,
    retry: false,
  });

  if (session.isPending) {
    return (
      <main className="auth-status" role="status">
        <span className="auth-mark">
          <Bot size={22} />
        </span>
        <p>Opening your personal agent…</p>
      </main>
    );
  }

  if (session.isError) {
    if (session.error instanceof APIError && session.error.status === 401) {
      return <LoginPage onAuthenticated={sessionSuccess(session.refetch)} />;
    }
    return (
      <main className="auth-status auth-status-error">
        <span className="auth-mark">
          <Bot size={22} />
        </span>
        <h1>AegisLink could not start</h1>
        <p>{session.error.message}</p>
        <button type="button" onClick={() => void session.refetch()}>
          <RefreshCw size={15} />
          Retry
        </button>
      </main>
    );
  }

  if (bootstrap.isPending) {
    return (
      <main className="auth-status" role="status">
        <span className="auth-mark">
          <Bot size={22} />
        </span>
        <p>Loading your Agent configuration…</p>
      </main>
    );
  }

  if (bootstrap.isError) {
    return (
      <main className="auth-status auth-status-error">
        <span className="auth-mark">
          <Bot size={22} />
        </span>
        <h1>Your Agent configuration could not load</h1>
        <p>{bootstrap.error.message}</p>
        <button type="button" onClick={() => void bootstrap.refetch()}>
          <RefreshCw size={15} />
          Retry
        </button>
      </main>
    );
  }

  if (bootstrap.data.setupRequired) {
    return (
      <AgentSetupPage
        agent={bootstrap.data.agent}
        onConfigured={() => {
          void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
        }}
        onSignedOut={() => {
          queryClient.clear();
          void session.refetch();
        }}
      />
    );
  }

  return (
    <InterfacePreferencesProvider>{children}</InterfacePreferencesProvider>
  );
}

function sessionSuccess(refetch: () => Promise<unknown>) {
  return (_result: AuthResponse) => {
    void refetch();
  };
}
