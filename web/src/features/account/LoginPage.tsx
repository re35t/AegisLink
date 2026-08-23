import { useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { Bot, LockKeyhole } from "lucide-react";

import { api, type AuthResponse } from "../../api/client";

interface LoginPageProps {
  onAuthenticated(result: AuthResponse): void;
}

export function LoginPage({ onAuthenticated }: LoginPageProps) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const authentication = useMutation({
    mutationFn: () =>
      mode === "login"
        ? api.login(email, password)
        : api.register(displayName, email, password),
    onSuccess: onAuthenticated,
  });

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    authentication.mutate();
  };

  const switchMode = (next: "login" | "register") => {
    if (authentication.isPending) return;
    setMode(next);
    authentication.reset();
  };

  return (
    <main className="auth-page">
      <section className="auth-intro" aria-labelledby="auth-product-title">
        <span className="auth-brand-mark">
          <Bot size={25} />
        </span>
        <p className="auth-eyebrow">Personal Agent OS</p>
        <h1 id="auth-product-title">Your agent, under your control.</h1>
        <p>
          Sign in to continue your conversations, or create an account to bind a
          private Personal Agent to your identity.
        </p>
        <div className="auth-trust-note">
          <LockKeyhole size={16} />
          <span>
            The session token is protected in an HttpOnly cookie and is not
            exposed to app JavaScript.
          </span>
        </div>
      </section>

      <section className="auth-panel" aria-labelledby="auth-form-title">
        <div className="auth-mode" role="tablist" aria-label="Account action">
          <button
            type="button"
            role="tab"
            aria-selected={mode === "login"}
            className={mode === "login" ? "active" : ""}
            onClick={() => switchMode("login")}
          >
            Sign in
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mode === "register"}
            className={mode === "register" ? "active" : ""}
            onClick={() => switchMode("register")}
          >
            Create account
          </button>
        </div>

        <header>
          <span className="eyebrow">AegisLink account</span>
          <h2 id="auth-form-title">
            {mode === "login" ? "Welcome back" : "Create your workspace"}
          </h2>
          <p>
            {mode === "login"
              ? "Use the account that owns your Personal Agent."
              : "We will create one User and bind one Personal Agent."}
          </p>
        </header>

        <form className="auth-form" onSubmit={submit}>
          {mode === "register" && (
            <label>
              <span>Display name</span>
              <input
                type="text"
                name="displayName"
                autoComplete="name"
                maxLength={80}
                required
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
              />
            </label>
          )}
          <label>
            <span>Email</span>
            <input
              type="email"
              name="email"
              autoComplete="email"
              maxLength={254}
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </label>
          <label>
            <span>Password</span>
            <input
              type="password"
              name="password"
              autoComplete={
                mode === "login" ? "current-password" : "new-password"
              }
              minLength={mode === "register" ? 10 : 1}
              maxLength={128}
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
            {mode === "register" && <small>Use at least 10 characters.</small>}
          </label>

          {authentication.isError && (
            <div className="auth-error" role="alert">
              {authentication.error.message}
            </div>
          )}

          <button
            type="submit"
            className="auth-submit"
            disabled={authentication.isPending}
          >
            {authentication.isPending
              ? mode === "login"
                ? "Signing in…"
                : "Creating account…"
              : mode === "login"
                ? "Sign in"
                : "Create account and agent"}
          </button>
        </form>
      </section>
    </main>
  );
}
