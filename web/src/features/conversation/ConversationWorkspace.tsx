import { Menu, Plus, Wifi, WifiOff } from "lucide-react";

import type { ConversationDetail } from "../../api/client";
import { AssistantThread } from "../../components/AssistantThread";

interface ConversationWorkspaceProps {
  conversationId?: string;
  detail?: ConversationDetail;
  loading: boolean;
  failed: boolean;
  streamState: "Ready" | "Streaming" | "Recovering" | "Reconnecting";
  agentId: string;
  agentName: string;
  agentDescription: string;
  modelLabel: string;
  creating: boolean;
  recovery?: React.ReactNode;
  onCreate(): void;
  onOpenNavigation(): void;
  onRetry(): void;
  onRunningChange(running: boolean): void;
  onSettled(): void;
}

export function ConversationWorkspace({
  conversationId,
  detail,
  loading,
  failed,
  streamState,
  agentId,
  agentName,
  agentDescription,
  modelLabel,
  creating,
  recovery,
  onCreate,
  onOpenNavigation,
  onRetry,
  onRunningChange,
  onSettled,
}: ConversationWorkspaceProps) {
  const title = detail?.conversation.title ?? agentName;

  return (
    <section className="conversation-workspace">
      <header className="chat-header">
        <button
          type="button"
          className="icon-button chat-menu-button"
          onClick={onOpenNavigation}
          title="Open navigation"
          aria-label="Open navigation"
        >
          <Menu size={19} />
        </button>
        <div className="chat-heading">
          <span className="eyebrow">
            {conversationId
              ? `Conversation with ${agentName}`
              : "Personal Agent"}
          </span>
          <h1>{title}</h1>
        </div>
        <RunStatus state={streamState} />
      </header>

      <div className="chat-content">
        {!conversationId && (
          <div className="empty-state">
            <span className="empty-kicker">Your private Agent workspace</span>
            <h2>Start with a conversation</h2>
            <p>
              Ask {agentName} to think, use available tools, and keep a durable
              conversation history.
            </p>
            <button type="button" onClick={onCreate} disabled={creating}>
              <Plus size={17} />
              {creating ? "Creating…" : "Create conversation"}
            </button>
          </div>
        )}

        {conversationId && loading && (
          <div className="conversation-loading" role="status">
            <span />
            <span />
            <span />
            <p>Loading conversation…</p>
          </div>
        )}

        {conversationId && failed && (
          <div className="conversation-error" role="alert">
            <h2>Conversation could not be loaded</h2>
            <p>Your navigation and other conversations are still available.</p>
            <button type="button" onClick={onRetry}>
              Retry conversation
            </button>
          </div>
        )}

        {conversationId && detail && recovery}

        {conversationId && detail && !recovery && (
          <AssistantThread
            key={conversationId}
            conversationId={conversationId}
            agentId={agentId}
            agentName={agentName}
            agentDescription={agentDescription}
            modelLabel={modelLabel}
            initialMessages={detail.messages}
            onRunningChange={onRunningChange}
            onSettled={onSettled}
          />
        )}
      </div>
    </section>
  );
}

function RunStatus({
  state,
}: {
  state: "Ready" | "Streaming" | "Recovering" | "Reconnecting";
}) {
  const offline = state === "Reconnecting";
  const active = state === "Streaming" || state === "Recovering";
  return (
    <div
      className={`run-status ${offline ? "offline" : ""} ${active ? "active" : ""}`}
      title={state}
      aria-label={state}
    >
      {offline ? (
        <WifiOff size={14} />
      ) : active ? (
        <Wifi size={14} />
      ) : (
        <span className="ready-dot" />
      )}
      <span>{state}</span>
    </div>
  );
}
