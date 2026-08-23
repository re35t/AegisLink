import { useEffect, useMemo, useRef, useState } from "react";
import { HttpAgent, type Message as AgUiMessage } from "@ag-ui/client";
import {
  ActionBarPrimitive,
  AssistantRuntimeProvider,
  AuiIf,
  ComposerPrimitive,
  ExportedMessageRepository,
  MessagePrimitive,
  ThreadPrimitive,
  type ThreadHistoryAdapter,
  type ToolCallMessagePartProps,
  useAuiState,
} from "@assistant-ui/react";
import { fromAgUiMessages, useAgUiRuntime } from "@assistant-ui/react-ag-ui";
import {
  ArrowDown,
  ArrowUp,
  CheckCircle2,
  CircleStop,
  CircleX,
  Copy,
  LoaderCircle,
  PanelRightClose,
  PanelRightOpen,
  ShieldCheck,
  Wrench,
} from "lucide-react";

import type { Message } from "../api/client";
import { MarkdownText } from "./MarkdownText";

interface AssistantThreadProps {
  conversationId: string;
  agentId: string;
  agentName: string;
  agentDescription: string;
  modelLabel: string;
  initialMessages: Message[];
  onRunningChange(running: boolean): void;
  onSettled(): void;
}

type RunActivityStatus =
  | "idle"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled";

interface ToolActivity {
  id: string;
  name: string;
  arguments: string;
  result?: string;
  status: "preparing" | "running" | "succeeded" | "failed";
}

interface RunActivity {
  runId?: string;
  status: RunActivityStatus;
  tools: ToolActivity[];
}

