import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Clock3, RefreshCw, Save, ShieldAlert, Unplug } from "lucide-react";

import { APIError, api } from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { useInterfacePreferences } from "../settings/preferences";

export function CollaborationSection({ agentId }: { agentId: string }) {
  const { t } = useInterfacePreferences();
  const query = useQuery({
    queryKey: ["agent-collaborations", agentId],
    queryFn: () => api.getAgentCollaborations(agentId),
    retry: false,
  });
  const [enabled, setEnabled] = useState(false);
  const [ttl, setTTL] = useState(3600);
  const [requests, setRequests] = useState(20);
  const [sessions, setSessions] = useState(5);

  useEffect(() => {
    if (!query.data) return;
    setEnabled(query.data.policy.enabled);
    setTTL(query.data.policy.maxSessionTtlSeconds);
    setRequests(query.data.policy.maxRequestsPerHour);
    setSessions(query.data.policy.maxActiveSessions);
  }, [query.data]);

  const update = useMutation({
    mutationFn: () =>
      api.updateAgentCollaborationPolicy(agentId, {
        expectedRevision: query.data!.policy.revision,
        enabled,
        maxSessionTtlSeconds: ttl,
        maxRequestsPerHour: requests,
        maxActiveSessions: sessions,
      }),
    onSuccess: (policy) =>
      queryClient.setQueryData(
        ["agent-collaborations", agentId],
        query.data ? { ...query.data, policy } : undefined,
      ),
  });
  const revoke = useMutation({
    mutationFn: (sessionId: string) =>
      api.revokeCollaborationSession(agentId, sessionId),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: ["agent-collaborations", agentId],
      }),
  });

  if (query.isPending)
    return (
      <CollaborationState
        text={t("Loading collaboration controls…", "正在加载协作控制…")}
      />
    );
  if (query.isError)
    return (
      <div className="settings-error" role="alert">
        <span>{friendlyError(query.error, t)}</span>
        <button type="button" onClick={() => void query.refetch()}>
          <RefreshCw size={15} /> {t("Retry", "重试")}
        </button>
      </div>
    );

  const item = query.data;
  const pending = update.isPending || revoke.isPending;
  return (
    <section
      className="agent-profile-section"
      aria-labelledby="collaboration-title"
    >
      <div className="agent-profile-section-heading">
        <div>
          <span className="eyebrow">A2A 1.0</span>
          <h2 id="collaboration-title">
            {t("Agent collaboration", "Agent 协作")}
          </h2>
          <p>
            {t(
              "Allow this Agent to evaluate text-only assistance requests. Accepted requests receive a scoped, expiring A2A Session; they never become owner Conversations.",
              "允许该 Agent 评估仅文本的协助请求。被接受的请求会获得有范围且会过期的 A2A Session，绝不会变成 Owner Conversation。",
            )}
          </p>
        </div>
        <span className="profile-version">rev {item.policy.revision}</span>
      </div>

      {!item.policy.encryptionReady && (
        <div className="disclosure-legend warning" role="status">
          <ShieldAlert size={16} />
          <span>
            {t(
              "AGENT_KEY_ENCRYPTION_KEY is required before collaboration can be enabled.",
              "启用协作前必须配置 AGENT_KEY_ENCRYPTION_KEY。",
            )}
          </span>
        </div>
      )}

      <form
        className="collaboration-policy-grid"
        onSubmit={(event: FormEvent) => {
          event.preventDefault();
          update.mutate();
        }}
      >
        <label className="collaboration-toggle">
          <input
            type="checkbox"
            checked={enabled}
            disabled={pending || !item.policy.encryptionReady}
            onChange={(event) => setEnabled(event.target.checked)}
          />
          <span>
            <strong>{t("Accept evaluations", "接受协作评估")}</strong>
            <small>
              {t("Off by default for every Agent", "每个 Agent 默认关闭")}
            </small>
          </span>
        </label>
        <label>
          <span>
            {t("Maximum Session TTL (seconds)", "Session 最大 TTL（秒）")}
          </span>
          <input
            type="number"
            min={300}
            max={86400}
            value={ttl}
            disabled={pending}
            onChange={(event) => setTTL(Number(event.target.value))}
          />
        </label>
        <label>
          <span>{t("Inbound requests per hour", "每小时入站请求")}</span>
          <input
            type="number"
            min={1}
            max={200}
            value={requests}
            disabled={pending}
            onChange={(event) => setRequests(Number(event.target.value))}
          />
        </label>
        <label>
          <span>{t("Active Sessions", "活跃 Session")}</span>
          <input
            type="number"
            min={1}
            max={50}
            value={sessions}
            disabled={pending}
            onChange={(event) => setSessions(Number(event.target.value))}
          />
        </label>
        <div className="profile-form-actions collaboration-save">
          <span>
            {t(
              "Only text/plain A2A Tasks; no Memory, Skills, or MCP.",
              "仅允许 text/plain A2A Task；不加载 Memory、Skill 或 MCP。",
            )}
          </span>
          <button
            disabled={pending || (enabled && !item.policy.encryptionReady)}
          >
            <Save size={15} />{" "}
            {update.isPending
              ? t("Saving…", "保存中…")
              : t("Save policy", "保存策略")}
          </button>
        </div>
      </form>
      {update.error && (
        <p className="inline-error" role="alert">
          {friendlyError(update.error, t)}
        </p>
      )}

      <h3>{t("Collaboration Sessions", "协作 Session")}</h3>
      {item.sessions.length === 0 ? (
        <CollaborationState
          text={t("No Collaboration Sessions yet.", "还没有协作 Session。")}
        />
      ) : (
        <div className="profile-object-list">
          {item.sessions.map((session) => {
            const active =
              session.status === "active" &&
              new Date(session.expiresAt).getTime() > Date.now();
            return (
              <article className="profile-object-row" key={session.id}>
                <div>
                  <strong>{session.targetAgentAddr}</strong>
                  <p>
                    {session.requesterAgentId} → {session.targetAgentId}
                  </p>
                  <small>
                    <Clock3 size={13} />{" "}
                    {active ? t("Active until", "有效至") : session.status}{" "}
                    {new Date(session.expiresAt).toLocaleString()}
                  </small>
                  <small>{session.scopes.join(" · ")}</small>
                </div>
                {active && (
                  <button
                    type="button"
                    className="danger-secondary"
                    disabled={pending}
                    onClick={() => {
                      if (
                        window.confirm(
                          t(
                            "Revoke this Collaboration Session now?",
                            "立即撤销该协作 Session？",
                          ),
                        )
                      )
                        revoke.mutate(session.id);
                    }}
                  >
                    <Unplug size={15} /> {t("Revoke", "撤销")}
                  </button>
                )}
              </article>
            );
          })}
        </div>
      )}

      <h3>{t("Assistance requests", "协助请求")}</h3>
      {item.requests.length === 0 ? (
        <CollaborationState
          text={t("No assistance requests yet.", "还没有协助请求。")}
        />
      ) : (
        <div className="profile-object-list">
          {item.requests.map((request) => (
            <article className="profile-object-row" key={request.id}>
              <div>
                <strong>
                  {request.status} · {request.targetAgentAddr}
                </strong>
                <p>{request.purpose}</p>
                <small>
                  {new Date(request.createdAt).toLocaleString()}{" "}
                  {request.decisionCode ? `· ${request.decisionCode}` : ""}
                </small>
              </div>
            </article>
          ))}
        </div>
      )}
      {revoke.error && (
        <p className="inline-error" role="alert">
          {friendlyError(revoke.error, t)}
        </p>
      )}
    </section>
  );
}

function CollaborationState({ text }: { text: string }) {
  return (
    <div className="agent-profile-empty">
      <Clock3 size={18} />
      <span>{text}</span>
    </div>
  );
}

function friendlyError(
  error: unknown,
  t: (english: string, chinese: string) => string,
) {
  if (error instanceof APIError && error.code === "collaboration_conflict")
    return t(
      "The policy changed elsewhere. Reload before saving.",
      "策略已在其他位置变化，请重新加载后保存。",
    );
  return error instanceof Error
    ? error.message
    : t("The request could not be completed.", "无法完成请求。");
}
