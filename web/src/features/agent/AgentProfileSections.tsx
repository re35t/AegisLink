import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  Archive,
  Check,
  Clock3,
  Copy,
  KeyRound,
  RefreshCw,
  RotateCw,
  Save,
  ShieldAlert,
  Trash2,
} from "lucide-react";

import {
  APIError,
  api,
  type AgentProfile,
  type FactCandidate,
  type Impression,
} from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { useInterfacePreferences } from "../settings/preferences";

export function OverviewSection({ profile }: { profile: AgentProfile }) {
  const { t } = useInterfacePreferences();
  return (
    <section
      className="agent-profile-section"
      aria-labelledby="profile-overview-title"
    >
      <div className="agent-profile-section-heading">
        <div>
          <span className="eyebrow">Internal self model</span>
          <h2 id="profile-overview-title">
            {t("Profile overview", "Profile 概览")}
          </h2>
          <p>
            {t(
              "Stable identity, effective capabilities, confirmed facts, and active short-term impressions are assembled here for Runtime context.",
              "稳定身份、有效能力、已确认事实和活跃短期印象会在这里聚合，并供 Runtime 使用。",
            )}
          </p>
        </div>
        <span className="profile-version">
          v{profile.version} · ctx {profile.contextRevision}
        </span>
      </div>
      <div className="profile-summary-grid">
        <ProfileMetric
          label={t("Capabilities", "能力")}
          value={profile.capabilities.length}
        />
        <ProfileMetric
          label={t("Confirmed facts", "已确认事实")}
          value={profile.confirmedFacts.length}
        />
        <ProfileMetric
          label={t("Active impressions", "活跃印象")}
          value={profile.impressions.length}
        />
        <ProfileMetric
          label={t("Pending review", "待审查")}
          value={profile.pendingFactCount}
        />
      </div>
      <div className="profile-object-list">
        {profile.capabilities.map((capability) => (
          <article key={capability.id} className="profile-object-row">
            <div>
              <span className="profile-object-kind">{capability.kind}</span>
              <strong>{capability.name}</strong>
              <p>{capability.description}</p>
            </div>
            <small>
              {capability.source} · {Math.round(capability.confidence * 100)}% ·{" "}
              {capability.callable
                ? t("callable", "可调用")
                : t("unavailable", "不可用")}
            </small>
          </article>
        ))}
      </div>
    </section>
  );
}

function ProfileMetric({ label, value }: { label: string; value: number }) {
  return (
    <div className="profile-metric">
      <strong>{value}</strong>
      <span>{label}</span>
    </div>
  );
}

export function ImpressionsSection({
  agentId,
  profile,
}: {
  agentId: string;
  profile: AgentProfile;
}) {
  const { t } = useInterfacePreferences();
  const [status, setStatus] = useState("active");
  const query = useQuery({
    queryKey: ["agent-impressions", agentId, status],
    queryFn: () => api.listAgentImpressions(agentId, status),
    retry: false,
  });
  const update = useMutation({
    mutationFn: ({
      item,
      summary,
      details,
      nextStatus,
    }: {
      item: Impression;
      summary?: string;
      details?: Record<string, unknown>;
      nextStatus?: "active" | "dismissed";
    }) =>
      api.updateAgentImpression(agentId, item.id, {
        expectedContextRevision: profile.contextRevision,
        ...(summary ? { summary } : {}),
        ...(details ? { details } : {}),
        ...(nextStatus ? { status: nextStatus } : {}),
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ["agent-impressions", agentId],
      });
      await queryClient.invalidateQueries({
        queryKey: ["agent-profile", agentId],
      });
    },
  });
  return (
    <section
      className="agent-profile-section"
      aria-labelledby="impressions-title"
    >
      <div className="agent-profile-section-heading">
        <div>
          <span className="eyebrow">Model-generated</span>
          <h2 id="impressions-title">Impressions</h2>
          <p>
            {t(
              "The model actively maintains rich, short-term observations from recent context and Memory. They may be wrong, never leave the Agent, and cannot override instructions.",
              "模型会从近期 context 与 Memory 主动维护丰富的短期观察。它们可能有误，永不对外披露，也不能覆盖指令。",
            )}
          </p>
        </div>
      </div>
      <label className="profile-filter">
        <span>{t("Status", "状态")}</span>
        <select
          value={status}
          onChange={(event) => setStatus(event.target.value)}
        >
          <option value="active">Active</option>
          <option value="stale">Stale</option>
          <option value="resolved">Resolved</option>
          <option value="dismissed">Dismissed</option>
          <option value="superseded">Superseded</option>
        </select>
      </label>
      {query.isPending ? (
        <ProfileState
          text={t("Loading Impressions…", "正在加载 Impression…")}
        />
      ) : query.isError ? (
        <ProfileError error={query.error} retry={() => void query.refetch()} />
      ) : query.data.length === 0 ? (
        <ProfileState
          text={t(
            "No Impressions in this state. They are created asynchronously after successful Runs.",
            "该状态下没有 Impression。成功 Run 完成后会异步创建。",
          )}
        />
      ) : (
        <div className="profile-object-list">
          {query.data.map((item) => (
            <ImpressionRow
              key={item.id}
              item={item}
              pending={update.isPending}
              onSave={(summary, details) =>
                update.mutate({ item, summary, details })
              }
              onStatus={(nextStatus) => update.mutate({ item, nextStatus })}
            />
          ))}
        </div>
      )}
      {update.error && (
        <p className="inline-error" role="alert">
          {friendlyError(update.error, t)}
        </p>
      )}
    </section>
  );
}

