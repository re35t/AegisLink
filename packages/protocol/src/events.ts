export type AgentEventType = "agent.registered" | "agent.message.routed" | "agent.notification.created";

export interface AgentEvent<TPayload = unknown> {
  eventId: string;
  type: AgentEventType;
  agentId: string;
  payload: TPayload;
  createdAt: string;
}
