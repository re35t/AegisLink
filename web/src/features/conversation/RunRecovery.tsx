import type { Message } from "../../api/client";

interface RunRecoveryProps {
  messages: Message[];
  streamed: string;
  agentName: string;
}

export function RunRecovery({
  messages,
  streamed,
  agentName,
}: RunRecoveryProps) {
  return (
    <div className="recovery-thread">
      <div className="recovery-banner" role="status">
        Reconnecting to the active run. Persisted events are being replayed.
      </div>
      {messages.map((message) => (
        <article key={message.id} className={`message ${message.role}`}>
          <div className="message-label">
            {message.role === "user" ? "You" : agentName}
          </div>
          <div className="message-content">{message.content}</div>
        </article>
      ))}
      {streamed && (
        <article className="message assistant streaming-message">
          <div className="message-label">{agentName}</div>
          <div className="message-content">
            {streamed}
            <span className="cursor" />
          </div>
        </article>
      )}
    </div>
  );
}
