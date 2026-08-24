import type { components } from "./schema";

export type Agent = components["schemas"]["Agent"];
export type AccountSettings = components["schemas"]["AccountSettingsResponse"];
export type AuthResponse = components["schemas"]["AuthResponse"];
export type Bootstrap = components["schemas"]["BootstrapResponse"];
export type Conversation = components["schemas"]["Conversation"];
export type ConversationDetail = components["schemas"]["ConversationDetail"];
export type Message = components["schemas"]["Message"];
export type Memory = components["schemas"]["Memory"];
export type McpServer = components["schemas"]["McpServer"];
export type McpTool = components["schemas"]["McpTool"];
export type MentionItem = components["schemas"]["MentionItem"];
export type MentionCatalogPage = components["schemas"]["MentionCatalogPage"];
export type Run = components["schemas"]["Run"];
export type RunEvent = components["schemas"]["RunEvent"];
export type Skill = components["schemas"]["Skill"];
export type SkillFile = components["schemas"]["SkillFile"];
export type User = components["schemas"]["User"];

export class APIError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly requestId: string,
    readonly status: number,
  ) {
    super(message);
  }
}

export const api = {
  session: () => request<AuthResponse>("/api/v1/auth/session"),
  register: (displayName: string, email: string, password: string) =>
    request<AuthResponse>("/api/v1/auth/register", {
      method: "POST",
      body: JSON.stringify({ displayName, email, password }),
    }),
  login: (email: string, password: string) =>
    request<AuthResponse>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),
  logout: () => request<void>("/api/v1/auth/logout", { method: "POST" }),
  getAccountSettings: () =>
    request<AccountSettings>("/api/v1/account/settings"),
  updateAccountSettings: (
    input: components["schemas"]["UpdateAccountSettingsRequest"],
  ) =>
    request<AccountSettings>("/api/v1/account/settings", {
      method: "PATCH",
      body: JSON.stringify(input),
    }),
  changePassword: (input: components["schemas"]["ChangePasswordRequest"]) =>
    request<void>("/api/v1/account/password", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  bootstrap: () => request<Bootstrap>("/api/v1/bootstrap"),
  listMemories: async (agentId: string) => {
    const response = await request<{ memories: Memory[] }>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/memories`,
    );
    return response.memories;
  },
  createMemory: (
    agentId: string,
    input: components["schemas"]["CreateMemoryRequest"],
  ) =>
    request<Memory>(`/api/v1/agents/${encodeURIComponent(agentId)}/memories`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  updateMemory: (
    agentId: string,
    memoryId: string,
    input: components["schemas"]["UpdateMemoryRequest"],
  ) =>
    request<Memory>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/memories/${encodeURIComponent(memoryId)}`,
      { method: "PATCH", body: JSON.stringify(input) },
    ),
  forgetMemory: (agentId: string, memoryId: string) =>
    request<void>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/memories/${encodeURIComponent(memoryId)}`,
      { method: "DELETE" },
    ),
  listSkills: async (agentId: string) => {
    const response = await request<{ skills: Skill[] }>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/skills`,
    );
    return response.skills;
  },
  installSkill: (agentId: string, content: string, version?: string) =>
    request<Skill>(`/api/v1/agents/${encodeURIComponent(agentId)}/skills`, {
      method: "POST",
      body: JSON.stringify({ content, ...(version ? { version } : {}) }),
    }),
  importSkill: (agentId: string, bundle: File, version?: string) => {
    const body = new FormData();
    body.set("bundle", bundle);
    if (version) body.set("version", version);
    return request<Skill>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/skills/import`,
      { method: "POST", body },
    );
  },
  updateSkill: (agentId: string, skillId: string, enabled: boolean) =>
    request<Skill>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/skills/${encodeURIComponent(skillId)}`,
      { method: "PATCH", body: JSON.stringify({ enabled }) },
    ),
  uninstallSkill: (agentId: string, skillId: string) =>
    request<void>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/skills/${encodeURIComponent(skillId)}`,
      { method: "DELETE" },
    ),
  listMcpServers: async (agentId: string) => {
    const response = await request<{ servers: McpServer[] }>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/mcp-servers`,
    );
    return response.servers;
  },
  createMcpServer: (agentId: string, name: string, endpoint: string) =>
    request<McpServer>("/api/v1/mcp-servers", {
      method: "POST",
      body: JSON.stringify({ agentId, name, endpoint }),
    }),
  updateMcpServer: (serverId: string, name: string, endpoint: string) =>
    request<McpServer>(`/api/v1/mcp-servers/${encodeURIComponent(serverId)}`, {
      method: "PATCH",
      body: JSON.stringify({ name, endpoint }),
    }),
  deleteMcpServer: (serverId: string) =>
    request<void>(`/api/v1/mcp-servers/${encodeURIComponent(serverId)}`, {
      method: "DELETE",
    }),
  refreshMcpServer: (serverId: string) =>
    request<McpServer>(
      `/api/v1/mcp-servers/${encodeURIComponent(serverId)}/refresh`,
      { method: "POST" },
    ),
  bindAgentMcpServer: (agentId: string, serverId: string) =>
    request<McpServer>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/mcp-servers/${encodeURIComponent(serverId)}`,
      { method: "PUT" },
    ),
  unbindAgentMcpServer: (agentId: string, serverId: string) =>
    request<void>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/mcp-servers/${encodeURIComponent(serverId)}`,
      { method: "DELETE" },
    ),
  updateMcpToolRisk: (
    serverId: string,
    toolId: string,
    riskLevel: McpTool["riskLevel"],
  ) =>
    request<McpServer>(
      `/api/v1/mcp-servers/${encodeURIComponent(serverId)}/tools/${encodeURIComponent(toolId)}`,
      { method: "PATCH", body: JSON.stringify({ riskLevel }) },
    ),
  bindAgentMcpTool: (agentId: string, toolId: string) =>
    request<McpTool>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/mcp-tools/${encodeURIComponent(toolId)}`,
      { method: "PUT" },
    ),
  unbindAgentMcpTool: (agentId: string, toolId: string) =>
    request<void>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/mcp-tools/${encodeURIComponent(toolId)}`,
      { method: "DELETE" },
    ),
  listAgentMentions: (agentId: string, query = "") => {
    const parameters = new URLSearchParams({ kinds: "mcp-tool", limit: "50" });
    if (query.trim()) parameters.set("query", query.trim());
    return request<MentionCatalogPage>(
      `/api/v1/agents/${encodeURIComponent(agentId)}/mentions?${parameters.toString()}`,
    );
  },
  listConversations: async () => {
    const response = await request<{ conversations: Conversation[] }>(
      "/api/v1/conversations",
    );
    return response.conversations;
  },
  createConversation: async (title = "New conversation") => {
    const response = await request<{ conversation: Conversation }>(
      "/api/v1/conversations",
      {
        method: "POST",
        body: JSON.stringify({ title }),
      },
    );
    return response.conversation;
  },
  getConversation: (id: string) =>
    request<ConversationDetail>(
      `/api/v1/conversations/${encodeURIComponent(id)}`,
    ),
  sendMessage: (conversationId: string, content: string) =>
    request<{ message: Message; run: Run }>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/messages`,
      { method: "POST", body: JSON.stringify({ content }) },
    ),
  cancelRun: (runId: string) =>
    request<{ status: string }>(
      `/api/v1/runs/${encodeURIComponent(runId)}/cancel`,
      { method: "POST" },
    ),
};

export function runEventsURL(runId: string): string {
  return `/api/v1/runs/${encodeURIComponent(runId)}/events`;
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body !== undefined && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  const response = await fetch(path, {
    ...init,
    credentials: "include",
    headers,
  });
  if (!response.ok) {
    const fallback = {
      code: "request_failed",
      message: `Request failed with status ${response.status}`,
      requestId: "",
    };
    const body = (await response
      .json()
      .catch(() => fallback)) as typeof fallback;
    throw new APIError(
      body.code,
      body.message,
      body.requestId,
      response.status,
    );
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}
