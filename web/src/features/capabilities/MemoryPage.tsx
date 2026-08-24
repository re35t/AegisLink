import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Check, Plus, RotateCcw, Trash2 } from "lucide-react";

import { api, type Memory } from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { CapabilityError, CapabilityShell } from "./CapabilityShell";

export function MemoryPage() {
  return (
    <CapabilityShell
      area="memory"
      eyebrow="Context"
      title="Memory"
      description="Curate durable facts and past events that this agent may use in future runs."
    >
      {(agentId) => <MemoryManager agentId={agentId} />}
    </CapabilityShell>
  );
}

function MemoryManager({ agentId }: { agentId: string }) {
  const [content, setContent] = useState("");
  const [kind, setKind] = useState<Memory["kind"]>("semantic");
  const memories = useQuery({
    queryKey: ["memories", agentId],
    queryFn: () => api.listMemories(agentId),
  });
  const create = useMutation({
    mutationFn: () =>
      api.createMemory(agentId, {
        kind,
        content: content.trim(),
        sourceUri: "manual://user",
        confidence: 1,
      }),
    onSuccess: async () => {
      setContent("");
      await invalidate(agentId);
    },
  });

  return (
    <div className="capability-stack">
      <form
        className="capability-composer"
        onSubmit={(event) => {
          event.preventDefault();
          if (content.trim()) create.mutate();
        }}
      >
        <div className="composer-heading">
          <div>
            <h2>Add a memory</h2>
            <p>Only this agent receives it in model context.</p>
          </div>
          <select
            value={kind}
            onChange={(event) => setKind(event.target.value as Memory["kind"])}
          >
            <option value="semantic">Fact</option>
            <option value="episodic">Event</option>
          </select>
        </div>
        <textarea
          value={content}
          onChange={(event) => setContent(event.target.value)}
          placeholder="Example: I prefer concise answers with concrete next steps."
          rows={3}
          maxLength={4000}
        />
        <div className="composer-actions">
          {create.isError && (
            <span className="inline-error">{create.error.message}</span>
          )}
          <button type="submit" disabled={!content.trim() || create.isPending}>
            <Plus size={16} />
            Save memory
          </button>
        </div>
      </form>

      {memories.isError ? (
        <CapabilityError
          message={memories.error.message}
          onRetry={() => void memories.refetch()}
        />
      ) : memories.isLoading ? (
        <div className="capability-empty">Loading memories…</div>
      ) : memories.data?.length ? (
        <div className="capability-list">
          {memories.data.map((memory) => (
            <MemoryCard key={memory.id} agentId={agentId} memory={memory} />
          ))}
        </div>
      ) : (
        <div className="capability-empty">
          <strong>No memory yet</strong>
          <span>Add one explicit, inspectable memory to start.</span>
        </div>
      )}
    </div>
  );
}

function MemoryCard({ agentId, memory }: { agentId: string; memory: Memory }) {
  const [content, setContent] = useState(memory.content);
  const update = useMutation({
    mutationFn: (confirmed: boolean) =>
      api.updateMemory(agentId, memory.id, {
        content: content.trim(),
        confirmed,
      }),
    onSuccess: () => invalidate(agentId),
  });
  const forget = useMutation({
    mutationFn: () => api.forgetMemory(agentId, memory.id),
    onSuccess: () => invalidate(agentId),
  });

  return (
    <article className="capability-card memory-card">
      <div className="card-meta">
        <span className={`kind-badge ${memory.kind}`}>
          {memory.kind === "semantic" ? "Fact" : "Event"}
        </span>
        <span>confidence {Math.round(memory.confidence * 100)}%</span>
        {memory.lastConfirmedAt && <span>confirmed</span>}
      </div>
      <textarea
        value={content}
        onChange={(event) => setContent(event.target.value)}
        rows={3}
      />
      <div className="card-actions">
        <span className="card-source">{memory.sourceUri}</span>
        <button
          type="button"
          className="secondary-button"
          disabled={
            !content.trim() || update.isPending || content === memory.content
          }
          onClick={() => update.mutate(false)}
        >
          <Check size={15} /> Save
        </button>
        <button
          type="button"
          className="secondary-button"
          onClick={() => update.mutate(true)}
        >
          <RotateCcw size={15} /> Confirm
        </button>
        <button
          type="button"
          className="danger-button"
          onClick={() => forget.mutate()}
        >
          <Trash2 size={15} /> Forget
        </button>
      </div>
      {(update.isError || forget.isError) && (
        <span className="inline-error">
          {update.error?.message ?? forget.error?.message}
        </span>
      )}
    </article>
  );
}

function invalidate(agentId: string) {
  return queryClient.invalidateQueries({ queryKey: ["memories", agentId] });
}