export function AssistantThread({
  conversationId,
  agentId,
  agentName,
  agentDescription,
  modelLabel,
  initialMessages,
  onRunningChange,
  onSettled,
}: AssistantThreadProps) {
  const [runtimeError, setRuntimeError] = useState<string>();
  const [inspectorOpen, setInspectorOpen] = useState(false);
  const [activity, setActivity] = useState<RunActivity>({
    status: "idle",
    tools: [],
  });
  const initialHistory = useRef<AgUiMessage[]>(
    initialMessages.map((message) => ({
      id: message.id,
      role: message.role,
      content: message.content,
    })),
  );
  const agent = useMemo(
    () =>
      new HttpAgent({
        url: "/api/v1/ag-ui",
        agentId,
        threadId: conversationId,
        description: agentName,
      }),
    [agentId, agentName, conversationId],
  );
  const history = useMemo<ThreadHistoryAdapter>(
    () => ({
      load: () =>
        Promise.resolve(
          ExportedMessageRepository.fromArray(
            fromAgUiMessages(initialHistory.current, { showThinking: false }),
          ),
        ),
      append: () => Promise.resolve(),
      update: () => Promise.resolve(),
    }),
    [],
  );

  useEffect(() => {
    const subscription = agent.subscribe({
      onRunStartedEvent: ({ event }) => {
        setRuntimeError(undefined);
        setActivity({ runId: event.runId, status: "running", tools: [] });
      },
      onToolCallStartEvent: ({ event }) => {
        setActivity((current) => ({
          ...current,
          tools: [
            ...current.tools.filter((tool) => tool.id !== event.toolCallId),
            {
              id: event.toolCallId,
              name: event.toolCallName,
              arguments: "",
              status: "preparing",
            },
          ],
        }));
      },
      onToolCallArgsEvent: ({ event }) => {
        setActivity((current) =>
          updateToolActivity(current, event.toolCallId, (tool) => ({
            ...tool,
            arguments: tool.arguments + event.delta,
          })),
        );
      },
      onToolCallEndEvent: ({ event }) => {
        setActivity((current) =>
          updateToolActivity(current, event.toolCallId, (tool) => ({
            ...tool,
            status: "running",
          })),
        );
      },
      onToolCallResultEvent: ({ event }) => {
        const failed = Boolean(
          (event as typeof event & { isError?: boolean }).isError,
        );
        setActivity((current) =>
          updateToolActivity(current, event.toolCallId, (tool) => ({
            ...tool,
            result: event.content,
            status: failed ? "failed" : "succeeded",
          })),
        );
      },
      onRunFinishedEvent: () => {
        setActivity((current) => ({ ...current, status: "succeeded" }));
        void onSettled();
      },
      onRunErrorEvent: ({ event }) => {
        if (isCancellationError(`${event.code ?? ""} ${event.message}`)) {
          setRuntimeError(undefined);
          setActivity((current) => ({ ...current, status: "cancelled" }));
          void onSettled();
          return;
        }
        setRuntimeError(event.message || "Agent run failed.");
        setActivity((current) => ({ ...current, status: "failed" }));
        void onSettled();
      },
      onRunFailed: ({ error }) => {
        if (isCancellationError(error)) {
          setRuntimeError(undefined);
          setActivity((current) => ({ ...current, status: "cancelled" }));
          void onSettled();
          return;
        }
        setRuntimeError(error.message);
        setActivity((current) => ({ ...current, status: "failed" }));
        void onSettled();
      },
    });
    return () => subscription.unsubscribe();
  }, [agent, onSettled]);

  const runtime = useAgUiRuntime({
    agent,
    showThinking: false,
    adapters: { history },
    onError: (error) => {
      if (!isCancellationError(error)) setRuntimeError(error.message);
    },
    onCancel: () => {
      setRuntimeError(undefined);
      setActivity((current) => ({ ...current, status: "cancelled" }));
      void onSettled();
    },
  });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <RunStateBridge onRunningChange={onRunningChange} />
      <div
        className={`assistant-runtime-shell ${inspectorOpen ? "inspector-open" : ""}`}
      >
        <button
          type="button"
          className="run-inspector-toggle"
          onClick={() => setInspectorOpen((open) => !open)}
          aria-expanded={inspectorOpen}
          aria-controls="run-inspector"
          title={inspectorOpen ? "Close run inspector" : "Open run inspector"}
        >
          {inspectorOpen ? (
            <PanelRightClose size={16} />
          ) : (
            <PanelRightOpen size={16} />
          )}
          <span>Activity</span>
          {activity.tools.length > 0 && (
            <strong>{activity.tools.length}</strong>
          )}
        </button>
        <ThreadView
          agentName={agentName}
          agentDescription={agentDescription}
          modelLabel={modelLabel}
          runtimeError={runtimeError}
          clearRuntimeError={() => setRuntimeError(undefined)}
        />
        {inspectorOpen && (
          <RunInspector
            activity={activity}
            onClose={() => setInspectorOpen(false)}
          />
        )}
      </div>
    </AssistantRuntimeProvider>
  );
}

function updateToolActivity(
  activity: RunActivity,
  toolCallId: string,
  update: (tool: ToolActivity) => ToolActivity,
): RunActivity {
  return {
    ...activity,
    tools: activity.tools.map((tool) =>
      tool.id === toolCallId ? update(tool) : tool,
    ),
  };
}

function RunStateBridge({
  onRunningChange,
}: {
  onRunningChange(running: boolean): void;
}) {
  const running = useAuiState((state) => state.thread.isRunning);
  useEffect(() => onRunningChange(running), [onRunningChange, running]);
  return null;
}