function ImpressionRow({
  item,
  pending,
  onSave,
  onStatus,
}: {
  item: Impression;
  pending: boolean;
  onSave(summary: string, details: Record<string, unknown>): void;
  onStatus(status: "active" | "dismissed"): void;
}) {
  const { t } = useInterfacePreferences();
  const [editing, setEditing] = useState(false);
  const [summary, setSummary] = useState(item.summary);
  const [details, setDetails] = useState(JSON.stringify(item.details, null, 2));
  const [jsonError, setJSONError] = useState("");
  const save = () => {
    try {
      const value = JSON.parse(details) as Record<string, unknown>;
      if (!value || Array.isArray(value) || typeof value !== "object")
        throw new Error();
      setJSONError("");
      onSave(summary.trim(), value);
      setEditing(false);
    } catch {
      setJSONError(
        t("Details must be a JSON object.", "Details 必须是 JSON 对象。"),
      );
    }
  };
  return (
    <article className="profile-object-row profile-editable-row">
      <div className="profile-object-copy">
        <span className="profile-object-kind">
          {item.scope} · {item.kind}
        </span>
        {editing ? (
          <>
            <textarea
              value={summary}
              maxLength={4000}
              onChange={(event) => setSummary(event.target.value)}
              aria-label={t("Impression summary", "Impression 摘要")}
            />
            <textarea
              className="profile-json-editor"
              value={details}
              onChange={(event) => setDetails(event.target.value)}
              aria-label={t(
                "Impression details JSON",
                "Impression details JSON",
              )}
            />
            {jsonError && <small className="inline-error">{jsonError}</small>}
          </>
        ) : (
          <>
            <strong>{item.summary}</strong>
            <pre>{JSON.stringify(item.details, null, 2)}</pre>
          </>
        )}
        <small>
          {Math.round(item.confidence * 100)}% confidence ·{" "}
          {Math.round(item.salience * 100)}% salience ·{" "}
          {Math.round(item.freshness * 100)}% freshness · {item.evidence.length}{" "}
          evidence
        </small>
      </div>
      <div className="profile-row-actions">
        {editing ? (
          <button
            type="button"
            disabled={pending || !summary.trim()}
            onClick={save}
          >
            <Save size={15} />
            {t("Save", "保存")}
          </button>
        ) : (
          <button
            type="button"
            className="secondary-button"
            onClick={() => setEditing(true)}
          >
            {t("Correct", "纠正")}
          </button>
        )}
        <button
          type="button"
          className="secondary-button"
          disabled={pending}
          onClick={() =>
            onStatus(item.status === "dismissed" ? "active" : "dismissed")
          }
        >
          {item.status === "dismissed" ? (
            <RefreshCw size={15} />
          ) : (
            <Archive size={15} />
          )}{" "}
          {item.status === "dismissed"
            ? t("Restore", "恢复")
            : t("Dismiss", "忽略")}
        </button>
      </div>
    </article>
  );
}

