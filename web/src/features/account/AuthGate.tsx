import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Bot, RefreshCw } from "lucide-react";

import { APIError, api, type AuthResponse } from "../../api/client";
import { LoginPage } from "./LoginPage";

interface AuthGateProps {
  children: ReactNode;
}

export function AuthGate({ children }: AuthGateProps) {
  const session = useQuery({
    queryKey: ["session"],
    queryFn: api.session,
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

  return children;
}

function sessionSuccess(refetch: () => Promise<unknown>) {
  return (_result: AuthResponse) => {
    void refetch();
  };
}
