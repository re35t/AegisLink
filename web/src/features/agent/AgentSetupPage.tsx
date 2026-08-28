import { useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { Bot, LogOut, Radar } from "lucide-react";

import { api, type Agent } from "../../api/client";

interface AgentSetupPageProps {
  agent: Agent;
  onConfigured(): void;
  onSignedOut(): void;
}

export function AgentSetupPage({
  agent,
  onConfigured,
  onSignedOut,
}: AgentSetupPageProps) {
  const [name, setName] = useState(agent.name);
  const [primaryFocus, setPrimaryFocus] = useState("");
  const setup = useMutation({
    mutationFn: () => api.configureAgent(agent.id, { name, primaryFocus }),
    onSuccess: onConfigured,
  });
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: onSignedOut,
  });

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setup.mutate();
  };

  return (
    <main className="auth-page setup-page">
      <section className="auth-intro" aria-labelledby="setup-product-title">
        <span className="auth-brand-mark">
          <Bot size={25} />
        </span>
        <p className="auth-eyebrow">One-minute setup</p>
        <h1 id="setup-product-title">Give your Agent a clear identity.</h1>
        <p>
          This identity becomes the starting point for conversations and helps
          other compatible Agents discover what yours is good at.
        </p>
        <div className="auth-trust-note">
          <Radar size={16} />
          <span>
            Only this public Agent identity is encoded for Index. Private User
            Facts and the original text are not sent to Index.
          </span>
        </div>
      </section>

      <section className="auth-panel" aria-labelledby="setup-form-title">
        <header>
          <span className="eyebrow">Basic configuration</span>
          <h2 id="setup-form-title">Configure your Agent</h2>
          <p>You can refine its Profile and disclosure settings later.</p>
        </header>

        <form className="auth-form" onSubmit={submit}>
          <label htmlFor="setup-agent-name">
            <span>Agent name</span>
            <input
              id="setup-agent-name"
              name="agentName"
              autoComplete="off"
              maxLength={80}
              required
              autoFocus
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </label>
          <label htmlFor="setup-primary-focus">
            <span>What is this Agent mainly good at?</span>
            <textarea
              id="setup-primary-focus"
              aria-label="What is this Agent mainly good at?"
              name="primaryFocus"
              maxLength={1000}
              rows={5}
              required
              placeholder="For example: Go backend architecture, API design, and reliable system maintenance."
              value={primaryFocus}
              onChange={(event) => setPrimaryFocus(event.target.value)}
            />
            <small>
              Write a concise public description. The configured encoder turns
              it into a vector before Index publication.
            </small>
          </label>

          {setup.isError && (
            <div className="auth-error" role="alert">
              {setup.error.message} Your local draft is preserved; retry when
              Index and the encoder are available.
            </div>
          )}

          <button
            type="submit"
            className="auth-submit"
            disabled={setup.isPending || logout.isPending}
          >
            {setup.isPending
              ? "Saving and registering…"
              : "Save profile and register Agent"}
          </button>
          <button
            type="button"
            className="setup-sign-out"
            disabled={setup.isPending || logout.isPending}
            onClick={() => logout.mutate()}
          >
            <LogOut size={14} />
            {logout.isPending ? "Signing out…" : "Sign out"}
          </button>
        </form>
      </section>
    </main>
  );
}
