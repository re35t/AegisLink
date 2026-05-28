export type AgentMessageType = "agent.ask" | "agent.notify" | "agent.discovery" | "agent.response";
export type CapabilityScope = "self" | "team" | "org" | "public";

export interface CapabilityClaim {
  subjectAgentId: string;
  audienceAgentId?: string;
  actions: string[];
  resources: string[];
  scope: CapabilityScope;
  expiresAt: string;
}

export interface SignedAgentMessage<TPayload = unknown> {
  messageId: string;
  type: AgentMessageType;
  senderAgentId: string;
  targetAgentId?: string;
  issuedAt: string;
  expiresAt: string;
  nonce: string;
  capability: CapabilityClaim;
  payload: TPayload;
  signature: string;
}

export interface AgentAskPayload {
  question: string;
  purpose: string;
}

export interface AgentNotifyPayload {
  event: string;
  body: Record<string, unknown>;
}
