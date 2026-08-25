import { useEffect, useMemo, useState } from "react";
import { Check, Database, LockKeyhole } from "lucide-react";

import type {
  AgentProfile,
  DisclosurePolicy,
  DisclosurePolicyChange,
} from "../../api/client";
import { useInterfacePreferences } from "../settings/preferences";

type SubjectType = DisclosurePolicyChange["subjectType"];
type Channel = DisclosurePolicy["channels"][number];

interface PolicySubject {
  type: SubjectType;
  id: string;
  label: string;
  detail: string;
  metadata: string;
  policy: DisclosurePolicy;
}

interface DisclosurePolicyEditorProps {
  profile: AgentProfile;
  pending: boolean;
  error: string;
  conflict: boolean;
  onReload(): void;
  onSave(changes: DisclosurePolicyChange[]): void;
}

const channelOptions: Array<{
  value: Channel;
  label: [string, string];
}> = [
  { value: "runtime-context", label: ["Runtime context", "运行时上下文"] },
  { value: "agent-facts", label: ["Agent Facts", "Agent Facts"] },
  { value: "agent-card", label: ["Agent Card", "Agent Card"] },
];

export function DisclosurePolicyEditor({
  profile,
  pending,
  error,
  conflict,
  onReload,
  onSave,
}: DisclosurePolicyEditorProps) {
  const { t } = useInterfacePreferences();
  const subjects = useMemo(() => profileSubjects(profile, t), [profile, t]);
  const [drafts, setDrafts] = useState<Record<string, DisclosurePolicy>>(() =>
    initialDrafts(subjects),
  );

  useEffect(
    () => setDrafts(initialDrafts(subjects)),
    [profile.version, subjects],
  );

  const changes = subjects.flatMap<DisclosurePolicyChange>((subject) => {
    const draft = drafts[subjectKey(subject)] ?? subject.policy;
    return equalPolicy(draft, subject.policy)
      ? []
      : [{ subjectType: subject.type, subjectId: subject.id, policy: draft }];
  });

  return (
    <section
      className="agent-profile-section"
      aria-labelledby="profile-disclosure-title"
    >
      <div className="agent-profile-section-heading">
        <div>
          <span className="eyebrow">Disclosure</span>
          <h2 id="profile-disclosure-title">
            {t("Disclosure policy", "披露策略")}
          </h2>
          <p>
            {t(
              "Choose where each Profile item may be projected. Public Agent Card and Agent Facts publishing are not enabled yet.",
              "选择每项 Profile 信息将来可被投影到哪里。公开 Agent Card 与 Agent Facts 发布当前尚未启用。",
            )}
          </p>
        </div>
        <span className="profile-version">v{profile.version}</span>
      </div>

      <div className="disclosure-legend" role="note">
        <LockKeyhole size={16} />
        <span>
          {t(
            "Private is the safe default. Runtime context remains local to this Agent.",
            "Private 是安全默认值；Runtime context 只属于当前 Agent。",
          )}
        </span>
      </div>

      <div className="disclosure-subjects">
        {subjects.map((subject) => {
          const key = subjectKey(subject);
          const policy = drafts[key] ?? subject.policy;
          return (
            <article className="disclosure-subject" key={key}>
              <div className="disclosure-subject-copy">
                <span className="profile-object-kind">
                  {subjectTypeLabel(subject.type, t)}
                </span>
                <strong>{subject.label}</strong>
                <p>{subject.detail}</p>
                <small>{subject.metadata}</small>
              </div>
              <div className="disclosure-controls">
                <label>
                  <span>{t("Visibility", "可见性")}</span>
                  <select
                    aria-label={`${subject.label} ${t("visibility", "可见性")}`}
                    value={policy.visibility}
                    disabled={pending}
                    onChange={(event) =>
                      setDrafts((current) => ({
                        ...current,
                        [key]: policyForVisibility(
                          policy,
                          event.target.value as DisclosurePolicy["visibility"],
                        ),
                      }))
                    }
                  >
                    <option value="private">Private</option>
                    <option value="authenticated">Authenticated</option>
                    <option value="restricted">Restricted</option>
                    <option value="public">Public</option>
                  </select>
                </label>

                <fieldset>
                  <legend>{t("Projection channels", "投影渠道")}</legend>
                  <div className="disclosure-channel-options">
                    {channelOptions.map((option) => {
                      const external = option.value !== "runtime-context";
                      const disabled =
                        pending ||
                        (policy.visibility === "private" && external);
                      return (
                        <label key={option.value}>
                          <input
                            type="checkbox"
                            checked={policy.channels.includes(option.value)}
                            disabled={disabled}
                            onChange={() =>
                              setDrafts((current) => ({
                                ...current,
                                [key]: toggleChannel(policy, option.value),
                              }))
                            }
                          />
                          <span>{t(option.label[0], option.label[1])}</span>
                        </label>
                      );
                    })}
                  </div>
                </fieldset>

                {policy.visibility === "restricted" && (
                  <label className="disclosure-audiences">
                    <span>{t("Audiences", "受众")}</span>
                    <input
                      type="text"
                      value={policy.audiences.join(", ")}
                      placeholder={t(
                        "team-security, partner",
                        "安全团队, 合作方",
                      )}
                      disabled={pending}
                      onChange={(event) =>
                        setDrafts((current) => ({
                          ...current,
                          [key]: {
                            ...policy,
                            audiences: event.target.value
                              .split(",")
                              .map((value) => value.trim())
                              .filter(Boolean),
                          },
                        }))
                      }
                    />
                  </label>
                )}

                <label className="disclosure-indexable">
                  <input
                    type="checkbox"
                    checked={policy.indexable}
                    disabled={
                      pending ||
                      policy.visibility !== "public" ||
                      !policy.channels.includes("agent-facts")
                    }
                    onChange={(event) =>
                      setDrafts((current) => ({
                        ...current,
                        [key]: { ...policy, indexable: event.target.checked },
                      }))
                    }
                  />
                  <span>{t("Allow indexing", "允许建立索引")}</span>
                </label>
              </div>
            </article>
          );
        })}
      </div>

      {subjects.length === 1 && (
        <div className="agent-profile-empty">
          <Database size={18} />
          <span>
            {t(
              "Capabilities appear after Skills or MCP Tools are bound. Facts and Memory Projections remain empty until a reviewed derivation workflow is added.",
              "绑定 Skills 或 MCP Tools 后会显示能力；在加入可审查的派生流程前，Facts 与 Memory Projections 保持为空。",
            )}
          </span>
        </div>
      )}

      <div className="profile-save-bar">
        <span aria-live="polite">
          {pending
            ? t("Saving disclosure policies…", "正在保存披露策略…")
            : error ||
              (changes.length
                ? t(
                    `${changes.length} unsaved changes`,
                    `${changes.length} 项未保存更改`,
                  )
                : t("Disclosure policies are saved.", "披露策略已保存。"))}
        </span>
        {conflict && (
          <button type="button" className="secondary-button" onClick={onReload}>
            {t("Reload Profile", "重新加载 Profile")}
          </button>
        )}
        <button
          type="button"
          disabled={
            pending ||
            changes.length === 0 ||
            hasInvalidRestrictedPolicy(changes)
          }
          onClick={() => onSave(changes)}
        >
          <Check size={16} />
          {t("Save disclosure", "保存披露策略")}
        </button>
      </div>
    </section>
  );
}

