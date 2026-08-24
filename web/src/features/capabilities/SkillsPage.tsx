import { useRef, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  CheckCircle2,
  FileArchive,
  FileCode2,
  FileUp,
  Pencil,
  Plus,
  RotateCcw,
  Trash2,
} from "lucide-react";

import { api, type Skill } from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { CapabilityError, CapabilityShell } from "./CapabilityShell";

const maximumImportBytes = 4 * 1024 * 1024;
const starterSkill = `---
name: concise-planning
description: Use when the user asks for a plan or next steps.
---

# Concise planning

Return a short ordered plan. State the expected outcome and the next executable step.`;

type ComposerMode = "custom" | "import";

export function SkillsPage() {
  return (
    <CapabilityShell
      area="skills"
      eyebrow="Instructions"
      title="Skills"
      description="Create private instructions or import a portable Skill bundle for this agent."
    >
      {(agentId) => <SkillsManager agentId={agentId} />}
    </CapabilityShell>
  );
}

function SkillsManager({ agentId }: { agentId: string }) {
  const editorRef = useRef<HTMLTextAreaElement>(null);
  const [mode, setMode] = useState<ComposerMode>("custom");
  const [content, setContent] = useState(starterSkill);
  const [version, setVersion] = useState("");
  const [customizing, setCustomizing] = useState<string>();
  const [bundle, setBundle] = useState<File>();
  const [importVersion, setImportVersion] = useState("");
  const [importInputKey, setImportInputKey] = useState(0);
  const [localImportError, setLocalImportError] = useState<string>();
  const [feedback, setFeedback] = useState<string>();
  const skills = useQuery({
    queryKey: ["skills", agentId],
    queryFn: () => api.listSkills(agentId),
  });
  const install = useMutation({
    mutationFn: () =>
      api.installSkill(agentId, content, version.trim() || undefined),
    onSuccess: async (skill) => {
      setFeedback(`${skill.name} ${skill.version} is enabled for this agent.`);
      setCustomizing(undefined);
      await invalidate(agentId);
    },
  });
  const importBundle = useMutation({
    mutationFn: (file: File) =>
      api.importSkill(agentId, file, importVersion.trim() || undefined),
    onSuccess: async (skill) => {
      setFeedback(
        `${skill.name} ${skill.version} imported with ${skill.files.length} stored file${skill.files.length === 1 ? "" : "s"}.`,
      );
      setBundle(undefined);
      setImportVersion("");
      setImportInputKey((key) => key + 1);
      await invalidate(agentId);
    },
  });

  const resetEditor = () => {
    setContent(starterSkill);
    setVersion("");
    setCustomizing(undefined);
    setFeedback(undefined);
    install.reset();
    editorRef.current?.focus();
  };

  const customizeSkill = (skill: Skill) => {
    setMode("custom");
    setContent(skill.content);
    setVersion("");
    setCustomizing(skill.name);
    setFeedback(undefined);
    install.reset();
    window.requestAnimationFrame(() => {
      editorRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
      editorRef.current?.focus();
    });
  };

  const selectBundle = (file?: File) => {
    setLocalImportError(undefined);
    importBundle.reset();
    if (!file) {
      setBundle(undefined);
      return;
    }
    const lowerName = file.name.toLowerCase();
    if (!lowerName.endsWith(".md") && !lowerName.endsWith(".zip")) {
      setBundle(undefined);
      setLocalImportError("Choose a .md SKILL file or a .zip Skill bundle.");
      return;
    }
    if (file.size > maximumImportBytes) {
      setBundle(undefined);
      setLocalImportError(
        "The selected file is larger than the 4 MB import limit.",
      );
      return;
    }
    setBundle(file);
  };

  return (
    <div className="capability-stack">
      <section
        className="skill-workbench"
        aria-labelledby="skill-workbench-title"
      >
        <div className="composer-heading">
          <div>
            <h2 id="skill-workbench-title">Add a skill</h2>
            <p>Create a private SKILL.md or import a reusable local bundle.</p>
          </div>
          <FileCode2 size={20} />
        </div>

        <div className="skill-mode-tabs" role="group" aria-label="Skill source">
          <button
            type="button"
            aria-pressed={mode === "custom"}
            className={mode === "custom" ? "active" : ""}
            onClick={() => setMode("custom")}
          >
            <Pencil size={15} /> Create or customize
          </button>
          <button
            type="button"
            aria-pressed={mode === "import"}
            className={mode === "import" ? "active" : ""}
            onClick={() => setMode("import")}
          >
            <FileUp size={15} /> Import file
          </button>
        </div>

        {mode === "custom" ? (
          <form
            className="skill-composer-panel"
            aria-label="Custom Skill editor"
            onSubmit={(event) => {
              event.preventDefault();
              setFeedback(undefined);
              install.mutate();
            }}
          >
            <div className="skill-panel-heading">
              <div>
                <strong>
                  {customizing
                    ? `New version of ${customizing}`
                    : "Custom SKILL.md"}
                </strong>
                <span>
                  Name and description belong in YAML frontmatter; instructions
                  follow in Markdown.
                </span>
              </div>
              <button
                type="button"
                className="ghost-button"
                onClick={resetEditor}
              >
                <RotateCcw size={14} /> Reset
              </button>
            </div>
            <label className="field-label" htmlFor="skill-content">
              Instructions
            </label>
            <textarea
              ref={editorRef}
              id="skill-content"
              className="code-input"
              value={content}
              onChange={(event) => setContent(event.target.value)}
              rows={12}
              spellCheck={false}
            />
            <VersionField value={version} onChange={setVersion} />
            <div className="composer-actions">
              {install.isError && (
                <span className="inline-error" role="alert">
                  {install.error.message}
                </span>
              )}
              <button
                type="submit"
                disabled={!content.trim() || install.isPending}
              >
                <Plus size={16} />
                {install.isPending
                  ? "Saving…"
                  : customizing
                    ? "Save new version"
                    : "Create and enable"}
              </button>
            </div>
          </form>
        ) : (
          <form
            className="skill-composer-panel"
            aria-label="Import Skill bundle"
            onSubmit={(event) => {
              event.preventDefault();
              setFeedback(undefined);
              if (bundle) importBundle.mutate(bundle);
            }}
          >
            <div className="skill-import-guidance">
              <FileArchive size={20} />
              <div>
                <strong>SKILL.md or ZIP bundle</strong>
                <span>
                  ZIP bundles may contain SKILL.md, references/, assets/, and
                  scripts/. Scripts are stored as read-only files and never run
                  automatically.
                </span>
              </div>
            </div>
            <label className="skill-file-field">
              <span>Local file</span>
              <input
                key={importInputKey}
                type="file"
                accept=".md,.zip,text/markdown,application/zip"
                onChange={(event) => selectBundle(event.target.files?.[0])}
              />
              <small>
                One file, up to 4 MB. Expanded bundles are validated
                server-side.
              </small>
            </label>
            {bundle && (
              <div className="selected-skill-file" role="status">
                <FileArchive size={16} />
                <span>
                  <strong>{bundle.name}</strong>
                  <small>{formatBytes(bundle.size)}</small>
                </span>
              </div>
            )}
            <VersionField value={importVersion} onChange={setImportVersion} />
            <div className="composer-actions">
              {(localImportError || importBundle.isError) && (
                <span className="inline-error" role="alert">
                  {localImportError ?? importBundle.error?.message}
                </span>
              )}
              <button
                type="submit"
                disabled={!bundle || importBundle.isPending}
              >
                <FileUp size={16} />
                {importBundle.isPending ? "Importing…" : "Import and enable"}
              </button>
            </div>
          </form>
        )}
      </section>

      {feedback && (
        <div className="skill-feedback" role="status">
          <CheckCircle2 size={16} /> {feedback}
        </div>
      )}

      <div className="skill-list-heading">
        <div>
          <h2>Installed skills</h2>
          <p>Each card is the version currently selected for this agent.</p>
        </div>
        {skills.data && <span>{skills.data.length}</span>}
      </div>

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
            <SkillCard
              key={skill.id}
              agentId={agentId}
              skill={skill}
              onCustomize={() => customizeSkill(skill)}
            />
          ))}
        </div>
      ) : (
        <div className="capability-empty">
          <strong>No installed skills</strong>
          <span>Create a private Skill above or import a portable bundle.</span>
        </div>
      )}
    </div>
  );
}