export function FactsSection({
  agentId,
  profile,
}: {
  agentId: string;
  profile: AgentProfile;
}) {
  const { t } = useInterfacePreferences();
  const candidates = useQuery({
    queryKey: ["agent-fact-candidates", agentId],
    queryFn: () => api.listAgentFactCandidates(agentId),
    retry: false,
  });
  const confirm = useMutation({
    mutationFn: ({
      candidate,
      namespace,
      key,
      value,
    }: {
      candidate: FactCandidate;
      namespace: string;
      key: string;
      value: Record<string, unknown>;
    }) =>
      api.confirmAgentFactCandidate(agentId, candidate.id, {
        expectedVersion: profile.version,
        expectedCandidateVersion: candidate.version,
        subject: candidate.subject,
        namespace,
        key,
        value,
      }),
    onSuccess: (next) => refreshProfile(agentId, next),
  });
  const reject = useMutation({
    mutationFn: (candidate: FactCandidate) =>
      api.rejectAgentFactCandidate(agentId, candidate.id, candidate.version),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: ["agent-fact-candidates", agentId],
      }),
  });
  const revoke = useMutation({
    mutationFn: (factId: string) =>
      api.revokeAgentConfirmedFact(agentId, factId, profile.version),
    onSuccess: (next) => refreshProfile(agentId, next),
  });
  return (
    <section className="agent-profile-section" aria-labelledby="facts-title">
      <div className="agent-profile-section-heading">
        <div>
          <span className="eyebrow">Owner-confirmed</span>
          <h2 id="facts-title">Confirmed Facts</h2>
          <p>
            {t(
              "Candidates are distilled from Impressions. Only your explicit confirmation creates a durable Fact; only agent-subject Facts can be disclosed externally.",
              "候选事实从 Impression 中提炼。只有你的明确确认才会创建长期 Fact；仅 subject=agent 的 Fact 可对外披露。",
            )}
          </p>
        </div>
        <span className="profile-version">
          {profile.pendingFactCount} pending
        </span>
      </div>
      <h3>{t("Review inbox", "审查收件箱")}</h3>
      {candidates.isPending ? (
        <ProfileState text={t("Loading candidates…", "正在加载候选…")} />
      ) : candidates.isError ? (
        <ProfileError
          error={candidates.error}
          retry={() => void candidates.refetch()}
        />
      ) : candidates.data.length === 0 ? (
        <ProfileState
          text={t(
            "No pending Fact candidates. The curator proposes them after it finds repeatable evidence in Impressions.",
            "暂无待确认 Fact 候选。Curator 会在 Impression 中发现可重复证据后提出候选。",
          )}
        />
      ) : (
        <div className="profile-object-list">
          {candidates.data.map((candidate) => (
            <CandidateRow
              key={candidate.id}
              candidate={candidate}
              pending={confirm.isPending || reject.isPending}
              onConfirm={(namespace, key, value) =>
                confirm.mutate({ candidate, namespace, key, value })
              }
              onReject={() => reject.mutate(candidate)}
            />
          ))}
        </div>
      )}
      <h3>{t("Active facts", "有效事实")}</h3>
      {profile.confirmedFacts.length === 0 ? (
        <ProfileState
          text={t("No confirmed Facts yet.", "尚无已确认 Fact。")}
        />
      ) : (
        <div className="profile-object-list">
          {profile.confirmedFacts.map((fact) => (
            <article className="profile-object-row" key={fact.id}>
              <div>
                <span className="profile-object-kind">{fact.subject}</span>
                <strong>
                  {fact.namespace}.{fact.key}
                </strong>
                <pre>{JSON.stringify(fact.value, null, 2)}</pre>
                <small>
                  {fact.confirmation.method} ·{" "}
                  {new Date(fact.confirmation.confirmedAt).toLocaleString()}
                </small>
              </div>
              <button
                type="button"
                className="danger-secondary"
                disabled={revoke.isPending}
                onClick={() => revoke.mutate(fact.id)}
              >
                <Trash2 size={15} />
                {t("Revoke", "撤销")}
              </button>
            </article>
          ))}
        </div>
      )}
      {(confirm.error || reject.error || revoke.error) && (
        <p className="inline-error" role="alert">
          {friendlyError(confirm.error || reject.error || revoke.error, t)}
        </p>
      )}
    </section>
  );
}