function profileSubjects(
  profile: AgentProfile,
  t: (english: string, chinese: string) => string,
): PolicySubject[] {
  return [
    {
      type: "identity",
      id: profile.identity.id,
      label: profile.identity.name,
      detail: profile.identity.description || t("Agent identity", "Agent 身份"),
      metadata: profile.identity.humanLinked
        ? t("Linked to its Human Principal", "已关联 Human Principal")
        : t("Not human-linked", "未关联 Human Principal"),
      policy: profile.identity.disclosure,
    },
    ...profile.capabilities.map((capability) => ({
      type: "capability" as const,
      id: capability.id,
      label: capability.name,
      detail: capability.description,
      metadata: `${capability.kind} · ${capability.source} · ${Math.round(capability.confidence * 100)}% · ${capability.callable ? t("callable", "可调用") : t("unavailable", "不可用")}`,
      policy: capability.disclosure,
    })),
    ...profile.facts.map((fact) => ({
      type: "fact" as const,
      id: fact.id,
      label: `${fact.namespace}.${fact.key}`,
      detail: JSON.stringify(fact.value),
      metadata: `${fact.source} · ${Math.round(fact.confidence * 100)}%`,
      policy: fact.disclosure,
    })),
    ...profile.memoryProjections.map((projection) => ({
      type: "projection" as const,
      id: projection.id,
      label: projection.type,
      detail: projection.summary,
      metadata: `${projection.status} · ${Math.round(projection.confidence * 100)}% · ${projection.sourceMemoryIds.length} ${t("sources", "个来源")}`,
      policy: projection.disclosure,
    })),
  ];
}

function initialDrafts(subjects: PolicySubject[]) {
  return Object.fromEntries(
    subjects.map((subject) => [
      subjectKey(subject),
      clonePolicy(subject.policy),
    ]),
  );
}

function subjectKey(subject: Pick<PolicySubject, "type" | "id">) {
  return `${subject.type}:${subject.id}`;
}

function clonePolicy(policy: DisclosurePolicy): DisclosurePolicy {
  return {
    ...policy,
    channels: [...(policy.channels ?? [])],
    audiences: [...(policy.audiences ?? [])],
  };
}

function equalPolicy(left: DisclosurePolicy, right: DisclosurePolicy) {
  return JSON.stringify(left) === JSON.stringify(right);
}

function policyForVisibility(
  policy: DisclosurePolicy,
  visibility: DisclosurePolicy["visibility"],
): DisclosurePolicy {
  if (visibility === "private") {
    return {
      visibility,
      channels: policy.channels.includes("runtime-context")
        ? ["runtime-context"]
        : [],
      indexable: false,
      audiences: [],
    };
  }
  return {
    ...policy,
    visibility,
    indexable: visibility === "public" ? policy.indexable : false,
    audiences: visibility === "restricted" ? policy.audiences : [],
  };
}

function toggleChannel(policy: DisclosurePolicy, channel: Channel) {
  const channels = policy.channels.includes(channel)
    ? policy.channels.filter((candidate) => candidate !== channel)
    : [...policy.channels, channel];
  return {
    ...policy,
    channels,
    indexable:
      policy.indexable &&
      channels.includes("agent-facts") &&
      policy.visibility === "public",
  };
}

function hasInvalidRestrictedPolicy(changes: DisclosurePolicyChange[]) {
  return changes.some(
    (change) =>
      change.policy.visibility === "restricted" &&
      change.policy.audiences.length === 0,
  );
}

function subjectTypeLabel(
  subjectType: SubjectType,
  t: (english: string, chinese: string) => string,
) {
  switch (subjectType) {
    case "identity":
      return t("Identity", "身份");
    case "capability":
      return t("Capability", "能力");
    case "fact":
      return "Fact";
    case "projection":
      return t("Memory Projection", "记忆投影");
  }
}
