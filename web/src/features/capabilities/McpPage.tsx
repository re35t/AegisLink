import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Plus, RefreshCw, Trash2 } from "lucide-react";

import { api, type McpServer, type McpTool } from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { CapabilityError, CapabilityShell } from "./CapabilityShell";

export function McpPage() {
  return (
    <CapabilityShell
      area="mcp"
      eyebrow="Tools"
      title="MCP plugin library"
      description="Install MCP servers once, then choose which plugins and read-only tools this agent may use."
    >
      {(agentId) => <McpManager agentId={agentId} />}
    </CapabilityShell>
  );
}

function McpManager({ agentId }: { agentId: string }) {
  const [name, setName] = useState("");
  const [endpoint, setEndpoint] = useState("");
  const servers = useQuery({
    queryKey: ["mcp-servers", agentId],
    queryFn: () => api.listMcpServers(agentId),
  });
  const create = useMutation({
    mutationFn: () =>
      api.createMcpServer(agentId, name.trim(), endpoint.trim()),
    onSuccess: async () => {
      setName("");
      setEndpoint("");
      await invalidate(agentId);
    },
  });

  return (
    <div className="capability-stack">
      <form
        className="capability-composer compact-composer"
        onSubmit={(event) => {
          event.preventDefault();
          create.mutate();
        }}
      >
        <div className="composer-heading">
          <div>
            <h2>Install an MCP plugin</h2>
            <p>
              Production endpoints must use HTTPS. Local HTTP requires an
              explicit server setting.
            </p>
          </div>
        </div>
        <div className="mcp-fields">
          <label>
            <span>Name</span>
            <input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="github"
            />
          </label>
          <label>
            <span>Streamable HTTP endpoint</span>
            <input
              type="url"
              value={endpoint}
              onChange={(event) => setEndpoint(event.target.value)}
              placeholder="https://mcp.example.com/mcp"
            />
          </label>
        </div>
        <div className="composer-actions">
          {create.isError && (
            <span className="inline-error">{create.error.message}</span>
          )}
          <button
            type="submit"
            disabled={!name.trim() || !endpoint.trim() || create.isPending}
          >
            <Plus size={16} /> Install plugin
          </button>
        </div>
      </form>

      <aside className="safety-note">
        Only enabled read-only tools are callable today. External-write and
        destructive tools remain blocked until the runtime has a per-call
        approval flow.
      </aside>

      {servers.isError ? (
        <CapabilityError
          message={servers.error.message}
          onRetry={() => void servers.refetch()}
        />
      ) : servers.isLoading ? (
        <div className="capability-empty">Loading MCP servers…</div>
      ) : servers.data?.length ? (
        <div className="capability-list">
          {servers.data.map((server) => (
            <McpServerCard key={server.id} agentId={agentId} server={server} />
          ))}
        </div>
      ) : (
        <div className="capability-empty">
          <strong>Your plugin library is empty</strong>
          <span>Install a server, then discover and enable its tools.</span>
        </div>
      )}
    </div>
  );
}

function McpServerCard({
  agentId,
  server,
}: {
  agentId: string;
  server: McpServer;
}) {
  const binding = useMutation({
    mutationFn: async () => {
      if (server.bound) await api.unbindAgentMcpServer(agentId, server.id);
      else await api.bindAgentMcpServer(agentId, server.id);
    },
    onSuccess: () => invalidate(agentId),
  });
  const refresh = useMutation({
    mutationFn: () => api.refreshMcpServer(server.id),
    onSuccess: () => invalidate(agentId),
  });
  const remove = useMutation({
    mutationFn: () => api.deleteMcpServer(server.id),
    onSuccess: () => invalidate(agentId),
  });

  return (
    <article className="capability-card mcp-card">
      <div className="card-title-row">
        <div>
          <div className="server-heading">
            <h3>{server.name}</h3>
            <span className={`status-badge ${server.status}`}>
              {server.status}
            </span>
          </div>
          <p className="endpoint-copy">{server.endpoint}</p>
        </div>
        <label className="switch-control">
          <input
            type="checkbox"
            checked={server.bound && server.enabled}
            disabled={binding.isPending}
            onChange={() => binding.mutate()}
          />
          <span>{server.bound ? "Enabled for agent" : "In library"}</span>
        </label>
      </div>

      {server.lastError && (
        <div className="inline-error">{server.lastError}</div>
      )}
      {server.tools.length > 0 && (
        <div className="tool-list">
          {server.tools.map((tool) => (
            <McpToolRow
              key={tool.id}
              agentId={agentId}
              serverId={server.id}
              tool={tool}
            />
          ))}
        </div>
      )}
      <div className="card-actions">
        <span className="card-source">
          {server.protocolVersion
            ? `MCP ${server.protocolVersion}`
            : "Not inspected"}{" "}
          · {server.tools.length} tools
        </span>
        <button
          type="button"
          className="secondary-button"
          disabled={refresh.isPending}
          onClick={() => refresh.mutate()}
        >
          <RefreshCw size={15} /> Discover
        </button>
        <button
          type="button"
          className="danger-button"
          onClick={() => {
            if (
              window.confirm(
                `Remove ${server.name} from your plugin library? This disables it for every Agent and removes its discovered tools.`,
              )
            ) {
              remove.mutate();
            }
          }}
        >
          <Trash2 size={15} /> Remove from library
        </button>
      </div>
      {(binding.isError || refresh.isError || remove.isError) && (
        <span className="inline-error">
          {binding.error?.message ??
            refresh.error?.message ??
            remove.error?.message}
        </span>
      )}
    </article>
  );
}

function McpToolRow({
  agentId,
  serverId,
  tool,
}: {
  agentId: string;
  serverId: string;
  tool: McpTool;
}) {
  const update = useMutation({
    mutationFn: (risk: McpTool["riskLevel"]) =>
      api.updateMcpToolRisk(serverId, tool.id, risk),
    onSuccess: () => invalidate(agentId),
  });
  const binding = useMutation({
    mutationFn: async () => {
      if (tool.enabled) await api.unbindAgentMcpTool(agentId, tool.id);
      else await api.bindAgentMcpTool(agentId, tool.id);
    },
    onSuccess: () => invalidate(agentId),
  });
  const executable = tool.riskLevel === "read-only";

  return (
    <div className="tool-row">
      <div>
        <strong>{tool.name}</strong>
        <span>{tool.description || "No description provided"}</span>
      </div>
      <select
        aria-label={`${tool.name} risk`}
        value={tool.riskLevel}
        onChange={(event) =>
          update.mutate(event.target.value as McpTool["riskLevel"])
        }
      >
        <option value="read-only">Read only</option>
        <option value="external-write">External write</option>
        <option value="destructive">Destructive</option>
      </select>
      <label
        className="switch-control compact-switch"
        title={
          executable ? "Expose to agent" : "Approval flow is not implemented"
        }
      >
        <input
          type="checkbox"
          checked={tool.enabled}
          disabled={!executable || update.isPending || binding.isPending}
          onChange={() => binding.mutate()}
        />
        <span>{tool.enabled ? "On" : "Off"}</span>
      </label>
      {(update.isError || binding.isError) && (
        <span className="inline-error">
          {update.error?.message ?? binding.error?.message}
        </span>
      )}
    </div>
  );
}

function invalidate(agentId: string) {
  return queryClient.invalidateQueries({ queryKey: ["mcp-servers", agentId] });
}