function CandidateRow({
  candidate,
  pending,
  onConfirm,
  onReject,
}: {
  candidate: FactCandidate;
  pending: boolean;
  onConfirm(
    namespace: string,
    key: string,
    value: Record<string, unknown>,
  ): void;
  onReject(): void;
}) {
  const { t } = useInterfacePreferences();
  const [namespace, setNamespace] = useState(candidate.namespace);
  const [key, setKey] = useState(candidate.key);
  const [value, setValue] = useState(JSON.stringify(candidate.value, null, 2));
  const [error, setError] = useState("");
  const confirm = () => {
    const normalizedNamespace = namespace.trim();
    const normalizedKey = key.trim();
    if (!normalizedNamespace || !normalizedKey) {
      setError(
        t("Namespace and key are required.", "Namespace 与 key 均不能为空。"),
      );
      return;
    }
    try {
      const parsed = JSON.parse(value) as Record<string, unknown>;
      if (!parsed || Array.isArray(parsed) || typeof parsed !== "object")
        throw new Error();
      setError("");
      onConfirm(normalizedNamespace, normalizedKey, parsed);
    } catch {
      setError(t("Value must be a JSON object.", "Value 必须是 JSON 对象。"));
    }
  };
  return (
    <article className="profile-object-row profile-editable-row">
      <div>
        <span className="profile-object-kind">{candidate.subject}</span>
        <div className="candidate-identity-fields">
          <label>
            <span>{t("Namespace", "Namespace")}</span>
            <input
              required
              maxLength={80}
              value={namespace}
              disabled={pending}
              onChange={(event) => setNamespace(event.target.value)}
            />
          </label>
          <label>
            <span>{t("Key", "Key")}</span>
            <input
              required
              maxLength={120}
              value={key}
              disabled={pending}
              onChange={(event) => setKey(event.target.value)}
            />
          </label>
        </div>
        <p>{candidate.rationale}</p>
        <textarea
          className="profile-json-editor"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          aria-label={`${candidate.namespace}.${candidate.key} JSON`}
        />
        {error && <small className="inline-error">{error}</small>}
        <small>
          {Math.round(candidate.confidence * 100)}% confidence ·{" "}
          {candidate.sourceImpressionIds.length} Impression sources
        </small>
      </div>
      <div className="profile-row-actions">
        <button type="button" disabled={pending} onClick={confirm}>
          <Check size={15} />
          {t("Confirm", "确认")}
        </button>
        <button
          type="button"
          className="secondary-button"
          disabled={pending}
          onClick={onReject}
        >
          {t("Reject", "拒绝")}
        </button>
      </div>
    </article>
  );
}

