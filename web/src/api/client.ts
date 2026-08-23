import type { components } from "./schema";

export type Agent = components["schemas"]["Agent"];
export type AuthResponse = components["schemas"]["AuthResponse"];
export type Bootstrap = components["schemas"]["BootstrapResponse"];
export type Conversation = components["schemas"]["Conversation"];
export type ConversationDetail = components["schemas"]["ConversationDetail"];
export type Message = components["schemas"]["Message"];
export type Run = components["schemas"]["Run"];
export type RunEvent = components["schemas"]["RunEvent"];
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
  bootstrap: () => request<Bootstrap>("/api/v1/bootstrap"),
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
  const response = await fetch(path, {
    ...init,
    credentials: "include",
    headers: { "Content-Type": "application/json", ...init.headers },
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
