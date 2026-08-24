import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { FileCode2, Plus, Trash2 } from "lucide-react";

import { api, type Skill } from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { CapabilityError, CapabilityShell } from "./CapabilityShell";

const starterSkill = `---
name: concise-planning
description: Use when the user asks for a plan or next steps.
---

# Concise planning

Return a short ordered plan. State the expected outcome and the next executable step.`;

export function SkillsPage() {
  return (
    <CapabilityShell
      area="skills"
      eyebrow="Instructions"
      title="Skills"
      description="Install portable SKILL.md instructions and decide which ones this agent may load."
    >
      {(agentId) => <SkillsManager agentId={agentId} />}
    </CapabilityShell>
  );
}

function SkillsManager({ agentId }: { agentId: string }) {
  const [content, setContent] = useState(starterSkill);
  const [version, setVersion] = useState("");
  const skills = useQuery({
    queryKey: ["skills", agentId],
    queryFn: () => api.listSkills(agentId),
  });
  const install = useMutation({
    mutationFn: () =>
      api.installSkill(agentId, content, version.trim() || undefined),
    onSuccess: () => invalidate(agentId),
  });

  return (
    <div className="capability-stack">
      <form
        className="capability-composer"
        onSubmit={(event) => {
          event.preventDefault();
          install.mutate();
        }}
      >
        <div className="composer-heading">
          <div>
            <h2>Install SKILL.md</h2>
            <p>YAML frontmatter must contain a valid name and description.</p>
          </div>
          <FileCode2 size={20} />
        </div>
        <textarea
          className="code-input"
          value={content}
          onChange={(event) => setContent(event.target.value)}
          rows={10}
          spellCheck={false}
        />
        <label className="skill-version-field">
          <span>Version label</span>
          <input
            value={version}
            onChange={(event) => setVersion(event.target.value)}
            placeholder="Optional · defaults to local-&lt;content hash&gt;"
            maxLength={80}
          />
          <small>
            Reusing a label with different content is rejected. Installing a new
            version only switches this Agent.
          </small>
        </label>
        <div className="composer-actions">
          {install.isError && (
            <span className="inline-error">{install.error.message}</span>
          )}
          <button type="submit" disabled={!content.trim() || install.isPending}>
            <Plus size={16} /> Install / select version
          </button>
        </div>
      </form>

      {skills.isError ? (
        <CapabilityError
          message={skills.error.message}
          onRetry={() => void skills.refetch()}
        />
      ) : skills.isLoading ? (
        <div className="capability-empty">Loading skills…</div>
      ) : skills.data?.length ? (
        <div className="capability-list">
          {skills.data.map((skill) => (
            <SkillCard key={skill.id} agentId={agentId} skill={skill} />
          ))}
        </div>
      ) : (
        <div className="capability-empty">
          <strong>No installed skills</strong>
          <span>
            Install the starter skill above or paste another compliant SKILL.md.
          </span>
        </div>
      )}
    </div>
  );
}

function SkillCard({ agentId, skill }: { agentId: string; skill: Skill }) {
  const enable = useMutation({
    mutationFn: () => api.updateSkill(agentId, skill.id, !skill.enabled),
    onSuccess: () => invalidate(agentId),
  });
  const uninstall = useMutation({
    mutationFn: () => api.uninstallSkill(agentId, skill.id),
    onSuccess: () => invalidate(agentId),
  });

  return (
    <article className="capability-card">
      <div className="card-title-row">
        <div>
          <h3>{skill.name}</h3>
          <p>{skill.description}</p>
        </div>
        <label className="switch-control">
          <input
            type="checkbox"
            checked={skill.enabled}
            disabled={enable.isPending}
            onChange={() => enable.mutate()}
          />
          <span>{skill.enabled ? "Enabled" : "Disabled"}</span>
        </label>
      </div>
      <div className="card-actions">
        <span className="card-source">
          {skill.sourceType} · {skill.version} ·{" "}
          {skill.contentHash.slice(0, 20)}…
        </span>
        <button
          type="button"
          className="danger-button"
          onClick={() => uninstall.mutate()}
        >
          <Trash2 size={15} /> Uninstall
        </button>
      </div>
      {(enable.isError || uninstall.isError) && (
        <span className="inline-error">
          {enable.error?.message ?? uninstall.error?.message}
        </span>
      )}
      <div className="skill-identifiers" aria-label="Skill storage identifiers">
        <span>package {skill.id}</span>
        <span>version {skill.versionId}</span>
      </div>
    </article>
  );
}

function invalidate(agentId: string) {
  return queryClient.invalidateQueries({ queryKey: ["skills", agentId] });
}