export function PublicationSection({ agentId }: { agentId: string }) {
  const { t } = useInterfacePreferences();
  const query = useQuery({
    queryKey: ["agent-publication", agentId],
    queryFn: () => api.getAgentPublication(agentId),
    retry: false,
  });
  const [hostname, setHostname] = useState("");
  const [tokenLabel, setTokenLabel] = useState("");
  const [audience, setAudience] = useState("");
  const [secret, setSecret] = useState("");
  useEffect(() => {
    if (query.data) setHostname(query.data.hostname);
  }, [query.data]);
  const update = useMutation({
    mutationFn: (input: { hostname?: string; enabled?: boolean }) =>
      api.updateAgentPublication(agentId, {
        expectedRevision: query.data!.revision,
        ...input,
      }),
    onSuccess: (next) =>
      queryClient.setQueryData(["agent-publication", agentId], next),
  });
  const verify = useMutation({
    mutationFn: () => api.verifyAgentPublicationHostname(agentId),
    onSuccess: (next) =>
      queryClient.setQueryData(["agent-publication", agentId], next),
  });
  const rotate = useMutation({
    mutationFn: () => api.rotateAgentPublicationKey(agentId),
    onSuccess: (next) =>
      queryClient.setQueryData(["agent-publication", agentId], next),
  });
  const createToken = useMutation({
    mutationFn: () =>
      api.createAgentAccessToken(agentId, { label: tokenLabel, audience }),
    onSuccess: async (created) => {
      setSecret(created.secret);
      setTokenLabel("");
      setAudience("");
      await queryClient.invalidateQueries({
        queryKey: ["agent-publication", agentId],
      });
    },
  });
  const revokeToken = useMutation({
    mutationFn: (tokenId: string) =>
      api.revokeAgentAccessToken(agentId, tokenId),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: ["agent-publication", agentId],
      }),
  });
  if (query.isPending)
    return (
      <ProfileState
        text={t("Loading publication settings…", "正在加载发布设置…")}
      />
    );
  if (query.isError)
    return (
      <ProfileError error={query.error} retry={() => void query.refetch()} />
    );
  const item = query.data;
  const pending =
    update.isPending ||
    verify.isPending ||
    rotate.isPending ||
    createToken.isPending ||
    revokeToken.isPending;
  const operationError =
    update.error ||
    verify.error ||
    rotate.error ||
    createToken.error ||
    revokeToken.error;
  return (
    <section
      className="agent-profile-section"
      aria-labelledby="publication-title"
    >
      <div className="agent-profile-section-heading">
        <div>
          <span className="eyebrow">Discovery & trust</span>
          <h2 id="publication-title">AgentFacts publication</h2>
          <p>
            {t(
              "Publish only explicitly disclosed claims under a verified hostname. AgentCard remains an owner-only draft until an A2A endpoint exists.",
              "仅在已验证域名下发布明确允许披露的声明。AgentCard 在存在 A2A endpoint 前仍只是 Owner Draft。",
            )}
          </p>
        </div>
        <span className="profile-version">rev {item.revision}</span>
      </div>
      {!item.signingKey.available && (
        <div className="disclosure-legend warning">
          <ShieldAlert size={16} />
          <span>
            {t(
              "AGENT_KEY_ENCRYPTION_KEY is not configured. Internal Profile and Impressions work normally, but publication is unavailable.",
              "未配置 AGENT_KEY_ENCRYPTION_KEY。内部 Profile 与 Impression 可正常使用，但无法发布。",
            )}
          </span>
        </div>
      )}
      <div className="publication-grid">
        <label>
          <span>{t("Hostname", "域名")}</span>
          <input
            value={hostname}
            placeholder="agent.example.com"
            disabled={pending}
            onChange={(event) => setHostname(event.target.value)}
          />
        </label>
        <button
          type="button"
          disabled={pending || hostname === item.hostname}
          onClick={() => update.mutate({ hostname })}
        >
          <Save size={15} />
          {t("Save hostname", "保存域名")}
        </button>
        <div>
          <strong>{t("DNS verification", "DNS 验证")}</strong>
          <code>_aegislink.{item.hostname || "your-domain"}</code>
          <code>aegislink-verification={item.dnsChallenge || "…"}</code>
        </div>
        <button
          type="button"
          className="secondary-button"
          disabled={pending || item.hostnameStatus === "unconfigured"}
          onClick={() => verify.mutate()}
        >
          <RefreshCw size={15} />
          {t("Verify TXT", "验证 TXT")}
        </button>
        <div>
          <strong>{t("Signing key", "签名密钥")}</strong>
          <span>{item.signingKey.fingerprint || item.signingKey.status}</span>
        </div>
        <button
          type="button"
          className="secondary-button"
          disabled={pending || !item.signingKey.available}
          onClick={() => rotate.mutate()}
        >
          <RotateCw size={15} />
          {t("Rotate key", "轮换密钥")}
        </button>
        <div>
          <strong>{t("Publication", "发布")}</strong>
          <span>
            {item.enabled ? t("Enabled", "已启用") : t("Disabled", "未启用")}
          </span>
        </div>
        <button
          type="button"
          disabled={
            pending ||
            !item.signingKey.available ||
            item.hostnameStatus !== "verified"
          }
          onClick={() => update.mutate({ enabled: !item.enabled })}
        >
          {item.enabled ? t("Disable", "关闭") : t("Enable", "启用")}
        </button>
      </div>
      <h3>{t("Query access tokens", "查询访问令牌")}</h3>
      <form
        className="token-form"
        onSubmit={(event: FormEvent) => {
          event.preventDefault();
          if (tokenLabel.trim() && audience.trim()) createToken.mutate();
        }}
      >
        <input
          required
          maxLength={80}
          value={tokenLabel}
          placeholder={t("Token label", "令牌标签")}
          onChange={(event) => setTokenLabel(event.target.value)}
        />
        <input
          required
          maxLength={120}
          value={audience}
          placeholder={t("Exact audience", "精确 audience")}
          onChange={(event) => setAudience(event.target.value)}
        />
        <button disabled={pending}>
          <KeyRound size={15} />
          {t("Create token", "创建令牌")}
        </button>
      </form>
      {secret && (
        <div className="one-time-secret" role="status">
          <strong>{t("Copy now — shown once", "立即复制，仅显示一次")}</strong>
          <code>{secret}</code>
          <button
            type="button"
            className="secondary-button"
            onClick={() => void navigator.clipboard.writeText(secret)}
          >
            <Copy size={15} />
            {t("Copy", "复制")}
          </button>
        </div>
      )}
      <div className="profile-object-list">
        {item.tokens.map((token) => (
          <article className="profile-object-row" key={token.id}>
            <div>
              <strong>{token.label}</strong>
              <p>{token.audience}</p>
              <small>
                <Clock3 size={13} />{" "}
                {new Date(token.expiresAt).toLocaleString()} ·{" "}
                {token.revoked ? "revoked" : "active"}
              </small>
            </div>
            {!token.revoked && (
              <button
                type="button"
                className="danger-secondary"
                disabled={pending}
                onClick={() => revokeToken.mutate(token.id)}
              >
                <Trash2 size={15} />
                {t("Revoke", "撤销")}
              </button>
            )}
          </article>
        ))}
      </div>
      <h3>AgentCard Draft</h3>
      <div className="disclosure-legend warning">
        <ShieldAlert size={16} />
        <span>
          {item.agentCard.readiness.blockers.join(", ") || t("Ready", "就绪")}
        </span>
      </div>
      <pre className="publication-preview">
        {JSON.stringify(item.agentCard.draft, null, 2)}
      </pre>
      {operationError && (
        <p className="inline-error" role="alert">
          {friendlyError(operationError, t)}
        </p>
      )}
    </section>
  );
}