function ThreadView({
  agentName,
  agentDescription,
  modelLabel,
  runtimeError,
  clearRuntimeError,
}: {
  agentName: string;
  agentDescription: string;
  modelLabel: string;
  runtimeError?: string;
  clearRuntimeError(): void;
}) {
  return (
    <ThreadPrimitive.Root className="aui-thread">
      <ThreadPrimitive.Viewport className="aui-thread-viewport">
        <AuiIf condition={(state) => state.thread.isEmpty}>
          <Welcome agentName={agentName} agentDescription={agentDescription} />
        </AuiIf>

        <ThreadPrimitive.Messages>
          {({ message }) =>
            message.role === "user" ? (
              <UserMessage />
            ) : (
              <AssistantMessage agentName={agentName} />
            )
          }
        </ThreadPrimitive.Messages>

        {runtimeError && (
          <div className="aui-run-error" role="alert">
            <span>{runtimeError}</span>
            <button type="button" onClick={clearRuntimeError}>
              Dismiss
            </button>
          </div>
        )}

        <ThreadPrimitive.ViewportFooter className="aui-thread-footer">
          <ThreadPrimitive.ScrollToBottom
            className="aui-scroll-bottom"
            aria-label="Scroll to latest message"
          >
            <ArrowDown size={17} />
          </ThreadPrimitive.ScrollToBottom>
          <Composer agentName={agentName} modelLabel={modelLabel} />
        </ThreadPrimitive.ViewportFooter>
      </ThreadPrimitive.Viewport>
    </ThreadPrimitive.Root>
  );
}

function Welcome({
  agentName,
  agentDescription,
}: {
  agentName: string;
  agentDescription: string;
}) {
  return (
    <section className="aui-welcome">
      <div className="empty-icon">
        <ShieldCheck size={28} />
      </div>
      <span className="empty-kicker">Personal Agent</span>
      <h2>How can {agentName} help today?</h2>
      <p>{agentDescription}</p>
      <div className="aui-suggestions">
        <ThreadPrimitive.Suggestion
          prompt="Summarize what this personal agent can do right now."
          send
        >
          Describe current capabilities
        </ThreadPrimitive.Suggestion>
        <ThreadPrimitive.Suggestion
          prompt="Use get_current_time to tell me the current time in Asia/Shanghai."
          send
        >
          Test the ReAct time tool
        </ThreadPrimitive.Suggestion>
      </div>
    </section>
  );
}

function UserMessage() {
  return (
    <MessagePrimitive.Root className="aui-message aui-message-user">
      <div className="aui-message-label">You</div>
      <div className="aui-message-bubble">
        <MessagePrimitive.Parts />
      </div>
    </MessagePrimitive.Root>
  );
}

function AssistantMessage({ agentName }: { agentName: string }) {
  return (
    <MessagePrimitive.Root className="aui-message aui-message-assistant">
      <AuiIf
        condition={(state) =>
          state.thread.isRunning || state.message.parts.length > 0
        }
      >
        <div className="aui-assistant-layout">
          <div className="message-avatar" aria-hidden="true">
            <ShieldCheck size={16} />
          </div>
          <div className="aui-assistant-body">
            <div className="aui-message-label">{agentName}</div>
            <div className="aui-message-content">
              <MessagePrimitive.Parts
                components={{
                  Text: MarkdownText,
                  tools: { Fallback: ToolCallCard },
                }}
              />
              <MessagePrimitive.Error />
            </div>
            <AuiIf condition={(state) => state.message.parts.length > 0}>
              <ActionBarPrimitive.Root
                className="aui-message-actions"
                hideWhenRunning
              >
                <ActionBarPrimitive.Copy
                  className="aui-message-action"
                  aria-label="Copy response"
                >
                  <Copy size={14} />
                  <span>Copy</span>
                </ActionBarPrimitive.Copy>
              </ActionBarPrimitive.Root>
            </AuiIf>
          </div>
        </div>
      </AuiIf>
    </MessagePrimitive.Root>
  );
}