function VersionField({
  value,
  onChange,
}: {
  value: string;
  onChange(value: string): void;
}) {
  return (
    <label className="skill-version-field">
      <span>Version label</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="Optional · defaults to local-<content hash>"
        maxLength={80}
      />
      <small>
        Reusing a label with different content is rejected. A new version only
        changes this agent’s binding.
      </small>
    </label>
  );
}

function SkillCard({
  agentId,
  skill,
  onCustomize,
}: {
  agentId: string;
  skill: Skill;
  onCustomize(): void;
}) {
  const enable = useMutation({
    mutationFn: () => api.updateSkill(agentId, skill.id, !skill.enabled),
    onSuccess: () => invalidate(agentId),
  });
  const uninstall = useMutation({
    mutationFn: () => api.uninstallSkill(agentId, skill.id),
    onSuccess: () => invalidate(agentId),
  });
  const resources = skill.files.filter((file) => file.path !== "SKILL.md");

  return (
    <article className="capability-card skill-card">
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
          <span>
            {enable.isPending
              ? "Updating…"
              : skill.enabled
                ? "Enabled"
                : "Disabled"}
          </span>
        </label>
      </div>
      <div className="skill-summary-row">
        <span className="card-source">
          {skill.sourceType} · {skill.version} ·{" "}
          {skill.contentHash.slice(0, 20)}…
        </span>
        <span className="skill-file-count">
          {skill.files.length} file{skill.files.length === 1 ? "" : "s"}
        </span>
      </div>
      {resources.length > 0 && (
        <details className="skill-files">
          <summary>Bundle resources ({resources.length})</summary>
          <ul>
            {resources.map((file) => (
              <li key={file.path}>
                <span>{file.path}</span>
                <small>
                  {formatBytes(file.sizeBytes)} ·{" "}
                  {file.textReadable ? "readable text" : "binary asset"}
                </small>
              </li>
            ))}
          </ul>
        </details>
      )}
      <div className="card-actions">
        <button
          type="button"
          className="secondary-button"
          onClick={onCustomize}
        >
          <Pencil size={14} /> Customize
        </button>
        <button
          type="button"
          className="danger-button"
          disabled={uninstall.isPending}
          onClick={() => uninstall.mutate()}
        >
          <Trash2 size={15} />
          {uninstall.isPending ? "Removing…" : "Remove from agent"}
        </button>
      </div>
      {(enable.isError || uninstall.isError) && (
        <span className="inline-error" role="alert">
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

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function invalidate(agentId: string) {
  return queryClient.invalidateQueries({ queryKey: ["skills", agentId] });
}