function ProfileState({ text }: { text: string }) {
  return (
    <div className="agent-profile-empty">
      <Clock3 size={18} />
      <span>{text}</span>
    </div>
  );
}
function ProfileError({ error, retry }: { error: unknown; retry(): void }) {
  const { t } = useInterfacePreferences();
  return (
    <div className="settings-error" role="alert">
      <span>{friendlyError(error, t)}</span>
      <button type="button" onClick={retry}>
        <RefreshCw size={15} />
        {t("Retry", "重试")}
      </button>
    </div>
  );
}
function friendlyError(
  error: unknown,
  t: (english: string, chinese: string) => string,
) {
  if (
    error instanceof APIError &&
    (error.code === "profile_version_conflict" ||
      error.code === "version_conflict")
  )
    return t(
      "This data changed elsewhere. Reload before trying again.",
      "数据已在其他位置变化，请重新加载后重试。",
    );
  return error instanceof Error
    ? error.message
    : t("The request could not be completed.", "无法完成请求。");
}
function refreshProfile(agentId: string, profile: AgentProfile) {
  queryClient.setQueryData(["agent-profile", agentId], profile);
  void queryClient.invalidateQueries({
    queryKey: ["agent-fact-candidates", agentId],
  });
  void queryClient.invalidateQueries({
    queryKey: ["agent-publication", agentId],
  });
}