function ToolCallCard({
  toolName,
  argsText,
  result,
  isError,
  status,
}: ToolCallMessagePartProps) {
  const running = status.type === "running" || result === undefined;
  const label = isError ? "Failed" : running ? "Running" : "Completed";
  return (
    <details
      className={`aui-tool-call ${isError ? "failed" : ""}`}
      open={running}
    >
      <summary>
        <span className="aui-tool-icon" aria-hidden="true">
          {isError ? (
            <CircleX size={15} />
          ) : running ? (
            <LoaderCircle className="spin" size={15} />
          ) : (
            <CheckCircle2 size={15} />
          )}
        </span>
        <span className="aui-tool-name">{toolName}</span>
        <span className="aui-tool-status">{label}</span>
      </summary>
      <ToolPayload label="Arguments" value={argsText || "{}"} />
      {result !== undefined && <ToolPayload label="Result" value={result} />}
    </details>
  );
}

function RunInspector({
  activity,
  onClose,
}: {
  activity: RunActivity;
  onClose(): void;
}) {
  return (
    <aside
      className="run-inspector"
      id="run-inspector"
      aria-label="Run and tool inspector"
    >
      <header>
        <div>
          <span className="eyebrow">Execution</span>
          <h2>Run inspector</h2>
        </div>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close run inspector"
        >
          <PanelRightClose size={17} />
        </button>
      </header>
      <section className="run-inspector-summary">
        <div>
          <span>Status</span>
          <strong className={`activity-status ${activity.status}`}>
            {activity.status}
          </strong>
        </div>
        {activity.runId && (
          <div>
            <span>Run ID</span>
            <code title={activity.runId}>{activity.runId}</code>
          </div>
        )}
      </section>
      <section className="run-inspector-tools">
        <div className="run-inspector-section-title">
          <Wrench size={14} />
          <span>Tool calls</span>
          <strong>{activity.tools.length}</strong>
        </div>
        {activity.tools.length === 0 ? (
          <p className="run-inspector-empty">
            Tool calls from the current run will appear here.
          </p>
        ) : (
          activity.tools.map((tool, index) => (
            <article className="run-inspector-tool" key={tool.id}>
              <header>
                <span>{index + 1}</span>
                <strong>{tool.name}</strong>
                <em className={tool.status}>{tool.status}</em>
              </header>
              <ToolPayload label="Arguments" value={tool.arguments || "{}"} />
              {tool.result !== undefined && (
                <ToolPayload label="Result" value={tool.result} />
              )}
            </article>
          ))
        )}
      </section>
    </aside>
  );
}

function ToolPayload({ label, value }: { label: string; value: unknown }) {
  return (
    <div className="tool-payload">
      <span>{label}</span>
      <pre>{formatToolValue(value)}</pre>
    </div>
  );
}

function formatToolValue(value: unknown): string {
  if (typeof value !== "string") return JSON.stringify(value, null, 2);
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

function isCancellationError(value: unknown): boolean {
  const message =
    value instanceof Error
      ? `${value.name} ${value.message}`
      : typeof value === "string"
        ? value
        : "";
  return /abort|cancel/i.test(message);
}

function Composer({
  agentName,
  modelLabel,
}: {
  agentName: string;
  modelLabel: string;
}) {
  return (
    <div className="aui-composer-wrap">
      <ComposerPrimitive.Root className="aui-composer">
        <ComposerPrimitive.Input
          className="aui-composer-input"
          placeholder={`Message ${agentName}`}
          rows={1}
          aria-label="Message"
        />
        <AuiIf condition={(state) => state.thread.isRunning}>
          <ComposerPrimitive.Cancel
            className="aui-composer-button stop"
            aria-label="Stop generation"
          >
            <CircleStop size={19} />
          </ComposerPrimitive.Cancel>
        </AuiIf>
        <AuiIf condition={(state) => !state.thread.isRunning}>
          <ComposerPrimitive.Send
            className="aui-composer-button"
            aria-label="Send message"
          >
            <ArrowUp size={18} />
          </ComposerPrimitive.Send>
        </AuiIf>
      </ComposerPrimitive.Root>
      <p className="aui-composer-note">
        {modelLabel} · AG-UI · Responses may contain mistakes
      </p>
    </div>
  );
}
