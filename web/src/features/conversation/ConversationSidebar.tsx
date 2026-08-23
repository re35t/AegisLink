import { useMemo, useState } from "react";
import { Bot, LogOut, MessageCircle, Plus, Search } from "lucide-react";

import type { Conversation } from "../../api/client";

interface ConversationSidebarProps {
  conversations?: Conversation[];
  activeConversationId?: string;
  loading: boolean;
  failed: boolean;
  creating: boolean;
  loggingOut: boolean;
  creationError?: string;
  agentName: string;
  modelLabel: string;
  onCreate(): void;
  onLogout(): void;
  onRetry(): void;
  onSelect(conversationId: string): void;
}

export function ConversationSidebar({
  conversations,
  activeConversationId,
  loading,
  failed,
  creating,
  loggingOut,
  creationError,
  agentName,
  modelLabel,
  onCreate,
  onLogout,
  onRetry,
  onSelect,
}: ConversationSidebarProps) {
  const [query, setQuery] = useState("");
  const filteredConversations = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase();
    if (!normalized) return conversations ?? [];
    return (conversations ?? []).filter((conversation) =>
      conversation.title.toLocaleLowerCase().includes(normalized),
    );
  }, [conversations, query]);

  return (
    <div className="conversation-sidebar">
      <header className="conversation-sidebar-header">
        <div>
          <span className="eyebrow">Workspace</span>
          <h1>Conversations</h1>
        </div>
        <button
          type="button"
          className="icon-button create-conversation-icon"
          onClick={onCreate}
          disabled={creating}
          aria-label="Create conversation"
          title="Create conversation"
        >
          <Plus size={18} />
        </button>
      </header>

      <button
        type="button"
        className="new-conversation"
        onClick={onCreate}
        disabled={creating}
      >
        <Plus size={17} />
        <span>{creating ? "Creating…" : "New conversation"}</span>
      </button>

      {creationError && (
        <div className="conversation-create-error" role="alert">
          {creationError}
        </div>
      )}

      <label className="conversation-search">
        <Search size={16} aria-hidden="true" />
        <span className="sr-only">Search conversations</span>
        <input
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search conversations"
        />
      </label>

      <div className="conversation-list-region">
        <div className="conversation-list-heading">
          <span>Recent</span>
          {!loading && !failed && <span>{filteredConversations.length}</span>}
        </div>

        <nav className="conversation-list" aria-label="Conversations">
          {loading && (
            <div className="sidebar-status" role="status">
              Loading conversations…
            </div>
          )}

          {failed && (
            <div className="sidebar-status sidebar-error" role="alert">
              <span>Conversations could not be loaded.</span>
              <button type="button" onClick={onRetry}>
                Retry
              </button>
            </div>
          )}

          {!loading &&
            !failed &&
            filteredConversations.map((conversation) => {
              const active = conversation.id === activeConversationId;
              return (
                <button
                  key={conversation.id}
                  type="button"
                  className={`conversation-link ${active ? "active" : ""}`}
                  aria-current={active ? "page" : undefined}
                  onClick={() => onSelect(conversation.id)}
                >
                  <MessageCircle size={16} />
                  <span>{conversation.title}</span>
                </button>
              );
            })}

          {!loading && !failed && conversations?.length === 0 && (
            <div className="sidebar-status">
              <span>No conversations yet.</span>
              <button type="button" onClick={onCreate}>
                Create conversation
              </button>
            </div>
          )}

          {!loading &&
            !failed &&
            conversations &&
            conversations.length > 0 &&
            filteredConversations.length === 0 && (
              <div className="sidebar-status">
                No conversations match “{query.trim()}”.
              </div>
            )}
        </nav>
      </div>

      <footer className="agent-summary">
        <div className="agent-avatar">
          <Bot size={17} />
        </div>
        <div className="agent-summary-copy">
          <strong>{agentName}</strong>
          <span>{modelLabel}</span>
        </div>
        <button
          type="button"
          className="icon-button agent-logout"
          onClick={onLogout}
          disabled={loggingOut}
          aria-label="Sign out"
          title="Sign out"
        >
          <LogOut size={15} />
        </button>
      </footer>
    </div>
  );
}
